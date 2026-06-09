package ghx

import "testing"

func TestParseFileResponse_Directory(t *testing.T) {
	data := map[string]interface{}{
		"entries": []interface{}{
			map[string]interface{}{"name": "index.ts", "type": "blob"},
			map[string]interface{}{"name": "utils", "type": "tree"},
		},
	}
	r := parseFileResponse("src/lib", data, "", &ReadOpts{})
	if r.NotFound {
		t.Fatal("directory reported as not found")
	}
	if len(r.DirEntries) != 2 {
		t.Fatalf("got %d dir entries, want 2", len(r.DirEntries))
	}
	if r.DirEntries[0].Name != "index.ts" || r.DirEntries[0].Type != "blob" {
		t.Errorf("entry[0] = %+v, want {index.ts blob}", r.DirEntries[0])
	}
	if r.DirEntries[1].Name != "utils" || r.DirEntries[1].Type != "tree" {
		t.Errorf("entry[1] = %+v, want {utils tree}", r.DirEntries[1])
	}
}

func TestParseFileResponse_EmptyDirectory(t *testing.T) {
	data := map[string]interface{}{
		"entries": []interface{}{},
	}
	r := parseFileResponse("empty-dir", data, "", &ReadOpts{})
	if r.NotFound {
		t.Fatal("empty directory reported as not found")
	}
	if r.DirEntries == nil {
		t.Fatal("DirEntries is nil for empty directory")
	}
	if len(r.DirEntries) != 0 {
		t.Fatalf("got %d entries, want 0", len(r.DirEntries))
	}
}

func TestParseFileResponse_Blob(t *testing.T) {
	data := map[string]interface{}{
		"text":     "hello world\n",
		"byteSize": float64(12),
	}
	r := parseFileResponse("file.txt", data, "", &ReadOpts{})
	if r.NotFound {
		t.Fatal("file reported as not found")
	}
	if r.Content != "hello world\n" {
		t.Errorf("content = %q, want %q", r.Content, "hello world\n")
	}
	if r.ByteSize != 12 {
		t.Errorf("byteSize = %d, want 12", r.ByteSize)
	}
}

func TestParseFileResponse_BlobWithGrep(t *testing.T) {
	data := map[string]interface{}{
		"text":     "line one\nfoo bar\nline three\n",
		"byteSize": float64(28),
	}
	r := parseFileResponse("file.txt", data, "", &ReadOpts{Grep: "foo"})
	if r.NotFound {
		t.Fatal("file reported as not found")
	}
	if len(r.GrepHits) == 0 {
		t.Fatal("no grep hits")
	}
	found := false
	for _, h := range r.GrepHits {
		if h.IsMatch && h.Line == "foo bar" {
			found = true
		}
	}
	if !found {
		t.Error("expected matching line 'foo bar' not found in grep hits")
	}
}

func TestParseFileResponse_BlobWithMap(t *testing.T) {
	data := map[string]interface{}{
		"text":     "import React from 'react'\nexport const App = () => {}\nconst x = 1\n",
		"byteSize": float64(70),
	}
	r := parseFileResponse("app.tsx", data, "", &ReadOpts{Map: true})
	if r.NotFound {
		t.Fatal("file reported as not found")
	}
	if len(r.MapLines) == 0 {
		t.Fatal("no map lines")
	}
	if r.MapChars == 0 {
		t.Error("mapChars should be > 0")
	}
	if r.MapEngine != "tree-sitter" {
		t.Errorf("mapEngine = %q, want tree-sitter", r.MapEngine)
	}
}

func TestParseFileResponse_BlobWithMapKind(t *testing.T) {
	data := map[string]interface{}{
		"text":     "package ghx\nimport \"fmt\"\ntype FileEntry struct {}\nfunc Explore() {}\n",
		"byteSize": float64(70),
	}
	r := parseFileResponse("explore.go", data, "", &ReadOpts{Map: true, MapKind: "func"})
	if r.NotFound {
		t.Fatal("file reported as not found")
	}
	if len(r.MapLines) != 1 {
		t.Fatalf("got %d map lines, want 1: %#v", len(r.MapLines), r.MapLines)
	}
	if r.MapLines[0] != "4: func Explore() {" {
		t.Errorf("map line = %q, want function only", r.MapLines[0])
	}
}

func TestParseFileResponse_BlobWithTreeSitterFallback(t *testing.T) {
	data := map[string]interface{}{
		"text":     "# readme\n",
		"byteSize": float64(18),
	}
	r := parseFileResponse("README.md", data, "", &ReadOpts{Map: true, MapEngine: "tree-sitter"})
	if r.MapEngine != "regex" {
		t.Errorf("mapEngine = %q, want regex fallback", r.MapEngine)
	}
	if len(r.MapWarnings) == 0 {
		t.Fatal("expected fallback warning")
	}
}

func TestParseFileResponse_BlobWithLines(t *testing.T) {
	data := map[string]interface{}{
		"text":     "line1\nline2\nline3\nline4\nline5\n",
		"byteSize": float64(30),
	}
	r := parseFileResponse("file.txt", data, "", &ReadOpts{Lines: "2-4"})
	if r.Content != "line2\nline3\nline4" {
		t.Errorf("content = %q, want %q", r.Content, "line2\nline3\nline4")
	}
}

func TestParseFileResponse_BlobWithGlobPattern(t *testing.T) {
	data := map[string]interface{}{
		"text":     "content\n",
		"byteSize": float64(8),
	}
	r := parseFileResponse("src/index.ts", data, "src/*.ts", &ReadOpts{})
	if r.GlobPattern != "src/*.ts" {
		t.Errorf("globPattern = %q, want %q", r.GlobPattern, "src/*.ts")
	}
}

func TestParseFileResponse_NilData(t *testing.T) {
	r := parseFileResponse("missing.txt", "not a map", "", &ReadOpts{})
	if !r.NotFound {
		t.Error("expected NotFound for non-map data")
	}
}

func TestParseFileResponse_EmptyText(t *testing.T) {
	data := map[string]interface{}{
		"text":     "",
		"byteSize": float64(0),
	}
	r := parseFileResponse("empty.txt", data, "", &ReadOpts{})
	if !r.NotFound {
		t.Error("expected NotFound for empty text")
	}
}

func TestNormalizeBRE(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"BRE pipe", `ref\|defs`, "ref|defs"},
		{"BRE parens", `\(group\)`, "(group)"},
		{"BRE braces", `a\{2,3\}`, "a{2,3}"},
		{"BRE plus", `a\+`, "a+"},
		{"BRE question", `a\?`, "a?"},
		{"escaped backslash then pipe", `\\|defs`, `\\|defs`},
		{"double escaped backslash", `\\\\`, `\\\\`},
		{"ERE pipe unchanged", "ref|defs", "ref|defs"},
		{"plain text unchanged", "hello", "hello"},
		{"other escapes pass through", `\n\d\w`, `\n\d\w`},
		{"trailing backslash", `abc\`, `abc\`},
		{"mixed BRE and ERE", `\(a|b\)`, "(a|b)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeBRE(tt.in)
			if got != tt.want {
				t.Errorf("normalizeBRE(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestGrepLines_RegexAlternation(t *testing.T) {
	text := "const ref = 1\nlet defs = {}\nvar definition = true\nlet globalDef = null\nlet unrelated = false"

	tests := []struct {
		name    string
		pattern string
		want    []string // expected matching lines (IsMatch == true)
	}{
		{"RE2 pipe", "ref|defs|definition|globalDef", []string{
			"const ref = 1", "let defs = {}", "var definition = true", "let globalDef = null",
		}},
		{"BRE backslash-pipe", `ref\|defs\|definition\|globalDef`, []string{
			"const ref = 1", "let defs = {}", "var definition = true", "let globalDef = null",
		}},
		{"simple substring", "unrelated", []string{"let unrelated = false"}},
		{"character class", "[gG]lobal", []string{"let globalDef = null"}},
		{"dot wildcard", "def.", []string{"let defs = {}", "var definition = true", "let globalDef = null"}},
		{"case insensitive", "CONST", []string{"const ref = 1"}},
		{"invalid regex falls back to literal", "[unclosed", []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches := grepLines(text, tt.pattern)
			var got []string
			for _, m := range matches {
				if m.IsMatch {
					got = append(got, m.Line)
				}
			}
			if len(got) != len(tt.want) {
				t.Errorf("got %d matches %v, want %d matches %v", len(got), got, len(tt.want), tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("match[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}
