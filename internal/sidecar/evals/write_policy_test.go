package evals

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	acp "github.com/coder/acp-go-sdk"
	"github.com/gkoreli/ghx/v2/internal/sidecar/evals/hosttask"
)

// newWorkspacePolicy builds a WorkspaceWritePolicy over a fresh temp
// workspace and returns it with the resolved root (t.TempDir can live behind
// symlinks, e.g. /var -> /private/var on darwin).
func newWorkspacePolicy(t *testing.T) (*WorkspaceWritePolicy, string) {
	t.Helper()
	dir := t.TempDir()
	policy, err := NewWorkspaceWritePolicy(dir)
	if err != nil {
		t.Fatal(err)
	}
	scope, err := hosttask.NewWorkspaceScope(dir)
	if err != nil {
		t.Fatal(err)
	}
	return policy, scope.Root()
}

func TestWorkspaceWritePolicyDecisions(t *testing.T) {
	policy, root := newWorkspacePolicy(t)
	if err := os.MkdirAll(filepath.Join(root, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()

	cases := []struct {
		name  string
		path  string
		allow bool
	}{
		{"existing dir inside", filepath.Join(root, "sub"), true},
		{"new file inside", filepath.Join(root, "sub", "new.go"), true},
		{"new nested file inside", filepath.Join(root, "a", "b", "c.go"), true},
		{"outside absolute", filepath.Join(outside, "f.txt"), false},
		{"dot-dot traversal escape", filepath.Join(root, "sub", "..", "..", "escape.txt"), false},
		{"relative path", "relative/f.txt", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resolved, err := policy.AllowWrite(tc.path)
			if tc.allow {
				if err != nil {
					t.Fatalf("AllowWrite(%q): unexpected denial: %v", tc.path, err)
				}
				if !strings.HasPrefix(resolved, root) {
					t.Fatalf("AllowWrite(%q) resolved to %q, want inside %q", tc.path, resolved, root)
				}
				return
			}
			if err == nil {
				t.Fatalf("AllowWrite(%q) = %q, want denial", tc.path, resolved)
			}
		})
	}
}

func TestWorkspaceWritePolicySymlinkEscape(t *testing.T) {
	policy, root := newWorkspacePolicy(t)
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		t.Fatal(err)
	}
	if _, err := policy.AllowWrite(filepath.Join(root, "escape", "f.txt")); err == nil {
		t.Fatal("want denial for a write through a symlink escaping the workspace")
	}
	// Dangling symlinks deny too: never guess where the write would land.
	if err := os.Symlink(filepath.Join(root, "missing"), filepath.Join(root, "dangling")); err != nil {
		t.Fatal(err)
	}
	if _, err := policy.AllowWrite(filepath.Join(root, "dangling", "f.txt")); err == nil {
		t.Fatal("want denial for a write through a dangling symlink")
	}
}

func TestNewWorkspaceWritePolicyMissingRoot(t *testing.T) {
	if _, err := NewWorkspaceWritePolicy(filepath.Join(t.TempDir(), "nope")); err == nil {
		t.Fatal("want error for a missing workspace root")
	}
}

// permissionRequest builds a write-kind permission request with allow/reject
// options, mirroring what ACP adapters send.
func permissionRequest(kind acp.ToolKind, title string, locations ...string) acp.RequestPermissionRequest {
	tc := acp.ToolCallUpdate{ToolCallId: "tc1", Kind: &kind, Title: &title}
	for _, p := range locations {
		tc.Locations = append(tc.Locations, acp.ToolCallLocation{Path: p})
	}
	return acp.RequestPermissionRequest{
		ToolCall: tc,
		Options: []acp.PermissionOption{
			{OptionId: "allow", Kind: acp.PermissionOptionKindAllowOnce},
			{OptionId: "reject", Kind: acp.PermissionOptionKindRejectOnce},
		},
	}
}

func selectedOption(t *testing.T, resp acp.RequestPermissionResponse) string {
	t.Helper()
	if resp.Outcome.Selected == nil {
		t.Fatalf("no option selected: %+v", resp)
	}
	return string(resp.Outcome.Selected.OptionId)
}

func TestEvalClientWritePermissions(t *testing.T) {
	policy, root := newWorkspacePolicy(t)
	outside := t.TempDir()

	t.Run("nil policy keeps the recon deny-all contract", func(t *testing.T) {
		c := &evalClient{}
		resp, err := c.RequestPermission(context.Background(), permissionRequest(acp.ToolKindEdit, "Edit file", filepath.Join(root, "f.go")))
		if err != nil {
			t.Fatal(err)
		}
		if got := selectedOption(t, resp); got != "reject" {
			t.Fatalf("selected %q, want reject", got)
		}
		if len(c.violations) != 1 || !strings.Contains(c.violations[0], "write-kind permission requested") {
			t.Fatalf("violations = %v", c.violations)
		}
	})

	t.Run("non-write kinds stay approved under any policy", func(t *testing.T) {
		for _, c := range []*evalClient{{}, {writes: policy}} {
			resp, err := c.RequestPermission(context.Background(), permissionRequest(acp.ToolKindExecute, "go test ./..."))
			if err != nil {
				t.Fatal(err)
			}
			if got := selectedOption(t, resp); got != "allow" {
				t.Fatalf("selected %q, want allow", got)
			}
			if len(c.violations) != 0 {
				t.Fatalf("violations = %v", c.violations)
			}
		}
	})

	t.Run("workspace write allowed without violation", func(t *testing.T) {
		c := &evalClient{writes: policy}
		resp, err := c.RequestPermission(context.Background(), permissionRequest(acp.ToolKindEdit, "Edit file", filepath.Join(root, "pkg", "f.go")))
		if err != nil {
			t.Fatal(err)
		}
		if got := selectedOption(t, resp); got != "allow" {
			t.Fatalf("selected %q, want allow", got)
		}
		if len(c.violations) != 0 {
			t.Fatalf("violations = %v", c.violations)
		}
	})

	t.Run("outside write denied and recorded", func(t *testing.T) {
		c := &evalClient{writes: policy}
		resp, err := c.RequestPermission(context.Background(), permissionRequest(acp.ToolKindEdit, "Edit file", filepath.Join(outside, "f.go")))
		if err != nil {
			t.Fatal(err)
		}
		if got := selectedOption(t, resp); got != "reject" {
			t.Fatalf("selected %q, want reject", got)
		}
		if len(c.violations) != 1 || !strings.Contains(c.violations[0], "write denied") {
			t.Fatalf("violations = %v", c.violations)
		}
	})

	t.Run("one bad location denies the whole call", func(t *testing.T) {
		c := &evalClient{writes: policy}
		resp, err := c.RequestPermission(context.Background(), permissionRequest(acp.ToolKindEdit, "Edit files", filepath.Join(root, "ok.go"), filepath.Join(outside, "bad.go")))
		if err != nil {
			t.Fatal(err)
		}
		if got := selectedOption(t, resp); got != "reject" {
			t.Fatalf("selected %q, want reject", got)
		}
		if len(c.violations) != 1 {
			t.Fatalf("violations = %v", c.violations)
		}
	})

	t.Run("unlocated write fails closed and is recorded", func(t *testing.T) {
		c := &evalClient{writes: policy}
		resp, err := c.RequestPermission(context.Background(), permissionRequest(acp.ToolKindEdit, "Edit something"))
		if err != nil {
			t.Fatal(err)
		}
		if got := selectedOption(t, resp); got != "reject" {
			t.Fatalf("selected %q, want reject", got)
		}
		if len(c.violations) != 1 || !strings.Contains(c.violations[0], "no target path reported") {
			t.Fatalf("violations = %v", c.violations)
		}
	})
}

func TestEvalClientWriteTextFile(t *testing.T) {
	policy, root := newWorkspacePolicy(t)
	outside := t.TempDir()

	t.Run("nil policy denies and records", func(t *testing.T) {
		c := &evalClient{}
		_, err := c.WriteTextFile(context.Background(), acp.WriteTextFileRequest{Path: filepath.Join(root, "f.txt"), Content: "x"})
		if err == nil {
			t.Fatal("want error under the nil (deny-all) policy")
		}
		if len(c.violations) != 1 || !strings.Contains(c.violations[0], "write attempted") {
			t.Fatalf("violations = %v", c.violations)
		}
	})

	t.Run("workspace write lands on disk", func(t *testing.T) {
		c := &evalClient{writes: policy}
		target := filepath.Join(root, "nested", "dir", "f.txt")
		if _, err := c.WriteTextFile(context.Background(), acp.WriteTextFileRequest{Path: target, Content: "hello"}); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(target)
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != "hello" {
			t.Fatalf("content = %q", data)
		}
		if len(c.violations) != 0 {
			t.Fatalf("violations = %v", c.violations)
		}
	})

	t.Run("outside write denied, recorded, and not performed", func(t *testing.T) {
		c := &evalClient{writes: policy}
		target := filepath.Join(outside, "f.txt")
		if _, err := c.WriteTextFile(context.Background(), acp.WriteTextFileRequest{Path: target, Content: "x"}); err == nil {
			t.Fatal("want denial for an outside write")
		}
		if _, err := os.Stat(target); !os.IsNotExist(err) {
			t.Fatalf("outside file must not exist, stat err = %v", err)
		}
		if len(c.violations) != 1 || !strings.Contains(c.violations[0], "write denied") {
			t.Fatalf("violations = %v", c.violations)
		}
	})
}
