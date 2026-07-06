package sidecar

import (
	"path/filepath"
	"testing"
)

func TestArtifactsRefFooterLine(t *testing.T) {
	if got := (ArtifactsRef{}).FooterLine(); got != "" {
		t.Fatalf("empty ref footer = %q, want empty", got)
	}
	ref := ArtifactsRef{SessionDir: "/home/u/.ghx/sessions/gin-gonic-gin"}
	if got, want := ref.FooterLine(), "artifacts: /home/u/.ghx/sessions/gin-gonic-gin"; got != want {
		t.Fatalf("footer = %q, want %q", got, want)
	}
	ref.TraceID = "4bf92f3577b34da6a3ce929d0e0e4736"
	want := "artifacts: /home/u/.ghx/sessions/gin-gonic-gin (trace 4bf92f3577b34da6a3ce929d0e0e4736)"
	if got := ref.FooterLine(); got != want {
		t.Fatalf("footer with trace = %q, want %q", got, want)
	}
}

func TestNewArtifactsRefResolvesAbsoluteSessionDir(t *testing.T) {
	ref := newArtifactsRef("relative/sessions", "s")
	if !filepath.IsAbs(ref.SessionDir) {
		t.Fatalf("SessionDir = %q, want absolute", ref.SessionDir)
	}
	if ref.TraceID != "" {
		t.Fatalf("fresh ref TraceID = %q, want empty", ref.TraceID)
	}
}
