#!/usr/bin/env node
// Regression tests for scripts/dogfood-week.mjs (ADR-0040 L4a weekly rollup).
// Run: node --test scripts/dogfood-week.test.mjs
//
// Pins the FRICTION.md 2026-08-22 fix: only `gen_ai.client.operation.duration`
// histograms (seconds) may feed latenciesSeconds/p50Seconds. Byte-valued
// `ghx.sidecar.report.size` histograms and non-histogram metrics
// (`gen_ai.client.token.usage`, an OTel sum) must never pollute the latency
// array — before the fix, report sizes 745–5234 inflated p50 ~3× (51.3s vs 17.5s).

import { test } from 'node:test';
import assert from 'node:assert/strict';
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const script = path.join(path.dirname(fileURLToPath(import.meta.url)), 'dogfood-week.mjs');

// Build one OTel metrics.jsonl line shaped like internal/sidecar emission:
// resourceMetrics[] → scopeMetrics[] → metrics[], histogram dataPoints carrying
// delta-temporality count (string) + sum (number).
function histogramLine(name, unit, points) {
  return JSON.stringify({
    resourceMetrics: [{
      resource: { attributes: [{ key: 'service.name', value: { stringValue: 'ghx-sidecar' } }] },
      scopeMetrics: [{
        scope: { name: 'github.com/gkoreli/ghx/v2/internal/sidecar', version: '1.41.0' },
        metrics: [{
          name,
          description: `test fixture: ${name}`,
          unit,
          histogram: {
            aggregationTemporality: 'AGGREGATION_TEMPORALITY_DELTA',
            dataPoints: points.map(([count, sum]) => ({
              startTimeUnixNano: '1787353000334687641',
              timeUnixNano: '1787353038280617141',
              count: String(count),
              sum,
              min: sum / count,
              max: sum / count,
            })),
          },
        }],
      }],
    }],
  }) + '\n';
}

// OTel sum metric (not a histogram) — the shape gen_ai.client.token.usage uses.
function sumLine(name, unit, value) {
  return JSON.stringify({
    resourceMetrics: [{
      scopeMetrics: [{
        scope: { name: 'github.com/gkoreli/ghx/v2/internal/sidecar' },
        metrics: [{
          name,
          unit,
          sum: {
            aggregationTemporality: 'AGGREGATION_TEMPORALITY_DELTA',
            dataPoints: [{ asInt: String(value) }],
          },
        }],
      }],
    }],
  }) + '\n';
}

// Materialize one session directory whose meta.createdAt falls inside the
// script's default trailing-7-day window.
function writeSession(sessionsDir, name, metricsText) {
  const dir = path.join(sessionsDir, name);
  fs.mkdirSync(dir, { recursive: true });
  fs.writeFileSync(path.join(dir, 'meta.json'), JSON.stringify({
    name,
    repo: 'owner/repo',
    createdAt: new Date().toISOString(),
  }));
  if (metricsText !== null) fs.writeFileSync(path.join(dir, 'metrics.jsonl'), metricsText);
  return dir;
}

function runRollup(sessionsDir) {
  const out = execFileSync(process.execPath, [script, '--sessions-dir', sessionsDir, '--json'], { encoding: 'utf8' });
  return JSON.parse(out);
}

test('report-size histograms do not pollute latency p50 (FRICTION 2026-08-22)', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'dogfood-week-test-'));
  try {
    // Durations encoded across both datapoint shapes: single-turn (count=1)
    // and multi-turn aggregation (count>1, sum/count = per-turn seconds).
    // Expected per-turn durations: 1.2, 9.8, 21.6, 82.1 → p50 = (9.8+21.6)/2 = 15.7.
    const metrics =
      histogramLine('gen_ai.client.operation.duration', 's', [[1, 1.2], [1, 82.1]]) +
      histogramLine('gen_ai.client.operation.duration', 's', [[1, 9.8], [2, 43.2]]) +
      histogramLine('ghx.sidecar.report.size', 'By', [[1, 745], [1, 5234]]) +
      sumLine('gen_ai.client.token.usage', '{token}', 4096);
    writeSession(tmp, 'fixture-session', metrics);

    const r = runRollup(tmp);
    assert.deepEqual(r.latenciesSeconds, [1.2, 9.8, 21.6, 82.1],
      'only operation-duration datapoints (sum/count) may enter the latency array');
    assert.equal(r.p50Seconds, 15.7);
    assert.equal(r.sessions, 1);
  } finally {
    fs.rmSync(tmp, { recursive: true, force: true });
  }
});

test('a session with only byte-valued histograms yields no latency samples', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'dogfood-week-test-'));
  try {
    writeSession(tmp, 'sizes-only',
      histogramLine('ghx.sidecar.report.size', 'By', [[1, 745], [1, 1234]]));
    const r = runRollup(tmp);
    assert.deepEqual(r.latenciesSeconds, []);
    assert.equal(r.p50Seconds, null);
  } finally {
    fs.rmSync(tmp, { recursive: true, force: true });
  }
});
