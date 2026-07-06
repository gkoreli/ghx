package telemetry

import (
	"os"
	"path/filepath"
	"sync"
)

// Run/session-level JSONL artifacts (logs.jsonl, metrics.jsonl, traces.jsonl)
// are shared, append-only files owned by this package (ADR-0022): both the eval
// path (SAFE) and the production sidecar.Ask path (SAF) append to them through
// WriteLogs, WriteMetrics, and the trace exporter below. Under episode-level
// parallelism (ADR-0025 D3, GHX_EVAL_PARALLEL > 1) several episodes finish
// concurrently and each appends a single, often large, JSON line to the same
// file; production Ask sessions may parallelize the same way in the future. The
// writer is shared infrastructure, so the serialization lives with it here.
//
// POSIX O_APPEND only guarantees atomic interleaving for writes up to the
// platform pipe buffer (PIPE_BUF, typically 512B–4KB); an OTLP episode line is
// routinely tens of kilobytes, so concurrent appends without external
// serialization can splice two records together and produce an unparseable
// line. That silently corrupts the trace/eval evidence — a truthfulness
// failure, not just a cosmetic one. A per-path mutex serializes appends to
// each file so every line is a whole record.
var (
	jsonlLocksMu sync.Mutex
	jsonlLocks   = map[string]*sync.Mutex{}
)

func jsonlLock(path string) *sync.Mutex {
	key := path
	if abs, err := filepath.Abs(path); err == nil {
		key = abs
	}
	jsonlLocksMu.Lock()
	defer jsonlLocksMu.Unlock()
	mu, ok := jsonlLocks[key]
	if !ok {
		mu = &sync.Mutex{}
		jsonlLocks[key] = mu
	}
	return mu
}

// AppendJSONLine appends one already-newline-terminated record to a shared
// run/session-level JSONL file, holding the file's per-path mutex across the
// whole mkdir+open+write+close so concurrent writers cannot interleave partial
// writes. Callers must include the trailing '\n' in line. This is the single
// append primitive for every shared telemetry artifact — the log, metric, and
// trace writers in this package all route through it, so both the eval and
// production emit paths are serialized without any caller changes.
func AppendJSONLine(path string, line []byte) error {
	mu := jsonlLock(path)
	mu.Lock()
	defer mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(line)
	return err
}
