package mapengine

import (
	"strings"
	"testing"
)

func TestRegexMapperCompact(t *testing.T) {
	content := []byte(strings.Join([]string{
		"import React from 'react'",
		"export const App = () => {}",
		"type Props = { name: string }",
		"const local = 1",
	}, "\n"))

	result, err := RegexMapper{}.Map("app.tsx", content, Options{Level: LevelCompact})
	if err != nil {
		t.Fatalf("Map returned error: %v", err)
	}

	if result.Engine != EngineRegex {
		t.Fatalf("engine = %q, want %q", result.Engine, EngineRegex)
	}
	if got, want := len(result.Lines), 4; got != want {
		t.Fatalf("got %d lines, want %d: %#v", got, want, result.Lines)
	}
	if result.Symbols[1].Kind != KindFunc {
		t.Fatalf("symbol[1].Kind = %q, want %q", result.Symbols[1].Kind, KindFunc)
	}
	if result.Symbols[1].Name != "App" {
		t.Fatalf("symbol[1].Name = %q, want App", result.Symbols[1].Name)
	}
}

func TestRegexMapperKindFilter(t *testing.T) {
	content := []byte(strings.Join([]string{
		"package ghx",
		"import \"fmt\"",
		"type FileEntry struct {}",
		"func Explore(repo string) error { return nil }",
	}, "\n"))

	result, err := RegexMapper{}.Map("explore.go", content, Options{Kind: KindFunc})
	if err != nil {
		t.Fatalf("Map returned error: %v", err)
	}

	if got, want := len(result.Lines), 1; got != want {
		t.Fatalf("got %d lines, want %d: %#v", got, want, result.Lines)
	}
	if !strings.Contains(result.Lines[0], "func Explore") {
		t.Fatalf("line = %q, want func Explore", result.Lines[0])
	}
}

func TestRegexMapperMinimalLevel(t *testing.T) {
	result, err := RegexMapper{}.Map("explore.go", []byte("func Explore(repo string) error { return nil }\n"), Options{Level: LevelMinimal})
	if err != nil {
		t.Fatalf("Map returned error: %v", err)
	}

	if got, want := result.Lines[0], "1: Explore"; got != want {
		t.Fatalf("line = %q, want %q", got, want)
	}
}

func TestMapAutoUsesGoASTForGoFiles(t *testing.T) {
	result, err := Map("explore.go", []byte("package ghx\n\nfunc Explore() {}\n"), Options{})
	if err != nil {
		t.Fatalf("Map returned error: %v", err)
	}

	if result.Engine != EngineGoAST {
		t.Fatalf("engine = %q, want go-ast", result.Engine)
	}
	if len(result.Lines) == 0 || !strings.Contains(strings.Join(result.Lines, "\n"), "func Explore") {
		t.Fatalf("lines = %#v, want func Explore", result.Lines)
	}
}

func TestGoASTMapperTopLevelOnly(t *testing.T) {
	content := []byte(strings.Join([]string{
		"package ghx",
		"",
		`import "fmt"`,
		"",
		"type FileEntry struct {",
		"  Name string",
		"}",
		"",
		"func Explore(repo string) error {",
		"  local := fmt.Sprintf(repo)",
		"  _ = local",
		"  return nil",
		"}",
	}, "\n"))

	result, err := GoASTMapper{}.Map("explore.go", content, Options{})
	if err != nil {
		t.Fatalf("Map returned error: %v", err)
	}

	if result.Engine != EngineGoAST {
		t.Fatalf("engine = %q, want go-ast", result.Engine)
	}

	got := strings.Join(result.Lines, "\n")

	// must have top-level declarations
	if !strings.Contains(got, "package ghx") {
		t.Errorf("missing package: %s", got)
	}
	if !strings.Contains(got, "import") {
		t.Errorf("missing import: %s", got)
	}
	if !strings.Contains(got, "type FileEntry struct {") {
		t.Errorf("missing type: %s", got)
	}
	if !strings.Contains(got, "func Explore") {
		t.Errorf("missing func: %s", got)
	}

	// must NOT have local variables
	if strings.Contains(got, "local") {
		t.Errorf("local variable leaked into map output: %s", got)
	}
}

func TestGoASTMapperGenericType(t *testing.T) {
	content := []byte("package ghx\n\ntype Result[T any] struct {\n\tValue T\n}\n")

	result, err := GoASTMapper{}.Map("result.go", content, Options{Kind: KindType})
	if err != nil {
		t.Fatalf("Map returned error: %v", err)
	}
	if len(result.Lines) != 1 {
		t.Fatalf("got %d lines, want 1: %#v", len(result.Lines), result.Lines)
	}
	if !strings.Contains(result.Lines[0], "[T any]") {
		t.Errorf("generic type params dropped from signature: %s", result.Lines[0])
	}
}

func TestGoASTMapperMultiLineSignature(t *testing.T) {
	content := []byte(strings.Join([]string{
		"package ghx",
		"",
		"func Read(",
		"  repo string,",
		"  files []string,",
		"  opts *ReadOpts,",
		") ([]FileResult, error) {",
		"  return nil, nil",
		"}",
	}, "\n"))

	result, err := GoASTMapper{}.Map("read.go", content, Options{})
	if err != nil {
		t.Fatalf("Map returned error: %v", err)
	}

	got := strings.Join(result.Lines, "\n")
	// signature should be compacted onto one line and include the full parameter list
	if !strings.Contains(got, "repo string") {
		t.Errorf("multi-line signature not compacted: %s", got)
	}
	if !strings.Contains(got, "[]FileResult, error") {
		t.Errorf("return type missing from signature: %s", got)
	}
}

func TestTreeSitterMapperCapturesTypeScriptClassMethod(t *testing.T) {
	content := []byte(strings.Join([]string{
		"export class App {",
		"  async render(name: string): Promise<string> {",
		"    return name",
		"  }",
		"}",
	}, "\n"))

	result, err := Map("app.ts", content, Options{Engine: EngineTreeSitter, Kind: KindFunc})
	if err != nil {
		t.Fatalf("Map returned error: %v", err)
	}

	if result.Engine != EngineTreeSitter {
		t.Fatalf("engine = %q, want tree-sitter", result.Engine)
	}
	if got := strings.Join(result.Lines, "\n"); !strings.Contains(got, "render") {
		t.Fatalf("lines = %#v, want render method", result.Lines)
	}
}

func TestTreeSitterMapperKindFilter(t *testing.T) {
	content := []byte(strings.Join([]string{
		"package ghx",
		"type FileEntry struct {",
		"  Name string",
		"}",
		"func Explore() {}",
	}, "\n"))

	result, err := Map("explore.go", content, Options{Engine: EngineTreeSitter, Kind: KindType})
	if err != nil {
		t.Fatalf("Map returned error: %v", err)
	}

	if got, want := len(result.Lines), 1; got != want {
		t.Fatalf("got %d lines, want %d: %#v", got, want, result.Lines)
	}
	if !strings.Contains(result.Lines[0], "FileEntry") {
		t.Fatalf("line = %q, want FileEntry", result.Lines[0])
	}
}

func TestTreeSitterLanguageConfigControlsRegexMerge(t *testing.T) {
	content := []byte(strings.Join([]string{
		"package ghx",
		"import \"fmt\"",
		"func Explore() {}",
	}, "\n"))

	mapper := TreeSitterMapper{
		Languages: map[string]TreeSitterLanguageConfig{
			"go": {
				MergeRegex: false,
			},
		},
	}

	result, err := mapper.Map("explore.go", content, Options{})
	if err != nil {
		t.Fatalf("Map returned error: %v", err)
	}

	got := strings.Join(result.Lines, "\n")
	if strings.Contains(got, "package ghx") || strings.Contains(got, "import") {
		t.Fatalf("regex-only package/import symbols leaked into tree-sitter result: %#v", result.Lines)
	}
	if !strings.Contains(got, "func Explore") {
		t.Fatalf("lines = %#v, want func Explore", result.Lines)
	}
}

func TestTreeSitterLanguageConfigControlsTagKinds(t *testing.T) {
	mapper := TreeSitterMapper{
		Languages: map[string]TreeSitterLanguageConfig{
			"go": {
				MergeRegex: false,
				TagKinds: map[string]Kind{
					"definition.function": KindOther,
				},
			},
		},
	}

	result, err := mapper.Map("explore.go", []byte("package ghx\n\nfunc Explore() {}\n"), Options{Kind: KindOther})
	if err != nil {
		t.Fatalf("Map returned error: %v", err)
	}

	if got, want := len(result.Lines), 1; got != want {
		t.Fatalf("got %d lines, want %d: %#v", got, want, result.Lines)
	}
	if result.Symbols[0].Kind != KindOther {
		t.Fatalf("kind = %q, want %q", result.Symbols[0].Kind, KindOther)
	}
}

// TestMapAutoUsesTreeSitterForTS proves that EngineAuto selects tree-sitter
// (not regex) for TypeScript files, and that the result.Engine field reflects it.
func TestMapAutoUsesTreeSitterForTS(t *testing.T) {
	content := []byte("export class App {\n  render(): string { return '' }\n}\n")

	result, err := Map("app.ts", content, Options{})
	if err != nil {
		t.Fatalf("Map returned error: %v", err)
	}
	if result.Engine != EngineTreeSitter {
		t.Fatalf("engine = %q, want tree-sitter for .ts files", result.Engine)
	}
	if result.Fallback {
		t.Fatalf("tree-sitter fell back to regex (warnings: %v)", result.Warnings)
	}
}

// TestMapAutoUsesTreeSitterForPython proves the same for Python files.
func TestMapAutoUsesTreeSitterForPython(t *testing.T) {
	content := []byte("class Foo:\n    def bar(self):\n        pass\n")

	result, err := Map("foo.py", content, Options{})
	if err != nil {
		t.Fatalf("Map returned error: %v", err)
	}
	if result.Engine != EngineTreeSitter {
		t.Fatalf("engine = %q, want tree-sitter for .py files", result.Engine)
	}
	if result.Fallback {
		t.Fatalf("tree-sitter fell back to regex (warnings: %v)", result.Warnings)
	}
}

// TestTreeSitterBetterThanRegexForClassMethods is the quality proof:
// regex is structurally blind to methods inside class bodies.
// tree-sitter captures them because it parses the AST, not lines.
//
// This test runs identical TypeScript through both engines and asserts
// tree-sitter finds the class methods while regex does not.
func TestTreeSitterBetterThanRegexForClassMethods(t *testing.T) {
	content := []byte(strings.Join([]string{
		"export class UserService {",
		"  async getUser(id: string): Promise<User> {",
		"    return fetch(id)",
		"  }",
		"  async createUser(data: CreateUserDto): Promise<User> {",
		"    return post(data)",
		"  }",
		"}",
	}, "\n"))

	tsResult, err := TreeSitterMapper{}.Map("user.service.ts", content, Options{Kind: KindFunc})
	if err != nil {
		t.Fatalf("tree-sitter Map error: %v", err)
	}
	rxResult, err := RegexMapper{}.Map("user.service.ts", content, Options{Kind: KindFunc})
	if err != nil {
		t.Fatalf("regex Map error: %v", err)
	}

	tsGot := strings.Join(tsResult.Lines, "\n")
	rxGot := strings.Join(rxResult.Lines, "\n")

	// tree-sitter must find both methods
	if !strings.Contains(tsGot, "getUser") {
		t.Errorf("tree-sitter missed getUser; lines: %#v", tsResult.Lines)
	}
	if !strings.Contains(tsGot, "createUser") {
		t.Errorf("tree-sitter missed createUser; lines: %#v", tsResult.Lines)
	}

	// regex must miss both (they are inside a class body, not at top-level function syntax)
	if strings.Contains(rxGot, "getUser") || strings.Contains(rxGot, "createUser") {
		t.Errorf("regex unexpectedly found class methods — proof no longer valid; lines: %#v", rxResult.Lines)
	}

	// tree-sitter should have more symbols than regex for this content
	if len(tsResult.Symbols) <= len(rxResult.Symbols) {
		t.Errorf("tree-sitter found %d symbols, regex found %d — expected tree-sitter > regex",
			len(tsResult.Symbols), len(rxResult.Symbols))
	}
}

// TestTreeSitterEngineFallsBackToRegex ensures unsupported file types
// degrade gracefully rather than returning an error.
func TestGoASTReceiverMethodPopulatesParent(t *testing.T) {
	content := []byte(strings.Join([]string{
		"package svc",
		"",
		"type UserService struct{}",
		"",
		"func (s *UserService) GetUser(id string) error { return nil }",
		"func Standalone() {}",
	}, "\n"))

	result, err := GoASTMapper{}.Map("svc.go", content, Options{Kind: KindFunc})
	if err != nil {
		t.Fatalf("Map returned error: %v", err)
	}

	var getUser, standalone *Symbol
	for i := range result.Symbols {
		switch result.Symbols[i].Name {
		case "GetUser":
			getUser = &result.Symbols[i]
		case "Standalone":
			standalone = &result.Symbols[i]
		}
	}

	if getUser == nil {
		t.Fatal("GetUser symbol not found")
	}
	if getUser.Parent != "UserService" {
		t.Errorf("GetUser.Parent = %q, want UserService", getUser.Parent)
	}
	if standalone == nil {
		t.Fatal("Standalone symbol not found")
	}
	if standalone.Parent != "" {
		t.Errorf("Standalone.Parent = %q, want empty", standalone.Parent)
	}
}

func TestGoASTParentAppearsInMinimalLevel(t *testing.T) {
	content := []byte("package svc\n\ntype S struct{}\n\nfunc (s *S) Run() {}\n")

	result, err := GoASTMapper{}.Map("svc.go", content, Options{Level: LevelMinimal, Kind: KindFunc})
	if err != nil {
		t.Fatalf("Map returned error: %v", err)
	}
	if len(result.Lines) != 1 {
		t.Fatalf("got %d lines, want 1: %v", len(result.Lines), result.Lines)
	}
	if !strings.Contains(result.Lines[0], "S.Run") {
		t.Errorf("minimal output = %q, want S.Run", result.Lines[0])
	}
}

func TestTreeSitterClassMethodPopulatesParent(t *testing.T) {
	content := []byte(strings.Join([]string{
		"export class UserService {",
		"  async getUser(id: string): Promise<string> {",
		"    return id",
		"  }",
		"}",
		"",
		"function standalone(): void {}",
	}, "\n"))

	result, err := TreeSitterMapper{}.Map("svc.ts", content, Options{Kind: KindFunc})
	if err != nil {
		t.Fatalf("Map returned error: %v", err)
	}

	var getUser, standalone *Symbol
	for i := range result.Symbols {
		switch result.Symbols[i].Name {
		case "getUser":
			getUser = &result.Symbols[i]
		case "standalone":
			standalone = &result.Symbols[i]
		}
	}

	if getUser == nil {
		t.Fatal("getUser symbol not found")
	}
	if getUser.Parent != "UserService" {
		t.Errorf("getUser.Parent = %q, want UserService", getUser.Parent)
	}
	if standalone != nil && standalone.Parent != "" {
		t.Errorf("standalone.Parent = %q, want empty", standalone.Parent)
	}
}

func TestMapAutoUsesTreeSitterForRust(t *testing.T) {
	content := []byte(strings.Join([]string{
		"use std::fmt;",
		"",
		"pub struct Point { pub x: f64, pub y: f64 }",
		"",
		"pub fn distance(a: &Point, b: &Point) -> f64 {",
		"    0.0",
		"}",
	}, "\n"))

	result, err := Map("geo.rs", content, Options{})
	if err != nil {
		t.Fatalf("Map returned error: %v", err)
	}
	if result.Engine != EngineTreeSitter {
		t.Fatalf("engine = %q, want tree-sitter for .rs files", result.Engine)
	}
	if result.Fallback {
		t.Fatalf("tree-sitter fell back to regex (warnings: %v)", result.Warnings)
	}
	got := strings.Join(result.Lines, "\n")
	if !strings.Contains(got, "distance") {
		t.Errorf("function not found in map output: %s", got)
	}
}

func TestTreeSitterEngineFallsBackToRegex(t *testing.T) {
	result, err := Map("README.md", []byte("# readme\n"), Options{Engine: EngineTreeSitter})
	if err != nil {
		t.Fatalf("Map returned error: %v", err)
	}

	if result.Engine != EngineRegex {
		t.Fatalf("engine = %q, want regex fallback", result.Engine)
	}
	if !result.Fallback {
		t.Fatal("fallback flag is false")
	}
	if len(result.Warnings) == 0 {
		t.Fatal("expected fallback warning")
	}
}
