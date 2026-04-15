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

func TestMapAutoUsesTreeSitterForGo(t *testing.T) {
	result, err := Map("explore.go", []byte("package ghx\n\nfunc Explore() {}\n"), Options{})
	if err != nil {
		t.Fatalf("Map returned error: %v", err)
	}

	if result.Engine != EngineTreeSitter {
		t.Fatalf("engine = %q, want tree-sitter", result.Engine)
	}
	if len(result.Lines) == 0 || !strings.Contains(strings.Join(result.Lines, "\n"), "func Explore") {
		t.Fatalf("lines = %#v, want func Explore", result.Lines)
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
