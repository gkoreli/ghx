#!/usr/bin/env python3
"""Replay an eval run's traces.jsonl into a local OTLP/HTTP viewer.

Usage:
    # one-time: install and start the viewer (UI on :8000, OTLP HTTP on :4318)
    go install github.com/CtrlSpice/otel-desktop-viewer@latest
    otel-desktop-viewer --db /tmp/ghx-evals.duckdb

    # replay a run
    python3 scripts/replay-eval-traces.py \
        internal/sidecar/evals/.ghx-evals/runs/<run-id>/traces.jsonl

Each traces.jsonl line is one OTLP JSON resourceSpans batch as written by
internal/sidecar/evals/otel_exporter.go. That exporter emits trace/span IDs
base64-encoded (protojson default for bytes), while the OTLP/JSON spec
requires lowercase hex — spec-compliant receivers reject the raw lines, so
this script transcodes IDs during replay. Fixing the exporter to emit hex
is recorded follow-up in ADR-0016.7.
"""

import base64
import json
import sys
import urllib.request

ID_KEYS = {"traceId", "spanId", "parentSpanId"}


def fix(obj):
    if isinstance(obj, dict):
        return {
            k: (
                base64.b64decode(v).hex()
                if k in ID_KEYS and isinstance(v, str) and v
                else fix(v)
            )
            for k, v in obj.items()
        }
    if isinstance(obj, list):
        return [fix(x) for x in obj]
    return obj


def main():
    if len(sys.argv) < 2:
        sys.exit(__doc__)
    path = sys.argv[1]
    endpoint = sys.argv[2] if len(sys.argv) > 2 else "http://localhost:4318/v1/traces"

    ok = fail = 0
    with open(path) as f:
        for line in f:
            line = line.strip()
            if not line:
                continue
            body = json.dumps(fix(json.loads(line))).encode()
            req = urllib.request.Request(
                endpoint, data=body, headers={"Content-Type": "application/json"}
            )
            try:
                urllib.request.urlopen(req).read()
                ok += 1
            except Exception as e:  # noqa: BLE001 — report and keep replaying
                fail += 1
                if fail <= 3:
                    print(f"ERR {e}", file=sys.stderr)
    print(f"replayed ok={ok} fail={fail}")
    if fail:
        sys.exit(1)


if __name__ == "__main__":
    main()
