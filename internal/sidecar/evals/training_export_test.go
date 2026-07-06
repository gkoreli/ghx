package evals

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestExportSFTGoldenAndCounts(t *testing.T) {
	out := filepath.Join(t.TempDir(), "sft.jsonl")
	manifest, err := ExportSFT(SFTExportOptions{
		RunDir:  filepath.Join("testdata", "export", "run"),
		OutPath: out,
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(filepath.Join("testdata", "export", "sft.golden.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("SFT JSONL mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
	if manifest.Counts.EpisodesRead != 4 {
		t.Fatalf("episodes read = %d, want 4", manifest.Counts.EpisodesRead)
	}
	if manifest.Counts.EpisodesIncluded != 1 {
		t.Fatalf("episodes included = %d, want 1", manifest.Counts.EpisodesIncluded)
	}
	if manifest.Counts.RecordsWritten != 1 {
		t.Fatalf("records written = %d, want 1", manifest.Counts.RecordsWritten)
	}
	assertExcludedCount(t, manifest, "reward_below_floor", 1)
	assertExcludedCount(t, manifest, "invalid_episode:fixture invalid", 1)
	assertExcludedCount(t, manifest, "contaminated:answer_doc_contamination", 1)
	if _, err := os.Stat(out + ".manifest.json"); err != nil {
		t.Fatalf("manifest not written: %v", err)
	}
}

func TestExportSFTDeterministicBytes(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "sft.jsonl")
	opts := SFTExportOptions{RunDir: filepath.Join("testdata", "export", "run"), OutPath: out}
	if _, err := ExportSFT(opts); err != nil {
		t.Fatal(err)
	}
	firstJSONL, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	firstManifest, err := os.ReadFile(out + ".manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ExportSFT(opts); err != nil {
		t.Fatal(err)
	}
	secondJSONL, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	secondManifest, err := os.ReadFile(out + ".manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(firstJSONL, secondJSONL) {
		t.Fatalf("JSONL exports are not byte-identical")
	}
	if !bytes.Equal(firstManifest, secondManifest) {
		t.Fatalf("manifest exports are not byte-identical")
	}
}

func assertExcludedCount(t *testing.T, manifest *SFTExportManifest, reason string, want int) {
	t.Helper()
	if got := manifest.Counts.Excluded[reason]; got != want {
		t.Fatalf("excluded[%q] = %d, want %d", reason, got, want)
	}
}
