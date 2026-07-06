package tier2

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gkoreli/ghx/v2/internal/mapengine"
)

// rankingFixture is a hand-checkable four-file repo shape:
//
//	lib/core.go   defines Compose — referenced by app.go (x3) and handlers.go (x1)
//	lib/util.go   defines Padding — referenced by app.go (x1)
//	app.go        imports "example.com/proj/lib" (edges into both lib files)
//	handlers.go   references Compose once, defines nothing referenced
//
// Expected order: lib/core.go collects the most reference weight plus import
// flow, lib/util.go collects less of both, and the two leaf files trail.
func rankingFixture() []FileFacts {
	return []FileFacts{
		{
			Path: "lib/core.go",
			Defs: []Definition{{Name: "Compose", Signature: "func Compose(parts ...string) string", Kind: mapengine.KindFunc}},
			Refs: map[string]int{"strings": 1},
		},
		{
			Path: "lib/util.go",
			Defs: []Definition{{Name: "Padding", Signature: "const Padding = 2", Kind: mapengine.KindConst}},
			Refs: map[string]int{},
		},
		{
			Path:    "app.go",
			Defs:    []Definition{{Name: "main", Signature: "func main()", Kind: mapengine.KindFunc}},
			Refs:    map[string]int{"Compose": 3, "Padding": 1},
			Imports: []string{"example.com/proj/lib"},
		},
		{
			Path: "handlers.go",
			Defs: []Definition{{Name: "handle", Signature: "func handle()", Kind: mapengine.KindFunc}},
			Refs: map[string]int{"Compose": 1},
		},
	}
}

func rankedPaths(r RepomapResult) []string {
	paths := make([]string, len(r.Files))
	for i, f := range r.Files {
		paths[i] = f.Path
	}
	return paths
}

func TestRankFilesHandCheckableOrder(t *testing.T) {
	result := RankFiles(rankingFixture(), RepomapRequest{})
	want := []string{"lib/core.go", "lib/util.go"}
	got := rankedPaths(result)
	if len(got) < 2 || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("ranking = %v, want %v first", got, want)
	}
	if result.TotalFiles != 4 || len(result.Files) != 4 {
		t.Errorf("all four files fit the default budget: %+v", result)
	}
	if result.Files[0].Score <= result.Files[1].Score {
		t.Errorf("core %.9f must outrank util %.9f", result.Files[0].Score, result.Files[1].Score)
	}
	if result.Backend != BackendRepomap {
		t.Errorf("backend = %q", result.Backend)
	}
	if len(result.Files[0].Definitions) == 0 || !strings.Contains(result.Files[0].Definitions[0], "Compose") {
		t.Errorf("top entry must show its definitions: %+v", result.Files[0])
	}
}

// TestRankFilesDeterministic proves the pure-function contract: shuffled
// input order and repeated runs produce byte-identical results.
func TestRankFilesDeterministic(t *testing.T) {
	req := RepomapRequest{QueryTerms: []string{"compose"}, BudgetTokens: 200}
	base := RankFiles(rankingFixture(), req)

	reversed := rankingFixture()
	for i, j := 0, len(reversed)-1; i < j; i, j = i+1, j-1 {
		reversed[i], reversed[j] = reversed[j], reversed[i]
	}
	again := RankFiles(reversed, req)

	baseJSON, _ := json.Marshal(base)
	againJSON, _ := json.Marshal(again)
	if string(baseJSON) != string(againJSON) {
		t.Errorf("input order changed the result:\n%s\nvs\n%s", baseJSON, againJSON)
	}
	third, _ := json.Marshal(RankFiles(rankingFixture(), req))
	if string(baseJSON) != string(third) {
		t.Errorf("repeated run changed the result")
	}
}

func TestRankFilesQueryBoost(t *testing.T) {
	// Unqueried, handlers.go is not the top file...
	plain := RankFiles(rankingFixture(), RepomapRequest{})
	if rankedPaths(plain)[0] == "handlers.go" {
		t.Fatal("fixture assumption broken: handlers.go should not lead unqueried")
	}
	// ...but a query matching its path (and its defined symbol) pulls it up.
	boosted := RankFiles(rankingFixture(), RepomapRequest{QueryTerms: []string{"handlers"}})
	posPlain := indexOf(rankedPaths(plain), "handlers.go")
	posBoosted := indexOf(rankedPaths(boosted), "handlers.go")
	if posBoosted >= posPlain {
		t.Errorf("query boost did not raise handlers.go: pos %d -> %d\nplain=%v\nboosted=%v",
			posPlain, posBoosted, rankedPaths(plain), rankedPaths(boosted))
	}
	// Symbol-name matching works too (term equals a defined name).
	symbol := RankFiles(rankingFixture(), RepomapRequest{QueryTerms: []string{"Padding"}})
	if indexOf(rankedPaths(symbol), "lib/util.go") >= indexOf(rankedPaths(plain), "lib/util.go") &&
		rankedPaths(symbol)[0] != "lib/util.go" {
		t.Errorf("symbol query did not favor lib/util.go: %v", rankedPaths(symbol))
	}
}

func indexOf(paths []string, want string) int {
	for i, p := range paths {
		if p == want {
			return i
		}
	}
	return len(paths)
}

func TestRankFilesBudgetDegradesThenStops(t *testing.T) {
	full := RankFiles(rankingFixture(), RepomapRequest{})
	if full.UsedTokens > full.BudgetTokens {
		t.Fatalf("used %d exceeds budget %d", full.UsedTokens, full.BudgetTokens)
	}

	// A budget below the first full entry degrades it to path-only.
	tiny := RankFiles(rankingFixture(), RepomapRequest{BudgetTokens: 6})
	if len(tiny.Files) == 0 {
		t.Fatal("a path-only entry fits 6 tokens")
	}
	if tiny.Files[0].Definitions != nil {
		t.Errorf("degraded entry must drop definitions: %+v", tiny.Files[0])
	}
	if tiny.UsedTokens > tiny.BudgetTokens {
		t.Errorf("used %d exceeds budget %d", tiny.UsedTokens, tiny.BudgetTokens)
	}
	if tiny.TotalFiles != 4 {
		t.Errorf("TotalFiles must count ranked candidates before the cut: %d", tiny.TotalFiles)
	}

	// Budget too small even for one path: empty output, still deterministic.
	none := RankFiles(rankingFixture(), RepomapRequest{BudgetTokens: 1})
	if len(none.Files) != 0 || none.UsedTokens != 0 {
		t.Errorf("1-token budget must fit nothing: %+v", none)
	}
}

func TestRepomapRequestArtifactKey(t *testing.T) {
	if got := (RepomapRequest{}).ArtifactKey(); got != "repomap:1024" {
		t.Errorf("default key = %q, want repomap:1024", got)
	}
	q := RepomapRequest{QueryTerms: []string{"route"}, BudgetTokens: 512}
	key := q.ArtifactKey()
	if !strings.HasPrefix(key, "repomap:512:") || len(key) > 32 {
		t.Errorf("query key = %q, want repomap:512:<digest12>", key)
	}
	if key != q.ArtifactKey() {
		t.Error("ArtifactKey must be deterministic")
	}
}

func TestExtractImports(t *testing.T) {
	goSrc := "package a\n\nimport \"fmt\"\n\nimport (\n\t\"strings\"\n\tme \"example.com/proj/lib\"\n)\n"
	if got := extractImports("a.go", goSrc); !reflect.DeepEqual(got, []string{"fmt", "strings", "example.com/proj/lib"}) {
		t.Errorf("go imports = %v", got)
	}
	tsSrc := "import { compose } from './lib/compose'\nimport 'reflect-metadata'\nconst x = require('node:path')\n"
	if got := extractImports("a.ts", tsSrc); !reflect.DeepEqual(got, []string{"./lib/compose", "reflect-metadata", "node:path"}) {
		t.Errorf("ts imports = %v", got)
	}
	pySrc := "from pkg.sub import thing\nimport os.path\n"
	if got := extractImports("a.py", pySrc); !reflect.DeepEqual(got, []string{"pkg/sub", "os/path"}) {
		t.Errorf("py imports = %v", got)
	}
}

func TestResolveImport(t *testing.T) {
	facts := []FileFacts{
		{Path: "app.ts"},
		{Path: "lib/compose.ts"},
		{Path: "lib/core.go"},
		{Path: "lib/util.go"},
	}
	idx := newFileIndex(facts)

	if got := idx.resolveImport("app.ts", "./lib/compose"); !reflect.DeepEqual(got, []int{1}) {
		t.Errorf("relative resolve = %v, want [1]", got)
	}
	// Module import suffix-matches the lib dir: both Go files, not the .ts.
	if got := idx.resolveImport("app.ts", "example.com/proj/lib"); !reflect.DeepEqual(got, []int{1, 2, 3}) {
		t.Errorf("dir resolve = %v, want [1 2 3] (files directly in lib/)", got)
	}
	if got := idx.resolveImport("app.ts", "fmt"); got != nil {
		t.Errorf("unresolvable stdlib import = %v, want nil", got)
	}
}

func TestCollectFileFactsFromSnapshotTree(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "main.go", "package main\n\nimport \"example.com/x/src\"\n\nfunc main() { src.Lib() }\n")
	writeFile(t, dir, filepath.Join("src", "lib.go"), "package src\n\nfunc Lib() int { return 1 }\n")
	writeFile(t, dir, "notes.md", "# not source\n")
	writeFile(t, dir, filepath.Join(".git", "config.go"), "package hidden\n")
	if err := os.WriteFile(filepath.Join(dir, "blob.go"), []byte("package b\x00inary"), 0o644); err != nil {
		t.Fatal(err)
	}

	facts, err := CollectFileFacts(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	byPath := map[string]FileFacts{}
	for _, f := range facts {
		paths = append(paths, f.Path)
		byPath[f.Path] = f
	}
	if !reflect.DeepEqual(paths, []string{"main.go", "src/lib.go"}) {
		t.Fatalf("collected = %v, want [main.go src/lib.go] (md, .git, binary skipped)", paths)
	}
	lib := byPath["src/lib.go"]
	if !hasDef(lib, "Lib") || !hasDef(lib, "src") {
		t.Errorf("lib defs = %+v, want func Lib and package src", lib.Defs)
	}
	main := byPath["main.go"]
	if !reflect.DeepEqual(main.Imports, []string{"example.com/x/src"}) {
		t.Errorf("main imports = %v", main.Imports)
	}
	if main.Refs["Lib"] == 0 {
		t.Errorf("main must reference Lib: %v", main.Refs)
	}
}

func hasDef(f FileFacts, name string) bool {
	for _, d := range f.Defs {
		if d.Name == name {
			return true
		}
	}
	return false
}

// TestServiceRunRepomapOverFixtureSnapshot is the end-to-end path: fixture
// repo -> snapshot -> collected facts -> deterministic ranking -> artifact
// hash in the snapshot ledger. main.go imports nothing local and src/lib.go
// defines Lib; the ranking must include both, and repeated runs must hash
// identically.
func TestServiceRunRepomapOverFixtureSnapshot(t *testing.T) {
	url, _, _, _ := initFixtureRepo(t)
	svc := newTestService(t)
	snap, err := svc.Snapshot(context.Background(), SnapshotRequest{Repo: "fixture/repo", RemoteURL: url})
	if err != nil {
		t.Fatal(err)
	}
	req := RepomapRequest{QueryTerms: []string{"lib"}}
	result, err := svc.RunRepomap(context.Background(), snap, req)
	if err != nil {
		t.Fatal(err)
	}
	if result.TotalFiles != 2 {
		t.Fatalf("TotalFiles = %d, want 2 (main.go, src/lib.go): %+v", result.TotalFiles, result)
	}
	// The query term "lib" matches src/lib.go by path: it must lead.
	if got := rankedPaths(result); got[0] != "src/lib.go" {
		t.Errorf("ranking = %v, want src/lib.go first under query 'lib'", got)
	}

	meta, err := LoadMetadata(snap.Dir)
	if err != nil {
		t.Fatal(err)
	}
	hash1, ok := meta.ToolArtifactHashes[req.ArtifactKey()]
	if !ok || hash1 == "" {
		t.Fatalf("artifact hash missing under %q: %v", req.ArtifactKey(), meta.ToolArtifactHashes)
	}

	// Determinism end to end: rerun, same hash.
	if _, err := svc.RunRepomap(context.Background(), snap, req); err != nil {
		t.Fatal(err)
	}
	meta2, _ := LoadMetadata(snap.Dir)
	if meta2.ToolArtifactHashes[req.ArtifactKey()] != hash1 {
		t.Errorf("repeated ranking changed the artifact hash: %q vs %q", meta2.ToolArtifactHashes[req.ArtifactKey()], hash1)
	}
}
