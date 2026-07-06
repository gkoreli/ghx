package tier2

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"math"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/gkoreli/ghx/v2/internal/mapengine"
)

// BackendRepomap is the canonical backend ID for repomap ranking, the value
// reports and traces must carry when this backend contributed evidence
// (ADR-0024.1 "Visibility Contract").
//
// The ranking algorithm is stolen, not vendored (ADR-0024.1 "Absorb vs
// Steal"): aider's repo map (github.com/Aider-AI/aider, Apache-2.0) proved
// that definition/reference graphs ranked with personalized PageRank under a
// strict token budget are the right shape for "which files matter?". ghx owns
// the model over its own artifacts — the local snapshot tree, mapengine
// symbols, and query terms — with no aider dependency.
const BackendRepomap = "local:repomap"

// DefaultRepomapBudget is the default output budget in estimated tokens.
const DefaultRepomapBudget = 1024

// maxRepomapFileBytes caps the size of a file the fact collector will parse.
const maxRepomapFileBytes = 1 << 20

// repomapDamping is the PageRank damping factor (the standard 0.85).
const repomapDamping = 0.85

// repomapIterations is the fixed PageRank iteration count. Fixed (not
// convergence-based) so identical inputs always do identical work.
const repomapIterations = 30

// repomapQueryBoost is the personalization weight multiplier for files that
// match a query term (by path or by defining the term as a symbol), relative
// to the base weight 1 of every other file.
const repomapQueryBoost = 10.0

// repomapImportWeight is the total edge weight one resolved import
// contributes, split across the files of the resolved target. Imports are a
// much stronger dependency signal than name mentions, so they weigh more
// than a single reference.
const repomapImportWeight = 5.0

// repomapMaxDefsShown caps the definition signatures rendered per ranked file.
const repomapMaxDefsShown = 8

// minRefIdentLen ignores identifiers shorter than this when counting
// references: 1-2 char names ("i", "ok", "db") are noise that would connect
// everything to everything.
const minRefIdentLen = 3

// repomapSourceExts is the set of file extensions the fact collector treats
// as source files — the languages mapengine can produce symbols for.
var repomapSourceExts = map[string]bool{
	".go": true, ".ts": true, ".tsx": true, ".js": true, ".jsx": true,
	".py": true, ".rs": true, ".java": true, ".kt": true, ".rb": true,
}

// RepomapRequest describes one ranking request over a snapshot.
type RepomapRequest struct {
	// QueryTerms bias the ranking toward files matching the question, via
	// PageRank personalization. Empty ranks by pure structural centrality.
	QueryTerms []string
	// BudgetTokens caps the rendered output size in estimated tokens
	// (~4 chars/token). Zero means DefaultRepomapBudget.
	BudgetTokens int
}

// budget returns the effective token budget.
func (r RepomapRequest) budget() int {
	if r.BudgetTokens > 0 {
		return r.BudgetTokens
	}
	return DefaultRepomapBudget
}

// ArtifactKey names the tool artifact slot in SnapshotMetadata
// .ToolArtifactHashes for this ranking, e.g. "repomap:1024:a1b2c3d4e5f6".
// Deterministic for the same budget and query terms.
func (r RepomapRequest) ArtifactKey() string {
	key := "repomap:" + strconv.Itoa(r.budget())
	if len(r.QueryTerms) > 0 {
		sum := sha256.Sum256([]byte(strings.Join(r.QueryTerms, "\x00")))
		key += ":" + hex.EncodeToString(sum[:6])
	}
	return key
}

// Definition is one symbol a file defines, extracted by mapengine.
type Definition struct {
	// Name is the symbol name (referenced by other files).
	Name string
	// Signature is the display form, e.g. "func RankFiles(...) RepomapResult".
	Signature string
	// Kind is the mapengine symbol kind (func, type, const, var, package).
	Kind mapengine.Kind
}

// FileFacts are the per-file inputs to the ranking function: what the file
// defines, which identifiers it mentions, and what it imports. Collected from
// artifacts ghx already owns — the local snapshot tree and mapengine symbols
// (ADR-0024.1 item 4).
type FileFacts struct {
	// Path is the slash-separated path relative to the snapshot root.
	Path string
	// Defs are the symbols this file defines.
	Defs []Definition
	// Refs counts identifier occurrences in the file body (candidate
	// references to other files' definitions).
	Refs map[string]int
	// Imports are the raw import path strings found in the file.
	Imports []string
}

// RankedFile is one entry of the budgeted ranking output.
type RankedFile struct {
	// Path is the snapshot-relative file path.
	Path string `json:"path"`
	// Score is the personalized PageRank score, rounded to 1e-9 so equal
	// inputs render byte-identical output on any platform.
	Score float64 `json:"score"`
	// Definitions are up to repomapMaxDefsShown signature lines the file
	// defines. Omitted when the entry was degraded to path-only to fit the
	// budget.
	Definitions []string `json:"definitions,omitempty"`
	// Tokens is the estimated token cost of this entry (~4 chars/token).
	Tokens int `json:"tokens"`
}

// RepomapResult is the budgeted ranking answer.
type RepomapResult struct {
	// Backend is always BackendRepomap.
	Backend string `json:"backend"`
	// QueryTerms echoes the personalization terms used.
	QueryTerms []string `json:"queryTerms,omitempty"`
	// BudgetTokens is the effective output budget.
	BudgetTokens int `json:"budgetTokens"`
	// UsedTokens is the estimated token total of the included entries.
	UsedTokens int `json:"usedTokens"`
	// TotalFiles is how many source files were ranked before the budget cut.
	TotalFiles int `json:"totalFiles"`
	// Files are the included entries, best first (score desc, path asc).
	Files []RankedFile `json:"files"`
}

// RankFiles is the pure ranking function (ADR-0024.1 item 4): given per-file
// facts and a request it deterministically returns the budgeted ranking. No
// I/O, no clock, no randomness — unit-testable without a network, and
// identical inputs always produce identical output.
//
// Shape of the stolen algorithm: build a file graph where an edge A->B means
// A references identifiers B defines (weight = mentions / number-of-definers)
// or A imports B (weight = repomapImportWeight split across the resolved
// files), run personalized PageRank with query-matching files boosted, then
// emit ranked entries until the token budget is spent.
func RankFiles(facts []FileFacts, req RepomapRequest) RepomapResult {
	// Deterministic node order regardless of collection order.
	facts = append([]FileFacts(nil), facts...)
	sort.Slice(facts, func(i, j int) bool { return facts[i].Path < facts[j].Path })
	n := len(facts)

	scores := pageRank(buildEdges(facts), personalization(facts, req.QueryTerms))

	order := make([]int, n)
	for i := range order {
		order[i] = i
	}
	rounded := make([]float64, n)
	for i, s := range scores {
		rounded[i] = math.Round(s*1e9) / 1e9
	}
	sort.Slice(order, func(a, b int) bool {
		i, j := order[a], order[b]
		if rounded[i] != rounded[j] {
			return rounded[i] > rounded[j]
		}
		return facts[i].Path < facts[j].Path
	})

	result := RepomapResult{
		Backend:      BackendRepomap,
		QueryTerms:   req.QueryTerms,
		BudgetTokens: req.budget(),
		TotalFiles:   n,
	}
	for _, i := range order {
		entry := renderEntry(facts[i], rounded[i])
		if result.UsedTokens+entry.Tokens > result.BudgetTokens {
			// Degrade to a path-only entry before giving up (the aider move:
			// shrink granularity to respect the budget).
			entry.Definitions = nil
			entry.Tokens = estimateTokens(len(entry.Path) + entryOverheadChars)
			if result.UsedTokens+entry.Tokens > result.BudgetTokens {
				break
			}
		}
		result.UsedTokens += entry.Tokens
		result.Files = append(result.Files, entry)
	}
	return result
}

// edge is one weighted directed graph edge to node to.
type edge struct {
	to int
	w  float64
}

// buildEdges assembles the reference+import graph in deterministic order:
// nodes ascending, per-node targets ascending, weights merged per (from, to).
func buildEdges(facts []FileFacts) [][]edge {
	n := len(facts)

	// defIndex: identifier -> defining node indices (ascending by node order).
	defIndex := make(map[string][]int)
	for i, f := range facts {
		seen := map[string]bool{}
		for _, d := range f.Defs {
			if len(d.Name) < minRefIdentLen || seen[d.Name] {
				continue
			}
			seen[d.Name] = true
			defIndex[d.Name] = append(defIndex[d.Name], i)
		}
	}

	index := newFileIndex(facts)
	adj := make([][]edge, n)
	for i, f := range facts {
		weights := map[int]float64{}

		// Reference edges: mentioning a name defined elsewhere links the
		// mentioning file to each definer, diluted by how many files define
		// the name (a name everyone defines carries little signal).
		for name, count := range f.Refs {
			definers := defIndex[name]
			others := 0
			for _, j := range definers {
				if j != i {
					others++
				}
			}
			if others == 0 {
				continue
			}
			w := float64(count) / float64(others)
			for _, j := range definers {
				if j != i {
					weights[j] += w
				}
			}
		}

		// Import edges: a resolved import is a precise dependency claim.
		for _, imp := range f.Imports {
			targets := index.resolveImport(f.Path, imp)
			others := make([]int, 0, len(targets))
			for _, j := range targets {
				if j != i {
					others = append(others, j)
				}
			}
			if len(others) == 0 {
				continue
			}
			w := repomapImportWeight / float64(len(others))
			for _, j := range others {
				weights[j] += w
			}
		}

		targets := make([]int, 0, len(weights))
		for j := range weights {
			targets = append(targets, j)
		}
		sort.Ints(targets)
		for _, j := range targets {
			adj[i] = append(adj[i], edge{to: j, w: weights[j]})
		}
	}
	return adj
}

// personalization builds the normalized PageRank teleport vector: every file
// gets base weight 1; files matching a query term (case-insensitive path
// substring, or defining a symbol whose name equals the term) get
// repomapQueryBoost instead.
func personalization(facts []FileFacts, terms []string) []float64 {
	lower := make([]string, 0, len(terms))
	for _, t := range terms {
		if t = strings.ToLower(strings.TrimSpace(t)); t != "" {
			lower = append(lower, t)
		}
	}
	p := make([]float64, len(facts))
	var total float64
	for i, f := range facts {
		p[i] = 1
		if matchesQuery(f, lower) {
			p[i] = repomapQueryBoost
		}
		total += p[i]
	}
	for i := range p {
		p[i] /= total
	}
	return p
}

// matchesQuery reports whether the file matches any lowercase query term.
func matchesQuery(f FileFacts, lowerTerms []string) bool {
	if len(lowerTerms) == 0 {
		return false
	}
	lowerPath := strings.ToLower(f.Path)
	for _, t := range lowerTerms {
		if strings.Contains(lowerPath, t) {
			return true
		}
		for _, d := range f.Defs {
			if strings.ToLower(d.Name) == t {
				return true
			}
		}
	}
	return false
}

// pageRank runs the fixed-iteration personalized PageRank. Deterministic:
// nodes and edges are iterated in index order, so float summation order — and
// therefore every bit of the result — is a function of the inputs alone.
func pageRank(adj [][]edge, p []float64) []float64 {
	n := len(adj)
	if n == 0 {
		return nil
	}
	outWeight := make([]float64, n)
	for i, edges := range adj {
		for _, e := range edges {
			outWeight[i] += e.w
		}
	}

	rank := append([]float64(nil), p...)
	next := make([]float64, n)
	for it := 0; it < repomapIterations; it++ {
		var dangling float64
		for i := range next {
			next[i] = (1 - repomapDamping) * p[i]
		}
		for i, edges := range adj {
			if outWeight[i] == 0 {
				dangling += repomapDamping * rank[i]
				continue
			}
			flow := repomapDamping * rank[i] / outWeight[i]
			for _, e := range edges {
				next[e.to] += flow * e.w
			}
		}
		// Dangling mass teleports along the personalization vector.
		for i := range next {
			next[i] += dangling * p[i]
		}
		rank, next = next, rank
	}
	return rank
}

// entryOverheadChars approximates the JSON/list framing around one entry.
const entryOverheadChars = 8

// estimateTokens converts a character count to the ~4 chars/token estimate,
// rounding up.
func estimateTokens(chars int) int {
	t := (chars + 3) / 4
	if t < 1 {
		t = 1
	}
	return t
}

// renderEntry builds the RankedFile for one node, with up to
// repomapMaxDefsShown definition signatures in file order.
func renderEntry(f FileFacts, score float64) RankedFile {
	entry := RankedFile{Path: f.Path, Score: score}
	chars := len(f.Path) + entryOverheadChars
	for _, d := range f.Defs {
		if len(entry.Definitions) >= repomapMaxDefsShown {
			break
		}
		sig := d.Signature
		if sig == "" {
			sig = string(d.Kind) + " " + d.Name
		}
		entry.Definitions = append(entry.Definitions, sig)
		chars += len(sig) + entryOverheadChars
	}
	entry.Tokens = estimateTokens(chars)
	return entry
}

// --- fact collection over a snapshot ---

// identRE tokenizes file bodies into candidate reference identifiers.
var identRE = regexp.MustCompile(`[A-Za-z_$][A-Za-z0-9_$]{2,}`)

// CollectFileFacts walks the snapshot working tree and extracts FileFacts for
// every source file (repomapSourceExts, ≤ maxRepomapFileBytes, non-binary).
// Symbols come from mapengine — the shipped Tier-1 file map primitive stays
// the single symbol owner (ADR-0024.1 "Tier Boundaries"). Directories named
// ".git" (and other dot-dirs) are skipped; walk order is lexical, so the
// collected facts are deterministic for a given snapshot.
func CollectFileFacts(ctx context.Context, snapshotDir string) ([]FileFacts, error) {
	var facts []FileFacts
	err := filepath.WalkDir(snapshotDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if cerr := ctx.Err(); cerr != nil {
			return cerr
		}
		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") && p != snapshotDir {
				return fs.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() || !repomapSourceExts[strings.ToLower(filepath.Ext(p))] {
			return nil
		}
		if info, ierr := d.Info(); ierr != nil || info.Size() > maxRepomapFileBytes {
			return nil
		}
		content, rerr := os.ReadFile(p)
		if rerr != nil {
			return rerr
		}
		if strings.IndexByte(string(content), 0) >= 0 {
			return nil // binary masquerading as source
		}
		rel, rerr := filepath.Rel(snapshotDir, p)
		if rerr != nil {
			return rerr
		}
		facts = append(facts, fileFactsFor(filepath.ToSlash(rel), content))
		return nil
	})
	if err != nil {
		return nil, err
	}
	return facts, nil
}

// fileFactsFor extracts one file's facts from its content.
func fileFactsFor(relPath string, content []byte) FileFacts {
	f := FileFacts{Path: relPath, Refs: map[string]int{}}

	// mapengine may fall back internally (tree-sitter -> regex); whatever
	// symbols came back are the definition inventory. Errors leave the file
	// as a reference-only node rather than failing the ranking.
	result, _ := mapengine.Map(relPath, content, mapengine.Options{})
	for _, s := range result.Symbols {
		switch s.Kind {
		case mapengine.KindFunc, mapengine.KindType, mapengine.KindConst, mapengine.KindVar, mapengine.KindPackage:
		default:
			continue
		}
		// Grouped declarations surface placeholder names ("const", "var");
		// they are not referenceable symbols.
		if s.Name == "" || s.Name == "const" || s.Name == "var" || s.Name == "import" {
			continue
		}
		f.Defs = append(f.Defs, Definition{Name: s.Name, Signature: s.Signature, Kind: s.Kind})
	}

	for _, ident := range identRE.FindAllString(string(content), -1) {
		f.Refs[ident]++
	}
	f.Imports = extractImports(relPath, string(content))
	return f
}

// importLineRules extracts quoted import targets per language family. The
// extraction is a deliberate heuristic (documented in ADR-0024.1
// Implementation Notes): precise resolution is stack-graphs territory, which
// stays out of this build.
var importLineRules = []*regexp.Regexp{
	regexp.MustCompile(`^\s*import\b[^"'` + "`" + `]*["']([^"']+)["']`),        // Go single import, ES side-effect/from-less
	regexp.MustCompile(`^\s*(?:import|export)\b.*\bfrom\s+["']([^"']+)["']`),   // ES import/export ... from "x"
	regexp.MustCompile(`\brequire\(\s*["']([^"']+)["']\s*\)`),                  // CJS require("x")
}

// pyImportRE extracts python module paths from import/from statements.
var pyImportRE = regexp.MustCompile(`^\s*(?:from\s+([\w.]+)\s+import|import\s+([\w.]+))`)

// goBlockImportRE extracts one quoted path inside a Go `import (...)` block.
var goBlockImportRE = regexp.MustCompile(`^\s*(?:[\w.]+\s+)?"([^"]+)"`)

// extractImports scans content line by line for import targets. Go grouped
// import blocks get stateful handling; Python modules convert dots to
// slashes; other languages use the quoted-target line rules.
func extractImports(relPath, content string) []string {
	var imports []string
	isGo := strings.HasSuffix(relPath, ".go")
	isPy := strings.HasSuffix(relPath, ".py")
	inGoBlock := false
	for _, line := range strings.Split(content, "\n") {
		if isGo {
			trimmed := strings.TrimSpace(line)
			if inGoBlock {
				if trimmed == ")" {
					inGoBlock = false
					continue
				}
				if m := goBlockImportRE.FindStringSubmatch(line); m != nil {
					imports = append(imports, m[1])
				}
				continue
			}
			if strings.HasPrefix(trimmed, "import (") {
				inGoBlock = true
				continue
			}
		}
		if isPy {
			if m := pyImportRE.FindStringSubmatch(line); m != nil {
				mod := m[1]
				if mod == "" {
					mod = m[2]
				}
				imports = append(imports, strings.ReplaceAll(mod, ".", "/"))
			}
			continue
		}
		for _, re := range importLineRules {
			if m := re.FindStringSubmatch(line); m != nil {
				imports = append(imports, m[1])
				break
			}
		}
	}
	return imports
}

// resolveImportExts are the filename extensions tried when resolving a
// relative import that omits its extension.
var resolveImportExts = []string{".ts", ".tsx", ".js", ".jsx", ".go", ".py", ".rs"}

// fileIndex is the lookup structure for import resolution, built once per
// ranking pass.
type fileIndex struct {
	// byPath maps a snapshot-relative file path to its node index.
	byPath map[string]int
	// filesInDir maps a snapshot-relative directory to the node indices of
	// the files directly inside it (ascending).
	filesInDir map[string][]int
}

// newFileIndex builds the resolution index over the (path-sorted) facts.
func newFileIndex(facts []FileFacts) *fileIndex {
	idx := &fileIndex{byPath: make(map[string]int, len(facts)), filesInDir: map[string][]int{}}
	for i, f := range facts {
		idx.byPath[f.Path] = i
		dir := path.Dir(f.Path)
		idx.filesInDir[dir] = append(idx.filesInDir[dir], i)
	}
	return idx
}

// resolveImport maps one raw import target to node indices.
// Relative targets resolve against the importing file's directory (trying
// known extensions and index files); absolute/module targets resolve by the
// longest directory whose snapshot-relative path is a suffix of the import
// path, linking to the files directly in that directory.
func (idx *fileIndex) resolveImport(fromPath, imp string) []int {
	if strings.HasPrefix(imp, "./") || strings.HasPrefix(imp, "../") {
		base := path.Join(path.Dir(fromPath), imp)
		candidates := []string{base}
		for _, ext := range resolveImportExts {
			candidates = append(candidates, base+ext, path.Join(base, "index"+ext))
		}
		for _, c := range candidates {
			if i, ok := idx.byPath[c]; ok {
				return []int{i}
			}
		}
		return nil
	}

	// Module/package import: longest snapshot dir that suffix-matches.
	imp = strings.TrimSuffix(imp, "/")
	bestDir := ""
	for dir := range idx.filesInDir {
		if dir == "." {
			continue
		}
		if (imp == dir || strings.HasSuffix(imp, "/"+dir)) && len(dir) > len(bestDir) {
			bestDir = dir
		}
	}
	if bestDir == "" {
		return nil
	}
	return idx.filesInDir[bestDir]
}

// String renders one provenance-style summary line for logs, e.g.
// "repomap: 42 files ranked, 12 in budget (1024 tokens, 987 used)".
func (r RepomapResult) String() string {
	return fmt.Sprintf("repomap: %d files ranked, %d in budget (%d tokens, %d used)",
		r.TotalFiles, len(r.Files), r.BudgetTokens, r.UsedTokens)
}
