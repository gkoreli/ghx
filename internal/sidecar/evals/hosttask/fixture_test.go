package hosttask

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// validFixture returns a fixture that passes Validate; tests mutate copies.
func validFixture() Fixture {
	return Fixture{
		ID:             "demo-task",
		WorkspaceRepo:  "acme/widgets",
		PinnedSHA:      strings.Repeat("ab", 20),
		Image:          "golang:1.24-bookworm",
		SetupCmds:      []string{"go mod download"},
		FailToPassCmds: []string{"go test ./pkg -run TestFix"},
		PassToPassCmds: []string{"go test ./pkg -run TestBasic"},
		ExplorationSubQuestions: []SubQuestion{
			{Question: "what changed upstream?", GroundTruth: "the API moved", VerifiedAt: "2026-07-06"},
		},
		MemorizationCanary: MemorizationCanary{CheckedAt: "2026-07-06", Result: CanaryPass},
	}
}

func TestFixtureValidateAcceptsWellFormed(t *testing.T) {
	if err := validFixture().Validate(); err != nil {
		t.Fatalf("valid fixture rejected: %v", err)
	}
}

func TestFixtureValidateRejections(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(*Fixture)
		wantSub string
	}{
		{"empty id", func(f *Fixture) { f.ID = "" }, "fixture id"},
		{"unsafe id", func(f *Fixture) { f.ID = "../evil" }, "fixture id"},
		{"bad repo", func(f *Fixture) { f.WorkspaceRepo = "no-slash" }, "workspaceRepo"},
		{"short sha", func(f *Fixture) { f.PinnedSHA = "abc123" }, "pinnedSha"},
		{"no image", func(f *Fixture) { f.Image = " " }, "image is required"},
		{"no failToPass", func(f *Fixture) { f.FailToPassCmds = nil }, "failToPass"},
		{"no passToPass", func(f *Fixture) { f.PassToPassCmds = nil }, "passToPass"},
		{"blank command", func(f *Fixture) { f.SetupCmds = []string{"  "} }, "setupCmds[0]"},
		{"no sub-questions", func(f *Fixture) { f.ExplorationSubQuestions = nil }, "sub-question"},
		{"sub-question no ground truth", func(f *Fixture) {
			f.ExplorationSubQuestions[0].GroundTruth = ""
		}, "groundTruth"},
		{"sub-question bad date", func(f *Fixture) {
			f.ExplorationSubQuestions[0].VerifiedAt = "yesterday"
		}, "verifiedAt"},
		{"canary bad date", func(f *Fixture) {
			f.MemorizationCanary.CheckedAt = "2026-13-99"
		}, "checkedAt"},
		{"canary failed", func(f *Fixture) {
			f.MemorizationCanary.Result = CanaryFail
		}, "disqualifies the task"},
		{"canary unknown result", func(f *Fixture) {
			f.MemorizationCanary.Result = "maybe"
		}, "memorizationCanary.result"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := validFixture()
			// Deep-copy the slice the mutation may touch.
			f.ExplorationSubQuestions = append([]SubQuestion(nil), f.ExplorationSubQuestions...)
			c.mutate(&f)
			err := f.Validate()
			if err == nil {
				t.Fatal("want validation error, got nil")
			}
			if !strings.Contains(err.Error(), c.wantSub) {
				t.Errorf("error %q does not mention %q", err, c.wantSub)
			}
		})
	}
}

func TestFixtureNetworkDefaultsToNone(t *testing.T) {
	f := validFixture()
	if got := f.NetworkMode(); got != "none" {
		t.Errorf("default NetworkMode = %q, want none", got)
	}
	f.Network = "bridge"
	if got := f.NetworkMode(); got != "bridge" {
		t.Errorf("explicit NetworkMode = %q, want bridge", got)
	}
}

func TestFixtureRemoteURL(t *testing.T) {
	if got := validFixture().RemoteURL(); got != "https://github.com/acme/widgets.git" {
		t.Errorf("RemoteURL = %q", got)
	}
}

// TestLoadFixturesFromTestdata pins the committed example document as the
// canonical schema reference: it must parse, validate, and round-trip.
func TestLoadFixturesFromTestdata(t *testing.T) {
	fixtures, err := LoadFixtures(filepath.Join("testdata", "fixtures"))
	if err != nil {
		t.Fatal(err)
	}
	if len(fixtures) != 1 {
		t.Fatalf("got %d fixtures, want 1", len(fixtures))
	}
	f := fixtures[0]
	if f.ID != "chi-middleware-order" || f.WorkspaceRepo != "acme/chi-service" ||
		f.Image != "golang:1.24-bookworm" || f.NetworkMode() != "none" ||
		len(f.ExplorationSubQuestions) != 1 || f.MemorizationCanary.Result != CanaryPass {
		t.Errorf("example fixture mis-parsed: %+v", f)
	}
}

func TestLoadFixturesRejectsInvalidAndDuplicates(t *testing.T) {
	write := func(dir, name string, f Fixture) {
		t.Helper()
		data, err := json.Marshal(f)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("invalid fixture", func(t *testing.T) {
		dir := t.TempDir()
		bad := validFixture()
		bad.PinnedSHA = "not-a-sha"
		write(dir, "a.json", bad)
		if _, err := LoadFixtures(dir); err == nil || !strings.Contains(err.Error(), "a.json") {
			t.Errorf("want file-attributed validation error, got %v", err)
		}
	})

	t.Run("duplicate ids", func(t *testing.T) {
		dir := t.TempDir()
		write(dir, "a.json", validFixture())
		write(dir, "b.json", validFixture())
		if _, err := LoadFixtures(dir); err == nil || !strings.Contains(err.Error(), "duplicate fixture id") {
			t.Errorf("want duplicate-id error, got %v", err)
		}
	})

	t.Run("empty dir", func(t *testing.T) {
		if _, err := LoadFixtures(t.TempDir()); err == nil {
			t.Error("want error for empty fixtures dir")
		}
	})
}
