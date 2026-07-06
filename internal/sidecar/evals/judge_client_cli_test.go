package evals

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestCLIJudgeEvaluateSuccessFromOutputLastMessage(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("PATH shim scripts are POSIX shell")
	}
	dir := t.TempDir()
	writeShim(t, dir, "codex", `#!/bin/sh
if [ "$1" = "--version" ]; then
  echo "codex-test 1.2.3"
  exit 0
fi
out=""
prev=""
for arg in "$@"; do
  if [ "$prev" = "--output-last-message" ]; then out="$arg"; fi
  prev="$arg"
done
cat >/dev/null
printf '%s\n' '{"type":"noise","message":"ignore"}'
cat > "$out" <<'JSON'
`+validVerdictJSON()+`
JSON
`)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	cfg := cliTestConfig(t)
	client, err := NewPrimaryCLIJudgeClient(cfg)
	if err != nil {
		t.Fatalf("NewPrimaryCLIJudgeClient: %v", err)
	}
	if got := client.RuntimeVersion(); got != "codex-test 1.2.3" {
		t.Fatalf("RuntimeVersion = %q", got)
	}
	v, err := client.Evaluate(context.Background(), "judge this")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if v.Overall != 4 {
		t.Fatalf("overall = %v, want 4", v.Overall)
	}
}

func TestCLIJudgeEvaluateExtractsLastCodexEvent(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("PATH shim scripts are POSIX shell")
	}
	dir := t.TempDir()
	writeShim(t, dir, "codex", `#!/bin/sh
if [ "$1" = "--version" ]; then
  echo "codex-test 1.2.3"
  exit 0
fi
cat >/dev/null
printf '%s\n' '{"type":"agent_message","message":"not json"}'
printf '%s\n' '{"type":"agent_message","message":`+shellJSONQuote(t, validVerdictJSON())+`}'
`)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	cfg := cliTestConfig(t)
	cfg.Primary.CLI.OutputLastMessage = false
	client, err := NewPrimaryCLIJudgeClient(cfg)
	if err != nil {
		t.Fatalf("NewPrimaryCLIJudgeClient: %v", err)
	}
	v, err := client.Evaluate(context.Background(), "judge this")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if v.Overall != 4 {
		t.Fatalf("overall = %v, want 4", v.Overall)
	}
}

func TestCLIJudgeMalformedRetriesFreshCall(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("PATH shim scripts are POSIX shell")
	}
	dir := t.TempDir()
	counter := filepath.Join(dir, "count")
	writeShim(t, dir, "codex", `#!/bin/sh
if [ "$1" = "--version" ]; then
  echo "codex-test 1.2.3"
  exit 0
fi
out=""
prev=""
for arg in "$@"; do
  if [ "$prev" = "--output-last-message" ]; then out="$arg"; fi
  prev="$arg"
done
cat >/dev/null
count_file="`+counter+`"
count=0
if [ -f "$count_file" ]; then count=$(cat "$count_file"); fi
count=$((count + 1))
echo "$count" > "$count_file"
if [ "$count" = "1" ]; then
  echo 'not json' > "$out"
else
  cat > "$out" <<'JSON'
`+validVerdictJSON()+`
JSON
fi
`)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	client, err := NewPrimaryCLIJudgeClient(cliTestConfig(t))
	if err != nil {
		t.Fatalf("NewPrimaryCLIJudgeClient: %v", err)
	}
	if _, err := client.Evaluate(context.Background(), "judge this"); err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	data, err := os.ReadFile(counter)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(data)) != "2" {
		t.Fatalf("calls = %s, want 2", data)
	}
}

func TestCLIJudgeNonzeroExit(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("PATH shim scripts are POSIX shell")
	}
	dir := t.TempDir()
	writeShim(t, dir, "codex", `#!/bin/sh
if [ "$1" = "--version" ]; then
  echo "codex-test 1.2.3"
  exit 0
fi
echo "boom" >&2
exit 7
`)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	client, err := NewPrimaryCLIJudgeClient(cliTestConfig(t))
	if err != nil {
		t.Fatalf("NewPrimaryCLIJudgeClient: %v", err)
	}
	if _, err := client.Evaluate(context.Background(), "judge this"); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("Evaluate error = %v, want stderr", err)
	}
}

func TestCLIJudgeTimeout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("PATH shim scripts are POSIX shell")
	}
	dir := t.TempDir()
	writeShim(t, dir, "codex", `#!/bin/sh
if [ "$1" = "--version" ]; then
  echo "codex-test 1.2.3"
  exit 0
fi
sleep 5
`)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	client, err := NewPrimaryCLIJudgeClient(cliTestConfig(t))
	if err != nil {
		t.Fatalf("NewPrimaryCLIJudgeClient: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := client.Evaluate(ctx, "judge this"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Evaluate error = %v, want DeadlineExceeded", err)
	}
}

func cliTestConfig(t *testing.T) *JudgeConfig {
	t.Helper()
	cfg, err := DefaultJudgeConfig()
	if err != nil {
		t.Fatalf("DefaultJudgeConfig: %v", err)
	}
	cfg.Primary.CLI.Command = "codex"
	return cfg
}

func writeShim(t *testing.T, dir, name, body string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}

func shellJSONQuote(t *testing.T, s string) string {
	t.Helper()
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
