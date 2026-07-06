package sidecar

import (
	"sort"
	"strings"
)

// This file is the shared question-matching primitive of ADR-0030.1 D6:
// deterministic tokenization plus weighted vocabulary overlap against stored
// session state. It has two consumers with separate accept policies — session
// routing (ADR-0030.1 D3, this package's route.go) and, later, anticipation
// serving (ADR-0031 / 0031.1). No model calls: every score is recomputable by
// hand from the inputs, which is the falsifiable form of "explainable"
// (visibility tenet).

// Vocabulary weights (ADR-0030.1 D3), by how strongly a hit indicates "this
// investigation": OpenQuestions are the model's own follow-up forecast
// (uncertainty + nextReads), so a hit there means the session predicted the
// question.
const (
	weightOpenQuestions = 3.0
	weightRelevantFiles = 2.0
	weightGrepPatterns  = 2.0
	weightRepoScope     = 2.0
	weightInspected     = 1.0
)

// matchStopwords is the fixed, versioned stopword table used by
// TokenizeQuestion. Changing it changes pinned decision-table test rows
// (ADR-0030.1 D7) — edit deliberately.
var matchStopwords = map[string]struct{}{
	"about": {}, "again": {}, "all": {}, "and": {}, "any": {}, "are": {},
	"been": {}, "being": {}, "between": {}, "but": {}, "can": {}, "could": {},
	"did": {}, "does": {}, "done": {}, "each": {}, "few": {}, "for": {},
	"from": {}, "had": {}, "has": {}, "have": {}, "her": {}, "here": {},
	"hers": {}, "him": {}, "his": {}, "how": {}, "into": {}, "its": {},
	"just": {}, "may": {}, "might": {}, "more": {}, "most": {}, "must": {},
	"nor": {}, "not": {}, "off": {}, "once": {}, "one": {}, "ones": {},
	"only": {}, "other": {}, "our": {}, "out": {}, "over": {}, "own": {},
	"please": {}, "same": {}, "shall": {}, "she": {}, "should": {},
	"some": {}, "such": {}, "than": {}, "that": {}, "the": {}, "their": {},
	"them": {}, "then": {}, "there": {}, "these": {}, "they": {}, "this": {},
	"those": {}, "through": {}, "too": {}, "under": {}, "very": {},
	"was": {}, "were": {}, "what": {}, "when": {}, "where": {}, "which": {},
	"who": {}, "whom": {}, "why": {}, "will": {}, "with": {}, "would": {},
	"yes": {}, "you": {}, "your": {},
}

// TokenizeQuestion normalizes a question into content tokens (ADR-0030.1 D3):
// lowercase, split on non-alphanumeric runs, drop stopwords and tokens
// shorter than 3 chars. Deterministic; shared by routing and (later)
// anticipation serving.
func TokenizeQuestion(question string) []string {
	var tokens []string
	var b strings.Builder
	flush := func() {
		if b.Len() == 0 {
			return
		}
		tok := b.String()
		b.Reset()
		if len(tok) < 3 {
			return
		}
		if _, stop := matchStopwords[tok]; stop {
			return
		}
		tokens = append(tokens, tok)
	}
	for _, r := range strings.ToLower(question) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			continue
		}
		flush()
	}
	flush()
	return tokens
}

// Vocabulary maps a content token to the strongest weight any session signal
// assigned it. Weights never stack across signal classes: a token seen in
// both OpenQuestions (×3) and InspectedPaths (×1) counts ×3.
type Vocabulary map[string]float64

func (v Vocabulary) add(token string, weight float64) {
	if v[token] < weight {
		v[token] = weight
	}
}

// addText tokenizes free text and adds each token at the given weight.
func (v Vocabulary) addText(text string, weight float64) {
	for _, tok := range TokenizeQuestion(text) {
		v.add(tok, weight)
	}
}

// addPathSegments splits a path or glob on separators and adds each segment's
// tokens at the given weight ("internal/sidecar/route.go" → internal,
// sidecar, route).
func (v Vocabulary) addPathSegments(path string, weight float64) {
	for _, seg := range strings.FieldsFunc(path, func(r rune) bool { return r == '/' || r == '\\' }) {
		v.addText(seg, weight)
	}
}

// BuildSessionVocabulary derives the weighted vocabulary of one session from
// its metadata and evidence ledger (ADR-0030.1 D3). The ledger is unusually
// literal input — paths, grep patterns, and the model's own next-question
// forecasts — which is why lexical overlap is the v1 scorer.
func BuildSessionVocabulary(meta *SessionMeta, ledger *Ledger) Vocabulary {
	v := Vocabulary{}
	if ledger != nil {
		for _, e := range ledger.OpenQuestions {
			v.addText(e.Value, weightOpenQuestions)
		}
		for _, f := range ledger.RelevantFiles {
			v.addPathSegments(f.Path, weightRelevantFiles)
		}
		for _, e := range ledger.GrepPatterns {
			v.addText(e.Value, weightGrepPatterns)
		}
		for _, e := range ledger.InspectedPaths {
			v.addPathSegments(e.Value, weightInspected)
		}
		for _, e := range ledger.MappedGlobs {
			v.addPathSegments(e.Value, weightInspected)
		}
	}
	repo, scope := "", ""
	if meta != nil {
		repo, scope = meta.Repo, meta.Scope
	}
	if repo == "" && ledger != nil {
		repo = ledger.Repo
	}
	if scope == "" && ledger != nil {
		scope = ledger.Scope
	}
	v.addPathSegments(repo, weightRepoScope)
	v.addText(scope, weightRepoScope)
	return v
}

// OverlapScore computes the weighted vocabulary overlap for a question
// (ADR-0030.1 D3): the sum of the vocabulary weight of each UNIQUE question
// token, normalized by the unique-token count — short questions are not
// penalized and long questions cannot win on volume. Range [0, 3].
func OverlapScore(questionTokens []string, vocab Vocabulary) float64 {
	unique := uniqueTokens(questionTokens)
	if len(unique) == 0 {
		return 0
	}
	var sum float64
	for _, tok := range unique {
		sum += vocab[tok]
	}
	return sum / float64(len(unique))
}

func uniqueTokens(tokens []string) []string {
	seen := make(map[string]struct{}, len(tokens))
	var out []string
	for _, t := range tokens {
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		out = append(out, t)
	}
	sort.Strings(out)
	return out
}
