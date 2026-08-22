#!/usr/bin/env node
// Usage: node scripts/dogfood-week.mjs [--since YYYY-MM-DD] [--until YYYY-MM-DD]
//                                     [--sessions-dir DIR] [--json] [--quiet]
//
// ADR-0040 L4a weekly rollup (docs/dogfood/FRICTION.md M5 bar): turns the raw
// sidecar session artifacts under ~/.ghx/sessions/ into the one-line weekly
// measurement — sessions, asks, answered/blocked/degraded counts, p50 latency,
// quota-ladder firings.
//
// Measurement rules (deliberate, keep stable across weeks):
// - session      = a directory under <sessions-dir> with meta.json whose
//                  createdAt falls inside [since, until] (inclusive, UTC).
// - ask          = one `turn.started` event in the session's live.jsonl.
// - completed    = matching `turn.completed`; ok:false marks a failed turn.
// - latency      = per-turn duration from metrics.jsonl
//                  `gen_ai.client.operation.duration` histogram datapoints,
//                  seconds, value = sum/count (OTel delta temporality).
//                  p50 = lower median of all per-turn values in the window.
// - answered     = reports/*.json whose report.answer does NOT start with
//                  "BLOCKED:" or "DEGRADED".
// - degraded     = reports whose answer starts with "DEGRADED" (ADR-0040 L3
//                  label: degraded:cache or degraded:model rungs).
// - blocked      = reports whose answer starts with "BLOCKED:".
// - quota firing = degraded reports PLUS logs.jsonl lines matching /quota/i
//                  (rung evidence even when no report survived).
//
// Exit codes follow the repo contract: 0 measured, 2 bad invocation.

import fs from 'node:fs';
import path from 'node:path';

function usage(msg) {
  if (msg) console.error(`error: ${msg}`);
  console.error('usage: node scripts/dogfood-week.mjs [--since YYYY-MM-DD] [--until YYYY-MM-DD] [--sessions-dir DIR] [--json] [--quiet]');
  process.exit(2);
}

const argv = process.argv.slice(2);
let since = null, until = null, sessionsDir = path.join(process.env.HOME || '', '.ghx', 'sessions'), jsonOut = false, quiet = false;
for (let i = 0; i < argv.length; i++) {
  const a = argv[i];
  if (a === '--since') { since = argv[++i]; }
  else if (a === '--until') { until = argv[++i]; }
  else if (a === '--sessions-dir') { sessionsDir = argv[++i]; }
  else if (a === '--json') { jsonOut = true; }
  else if (a === '--quiet') { quiet = true; }
  else if (a === '--help' || a === '-h') { usage(); }
  else usage(`unknown argument ${a}`);
}
if (since && !/^\d{4}-\d{2}-\d{2}$/.test(since)) usage('--since expects YYYY-MM-DD');
if (until && !/^\d{4}-\d{2}-\d{2}$/.test(until)) usage('--until expects YYYY-MM-DD');

// Default window: the trailing 7 days ending today (UTC), matching the weekly rollup cadence.
const dayMs = 24 * 60 * 60 * 1000;
const [ty, tm, td] = new Date().toISOString().slice(0, 10).split('-').map(Number);
const today = new Date(Date.UTC(ty, tm - 1, td)); // UTC months are 0-indexed
const sinceTs = since ? Date.parse(since + 'T00:00:00Z') : today.getTime() - 6 * dayMs;
const untilTs = until ? Date.parse(until + 'T00:00:00Z') + dayMs - 1 : today.getTime() + dayMs - 1;

function readJsonl(file) {
  const out = [];
  let text;
  try { text = fs.readFileSync(file, 'utf8'); } catch { return out; }
  for (const line of text.split('\n')) {
    const t = line.trim();
    if (!t) continue;
    try { out.push(JSON.parse(t)); } catch { /* tolerate torn tail lines */ }
  }
  return out;
}

const fmtDate = ts => new Date(ts).toISOString().slice(0, 10);

const result = {
  window: { since: fmtDate(sinceTs), until: fmtDate(untilTs) },
  sessions: 0, asks: 0, failedTurns: 0, reportsAnswered: 0, reportsBlocked: 0,
  reportsDegraded: 0, quotaFirings: 0, latenciesSeconds: [], p50Seconds: null,
  perSession: [],
};

let entries = [];
try { entries = fs.readdirSync(sessionsDir, { withFileTypes: true }).filter(e => e.isDirectory()).map(e => e.name); }
catch { console.error(`error: cannot read sessions dir ${sessionsDir}`); process.exit(2); }

for (const name of entries.sort()) {
  const dir = path.join(sessionsDir, name);
  const metaFile = path.join(dir, 'meta.json');
  if (!fs.existsSync(metaFile)) continue;
  let meta;
  try { meta = JSON.parse(fs.readFileSync(metaFile, 'utf8')); } catch { continue; }
  const created = Date.parse(meta.createdAt || '');
  if (!Number.isFinite(created)) continue;
  if (created < sinceTs || created > untilTs) continue;

  result.sessions += 1;

  // Asks + failures from live.jsonl turn events.
  let asks = 0, failed = 0;
  for (const ev of readJsonl(path.join(dir, 'live.jsonl'))) {
    if (ev.event === 'turn.started') asks += 1;
    if (ev.event === 'turn.completed' && ev.ok === false) failed += 1;
  }

  // Per-turn latency from the OTel metrics file.
  const latencies = [];
  for (const line of readJsonl(path.join(dir, 'metrics.jsonl'))) {
    for (const rm of line.resourceMetrics || []) {
      for (const sm of rm.scopeMetrics || []) {
        for (const m of sm.metrics || []) {
          const h = m.histogram;
          if (!h) continue;
          for (const dp of h.dataPoints || []) {
            const count = Number(dp.count || 0);
            if (count <= 0) continue;
            const v = Number(dp.sum) / count;
            if (Number.isFinite(v) && v > 0) latencies.push(v);
          }
        }
      }
    }
  }
  result.latenciesSeconds.push(...latencies);
  result.asks += asks;
  result.failedTurns += failed;

  // Report outcome labels.
  let answered = 0, blocked = 0, degraded = 0;
  const repDir = path.join(dir, 'reports');
  let reps = [];
  try { reps = fs.readdirSync(repDir).filter(f => f.endsWith('.json')); } catch { /* no reports */ }
  for (const f of reps) {
    let r;
    try { r = JSON.parse(fs.readFileSync(path.join(repDir, f), 'utf8')); } catch { continue; }
    const ans = String((r.report && r.report.answer) || r.answer || '');
    if (ans.startsWith('DEGRADED')) degraded += 1;
    else if (ans.startsWith('BLOCKED')) blocked += 1;
    else if (ans.trim()) answered += 1;
  }
  result.reportsAnswered += answered;
  result.reportsBlocked += blocked;
  result.reportsDegraded += degraded;

  // Quota-ladder evidence beyond degraded reports.
  let quotaLogLines = 0;
  for (const l of readJsonl(path.join(dir, 'logs.jsonl'))) {
    const s = JSON.stringify(l);
    if (/quota/i.test(s)) quotaLogLines += 1;
  }
  result.quotaFirings += degraded + Math.min(quotaLogLines, 1); // per session: report-level truth + at most one log-evidence flag

  result.perSession.push({ session: name, createdAt: meta.createdAt, repo: meta.repo || null, asks, failedTurns: failed, reports: reps.length, answered, blocked, degraded, quotaEvidence: quotaLogLines > 0 });
}

const totalReports = result.reportsAnswered + result.reportsBlocked + result.reportsDegraded;
result.degradedRatePct = totalReports ? +(100 * result.reportsDegraded / totalReports).toFixed(1) : 0.0;
result.latenciesSeconds.sort((a, b) => a - b);
if (result.latenciesSeconds.length) {
  const mid = Math.floor(result.latenciesSeconds.length / 2);
  result.p50Seconds = result.latenciesSeconds.length % 2
    ? +result.latenciesSeconds[mid].toFixed(1)
    : +((result.latenciesSeconds[mid - 1] + result.latenciesSeconds[mid]) / 2).toFixed(1);
}

if (jsonOut) {
  console.log(JSON.stringify(result, null, 2));
} else {
  if (!quiet) {
    for (const s of result.perSession) {
      console.log(`  ${fmtDate(Date.parse(s.createdAt))}  ${s.session.padEnd(36)} asks=${s.asks} reports=${s.reports} answered=${s.answered} blocked=${s.blocked} degraded=${s.degraded}${s.failedTurns ? ` FAILED_TURNS=${s.failedTurns}` : ''}`);
    }
  }
  console.log(
    `dogfood ${result.window.since}..${result.window.until}: sessions=${result.sessions}` +
    ` asks=${result.asks}` +
    ` answered=${result.reportsAnswered}` +
    ` blocked=${result.reportsBlocked}` +
    ` degraded=${result.reportsDegraded}` +
    ` quotaFirings=${result.quotaFirings}` +
    ` p50=${result.p50Seconds === null ? 'n/a' : result.p50Seconds + 's'}` +
    ` degradedRate=${result.degradedRatePct}%`
  );
}
