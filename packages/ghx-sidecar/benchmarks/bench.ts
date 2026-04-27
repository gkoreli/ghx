#!/usr/bin/env node
/**
 * ghx-sidecar A/B bench
 *
 * Runs the same 2-turn scenario against two agents:
 *   A — full ghx-sidecar session (sidecar prompt + ghx grounding)
 *   B — plain ACP session (same agent, no sidecar context)
 *
 * Run from packages/ghx-sidecar/:
 *   npx tsx experiments/bench.ts
 */

import { acpxSend } from '../src/acpx.js';
import { createSidecarPrompt } from '../src/core/prompt.js';
import { loadConfig } from '../src/core/config.js';
import type { SidecarReport } from '../src/schema.js';
import type { AcpRuntimeEvent } from 'acpx/runtime';

// ── Scenario ──────────────────────────────────────────────────────────────────
//
// Hono middleware — 2 turns, known ground truth.
// Turn 1: find the implementation.
// Turn 2: follow-up that requires context from turn 1.

const SCENARIO = {
  repo: 'honojs/hono',
  turns: [
    'Where is middleware composition implemented? What function and file handle it?',
    'How do errors propagate through that middleware chain — is there explicit error handling or does it bubble?',
  ],
  // Files the correct answer must mention to count as a hit.
  groundTruth: ['src/compose.ts'],
} as const;

// ── Agent result ──────────────────────────────────────────────────────────────

interface AgentResult {
  filesFound: string[];
  report: SidecarReport | undefined;
  commandsRun: number;
}

// ── Agent A: with sidecar ─────────────────────────────────────────────────────

async function runWithSidecar(runId: string): Promise<AgentResult> {
  const config = loadConfig();
  let report: SidecarReport | undefined;

  for (let i = 0; i < SCENARIO.turns.length; i++) {
    const question = SCENARIO.turns[i];

    let prompt: string;
    if (i === 0) {
      const sys = createSidecarPrompt({
        session: `bench:a-${runId}`,
        repo: SCENARIO.repo,
        question,
        depth: 'normal',
        output: 'evidence',
        allowedBackends: ['remote'],
      });
      prompt = sys + '\n\n---\n\nQuestion: ' + question;
    } else {
      prompt = question;
    }

    await acpxSend({
      agent: config.agent,
      session: `bench:a-${runId}`,
      prompt,
      onReport: (r) => { report = r; },
    });
  }

  return {
    filesFound: report?.relevantFiles?.map(f => f.path) ?? [],
    report,
    commandsRun: report?.commandsRun?.length ?? 0,
  };
}

// ── Agent B: without sidecar ──────────────────────────────────────────────────

async function runWithout(runId: string): Promise<AgentResult> {
  const config = loadConfig();
  const captured: string[] = [];

  for (let i = 0; i < SCENARIO.turns.length; i++) {
    const question = SCENARIO.turns[i];
    const prefix = i === 0 ? `Repo: ${SCENARIO.repo}\n\n` : '';

    await acpxSend({
      agent: config.agent,
      session: `bench:b-${runId}`,
      prompt: prefix + question,
      onEvent: (e: AcpRuntimeEvent) => {
        if (e.type === 'text_delta') captured.push(e.text);
      },
    });
  }

  const filesFound = extractFilePaths(captured.join(''));
  return { filesFound, report: undefined, commandsRun: 0 };
}

// ── Scoring ───────────────────────────────────────────────────────────────────

interface Score {
  hits: number;
  precision: number;
  recall: number;
}

function score(found: string[], truth: readonly string[]): Score {
  if (found.length === 0 || truth.length === 0) return { hits: 0, precision: 0, recall: 0 };
  const basename = (p: string) => p.split('/').pop()!.toLowerCase();
  const foundNames = found.map(basename);
  const truthNames = truth.map(basename);
  const hits = foundNames.filter(f => truthNames.includes(f)).length;
  return {
    hits,
    precision: hits / found.length,
    recall: hits / truth.length,
  };
}

// Extract file path mentions from free-form text.
function extractFilePaths(text: string): string[] {
  const matches = text.match(/\b[\w./-]+\.(?:ts|tsx|js|go|py|rs|rb|java)\b/g) ?? [];
  return [...new Set(matches)].filter(f => f.includes('/') && !f.startsWith('.'));
}

// ── Main ──────────────────────────────────────────────────────────────────────

const HR = '━'.repeat(60);

async function main(): Promise<void> {
  const runId = Date.now().toString(36);

  process.stdout.write(`${HR}\nghx-sidecar A/B bench  (run ${runId})\n`);
  process.stdout.write(`Repo:         ${SCENARIO.repo}\n`);
  process.stdout.write(`Turns:        ${SCENARIO.turns.length}\n`);
  process.stdout.write(`Ground truth: ${SCENARIO.groundTruth.join(', ')}\n`);
  process.stdout.write(`${HR}\n`);

  process.stdout.write('\n[ Agent A — WITH sidecar ]\n\n');
  const resultA = await runWithSidecar(runId);

  process.stdout.write('\n[ Agent B — WITHOUT sidecar ]\n\n');
  const resultB = await runWithout(runId);

  const scoreA = score(resultA.filesFound, SCENARIO.groundTruth);
  const scoreB = score(resultB.filesFound, SCENARIO.groundTruth);

  process.stdout.write(`\n${HR}\nCOMPARISON\n${HR}\n`);

  process.stdout.write('\nAgent A (sidecar):\n');
  process.stdout.write(`  Files:      ${resultA.filesFound.slice(0, 6).join(', ') || '(none)'}\n`);
  process.stdout.write(`  Recall:     ${pct(scoreA.recall)}   Precision: ${pct(scoreA.precision)}\n`);
  process.stdout.write(`  Commands:   ${resultA.commandsRun}\n`);
  if (resultA.report?.answer !== undefined) {
    process.stdout.write(`  Answer:     ${resultA.report.answer.slice(0, 120)}\n`);
  }

  process.stdout.write('\nAgent B (plain):\n');
  process.stdout.write(`  Files:      ${resultB.filesFound.slice(0, 6).join(', ') || '(none)'}\n`);
  process.stdout.write(`  Recall:     ${pct(scoreB.recall)}   Precision: ${pct(scoreB.precision)}\n`);

  const verdict =
    scoreA.recall > scoreB.recall ? 'A (sidecar) wins' :
    scoreB.recall > scoreA.recall ? 'B (plain) wins — investigate!' :
    'Tie';

  process.stdout.write(`\nVerdict: ${verdict}\n${HR}\n`);
}

function pct(n: number): string {
  return `${(n * 100).toFixed(0)}%`;
}

main().catch(err => {
  process.stderr.write(`bench: ${err instanceof Error ? err.message : String(err)}\n`);
  process.exit(1);
});
