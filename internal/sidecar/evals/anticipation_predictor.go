package evals

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/gkoreli/ghx/v2/internal/sidecar"
)

// AnticipationMatchingRule is the exact D1 predictor rule used by the July
// 2026 offline miner. Keep this text in sync with README output so the
// committed verdict is recomputable from code and artifacts.
const AnticipationMatchingRule = "Paths are normalized by trimming quotes/backticks, line suffixes, shell punctuation, leading ./, repo prefixes, and owner/repo: prefixes, then converting to slash paths. A nextReads item is one prediction entry; the miner extracts path-like candidates from that entry. Candidates with a file extension, or a single path token such as tree.go, are exact-file candidates and match only the same normalized path. Candidates without a file extension but containing a slash, single-token directory names such as docs, or entries whose wording names a directory/area, are area candidates and match an actual read when the actual normalized path is equal to the area or has area/ as a prefix. If one nextReads entry contains several candidates joined by words such as or/and, the entry is a hit when any candidate matches; precision denominator remains the original nextReads entry count. Recall denominator is the count of distinct files actually read in turn N+1."

// AnticipationPairResult is one consecutive turn-pair measurement.
type AnticipationPairResult struct {
	Corpus             string             `json:"corpus"`
	EpisodeID          string             `json:"episodeId,omitempty"`
	SessionID          string             `json:"sessionId,omitempty"`
	TaskID             string             `json:"taskId,omitempty"`
	Repo               string             `json:"repo,omitempty"`
	Profile            string             `json:"profile,omitempty"`
	FromTurn           int                `json:"fromTurn"`
	ToTurn             int                `json:"toTurn"`
	NextReads          []string           `json:"nextReads"`
	Predictions        []PredictionRecord `json:"predictions"`
	ActualReads        []string           `json:"actualReads"`
	RecallHits         []string           `json:"recallHits"`
	PrecisionHits      []string           `json:"precisionHits"`
	Recall             float64            `json:"recall"`
	Precision          float64            `json:"precision"`
	HasPrediction      bool               `json:"hasPrediction"`
	HasActualRead      bool               `json:"hasActualRead"`
	SourceArtifactPath string             `json:"sourceArtifactPath,omitempty"`
}

// PredictionRecord is a normalized representation of one nextReads entry.
type PredictionRecord struct {
	Entry      string   `json:"entry"`
	Candidates []string `json:"candidates"`
	Kind       string   `json:"kind"`
}

// AnticipationCorpusSummary aggregates pair measurements for one corpus.
type AnticipationCorpusSummary struct {
	Corpus                  string                          `json:"corpus"`
	EpisodesOrSessions      int                             `json:"episodesOrSessions"`
	MultiTurnUnits          int                             `json:"multiTurnUnits"`
	Pairs                   int                             `json:"pairs"`
	SeededPairs             int                             `json:"seededPairs"`
	PairsWithActualReads    int                             `json:"pairsWithActualReads"`
	TotalPredictions        int                             `json:"totalPredictions"`
	TotalActualReads        int                             `json:"totalActualReads"`
	TotalRecallHits         int                             `json:"totalRecallHits"`
	TotalPrecisionHits      int                             `json:"totalPrecisionHits"`
	MicroRecall             float64                         `json:"microRecall"`
	MicroPrecision          float64                         `json:"microPrecision"`
	SeededMicroRecall       float64                         `json:"seededMicroRecall"`
	SeededMicroPrecision    float64                         `json:"seededMicroPrecision"`
	MacroRecall             float64                         `json:"macroRecall"`
	MacroPrecision          float64                         `json:"macroPrecision"`
	SeededMacroRecall       float64                         `json:"seededMacroRecall"`
	SeededMacroPrecision    float64                         `json:"seededMacroPrecision"`
	GateThreshold           float64                         `json:"gateThreshold"`
	GatePassed              bool                            `json:"gatePassed"`
	PerTask                 map[string]AnticipationTaskRoll `json:"perTask,omitempty"`
	HitConcentrationTopTask string                          `json:"hitConcentrationTopTask,omitempty"`
	HitConcentrationShare   float64                         `json:"hitConcentrationShare,omitempty"`
}

// AnticipationTaskRoll aggregates measurements by task or local session.
type AnticipationTaskRoll struct {
	Pairs             int     `json:"pairs"`
	SeededPairs       int     `json:"seededPairs"`
	Predictions       int     `json:"predictions"`
	ActualReads       int     `json:"actualReads"`
	RecallHits        int     `json:"recallHits"`
	PrecisionHits     int     `json:"precisionHits"`
	MicroRecall       float64 `json:"microRecall"`
	MicroPrecision    float64 `json:"microPrecision"`
	SeededMicroRecall float64 `json:"seededMicroRecall"`
	SeededMicroPrec   float64 `json:"seededMicroPrecision"`
}

// MineAnticipationEvalCorpus mines committed eval episode JSONs.
func MineAnticipationEvalCorpus(runDir string) ([]AnticipationPairResult, AnticipationCorpusSummary, error) {
	paths, err := filepath.Glob(filepath.Join(runDir, "*.json"))
	if err != nil {
		return nil, AnticipationCorpusSummary{}, err
	}
	sort.Strings(paths)
	var pairs []AnticipationPairResult
	episodes := 0
	multiTurn := 0
	for _, path := range paths {
		if filepath.Base(path) == "manifest.json" {
			continue
		}
		ep, err := LoadEpisode(path)
		if err != nil {
			return nil, AnticipationCorpusSummary{}, err
		}
		episodes++
		if len(ep.Turns) >= 2 {
			multiTurn++
		}
		for i := 0; i+1 < len(ep.Turns); i++ {
			pairs = append(pairs, measureAnticipationPair(pairInput{
				Corpus:             "committed-evals",
				EpisodeID:          ep.ID,
				TaskID:             ep.TaskID,
				Repo:               ep.Repo,
				Profile:            string(ep.Profile),
				FromTurn:           ep.Turns[i].Turn,
				ToTurn:             ep.Turns[i+1].Turn,
				NextReads:          reportNextReads(ep.Turns[i].Report),
				ActualReads:        actualReadsFromEvalTurn(ep.Repo, ep.Turns[i+1]),
				SourceArtifactPath: path,
			}))
		}
	}
	return pairs, summarizeAnticipationPairs("committed-evals", episodes, multiTurn, pairs), nil
}

// MineAnticipationLocalSessions mines local ghx dogfood sessions.
func MineAnticipationLocalSessions(sessionsDir string) ([]AnticipationPairResult, AnticipationCorpusSummary, error) {
	metaPaths, err := filepath.Glob(filepath.Join(sessionsDir, "*", "meta.json"))
	if err != nil {
		return nil, AnticipationCorpusSummary{}, err
	}
	sort.Strings(metaPaths)
	var pairs []AnticipationPairResult
	units := 0
	multiTurn := 0
	for _, metaPath := range metaPaths {
		meta, err := loadSessionMeta(metaPath)
		if err != nil {
			return nil, AnticipationCorpusSummary{}, err
		}
		units++
		if meta.TurnCount < 2 {
			continue
		}
		multiTurn++
		sessionDir := filepath.Dir(metaPath)
		reports, err := loadSessionReports(filepath.Join(sessionDir, "reports"))
		if err != nil {
			return nil, AnticipationCorpusSummary{}, err
		}
		tracesByTurn, err := loadSessionTraceReads(meta.Repo, filepath.Join(sessionDir, "traces.jsonl"))
		if err != nil {
			return nil, AnticipationCorpusSummary{}, err
		}
		for turn := 1; turn < meta.TurnCount; turn++ {
			from := reports[turn]
			pairs = append(pairs, measureAnticipationPair(pairInput{
				Corpus:             "local-dogfood",
				SessionID:          filepath.Base(sessionDir),
				TaskID:             filepath.Base(sessionDir),
				Repo:               meta.Repo,
				Profile:            "ghx-sidecar",
				FromTurn:           turn,
				ToTurn:             turn + 1,
				NextReads:          reportNextReads(from),
				ActualReads:        setToSortedSlice(tracesByTurn[turn+1]),
				SourceArtifactPath: sessionDir,
			}))
		}
	}
	return pairs, summarizeAnticipationPairs("local-dogfood", units, multiTurn, pairs), nil
}

// WriteAnticipationPredictorArtifacts writes the D1 JSONL and README report.
func WriteAnticipationPredictorArtifacts(outDir string, evalPairs, localPairs []AnticipationPairResult, evalSummary, localSummary AnticipationCorpusSummary) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	allPairs := append([]AnticipationPairResult{}, evalPairs...)
	allPairs = append(allPairs, localPairs...)
	if err := writePairJSONL(filepath.Join(outDir, "pairs.jsonl"), allPairs); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outDir, "README.md"), []byte(renderAnticipationREADME(evalSummary, localSummary)), 0o644)
}

type pairInput struct {
	Corpus             string
	EpisodeID          string
	SessionID          string
	TaskID             string
	Repo               string
	Profile            string
	FromTurn           int
	ToTurn             int
	NextReads          []string
	ActualReads        []string
	SourceArtifactPath string
}

func measureAnticipationPair(in pairInput) AnticipationPairResult {
	predictions := normalizePredictions(in.Repo, in.NextReads)
	actualSet := map[string]bool{}
	for _, path := range in.ActualReads {
		if normalized := normalizeRepoPath(in.Repo, path); normalized != "" {
			actualSet[normalized] = true
		}
	}
	actual := setToSortedSlice(actualSet)

	recallHits := map[string]bool{}
	precisionHits := map[string]bool{}
	for _, actualPath := range actual {
		for _, prediction := range predictions {
			if predictionMatchesPath(prediction, actualPath) {
				recallHits[actualPath] = true
				precisionHits[prediction.Entry] = true
			}
		}
	}

	result := AnticipationPairResult{
		Corpus:             in.Corpus,
		EpisodeID:          in.EpisodeID,
		SessionID:          in.SessionID,
		TaskID:             in.TaskID,
		Repo:               in.Repo,
		Profile:            in.Profile,
		FromTurn:           in.FromTurn,
		ToTurn:             in.ToTurn,
		NextReads:          append([]string{}, in.NextReads...),
		Predictions:        predictions,
		ActualReads:        actual,
		RecallHits:         setToSortedSlice(recallHits),
		PrecisionHits:      setToSortedSlice(precisionHits),
		HasPrediction:      len(predictions) > 0,
		HasActualRead:      len(actual) > 0,
		SourceArtifactPath: in.SourceArtifactPath,
	}
	result.Recall = anticipationRatio(len(result.RecallHits), len(result.ActualReads))
	result.Precision = anticipationRatio(len(result.PrecisionHits), len(result.Predictions))
	return result
}

func reportNextReads(report *sidecar.Report) []string {
	if report == nil {
		return nil
	}
	return nonBlankStrings(report.NextReads)
}

func actualReadsFromEvalTurn(repo string, turn TurnRecord) []string {
	paths := map[string]bool{}
	for _, command := range turn.ToolCalls {
		addActualReadsFromCommand(repo, command, "", paths)
	}
	for _, command := range turn.ReportCommands() {
		addActualReadsFromCommand(repo, command, "", paths)
	}
	for _, trace := range turn.ToolTraces {
		if strings.EqualFold(trace.Kind, "read") {
			addReadToolPath(repo, trace.RawInput, paths)
		}
		command := commandFromTrace(trace.RawInput)
		if command == "" {
			command = trace.Title
		}
		addActualReadsFromCommand(repo, command, trace.OutputExcerpt, paths)
	}
	return setToSortedSlice(paths)
}

// ReportCommands returns report commands for one turn. It is a method so the
// miner can name the report command source without exposing report internals at
// every call site.
func (tr TurnRecord) ReportCommands() []string {
	if tr.Report == nil {
		return nil
	}
	return tr.Report.CommandsRun
}

type sessionMeta struct {
	Repo      string `json:"repo"`
	TurnCount int    `json:"turnCount"`
}

func loadSessionMeta(path string) (sessionMeta, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return sessionMeta{}, err
	}
	var meta sessionMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return sessionMeta{}, fmt.Errorf("parse %s: %w", path, err)
	}
	return meta, nil
}

func loadSessionReports(reportsDir string) (map[int]*sidecar.Report, error) {
	paths, err := filepath.Glob(filepath.Join(reportsDir, "*.json"))
	if err != nil {
		return nil, err
	}
	reports := map[int]*sidecar.Report{}
	for _, path := range paths {
		turn, ok := turnFromReportFilename(filepath.Base(path))
		if !ok {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		var report sidecar.Report
		if err := json.Unmarshal(data, &report); err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
		reports[turn] = &report
	}
	return reports, nil
}

func turnFromReportFilename(name string) (int, bool) {
	prefix, _, ok := strings.Cut(name, "-")
	if !ok {
		return 0, false
	}
	turn, err := strconv.Atoi(prefix)
	return turn, err == nil
}

func loadSessionTraceReads(repo, tracePath string) (map[int]map[string]bool, error) {
	file, err := os.Open(tracePath)
	if err != nil {
		if os.IsNotExist(err) {
			return map[int]map[string]bool{}, nil
		}
		return nil, err
	}
	defer file.Close()

	reads := map[int]map[string]bool{}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 1024*1024), 64*1024*1024)
	line := 0
	for scanner.Scan() {
		line++
		var doc map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &doc); err != nil {
			return nil, fmt.Errorf("parse %s line %d: %w", tracePath, line, err)
		}
		for _, span := range otlpSpans(doc) {
			turn := intAttr(span, "ghx.sidecar.turn")
			if turn == 0 {
				continue
			}
			title := stringAttr(span, "ghx.sidecar.tool.title")
			kind := stringAttr(span, "ghx.sidecar.tool.kind")
			excerpt := stringAttr(span, "ghx.sidecar.tool.output_excerpt")
			if reads[turn] == nil {
				reads[turn] = map[string]bool{}
			}
			if strings.EqualFold(kind, "read") {
				addReadToolPath(repo, span["rawInput"], reads[turn])
			}
			addActualReadsFromCommand(repo, title, excerpt, reads[turn])
		}
	}
	return reads, scanner.Err()
}

func addReadToolPath(repo string, raw any, paths map[string]bool) {
	m, ok := raw.(map[string]any)
	if !ok {
		return
	}
	for _, key := range []string{"file_path", "path"} {
		if value, ok := m[key].(string); ok {
			if normalized := normalizeRepoPath(repo, value); normalized != "" {
				paths[normalized] = true
			}
		}
	}
}

func commandFromTrace(raw any) string {
	m, ok := raw.(map[string]any)
	if !ok {
		return ""
	}
	if command, ok := m["command"].(string); ok {
		return command
	}
	return ""
}

func addActualReadsFromCommand(repo, command, output string, paths map[string]bool) {
	for _, invocation := range ghxInvocations(command) {
		fields := shellFields(invocation)
		if len(fields) < 2 || fields[0] != "ghx" {
			continue
		}
		switch fields[1] {
		case "read":
			for _, path := range pathsFromGhxRead(repo, fields[2:]) {
				paths[path] = true
			}
		case "inspect", "search":
			for _, path := range pathCandidates(repo, output) {
				paths[path] = true
			}
		}
	}
}

func pathsFromGhxRead(repo string, args []string) []string {
	var out []string
	skipNext := false
	sawRepo := false
	for i, arg := range args {
		if skipNext {
			skipNext = false
			continue
		}
		if arg == "|" || arg == ";" || arg == "&&" || arg == "||" {
			break
		}
		if strings.HasPrefix(arg, "-") {
			if flagTakesValue(arg) && i+1 < len(args) {
				skipNext = true
			}
			continue
		}
		if !sawRepo && looksLikeRepo(arg) {
			sawRepo = true
			continue
		}
		if normalized := normalizeRepoPath(repo, arg); normalized != "" && looksLikeFilePath(normalized) {
			out = append(out, normalized)
		}
	}
	return uniqueStrings(out)
}

func flagTakesValue(flag string) bool {
	switch flag {
	case "--lines", "--grep", "--around", "--start", "--end", "--offset", "--limit", "--line-range":
		return true
	default:
		return false
	}
}

func normalizePredictions(repo string, entries []string) []PredictionRecord {
	var predictions []PredictionRecord
	for _, entry := range nonBlankStrings(entries) {
		candidates := pathCandidates(repo, entry)
		if len(candidates) == 0 {
			if area := normalizeAreaEntry(repo, entry); area != "" {
				candidates = []string{area}
			}
		}
		if len(candidates) == 0 {
			continue
		}
		sort.Strings(candidates)
		predictions = append(predictions, PredictionRecord{
			Entry:      entry,
			Candidates: candidates,
			Kind:       predictionKind(candidates),
		})
	}
	return predictions
}

func predictionKind(candidates []string) string {
	hasExact := false
	hasArea := false
	for _, candidate := range candidates {
		if looksLikeFilePath(candidate) {
			hasExact = true
		} else {
			hasArea = true
		}
	}
	switch {
	case hasExact && hasArea:
		return "mixed"
	case hasExact:
		return "exact"
	default:
		return "area"
	}
}

func predictionMatchesPath(prediction PredictionRecord, actual string) bool {
	for _, candidate := range prediction.Candidates {
		if looksLikeFilePath(candidate) {
			if candidate == actual {
				return true
			}
			continue
		}
		if candidate == actual || strings.HasPrefix(actual, strings.TrimSuffix(candidate, "/")+"/") {
			return true
		}
	}
	return false
}

var anticipationPathTokenRE = regexp.MustCompile(`(?:[A-Za-z0-9_.-]+/[A-Za-z0-9_./@+-]+|[A-Za-z0-9_.@+-]+\.[A-Za-z0-9][A-Za-z0-9]+)(?::[0-9]+(?:-[0-9]+)?)?`)

func pathCandidates(repo, text string) []string {
	var out []string
	clean := strings.NewReplacer("`", " ", "\"", " ", "'", " ", "(", " ", ")", " ", "[", " ", "]", " ", "{", " ", "}", " ").Replace(text)
	for _, token := range anticipationPathTokenRE.FindAllString(clean, -1) {
		normalized := normalizeRepoPath(repo, token)
		if normalized != "" && (strings.Contains(normalized, "/") || looksLikeFilePath(normalized)) {
			out = append(out, normalized)
		}
	}
	return uniqueStrings(out)
}

func normalizeAreaEntry(repo, entry string) string {
	entry = strings.TrimSpace(entry)
	entry = strings.Trim(entry, "`'\". ")
	entry = strings.TrimPrefix(entry, "ghx read ")
	entry = strings.TrimPrefix(entry, "ghx inspect ")
	entry = strings.TrimPrefix(entry, "ghx search ")
	entry = normalizeRepoPath(repo, entry)
	if entry == "" || strings.Contains(entry, " ") || looksLikeFilePath(entry) {
		return ""
	}
	if strings.Contains(entry, "/") || singleTokenArea(entry) {
		return strings.TrimSuffix(entry, "/")
	}
	return ""
}

func singleTokenArea(entry string) bool {
	if entry == "" || strings.ContainsAny(entry, " \t\n") || strings.Contains(entry, ".") || looksLikeRepo(entry) {
		return false
	}
	switch strings.ToLower(entry) {
	case "src", "lib", "internal", "cmd", "docs", "test", "tests", "examples", "packages", "pkg", "app", "apps":
		return true
	default:
		return false
	}
}

func normalizeRepoPath(repo, path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	path = strings.Trim(path, "`'\".,;:()[]{}<>")
	path = strings.ReplaceAll(path, "\\", "/")
	path = strings.TrimPrefix(path, "./")
	path = stripLineSuffix(path)
	if repo != "" {
		path = strings.TrimPrefix(path, repo+":")
		path = strings.TrimPrefix(path, strings.ToLower(repo)+":")
		if strings.HasPrefix(path, repo+"/") {
			path = strings.TrimPrefix(path, repo+"/")
		}
	}
	if looksLikeRepo(path) {
		return ""
	}
	if strings.HasPrefix(path, "/dev/") {
		return ""
	}
	if strings.HasPrefix(path, "/") {
		parts := strings.Split(path, "/")
		for i, part := range parts {
			if strings.EqualFold(part, "src") || strings.EqualFold(part, "lib") || strings.EqualFold(part, "internal") || strings.EqualFold(part, "cmd") || strings.EqualFold(part, "docs") || strings.Contains(part, ".") {
				path = strings.Join(parts[i:], "/")
				break
			}
		}
	}
	path = strings.Trim(path, "/")
	if strings.Contains(path, "://") || strings.Contains(path, "$") {
		return ""
	}
	if strings.Contains(path, ".com/") || strings.Contains(path, ".org/") || strings.Contains(path, ".net/") {
		return ""
	}
	return path
}

var lineSuffixRE = regexp.MustCompile(`(?i)(?::[0-9]+(?:-[0-9]+)?|#L[0-9]+(?:-L[0-9]+)?|\s+lines?\s+[0-9]+.*)$`)

func stripLineSuffix(path string) string {
	return strings.TrimSpace(lineSuffixRE.ReplaceAllString(path, ""))
}

func looksLikeRepo(value string) bool {
	parts := strings.Split(value, "/")
	return len(parts) == 2 && parts[0] != "" && parts[1] != "" && !strings.Contains(parts[1], ".")
}

func looksLikeFilePath(path string) bool {
	base := filepath.Base(path)
	if base == "." || base == "/" || base == "" {
		return false
	}
	ext := filepath.Ext(base)
	if ext == "" {
		return false
	}
	if len(ext) > 12 {
		return false
	}
	return knownRepoFileExt(strings.TrimPrefix(strings.ToLower(ext), "."))
}

func knownRepoFileExt(ext string) bool {
	switch ext {
	case "go", "js", "jsx", "ts", "tsx", "mjs", "cjs",
		"py", "rs", "java", "kt", "kts", "rb", "php",
		"c", "cc", "cpp", "cxx", "h", "hpp", "cs", "swift", "m", "mm",
		"scala", "clj", "cljs", "ex", "exs", "erl", "hrl",
		"sql", "html", "css", "scss", "sass", "svelte", "vue",
		"json", "yml", "yaml", "toml", "xml", "proto",
		"md", "mdx", "txt", "rst", "adoc",
		"sh", "zsh", "bash", "fish", "ps1",
		"mod", "sum", "lock":
		return true
	default:
		return false
	}
}

func ghxInvocations(command string) []string {
	command = strings.ReplaceAll(command, "\n", ";")
	parts := regexp.MustCompile(`(?:^|[;&|]\s*)(ghx\s+[^;&|]+)`).FindAllStringSubmatch(command, -1)
	var out []string
	for _, part := range parts {
		if len(part) > 1 {
			out = append(out, strings.TrimSpace(part[1]))
		}
	}
	if strings.HasPrefix(strings.TrimSpace(command), "ghx ") && len(out) == 0 {
		out = append(out, strings.TrimSpace(command))
	}
	return out
}

func shellFields(command string) []string {
	var fields []string
	var b strings.Builder
	var quote rune
	escaped := false
	for _, r := range command {
		switch {
		case escaped:
			b.WriteRune(r)
			escaped = false
		case r == '\\':
			escaped = true
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				b.WriteRune(r)
			}
		case r == '\'' || r == '"':
			quote = r
		case r == ' ' || r == '\t' || r == '\n':
			if b.Len() > 0 {
				fields = append(fields, b.String())
				b.Reset()
			}
		default:
			b.WriteRune(r)
		}
	}
	if b.Len() > 0 {
		fields = append(fields, b.String())
	}
	return fields
}

func summarizeAnticipationPairs(corpus string, units, multiTurn int, pairs []AnticipationPairResult) AnticipationCorpusSummary {
	summary := AnticipationCorpusSummary{
		Corpus:             corpus,
		EpisodesOrSessions: units,
		MultiTurnUnits:     multiTurn,
		Pairs:              len(pairs),
		GateThreshold:      0.30,
		PerTask:            map[string]AnticipationTaskRoll{},
	}
	var macroRecall, macroPrecision, seededMacroRecall, seededMacroPrecision float64
	taskHits := map[string]int{}
	for _, pair := range pairs {
		task := pair.TaskID
		if task == "" {
			task = pair.SessionID
		}
		roll := summary.PerTask[task]
		roll.Pairs++
		roll.Predictions += len(pair.Predictions)
		roll.ActualReads += len(pair.ActualReads)
		roll.RecallHits += len(pair.RecallHits)
		roll.PrecisionHits += len(pair.PrecisionHits)
		if pair.HasActualRead {
			summary.PairsWithActualReads++
		}
		if pair.HasPrediction {
			summary.SeededPairs++
			roll.SeededPairs++
			seededMacroRecall += pair.Recall
			seededMacroPrecision += pair.Precision
		}
		macroRecall += pair.Recall
		macroPrecision += pair.Precision
		summary.TotalPredictions += len(pair.Predictions)
		summary.TotalActualReads += len(pair.ActualReads)
		summary.TotalRecallHits += len(pair.RecallHits)
		summary.TotalPrecisionHits += len(pair.PrecisionHits)
		taskHits[task] += len(pair.RecallHits)
		summary.PerTask[task] = roll
	}
	summary.MicroRecall = anticipationRatio(summary.TotalRecallHits, summary.TotalActualReads)
	summary.MicroPrecision = anticipationRatio(summary.TotalPrecisionHits, summary.TotalPredictions)
	if len(pairs) > 0 {
		summary.MacroRecall = macroRecall / float64(len(pairs))
		summary.MacroPrecision = macroPrecision / float64(len(pairs))
	}
	if summary.SeededPairs > 0 {
		summary.SeededMacroRecall = seededMacroRecall / float64(summary.SeededPairs)
		summary.SeededMacroPrecision = seededMacroPrecision / float64(summary.SeededPairs)
	}
	seededActual := 0
	seededRecallHits := 0
	seededPredictions := 0
	seededPrecisionHits := 0
	for _, pair := range pairs {
		if !pair.HasPrediction {
			continue
		}
		seededActual += len(pair.ActualReads)
		seededRecallHits += len(pair.RecallHits)
		seededPredictions += len(pair.Predictions)
		seededPrecisionHits += len(pair.PrecisionHits)
	}
	summary.SeededMicroRecall = anticipationRatio(seededRecallHits, seededActual)
	summary.SeededMicroPrecision = anticipationRatio(seededPrecisionHits, seededPredictions)
	summary.GatePassed = summary.MicroRecall >= summary.GateThreshold

	for task, roll := range summary.PerTask {
		roll.MicroRecall = anticipationRatio(roll.RecallHits, roll.ActualReads)
		roll.MicroPrecision = anticipationRatio(roll.PrecisionHits, roll.Predictions)
		roll.SeededMicroRecall = roll.MicroRecall
		roll.SeededMicroPrec = roll.MicroPrecision
		summary.PerTask[task] = roll
	}
	topHits := 0
	for task, hits := range taskHits {
		if hits > topHits || (hits == topHits && (summary.HitConcentrationTopTask == "" || task < summary.HitConcentrationTopTask)) {
			topHits = hits
			summary.HitConcentrationTopTask = task
		}
	}
	summary.HitConcentrationShare = anticipationRatio(topHits, summary.TotalRecallHits)
	return summary
}

func writePairJSONL(path string, pairs []AnticipationPairResult) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	enc := json.NewEncoder(file)
	for _, pair := range pairs {
		if err := enc.Encode(pair); err != nil {
			return err
		}
	}
	return nil
}

func renderAnticipationREADME(evalSummary, localSummary AnticipationCorpusSummary) string {
	var b strings.Builder
	b.WriteString("# Anticipation Predictor D1 — 2026-07\n\n")
	b.WriteString("Deterministic offline miner for ADR-0031.1 D1. No live episodes and no LLM calls are used; the miner reads committed eval artifacts plus local `~/.ghx/sessions` traces.\n\n")
	b.WriteString("## Matching Rule\n\n")
	b.WriteString(AnticipationMatchingRule)
	b.WriteString("\n\n")
	b.WriteString("## Corpus Summary\n\n")
	b.WriteString("| corpus | units | multi-turn units | pairs | seeded pairs | actual-read pairs | micro recall | micro precision | seeded micro recall | seeded micro precision | D1 gate |\n")
	b.WriteString("|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|\n")
	for _, summary := range []AnticipationCorpusSummary{evalSummary, localSummary} {
		b.WriteString(fmt.Sprintf("| %s | %d | %d | %d | %d | %d | %.3f | %.3f | %.3f | %.3f | %s |\n",
			summary.Corpus,
			summary.EpisodesOrSessions,
			summary.MultiTurnUnits,
			summary.Pairs,
			summary.SeededPairs,
			summary.PairsWithActualReads,
			summary.MicroRecall,
			summary.MicroPrecision,
			summary.SeededMicroRecall,
			summary.SeededMicroPrecision,
			gateLabel(summary),
		))
	}
	b.WriteString("\n")
	writeTaskTable(&b, "Committed evals per task", evalSummary)
	writeTaskTable(&b, "Local dogfood per session", localSummary)
	b.WriteString("## Distribution Notes\n\n")
	writeDistributionNotes(&b, evalSummary)
	writeDistributionNotes(&b, localSummary)
	b.WriteString("\n## D1 Gate Verdict\n\n")
	b.WriteString(fmt.Sprintf("- committed-evals: %s at micro recall %.3f against threshold %.2f.\n", gateLabel(evalSummary), evalSummary.MicroRecall, evalSummary.GateThreshold))
	b.WriteString(fmt.Sprintf("- local-dogfood: %s at micro recall %.3f against threshold %.2f.\n", gateLabel(localSummary), localSummary.MicroRecall, localSummary.GateThreshold))
	b.WriteString("\nCaveat: the committed eval corpus has only 30 consecutive pairs and only 4 seeded pairs; the local dogfood corpus has only 5 consecutive pairs and 0 seeded pairs. These are honest availability measurements for D1, not stable behavioral estimates for all future sidecar sessions.\n\n")
	b.WriteString("## Raw Data\n\n")
	b.WriteString("Raw per-pair measurements are in `pairs.jsonl` in this directory.\n")
	return b.String()
}

func writeTaskTable(b *strings.Builder, title string, summary AnticipationCorpusSummary) {
	b.WriteString("## " + title + "\n\n")
	b.WriteString("| task/session | pairs | seeded pairs | predictions | actual reads | recall hits | precision hits | micro recall | micro precision |\n")
	b.WriteString("|---|---:|---:|---:|---:|---:|---:|---:|---:|\n")
	keys := make([]string, 0, len(summary.PerTask))
	for key := range summary.PerTask {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		roll := summary.PerTask[key]
		b.WriteString(fmt.Sprintf("| %s | %d | %d | %d | %d | %d | %d | %.3f | %.3f |\n",
			key, roll.Pairs, roll.SeededPairs, roll.Predictions, roll.ActualReads, roll.RecallHits, roll.PrecisionHits, roll.MicroRecall, roll.MicroPrecision))
	}
	b.WriteString("\n")
}

func writeDistributionNotes(b *strings.Builder, summary AnticipationCorpusSummary) {
	if summary.TotalRecallHits == 0 {
		b.WriteString(fmt.Sprintf("- %s: no recall hits; hits are not concentrated because there are none. Seed sparsity dominates this corpus (%d/%d pairs seeded).\n", summary.Corpus, summary.SeededPairs, summary.Pairs))
		return
	}
	b.WriteString(fmt.Sprintf("- %s: top hit task/session `%s` accounts for %.1f%% of recall hits (%d total hits), so concentration is %s.\n",
		summary.Corpus,
		summary.HitConcentrationTopTask,
		summary.HitConcentrationShare*100,
		summary.TotalRecallHits,
		concentrationLabel(summary.HitConcentrationShare),
	))
}

func gateLabel(summary AnticipationCorpusSummary) string {
	if summary.GatePassed {
		return "PASS"
	}
	return "FAIL"
}

func concentrationLabel(share float64) string {
	if share >= 0.75 {
		return "high"
	}
	if share >= 0.50 {
		return "moderate"
	}
	return "low"
}

func anticipationRatio(num, den int) float64 {
	if den == 0 {
		return 0
	}
	return float64(num) / float64(den)
}

func nonBlankStrings(values []string) []string {
	var out []string
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	for _, value := range values {
		if value != "" {
			seen[value] = true
		}
	}
	return setToSortedSlice(seen)
}

func setToSortedSlice(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for value := range set {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func otlpSpans(doc map[string]any) []map[string]any {
	var spans []map[string]any
	for _, rs := range anySlice(doc["resourceSpans"]) {
		for _, ss := range anySlice(mapValue(rs)["scopeSpans"]) {
			for _, span := range anySlice(mapValue(ss)["spans"]) {
				if m := mapValue(span); m != nil {
					spans = append(spans, m)
				}
			}
		}
	}
	return spans
}

func stringAttr(span map[string]any, key string) string {
	for _, attr := range anySlice(span["attributes"]) {
		m := mapValue(attr)
		if m == nil || m["key"] != key {
			continue
		}
		value := mapValue(m["value"])
		if s, ok := value["stringValue"].(string); ok {
			return s
		}
	}
	return ""
}

func intAttr(span map[string]any, key string) int {
	for _, attr := range anySlice(span["attributes"]) {
		m := mapValue(attr)
		if m == nil || m["key"] != key {
			continue
		}
		value := mapValue(m["value"])
		switch v := value["intValue"].(type) {
		case string:
			n, _ := strconv.Atoi(v)
			return n
		case float64:
			return int(v)
		}
	}
	return 0
}

func anySlice(value any) []any {
	s, _ := value.([]any)
	return s
}

func mapValue(value any) map[string]any {
	m, _ := value.(map[string]any)
	return m
}
