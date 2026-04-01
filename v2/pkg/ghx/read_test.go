package ghx

import "testing"

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
