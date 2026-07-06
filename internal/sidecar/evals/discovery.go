package evals

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const (
	discoveryKind = "discovery"

	dg1RelFactor = 0.90
	dg1AbsFloor  = 0.65
	dg2Floor     = 0.70
	dg3Evidence  = 0.75
	dg3Honesty   = 0.80
	dg4Gap       = 0.25
	dg5Factor    = 0.35

	minDiscoveryGateRunTasks   = 4
	minDiscoveryGateRunTargets = 3
	minDiscoveryGateRunTrials  = 5
)

// DiscoveryTask is a cross-repository discovery scenario from ADR-0019.2.
// Its answer is scored against a pre-registered repository target set rather
// than repo-local paths and symbols.
type DiscoveryTask struct {
	ID     string          `json:"id"`
	Kind   string          `json:"kind"`
	Turns  []string        `json:"turns"`
	Checks DiscoveryChecks `json:"checks"`
	Tags   []string        `json:"tags,omitempty"`
}

// DiscoveryChecks holds the deterministic target-set contract for one
// discovery task.
type DiscoveryChecks struct {
	TargetRepos          []DiscoveryTargetRepo    `json:"targetRepos"`
	AcceptableAlternates []DiscoveryAlternateRepo `json:"acceptableAlternates,omitempty"`
	ExcludedRepos        []DiscoveryExcludedRepo  `json:"excludedRepos,omitempty"`
	ContaminationPaths   []string                 `json:"contaminationPaths,omitempty"`
	Boundary             string                   `json:"boundary,omitempty"`
}

// DiscoveryTargetRepo is one recall-denominator repository.
type DiscoveryTargetRepo struct {
	Repo             string   `json:"repo"`
	RequiredEvidence []string `json:"requiredEvidence,omitempty"`
	Famous           bool     `json:"famous,omitempty"`
	Aliases          []string `json:"aliases,omitempty"`
}

// DiscoveryAlternateRepo is a pre-registered valid non-target answer.
type DiscoveryAlternateRepo struct {
	Repo    string   `json:"repo"`
	Policy  string   `json:"policy"`
	Reason  string   `json:"reason"`
	Aliases []string `json:"aliases,omitempty"`
}

// DiscoveryExcludedRepo is a common false positive that penalizes precision.
type DiscoveryExcludedRepo struct {
	Repo    string   `json:"repo"`
	Reason  string   `json:"reason"`
	Aliases []string `json:"aliases,omitempty"`
}

// DiscoveryRewardBreakdown is the ADR-0019.2 D3 score for one discovery
// episode. These metrics are separate from repo-scoped RewardBreakdown.
type DiscoveryRewardBreakdown struct {
	VerifiedRecall    float64 `json:"verifiedRecall"`
	NamedRecall       float64 `json:"namedRecall"`
	VerifiedPrecision float64 `json:"verifiedPrecision"`
	NamedPrecision    float64 `json:"namedPrecision"`
	Evidence          float64 `json:"evidence"`
	InferenceHonesty  float64 `json:"inferenceHonesty"`
	Compression       float64 `json:"compression"`
	Safety            float64 `json:"safety"`
	FamiliarityGap    float64 `json:"familiarityGap"`
}

// DiscoveryProfileAggregate holds per-profile means for discovery episodes.
type DiscoveryProfileAggregate struct {
	Profile  Profile `json:"profile"`
	Episodes int     `json:"episodes"`

	MeanVerifiedRecall    float64 `json:"meanVerifiedRecall"`
	MeanNamedRecall       float64 `json:"meanNamedRecall"`
	MeanVerifiedPrecision float64 `json:"meanVerifiedPrecision"`
	MeanNamedPrecision    float64 `json:"meanNamedPrecision"`
	MeanEvidence          float64 `json:"meanEvidence"`
	MeanInferenceHonesty  float64 `json:"meanInferenceHonesty"`
	MeanCompression       float64 `json:"meanCompression"`
	MeanSafety            float64 `json:"meanSafety"`
	MeanFamiliarityGap    float64 `json:"meanFamiliarityGap"`
	MeanMainAgentChars    float64 `json:"meanMainAgentChars"`
}

// DiscoveryVerdict is the run-level D-G* outcome for discovery-class evals.
type DiscoveryVerdict struct {
	Aggregates         map[Profile]*DiscoveryProfileAggregate `json:"aggregates"`
	Gates              []GateResult                           `json:"gates"`
	Labels             []string                               `json:"labels,omitempty"`
	Anomalies          []AnomalyCount                         `json:"anomalies,omitempty"`
	DiscoverySupported bool                                   `json:"discoverySupported"`
	Preliminary        bool                                   `json:"preliminary"`
	Valid              bool                                   `json:"valid"`
	DataSufficient     bool                                   `json:"dataSufficient"`
	Notes              []string                               `json:"notes"`
}

// LoadDiscoveryTasks reads and validates discovery task fixtures in stable
// filename order.
func LoadDiscoveryTasks(dir string) ([]DiscoveryTask, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read discovery tasks dir: %w", err)
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".json" {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	var tasks []DiscoveryTask
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		var task DiscoveryTask
		if err := json.Unmarshal(data, &task); err != nil {
			return nil, fmt.Errorf("parse %s: %w", name, err)
		}
		if err := task.Validate(); err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		tasks = append(tasks, task)
	}
	if len(tasks) == 0 {
		return nil, fmt.Errorf("no discovery task files in %s", dir)
	}
	return tasks, nil
}

// Validate reports whether the discovery task is well formed and enforces the
// ADR-0019.2 D4 per-task minimum of at least three target repositories.
func (t DiscoveryTask) Validate() error {
	if t.ID == "" {
		return fmt.Errorf("discovery task: id is required")
	}
	if t.Kind != discoveryKind {
		return fmt.Errorf("discovery task %s: kind must be %q", t.ID, discoveryKind)
	}
	if len(t.Turns) == 0 {
		return fmt.Errorf("discovery task %s: at least one turn is required", t.ID)
	}
	if len(t.Checks.TargetRepos) < minDiscoveryGateRunTargets {
		return fmt.Errorf("discovery task %s: targetRepos has %d entries, want at least %d", t.ID, len(t.Checks.TargetRepos), minDiscoveryGateRunTargets)
	}
	seen := map[string]string{}
	for i, target := range t.Checks.TargetRepos {
		repo, err := normalizeRepoSlug(target.Repo)
		if err != nil {
			return fmt.Errorf("discovery task %s: targetRepos[%d]: %w", t.ID, i, err)
		}
		if prior := seen[repo]; prior != "" {
			return fmt.Errorf("discovery task %s: %s duplicates %s", t.ID, target.Repo, prior)
		}
		seen[repo] = "targetRepos"
		if err := validateNonBlankList(t.ID, "requiredEvidence", target.RequiredEvidence); err != nil {
			return err
		}
		if err := validateNonBlankList(t.ID, "aliases", target.Aliases); err != nil {
			return err
		}
	}
	for i, alt := range t.Checks.AcceptableAlternates {
		repo, err := normalizeRepoSlug(alt.Repo)
		if err != nil {
			return fmt.Errorf("discovery task %s: acceptableAlternates[%d]: %w", t.ID, i, err)
		}
		if alt.Policy != "accepted-extra" && alt.Policy != "family-member" && !strings.HasPrefix(alt.Policy, "substitute-for:") {
			return fmt.Errorf("discovery task %s: acceptableAlternates[%d] policy %q is not pre-registered", t.ID, i, alt.Policy)
		}
		if strings.TrimSpace(alt.Reason) == "" {
			return fmt.Errorf("discovery task %s: acceptableAlternates[%d] reason is required", t.ID, i)
		}
		if prior := seen[repo]; prior != "" {
			return fmt.Errorf("discovery task %s: %s duplicates %s", t.ID, alt.Repo, prior)
		}
		seen[repo] = "acceptableAlternates"
		if err := validateNonBlankList(t.ID, "aliases", alt.Aliases); err != nil {
			return err
		}
	}
	for i, excluded := range t.Checks.ExcludedRepos {
		repo, err := normalizeRepoSlug(excluded.Repo)
		if err != nil {
			return fmt.Errorf("discovery task %s: excludedRepos[%d]: %w", t.ID, i, err)
		}
		if strings.TrimSpace(excluded.Reason) == "" {
			return fmt.Errorf("discovery task %s: excludedRepos[%d] reason is required", t.ID, i)
		}
		if prior := seen[repo]; prior != "" {
			return fmt.Errorf("discovery task %s: %s duplicates %s", t.ID, excluded.Repo, prior)
		}
		seen[repo] = "excludedRepos"
		if err := validateNonBlankList(t.ID, "aliases", excluded.Aliases); err != nil {
			return err
		}
	}
	return validateNonBlankList(t.ID, "contaminationPaths", t.Checks.ContaminationPaths)
}

func validateNonBlankList(taskID, field string, vals []string) error {
	for _, val := range vals {
		if strings.TrimSpace(val) == "" {
			return fmt.Errorf("discovery task %s: %s contains a blank entry", taskID, field)
		}
	}
	return nil
}

// ComputeDiscoveryRewards scores one episode against a discovery task using
// the target-set metrics from ADR-0019.2 D3.
func ComputeDiscoveryRewards(task DiscoveryTask, ep *Episode) DiscoveryRewardBreakdown {
	index := newDiscoveryIndex(task.Checks)
	state := discoveryEpisodeState(ep, index)

	targetsNamed, targetsVerified := 0, 0
	for _, target := range task.Checks.TargetRepos {
		repo, _ := normalizeRepoSlug(target.Repo)
		if state.named[repo] {
			targetsNamed++
		}
		if state.verified[repo] {
			targetsVerified++
		}
	}

	knownNamed := map[string]bool{}
	for repo := range state.named {
		if index.accepted[repo] || index.excluded[repo] {
			knownNamed[repo] = true
		}
	}
	for repo := range state.unknownNamed {
		knownNamed[repo] = true
	}

	namedAccepted, verifiedAccepted, verifiedNamed := 0, 0, 0
	for repo := range knownNamed {
		if index.accepted[repo] {
			namedAccepted++
		}
		if state.verified[repo] {
			verifiedNamed++
			if index.accepted[repo] {
				verifiedAccepted++
			}
		}
	}

	evidenceClaims, verifiedClaims := 0, 0
	for repo := range state.verified {
		verifiedClaims++
		if state.citedReadPath[repo] {
			evidenceClaims++
		}
	}

	nonVerifiedKnown, honest := 0, 0
	for repo := range state.named {
		if state.excluded[repo] {
			continue
		}
		if state.verified[repo] {
			continue
		}
		nonVerifiedKnown++
		if state.inferredOrUnverified[repo] && !state.verifiedClaim[repo] {
			honest++
		}
	}

	r := DiscoveryRewardBreakdown{
		VerifiedRecall:    ratio(targetsVerified, len(task.Checks.TargetRepos)),
		NamedRecall:       ratio(targetsNamed, len(task.Checks.TargetRepos)),
		VerifiedPrecision: ratio(verifiedAccepted, verifiedNamed),
		NamedPrecision:    ratio(namedAccepted, len(knownNamed)),
		Evidence:          mean([]float64{ratio(evidenceClaims, verifiedClaims), commandTrailPresence(ep)}),
		InferenceHonesty:  1,
		Compression:       compressionReward(ep),
		Safety:            safetyReward(ep),
	}
	if nonVerifiedKnown > 0 {
		r.InferenceHonesty = ratio(honest, nonVerifiedKnown)
	}
	r.FamiliarityGap = r.NamedRecall - r.VerifiedRecall
	return r
}

type discoveryIndex struct {
	canonical        map[string]string
	accepted         map[string]bool
	excluded         map[string]bool
	requiredEvidence map[string][]string
}

func newDiscoveryIndex(checks DiscoveryChecks) discoveryIndex {
	idx := discoveryIndex{
		canonical:        map[string]string{},
		accepted:         map[string]bool{},
		excluded:         map[string]bool{},
		requiredEvidence: map[string][]string{},
	}
	add := func(repo string, aliases []string) string {
		norm, _ := normalizeRepoSlug(repo)
		idx.canonical[norm] = norm
		for _, alias := range aliases {
			if strings.TrimSpace(alias) != "" {
				idx.canonical[strings.ToLower(strings.TrimSpace(alias))] = norm
			}
		}
		return norm
	}
	for _, target := range checks.TargetRepos {
		repo := add(target.Repo, target.Aliases)
		idx.accepted[repo] = true
		idx.requiredEvidence[repo] = normalizeEvidencePaths(target.RequiredEvidence)
	}
	for _, alt := range checks.AcceptableAlternates {
		repo := add(alt.Repo, alt.Aliases)
		idx.accepted[repo] = true
	}
	for _, excluded := range checks.ExcludedRepos {
		repo := add(excluded.Repo, excluded.Aliases)
		idx.excluded[repo] = true
	}
	return idx
}

type discoveryState struct {
	named                map[string]bool
	unknownNamed         map[string]bool
	excluded             map[string]bool
	verified             map[string]bool
	verifiedClaim        map[string]bool
	inferredOrUnverified map[string]bool
	citedReadPath        map[string]bool
}

func discoveryEpisodeState(ep *Episode, idx discoveryIndex) discoveryState {
	state := discoveryState{
		named:                map[string]bool{},
		unknownNamed:         map[string]bool{},
		excluded:             map[string]bool{},
		verified:             map[string]bool{},
		verifiedClaim:        map[string]bool{},
		inferredOrUnverified: map[string]bool{},
		citedReadPath:        map[string]bool{},
	}
	report := scoringReport(ep)
	claimText := discoveryAnswerText(ep)
	for _, repo := range reposMentioned(claimText, idx) {
		if idx.accepted[repo] || idx.excluded[repo] {
			state.named[repo] = true
			if idx.excluded[repo] {
				state.excluded[repo] = true
			}
		} else {
			state.unknownNamed[repo] = true
		}
	}
	if report == nil {
		for repo := range state.named {
			if discoveryRepoVerified(repo, claimText, ep, idx.requiredEvidence[repo]) {
				state.verified[repo] = true
				state.citedReadPath[repo] = true
			}
		}
		if hasInferenceHonestyMarker(claimText) {
			for repo := range state.named {
				state.inferredOrUnverified[repo] = true
			}
		}
		return state
	}
	for _, claim := range report.Verified {
		text := strings.ToLower(claim.Summary + "\n" + claim.Evidence)
		for _, repo := range reposMentioned(text, idx) {
			state.verifiedClaim[repo] = true
			if discoveryRepoVerified(repo, claim.Evidence, ep, idx.requiredEvidence[repo]) {
				state.verified[repo] = true
				state.citedReadPath[repo] = true
			}
		}
	}
	inferredText := strings.ToLower(strings.Join(report.Uncertainty, "\n"))
	for _, claim := range report.Inferred {
		inferredText += "\n" + claim.Summary + "\n" + claim.Evidence
	}
	for _, claim := range report.Unverified {
		inferredText += "\n" + claim.Summary + "\n" + claim.Evidence
	}
	for _, repo := range reposMentioned(inferredText, idx) {
		state.inferredOrUnverified[repo] = true
	}
	return state
}

func hasInferenceHonestyMarker(text string) bool {
	for _, marker := range []string{"inferred", "unverified", "not verified", "did not verify", "search-only", "metadata-only"} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

func discoveryAnswerText(ep *Episode) string {
	var sb strings.Builder
	if report := scoringReport(ep); report != nil {
		sb.WriteString(report.Answer)
		sb.WriteByte('\n')
		for _, claim := range report.Verified {
			sb.WriteString(claim.Summary)
			sb.WriteByte('\n')
		}
		for _, claim := range report.Inferred {
			sb.WriteString(claim.Summary)
			sb.WriteByte('\n')
		}
		for _, claim := range report.Unverified {
			sb.WriteString(claim.Summary)
			sb.WriteByte('\n')
		}
		for _, file := range report.RelevantFiles {
			sb.WriteString(file.Path)
			sb.WriteByte('\n')
			sb.WriteString(file.Reason)
			sb.WriteByte('\n')
		}
		for _, uncertainty := range report.Uncertainty {
			sb.WriteString(uncertainty)
			sb.WriteByte('\n')
		}
	}
	for _, turn := range ep.Turns {
		sb.WriteString(turn.Text)
		sb.WriteByte('\n')
	}
	return strings.ToLower(sb.String())
}

var ownerRepoRE = regexp.MustCompile(`\b[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+\b`)

func reposMentioned(text string, idx discoveryIndex) []string {
	seen := map[string]bool{}
	add := func(raw string) {
		raw = strings.Trim(strings.ToLower(raw), "`.,;:()[]{}")
		if canonical := idx.canonical[raw]; canonical != "" {
			seen[canonical] = true
			return
		}
		if _, err := normalizeRepoSlug(raw); err == nil {
			seen[raw] = true
		}
	}
	for raw := range idx.canonical {
		if !strings.Contains(raw, "/") && wholePhraseContains(text, raw) {
			add(raw)
		}
	}
	for _, raw := range ownerRepoRE.FindAllString(text, -1) {
		add(raw)
	}
	var out []string
	for repo := range seen {
		out = append(out, repo)
	}
	sort.Strings(out)
	return out
}

func wholePhraseContains(text, phrase string) bool {
	re, err := regexp.Compile(`(?i)\b` + regexp.QuoteMeta(phrase) + `\b`)
	if err != nil {
		return strings.Contains(text, strings.ToLower(phrase))
	}
	return re.MatchString(text)
}

func discoveryRepoVerified(repo, evidence string, ep *Episode, required []string) bool {
	cited := citedRepoPaths(repo, evidence)
	if len(cited) == 0 {
		return false
	}
	reads := readPathsForRepo(ep, repo)
	for _, citedPath := range cited {
		if !matchesRequiredEvidence(citedPath, required) {
			continue
		}
		for _, readPath := range reads {
			if sameOrParentPath(readPath, citedPath) || sameOrParentPath(citedPath, readPath) {
				return true
			}
		}
	}
	return false
}

func citedRepoPaths(repo, evidence string) []string {
	lower := strings.ToLower(evidence)
	prefix := strings.ToLower(repo) + ":"
	var paths []string
	for _, field := range strings.FieldsFunc(lower, func(r rune) bool {
		return r == ' ' || r == '\n' || r == '\t' || r == ',' || r == ';'
	}) {
		field = strings.Trim(field, "`'\"()[]{}")
		if strings.HasPrefix(field, prefix) {
			path := strings.Trim(strings.TrimPrefix(field, prefix), "`'\"()[]{}")
			if path != "" {
				paths = append(paths, path)
			}
		}
	}
	return paths
}

func readPathsForRepo(ep *Episode, repo string) []string {
	seen := map[string]bool{}
	add := func(path string) {
		path = strings.Trim(strings.ToLower(path), "`'\"")
		if path != "" {
			seen[path] = true
		}
	}
	for _, cmd := range allDiscoveryCommands(ep) {
		if readRepo, path, ok := parseDiscoveryRead(cmd); ok && readRepo == repo {
			add(path)
		}
	}
	var out []string
	for path := range seen {
		out = append(out, path)
	}
	sort.Strings(out)
	return out
}

func allDiscoveryCommands(ep *Episode) []string {
	cmds := allToolCalls(ep)
	if report := scoringReport(ep); report != nil {
		cmds = append(cmds, report.CommandsRun...)
	}
	for _, action := range ep.Actions {
		if strings.TrimSpace(action.Input) != "" {
			cmds = append(cmds, action.Input)
		}
	}
	return cmds
}

func parseDiscoveryRead(call string) (repo, path string, ok bool) {
	cmd := normalizeToolCommand(call)
	fields := strings.Fields(cmd)
	for i := 0; i+2 < len(fields); i++ {
		if rawTokenInvokesGhx(fields[i]) && fields[i+1] == "read" && i+3 < len(fields) {
			repo, err := normalizeRepoSlug(strings.Trim(fields[i+2], `"'`))
			if err != nil {
				return "", "", false
			}
			return repo, strings.ToLower(strings.Trim(fields[i+3], `"'`)), true
		}
		if rawTokenInvokesGhx(fields[i]) && fields[i+1] == "explore" {
			repo, err := normalizeRepoSlug(strings.Trim(fields[i+2], `"'`))
			if err != nil {
				return "", "", false
			}
			return repo, "README.md", true
		}
		if fields[i] == "gh" && fields[i+1] == "api" {
			if repo, path, ok := parseGHContentsPath(fields[i+2]); ok {
				return repo, path, true
			}
		}
	}
	if repo, path, ok := parseRawGitHubURL(cmd); ok {
		return repo, path, true
	}
	return "", "", false
}

func parseGHContentsPath(raw string) (repo, path string, ok bool) {
	raw = strings.Trim(raw, `"'`)
	parts := strings.Split(raw, "/")
	for i := 0; i+4 < len(parts); i++ {
		if parts[i] == "repos" && parts[i+3] == "contents" {
			repo, err := normalizeRepoSlug(parts[i+1] + "/" + parts[i+2])
			if err != nil {
				return "", "", false
			}
			return repo, strings.ToLower(strings.Join(parts[i+4:], "/")), true
		}
	}
	return "", "", false
}

func parseRawGitHubURL(raw string) (repo, path string, ok bool) {
	idx := strings.Index(raw, "raw.githubusercontent.com/")
	if idx < 0 {
		return "", "", false
	}
	tail := raw[idx+len("raw.githubusercontent.com/"):]
	tail = strings.Trim(tail, `"' <>`)
	parts := strings.Split(tail, "/")
	if len(parts) < 4 {
		return "", "", false
	}
	repo, err := normalizeRepoSlug(parts[0] + "/" + parts[1])
	if err != nil {
		return "", "", false
	}
	return repo, strings.ToLower(strings.Join(parts[3:], "/")), true
}

func matchesRequiredEvidence(path string, required []string) bool {
	if len(required) == 0 {
		return true
	}
	for _, req := range required {
		if sameOrParentPath(path, req) || sameOrParentPath(req, path) {
			return true
		}
	}
	return false
}

func sameOrParentPath(a, b string) bool {
	a = strings.Trim(strings.ToLower(a), "/")
	b = strings.Trim(strings.ToLower(b), "/")
	if a == b {
		return true
	}
	return strings.HasPrefix(a, b+"/") || strings.HasPrefix(b, a+"/")
}

func normalizeEvidencePaths(paths []string) []string {
	out := make([]string, 0, len(paths))
	for _, path := range paths {
		out = append(out, strings.Trim(strings.ToLower(path), "/"))
	}
	return out
}

func normalizeRepoSlug(repo string) (string, error) {
	repo = strings.Trim(strings.ToLower(repo), "`'\" ")
	parts := strings.Split(repo, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", fmt.Errorf("repo %q must be owner/repo", repo)
	}
	return repo, nil
}

func ratio(num, denom int) float64 {
	if denom == 0 {
		return 0
	}
	return float64(num) / float64(denom)
}

func commandTrailPresence(ep *Episode) float64 {
	if len(allDiscoveryCommands(ep)) > 0 {
		return 1
	}
	return 0
}

// AggregateDiscovery computes per-profile means over discovery rewards.
func AggregateDiscovery(episodes []*Episode) map[Profile]*DiscoveryProfileAggregate {
	agg := map[Profile]*DiscoveryProfileAggregate{}
	for _, p := range AllProfiles() {
		agg[p] = &DiscoveryProfileAggregate{Profile: p}
	}
	type sums struct {
		verifiedRecall, namedRecall, verifiedPrecision, namedPrecision float64
		evidence, honesty, compression, safety, gap, chars             float64
	}
	bag := map[Profile]*sums{}
	for _, p := range AllProfiles() {
		bag[p] = &sums{}
	}
	for _, ep := range episodes {
		if ep == nil {
			continue
		}
		a := agg[ep.Profile]
		if a == nil {
			continue
		}
		if ep.DiscoveryRewards == nil {
			continue
		}
		r := *ep.DiscoveryRewards
		a.Episodes++
		s := bag[ep.Profile]
		s.verifiedRecall += r.VerifiedRecall
		s.namedRecall += r.NamedRecall
		s.verifiedPrecision += r.VerifiedPrecision
		s.namedPrecision += r.NamedPrecision
		s.evidence += r.Evidence
		s.honesty += r.InferenceHonesty
		s.compression += r.Compression
		s.safety += r.Safety
		s.gap += r.FamiliarityGap
		s.chars += float64(ep.Context.MainAgentChars)
	}
	for p, a := range agg {
		if a.Episodes == 0 {
			continue
		}
		n := float64(a.Episodes)
		s := bag[p]
		a.MeanVerifiedRecall = s.verifiedRecall / n
		a.MeanNamedRecall = s.namedRecall / n
		a.MeanVerifiedPrecision = s.verifiedPrecision / n
		a.MeanNamedPrecision = s.namedPrecision / n
		a.MeanEvidence = s.evidence / n
		a.MeanInferenceHonesty = s.honesty / n
		a.MeanCompression = s.compression / n
		a.MeanSafety = s.safety / n
		a.MeanFamiliarityGap = s.gap / n
		a.MeanMainAgentChars = s.chars / n
	}
	return agg
}

// EvaluateDiscoveryGates applies the separate ADR-0019.2 D4 discovery gates.
// It never reads or writes repo-scoped G1-G5 rewards.
func EvaluateDiscoveryGates(episodes []*Episode) DiscoveryVerdict {
	filtered, validityNotes, valid := validateDiscoveryEpisodesForVerdict(episodes)
	agg := AggregateDiscovery(filtered)
	sc := agg[ProfileSidecar]
	gx := agg[ProfileGhx]

	v := DiscoveryVerdict{Aggregates: agg, Anomalies: CountAnomalies(episodes), Valid: valid}
	v.Notes = append(v.Notes, validityNotes...)
	if sc.Episodes == 0 || gx.Episodes == 0 {
		v.Notes = append(v.Notes, fmt.Sprintf(
			"INSUFFICIENT DATA: sidecar discovery episodes=%d, ghx discovery episodes=%d — D-G gates need both profiles present",
			sc.Episodes, gx.Episodes))
	}
	var sufficiencyNotes []string
	v.DataSufficient, sufficiencyNotes = discoverySampleSufficiency(filtered)
	v.Preliminary = !v.DataSufficient
	v.Notes = append(v.Notes, sufficiencyNotes...)

	g1 := GateResult{
		ID:   "D-G1",
		Desc: fmt.Sprintf("verified recall: sidecar ≥ %.2f absolute and ≥ %.2f × ghx", dg1AbsFloor, dg1RelFactor),
		Pass: sc.MeanVerifiedRecall >= dg1AbsFloor && sc.MeanVerifiedRecall >= dg1RelFactor*gx.MeanVerifiedRecall,
		Detail: fmt.Sprintf("sidecar %.3f vs ghx %.3f (rel floor %.3f, abs floor %.2f)",
			sc.MeanVerifiedRecall, gx.MeanVerifiedRecall, dg1RelFactor*gx.MeanVerifiedRecall, dg1AbsFloor),
	}
	g2 := GateResult{
		ID:     "D-G2",
		Desc:   fmt.Sprintf("verified precision: sidecar ≥ %.2f", dg2Floor),
		Pass:   sc.MeanVerifiedPrecision >= dg2Floor,
		Detail: fmt.Sprintf("sidecar %.3f", sc.MeanVerifiedPrecision),
	}
	g3 := GateResult{
		ID:     "D-G3",
		Desc:   fmt.Sprintf("evidence honesty: sidecar evidence ≥ %.2f and inferenceHonesty ≥ %.2f", dg3Evidence, dg3Honesty),
		Pass:   sc.MeanEvidence >= dg3Evidence && sc.MeanInferenceHonesty >= dg3Honesty,
		Detail: fmt.Sprintf("evidence %.3f; inferenceHonesty %.3f", sc.MeanEvidence, sc.MeanInferenceHonesty),
	}
	g4 := GateResult{
		ID:     "D-G4",
		Desc:   fmt.Sprintf("familiarity gap: sidecar namedRecall - verifiedRecall ≤ %.2f", dg4Gap),
		Pass:   sc.MeanFamiliarityGap <= dg4Gap,
		Detail: fmt.Sprintf("sidecar gap %.3f", sc.MeanFamiliarityGap),
	}
	g5 := GateResult{
		ID:   "D-G5",
		Desc: fmt.Sprintf("compression: sidecar main-agent chars ≤ %.2f × ghx", dg5Factor),
		Pass: gx.MeanMainAgentChars > 0 && sc.MeanMainAgentChars <= dg5Factor*gx.MeanMainAgentChars,
		Detail: fmt.Sprintf("sidecar %.0f vs ghx %.0f chars (threshold %.0f)",
			sc.MeanMainAgentChars, gx.MeanMainAgentChars, dg5Factor*gx.MeanMainAgentChars),
	}
	g6 := GateResult{
		ID:     "D-G6",
		Desc:   "safety: 1.0 on every sidecar discovery episode",
		Pass:   sc.Episodes > 0 && sc.MeanSafety == 1.0,
		Detail: fmt.Sprintf("mean safety %.3f over %d episodes", sc.MeanSafety, sc.Episodes),
	}
	v.Gates = []GateResult{g1, g2, g3, g4, g5, g6}
	if !g6.Pass && sc.Episodes > 0 {
		v.Valid = false
	}
	v.DiscoverySupported = v.Valid && v.DataSufficient && g1.Pass && g2.Pass && g3.Pass && g4.Pass && g5.Pass && g6.Pass
	if !v.DataSufficient && v.Valid && discoverySupportedGatesPass(v.Gates) {
		v.Notes = append(v.Notes,
			"PRELIMINARY: all discovery gates pass but the sample is below the pre-registered discovery gate-run minimums — discoverySupported stays false until a sufficient run measures it")
	}
	if !g6.Pass && sc.Episodes > 0 {
		v.Notes = append(v.Notes,
			"D-G6 failed: safety violation on a sidecar discovery episode invalidates the run as a contract bug")
	}
	return v
}

// SaveDiscoveryVerdict writes discovery-verdict.json and
// discovery-verdict.md into runDir. Discovery verdict artifacts are separate
// from repo-scoped verdict.json/verdict.md so D-G metrics cannot be confused
// with G1-G5.
func SaveDiscoveryVerdict(runDir string, v DiscoveryVerdict) (string, error) {
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		return "", err
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(runDir, "discovery-verdict.json"), data, 0o644); err != nil {
		return "", err
	}
	mdPath := filepath.Join(runDir, "discovery-verdict.md")
	return mdPath, os.WriteFile(mdPath, []byte(FormatDiscoveryVerdict(v)), 0o644)
}

func validateDiscoveryEpisodesForVerdict(episodes []*Episode) ([]*Episode, []string, bool) {
	valid := true
	var notes []string
	var filtered []*Episode
	for _, ep := range episodes {
		if ep == nil {
			continue
		}
		exclude := ep.Invalid
		if ep.Profile == ProfilePlain && invokesGhx(ep) {
			exclude = true
			ep.Invalid = true
			reason := "COMPLIANCE: plain profile invoked ghx"
			if !containsString(ep.ExclusionReasons, reason) {
				ep.ExclusionReasons = append(ep.ExclusionReasons, reason)
			}
			notes = append(notes, fmt.Sprintf("%s in discovery episode %s — excluded from discovery gate aggregates", reason, episodeLabel(ep)))
		}
		if exclude {
			valid = false
			for _, reason := range ep.ExclusionReasons {
				if !strings.Contains(reason, "plain profile invoked ghx") {
					notes = append(notes, fmt.Sprintf("COMPLIANCE: discovery episode %s excluded: %s", episodeLabel(ep), reason))
				}
			}
			continue
		}
		if detail, contaminated := discoveryContamination(ep); contaminated {
			notes = append(notes, fmt.Sprintf(
				"CONTAMINATION: discovery episode %s excluded from D-G aggregates — %s",
				episodeLabel(ep), detail))
			continue
		}
		filtered = append(filtered, ep)
	}
	return filtered, notes, valid
}

func discoveryContamination(ep *Episode) (string, bool) {
	if ep.DiscoveryChecks == nil || len(ep.DiscoveryChecks.ContaminationPaths) == 0 {
		return "", false
	}
	for _, cmd := range allDiscoveryCommands(ep) {
		_, path, ok := parseDiscoveryRead(cmd)
		if !ok {
			continue
		}
		for _, prefix := range ep.DiscoveryChecks.ContaminationPaths {
			if strings.HasPrefix(strings.ToLower(path), strings.ToLower(prefix)) {
				return fmt.Sprintf("read %s matches discovery contamination path %s", path, prefix), true
			}
		}
	}
	return "", false
}

func discoverySampleSufficiency(episodes []*Episode) (bool, []string) {
	tasks := map[string]bool{}
	trials := map[string]int{}
	targetShortfalls := map[string]int{}
	for _, ep := range episodes {
		tasks[ep.TaskID] = true
		trials[ep.TaskID+"/"+string(ep.Profile)]++
		if ep.DiscoveryChecks == nil {
			continue
		}
		if n := len(ep.DiscoveryChecks.TargetRepos); n > 0 && n < minDiscoveryGateRunTargets {
			targetShortfalls[ep.TaskID] = n
		}
	}
	minTrials := 0
	if len(tasks) > 0 {
		minTrials = int(^uint(0) >> 1)
		for task := range tasks {
			for _, p := range AllProfiles() {
				if n := trials[task+"/"+string(p)]; n < minTrials {
					minTrials = n
				}
			}
		}
	}
	var notes []string
	if len(tasks) < minDiscoveryGateRunTasks {
		notes = append(notes, fmt.Sprintf("PRELIMINARY: %d distinct discovery tasks < gate-run minimum %d", len(tasks), minDiscoveryGateRunTasks))
	}
	if len(targetShortfalls) > 0 {
		notes = append(notes, fmt.Sprintf("PRELIMINARY: %d discovery task(s) have fewer than %d targets", len(targetShortfalls), minDiscoveryGateRunTargets))
	}
	if minTrials < minDiscoveryGateRunTrials {
		notes = append(notes, fmt.Sprintf("PRELIMINARY: smallest discovery task × profile cell has %d trials < gate-run minimum %d", minTrials, minDiscoveryGateRunTrials))
	}
	return len(notes) == 0, notes
}

func discoverySupportedGatesPass(gates []GateResult) bool {
	for _, gate := range gates {
		if !gate.Pass {
			return false
		}
	}
	return len(gates) > 0
}

// FormatDiscoveryVerdict renders a standalone discovery verdict section so
// D-G metrics are never averaged into repo-scoped G1-G5 output.
func FormatDiscoveryVerdict(v DiscoveryVerdict) string {
	var sb strings.Builder
	sb.WriteString("# Discovery Eval Verdict (ADR-0019.2)\n\n")
	sb.WriteString("## Discovery Profiles\n\n")
	sb.WriteString("| profile | episodes | verified recall | named recall | verified precision | named precision | evidence | inference honesty | familiarity gap | compression | safety | main-agent chars |\n")
	sb.WriteString("|---------|----------|-----------------|--------------|--------------------|-----------------|----------|-------------------|-----------------|-------------|--------|------------------|\n")
	for _, p := range AllProfiles() {
		a := v.Aggregates[p]
		if a == nil {
			continue
		}
		fmt.Fprintf(&sb, "| %s | %d | %.3f | %.3f | %.3f | %.3f | %.3f | %.3f | %.3f | %.3f | %.3f | %.0f |\n",
			a.Profile, a.Episodes, a.MeanVerifiedRecall, a.MeanNamedRecall,
			a.MeanVerifiedPrecision, a.MeanNamedPrecision, a.MeanEvidence,
			a.MeanInferenceHonesty, a.MeanFamiliarityGap, a.MeanCompression,
			a.MeanSafety, a.MeanMainAgentChars)
	}
	sb.WriteString("\n## Discovery Gates\n\n")
	sb.WriteString("| gate | check | result | detail |\n|------|-------|--------|--------|\n")
	for _, gate := range v.Gates {
		status := "FAIL"
		if gate.Pass {
			status = "PASS"
		}
		fmt.Fprintf(&sb, "| %s | %s | %s | %s |\n", gate.ID, gate.Desc, status, gate.Detail)
	}
	sb.WriteString("\n## Discovery Verdict\n\n")
	prefix := ""
	if v.Preliminary {
		prefix += "**PRELIMINARY (below pre-registered discovery gate-run sample)** "
	}
	if !v.Valid {
		prefix += "**INVALID (compliance checks failed)** "
	}
	switch {
	case v.DiscoverySupported:
		sb.WriteString(prefix + "**DISCOVERY SUPPORTED** — D-G1 through D-G6 pass.\n")
	case !gateByID(v.Gates, "D-G6").Pass:
		sb.WriteString(prefix + "**DISCOVERY INVALID** — D-G6 safety failed.\n")
	case !gateByID(v.Gates, "D-G1").Pass || !gateByID(v.Gates, "D-G5").Pass:
		sb.WriteString(prefix + "**DISCOVERY NOT SUPPORTED** — D-G1 or D-G5 failed.\n")
	case !gateByID(v.Gates, "D-G2").Pass || !gateByID(v.Gates, "D-G3").Pass || !gateByID(v.Gates, "D-G4").Pass:
		sb.WriteString(prefix + "**DISCOVERY EVIDENCE WEAK** — D-G2, D-G3, or D-G4 failed while D-G1/D-G5 passed.\n")
	default:
		sb.WriteString(prefix + "**DISCOVERY NOT SUPPORTED** — discovery gates did not pass.\n")
	}
	if len(v.Notes) > 0 {
		sb.WriteString("\n## Notes\n\n")
		for _, note := range v.Notes {
			fmt.Fprintf(&sb, "- %s\n", note)
		}
	}
	return sb.String()
}

func gateByID(gates []GateResult, id string) GateResult {
	for _, gate := range gates {
		if gate.ID == id {
			return gate
		}
	}
	return GateResult{}
}
