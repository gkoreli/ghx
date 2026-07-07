package ghx

import (
	"strings"
	"testing"
)

func TestIsGlob(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"src/main.go", false},
		{"src/*.go", true},
		{"src/**/*.ts", true},
		{"src/[abc].go", true},
		{"src/{a,b}.go", true},
		{"src/file?.go", true},
		{"path/to/exact/file.ts", false},
	}
	for _, tt := range tests {
		if got := isGlob(tt.path); got != tt.want {
			t.Errorf("isGlob(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestExpandGlobs(t *testing.T) {
	tree := []treeEntry{
		{"src/main.go", "blob"},
		{"src/util.go", "blob"},
		{"src/lib/helper.go", "blob"},
		{"src/lib/helper_test.go", "blob"},
		{"docs/readme.md", "blob"},
		{"src", "tree"},
		{"src/lib", "tree"},
		{"docs", "tree"},
	}

	tests := []struct {
		name     string
		patterns []string
		max      int
		wantN    int // expected number of files
		wantGlob int // expected number of GlobResults
	}{
		{"exact path", []string{"src/main.go"}, 10, 1, 0},
		{"star glob", []string{"src/*.go"}, 10, 2, 1},
		{"doublestar glob", []string{"src/**/*.go"}, 10, 4, 1},
		{"mixed exact and glob", []string{"docs/readme.md", "src/*.go"}, 10, 3, 1},
		{"no matches", []string{"*.rs"}, 10, 0, 1},
		{"truncated by max", []string{"**/*.go"}, 2, 2, 1},
		{"dedup across patterns", []string{"src/main.go", "src/*.go"}, 10, 2, 1},
		{"brace expansion", []string{"src/*.{go,ts}"}, 10, 2, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files, globs := expandGlobs(tt.patterns, tree, tt.max)
			if len(files) != tt.wantN {
				t.Errorf("got %d files %v, want %d", len(files), files, tt.wantN)
			}
			if len(globs) != tt.wantGlob {
				t.Errorf("got %d globs, want %d", len(globs), tt.wantGlob)
			}
		})
	}
}

func TestExpandGlobs_SkipsTreeEntries(t *testing.T) {
	tree := []treeEntry{
		{"src", "tree"},
		{"src/main.go", "blob"},
	}
	files, _ := expandGlobs([]string{"src/**"}, tree, 10)
	// Should only match the blob, not the tree entry
	if len(files) != 1 || files[0] != "src/main.go" {
		t.Errorf("got %v, want [src/main.go]", files)
	}
}

func TestExpandGlobs_PreservesOrder(t *testing.T) {
	tree := []treeEntry{
		{"a.go", "blob"},
		{"b.go", "blob"},
		{"c.go", "blob"},
	}
	files, _ := expandGlobs([]string{"c.go", "*.go"}, tree, 10)
	// c.go should come first (exact path), then a.go, b.go from glob
	if len(files) != 3 || files[0] != "c.go" {
		t.Errorf("got %v, want [c.go a.go b.go]", files)
	}
}

func TestWrapTreeBadSlugReturnsErrorWithoutPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("wrapTree panicked for invalid repo: %v", r)
		}
	}()

	_, err := wrapTree(map[string]any{"repo": "noslash"})
	if err == nil {
		t.Fatal("wrapTree error = nil, want invalid repo error")
	}
	if !strings.Contains(err.Error(), "invalid repo") {
		t.Fatalf("wrapTree error = %q, want invalid repo", err)
	}
}
