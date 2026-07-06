package sidecar

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// This file implements the ADR-0030.1 D1 routing cascade: five ordered,
// deterministic rules evaluated top to bottom; the first that fires wins and
// is recorded as the decision's Source. No LLM and no embedding model in the
// routing path (considered and rejected in the ADR). Given the RouteDecision
// and the session states at decision time, the route is recomputable by hand
// — the falsifiable form of "explainable" (D7).

// RouteSource labels which cascade rule produced a RouteDecision.
type RouteSource string

// Cascade rule labels (ADR-0030.1 D1 table).
const (
	RouteSourceExplicit     RouteSource = "explicit"     // R1: session explicitly provided
	RouteSourceRepo         RouteSource = "repo"         // R2: repo param or owner/repo token in the question
	RouteSourceContinuation RouteSource = "continuation" // R3: continuation marker + exactly one warm session
	RouteSourceOverlap      RouteSource = "overlap"      // R4: ledger-overlap score above threshold with margin
	RouteSourceNew          RouteSource = "new"          // R5: new discovery session (safe default)
)

// Session naming origins recorded in SessionMeta.NamedBy at creation.
// Explicitly-named sessions are deliberate isolation ("parallel investigation
// threads") and are reachable only via R1 — R3/R4 never auto-join them
// (ADR-0030.1 D1/D7: zero false joins into named sessions is structural).
const (
	SessionNamedExplicit = "explicit"
	SessionNamedRepo     = "repo"
	SessionNamedQuestion = "question"
)

// Routing defaults (ADR-0030.1 D2/D3). The decision-table tests in
// route_test.go are the pinned specification of these values — changing a
// default means changing a visible test row.
const (
	// defaultRouteOverlapThreshold is the minimum R4 score (weighted overlap
	// normalized by unique question tokens, recency-decayed) to join an
	// existing session. Conservative because a false join pollutes a live
	// ledger (wrong-existing is strictly worse than wrong-new).
	defaultRouteOverlapThreshold = 0.5
	// defaultRouteOverlapMargin is the minimum lead over the runner-up; when
	// two sessions look alike, R5 (new session) is the safe default.
	defaultRouteOverlapMargin = 0.25
	// defaultContinuationWindow mirrors the daemon warm-worker TTL (ADR-0030
	// D4): "the session you were just talking to is still warm" is the honest
	// meaning of "continuation".
	defaultContinuationWindow = 30 * time.Minute
	// defaultRoutingWindow bounds R4 candidates: ledgers persist, and
	// yesterday's discovery thread is a legitimate continuation target even
	// though its worker went cold.
	defaultRoutingWindow = 7 * 24 * time.Hour
)

// continuationMarkerTableVersion versions the D2 marker tables below so route
// records stay recomputable after the tables evolve from dogfood evidence.
const continuationMarkerTableVersion = 1

// continuationConnectives are leading connectives that mark a follow-up
// question (ADR-0030.1 D2), matched as prefixes of the normalized question.
var continuationConnectives = []string{
	"and ", "also ", "what about", "how about", "then ", "ok so", "ok, so",
	"now ", "so ",
}

// continuationDeixis are bare pronouns/deictic words whose presence (with no
// repo token and few content tokens) marks anaphora: "how mature is the
// second one" names nothing an overlap scorer could catch.
var continuationDeixis = map[string]struct{}{
	"it": {}, "its": {}, "that": {}, "this": {}, "these": {}, "those": {},
	"them": {}, "they": {}, "one": {}, "ones": {},
}

// continuationMaxContentTokens caps how many content tokens (after stopword
// removal) a question may carry and still count as a bare follow-up.
const continuationMaxContentTokens = 6

// repoTokenPathDenylist rejects owner/repo lookalikes whose first segment is
// a common source-tree directory: "internal/sidecar" is a repo path, not a
// GitHub repo (versioned with the marker table; extend from dogfood
// evidence).
var repoTokenPathDenylist = map[string]struct{}{
	"app": {}, "apps": {}, "assets": {}, "bin": {}, "build": {}, "cmd": {},
	"dist": {}, "doc": {}, "docs": {}, "example": {}, "examples": {},
	"internal": {}, "lib": {}, "node_modules": {}, "out": {}, "packages": {},
	"pkg": {}, "public": {}, "scripts": {}, "src": {}, "static": {},
	"test": {}, "tests": {}, "tools": {}, "util": {}, "utils": {},
	"vendor": {},
}

// RouteConfig is the resolved routing configuration used by RouteQuestion.
// Zero values are invalid; build it with RouteConfigFor.
type RouteConfig struct {
	// OverlapThreshold gates R4 acceptance (D3).
	OverlapThreshold float64
	// OverlapMargin is the required lead over the runner-up (D3).
	OverlapMargin float64
	// ContinuationWindow bounds R3 warm candidates (D2).
	ContinuationWindow time.Duration
	// RoutingWindow bounds R4 candidates (D3).
	RoutingWindow time.Duration
	// Now supplies the decision timestamp; tests inject a fixed clock.
	Now func() time.Time
}

// RouteConfigFor resolves the routing knobs from config-file overrides over
// the pinned defaults (ADR-0030.1 v1 scope; the knobs are covered by the
// daemon config digest, ADR-0030 D6).
func RouteConfigFor(cfg Config) RouteConfig {
	rc := RouteConfig{
		OverlapThreshold:   defaultRouteOverlapThreshold,
		OverlapMargin:      defaultRouteOverlapMargin,
		ContinuationWindow: defaultContinuationWindow,
		RoutingWindow:      defaultRoutingWindow,
		Now:                func() time.Time { return time.Now().UTC() },
	}
	r := cfg.Route
	if r == nil {
		return rc
	}
	if r.OverlapThreshold != nil {
		rc.OverlapThreshold = *r.OverlapThreshold
	}
	if r.OverlapMargin != nil {
		rc.OverlapMargin = *r.OverlapMargin
	}
	if r.ContinuationWindowMinutes > 0 {
		rc.ContinuationWindow = time.Duration(r.ContinuationWindowMinutes) * time.Minute
	}
	if r.RoutingWindowHours > 0 {
		rc.RoutingWindow = time.Duration(r.RoutingWindowHours) * time.Hour
	}
	return rc
}

func (rc RouteConfig) now() time.Time {
	if rc.Now != nil {
		return rc.Now()
	}
	return time.Now().UTC()
}

// RouteCandidate is one scored session considered by R4, recorded on the
// decision for auditability (D7: top-3 "session=score" pairs).
type RouteCandidate struct {
	// Session is the candidate session name.
	Session string `json:"session"`
	// Score is the recency-decayed overlap score (D3).
	Score float64 `json:"score"`
}

// RouteDecision is the auditable output of the routing cascade. It travels on
// the TurnResult, the trace attributes, the session log, and the CLI/footer
// route line (D7): every route is a logged, recomputable record.
type RouteDecision struct {
	// Session is the resolved session name the ask runs in.
	Session string `json:"session"`
	// Source is the cascade rule that fired (D1 table).
	Source RouteSource `json:"source"`
	// DetectedRepo is the owner/repo token found in the question text when R2
	// fired without an explicit repo param.
	DetectedRepo string `json:"detectedRepo,omitempty"`
	// Score is the best overlap score when R4 was evaluated.
	Score float64 `json:"score,omitempty"`
	// Margin is best − runner-up when R4 was evaluated.
	Margin float64 `json:"margin,omitempty"`
	// Candidates are the top-3 scored sessions when R4 was evaluated.
	Candidates []RouteCandidate `json:"candidates,omitempty"`
	// WindowHit describes the continuation-window state when the R3 marker
	// test fired (hit or declined and why).
	WindowHit string `json:"windowHit,omitempty"`
}

// routed reports whether R4 scoring was evaluated on this decision (R3-R5
// paths); explicit/repo decisions short-circuit before any disk scan.
func (d RouteDecision) scored() bool { return len(d.Candidates) > 0 }

// Provenance renders the human/agent-readable route explanation used by the
// footer and stderr line (D5.2), e.g. "overlap 0.62, next 0.21".
func (d RouteDecision) Provenance() string {
	switch d.Source {
	case RouteSourceRepo:
		if d.DetectedRepo != "" {
			return "repo " + d.DetectedRepo
		}
		return "repo"
	case RouteSourceContinuation:
		return "continuation, " + d.WindowHit
	case RouteSourceOverlap:
		if len(d.Candidates) > 1 {
			return fmt.Sprintf("overlap %.2f, next %.2f", d.Score, d.Candidates[1].Score)
		}
		return fmt.Sprintf("overlap %.2f", d.Score)
	case RouteSourceNew:
		return "new"
	default:
		return string(d.Source)
	}
}

// Line renders the full route provenance line: "session: <name> (routed: ...)".
func (d RouteDecision) Line() string {
	return fmt.Sprintf("session: %s (routed: %s)", d.Session, d.Provenance())
}

// sessionNamedBy maps the firing rule to the NamedBy origin recorded when the
// routed-to session must be created. Continuation/overlap never create.
func (d RouteDecision) sessionNamedBy() string {
	switch d.Source {
	case RouteSourceRepo:
		return SessionNamedRepo
	case RouteSourceNew:
		return SessionNamedQuestion
	default:
		return SessionNamedExplicit
	}
}

// RouteQuestion evaluates the ADR-0030.1 D1 cascade for one ask against the
// on-disk session registry. R1/R2 preserve the ADR-0019.1 D2 precedence
// byte-for-byte and never touch the disk; R3-R5 replace the unconditional
// question-slug fallback.
func RouteQuestion(sessionsDir string, req AskRequest, rc RouteConfig) RouteDecision {
	// R1: explicit session wins over everything.
	if s := strings.TrimSpace(req.Session); s != "" {
		return RouteDecision{Session: s, Source: RouteSourceExplicit}
	}
	// R2: repo param, unchanged repo-slug naming.
	if r := strings.TrimSpace(req.Repo); r != "" {
		return RouteDecision{Session: Slug(r, "repo"), Source: RouteSourceRepo}
	}
	// R2: owner/repo token in the question text.
	if repo := DetectRepoToken(req.Question); repo != "" {
		return RouteDecision{Session: Slug(repo, "repo"), Source: RouteSourceRepo, DetectedRepo: repo}
	}

	now := rc.now()
	candidates := loadRouteCandidates(sessionsDir, rc, now)

	// R3: continuation fast path (D2). Fires only when the marker test passes
	// AND exactly one joinable session is warm — recency alone must never
	// pick between live topics.
	var windowHit string
	if hasContinuationMarker(req.Question) {
		var warm []routeCandidateState
		for _, c := range candidates {
			if now.Sub(c.updatedAt) <= rc.ContinuationWindow {
				warm = append(warm, c)
			}
		}
		switch len(warm) {
		case 1:
			return RouteDecision{
				Session:   warm[0].name,
				Source:    RouteSourceContinuation,
				WindowHit: fmt.Sprintf("1 warm session in %s window", rc.ContinuationWindow),
			}
		case 0:
			windowHit = fmt.Sprintf("marker hit, declined: no warm session in %s window", rc.ContinuationWindow)
		default:
			windowHit = fmt.Sprintf("marker hit, declined: %d warm sessions in %s window", len(warm), rc.ContinuationWindow)
		}
	}

	// R4: ledger-overlap scoring (D3).
	tokens := TokenizeQuestion(req.Question)
	scored := make([]RouteCandidate, 0, len(candidates))
	for _, c := range candidates {
		score := OverlapScore(tokens, c.vocab) * recencyDecay(now.Sub(c.updatedAt), rc.RoutingWindow)
		if score <= 0 {
			continue
		}
		scored = append(scored, RouteCandidate{Session: c.name, Score: score})
	}
	sort.Slice(scored, func(i, j int) bool {
		if scored[i].Score != scored[j].Score {
			return scored[i].Score > scored[j].Score
		}
		return scored[i].Session < scored[j].Session
	})
	top := scored
	if len(top) > 3 {
		top = top[:3]
	}
	var best, margin float64
	if len(scored) > 0 {
		best = scored[0].Score
		margin = best
		if len(scored) > 1 {
			margin = best - scored[1].Score
		}
		if best >= rc.OverlapThreshold && margin >= rc.OverlapMargin {
			return RouteDecision{
				Session:    scored[0].Session,
				Source:     RouteSourceOverlap,
				Score:      best,
				Margin:     margin,
				Candidates: top,
				WindowHit:  windowHit,
			}
		}
	}

	// R5: new discovery session — the explicit safe default, exactly today's
	// question-slug behavior (ADR-0019.1 D2).
	return RouteDecision{
		Session:    QuestionSlug(req.Question),
		Source:     RouteSourceNew,
		Score:      best,
		Margin:     margin,
		Candidates: top,
		WindowHit:  windowHit,
	}
}

// routeCandidateState is one joinable session loaded for R3/R4 evaluation.
type routeCandidateState struct {
	name      string
	updatedAt time.Time
	vocab     Vocabulary
}

// loadRouteCandidates loads the joinable sessions within the routing window:
// explicitly-named sessions are excluded (reachable only via R1), and
// window-expired sessions never compete.
func loadRouteCandidates(sessionsDir string, rc RouteConfig, now time.Time) []routeCandidateState {
	metas, err := ListSessions(sessionsDir)
	if err != nil {
		return nil
	}
	var out []routeCandidateState
	for i := range metas {
		meta := metas[i]
		if sessionOrigin(meta) == SessionNamedExplicit {
			continue
		}
		updatedAt, err := time.Parse(time.RFC3339, meta.UpdatedAt)
		if err != nil {
			continue
		}
		if now.Sub(updatedAt) > rc.RoutingWindow {
			continue
		}
		ledger, err := LoadLedger(sessionsDir, meta.Name)
		if err != nil {
			continue
		}
		out = append(out, routeCandidateState{
			name:      meta.Name,
			updatedAt: updatedAt,
			vocab:     BuildSessionVocabulary(&meta, ledger),
		})
	}
	return out
}

// sessionOrigin resolves how a session got its name. Sessions created before
// NamedBy existed are classified from the name shape, conservatively: a name
// that is neither the repo slug nor question-slug shaped is treated as
// explicitly named (excluded from auto-join).
func sessionOrigin(meta SessionMeta) string {
	if meta.NamedBy != "" {
		return meta.NamedBy
	}
	if meta.Repo != "" && meta.Name == Slug(meta.Repo, "repo") {
		return SessionNamedRepo
	}
	if isQuestionSlugShaped(meta.Name) {
		return SessionNamedQuestion
	}
	return SessionNamedExplicit
}

// isQuestionSlugShaped reports whether a name ends in the QuestionSlug
// "-<8 hex chars>" content-hash suffix.
func isQuestionSlugShaped(name string) bool {
	i := strings.LastIndex(name, "-")
	if i <= 0 || len(name)-i-1 != 8 {
		return false
	}
	for _, r := range name[i+1:] {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}

// recencyDecay discounts a candidate's overlap score by its age inside the
// routing window: 1.0 fresh, linearly down to 0.5 at the window edge — an
// equally-overlapping fresh session beats a stale one (D3).
func recencyDecay(age, window time.Duration) float64 {
	if age <= 0 {
		return 1
	}
	if window <= 0 || age >= window {
		return 0.5
	}
	return 1 - 0.5*float64(age)/float64(window)
}

// hasContinuationMarker applies the D2 marker test: a leading connective or a
// bare deictic word, few content tokens, and no owner/repo token. Version:
// continuationMarkerTableVersion.
func hasContinuationMarker(question string) bool {
	q := strings.ToLower(strings.TrimSpace(question))
	if q == "" {
		return false
	}
	if DetectRepoToken(question) != "" {
		return false
	}
	if len(TokenizeQuestion(question)) > continuationMaxContentTokens {
		return false
	}
	for _, c := range continuationConnectives {
		if strings.HasPrefix(q, c) {
			return true
		}
	}
	for _, w := range strings.Fields(q) {
		if _, ok := continuationDeixis[trimWordPunct(w)]; ok {
			return true
		}
	}
	return false
}

// DetectRepoToken scans question text for a GitHub owner/repo token: exactly
// one slash, both segments identifier-shaped, first segment not a common
// source-tree directory (repoTokenPathDenylist). Returns the first match.
func DetectRepoToken(question string) string {
	for _, w := range strings.Fields(question) {
		w = trimWordPunct(w)
		if strings.Count(w, "/") != 1 {
			continue
		}
		owner, repo, _ := strings.Cut(w, "/")
		if !repoSegmentOK(owner) || !repoSegmentOK(repo) {
			continue
		}
		if _, deny := repoTokenPathDenylist[strings.ToLower(owner)]; deny {
			continue
		}
		return w
	}
	return ""
}

// repoSegmentOK matches GitHub owner/repo segment shape: leading
// alphanumeric, then alphanumerics, dash, underscore, or dot.
func repoSegmentOK(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		alnum := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
		if i == 0 && !alnum {
			return false
		}
		if !alnum && r != '-' && r != '_' && r != '.' {
			return false
		}
	}
	return true
}

// trimWordPunct strips surrounding punctuation from a whitespace-split word
// so "gin-gonic/gin?" and "(vercel/next.js)" resolve to the bare token.
func trimWordPunct(w string) string {
	return strings.Trim(w, ".,;:!?()[]{}<>\"'`")
}
