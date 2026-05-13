/**
 * Result formatting and display for the ghx-bench eval engine.
 *
 * Design principle: every sub-score is shown separately. Aggregates cannot
 * hide why sidecar won or lost. The main-agent burden section is always
 * printed prominently — it is the architectural proof metric.
 */

import type { TrialResult, ExperimentResult, AgentScore, AgentTrace } from './types.js';

const HR = '━'.repeat(66);
const hr = '─'.repeat(66);

export function printTrialResult(result: TrialResult): void {
  process.stdout.write(`\n${HR}\nTRIAL  ${result.id}\n${HR}\n`);
  process.stdout.write(`Task:     ${result.taskId}\n`);
  process.stdout.write(`Agents:   ${Object.keys(result.traces).join(', ')}\n`);

  // ── Scorer results ──────────────────────────────────────────────────────
  for (const scorer of result.scores) {
    if (scorer.scores.length === 0) continue;
    process.stdout.write(`\n${hr}\nScorer: ${scorer.scorerId}\n${hr}\n`);
    for (const s of scorer.scores) printAgentScore(s);
    process.stdout.write(`\n  Verdict: ${scorer.verdict}\n`);
    process.stdout.write(`  Summary: ${scorer.summary}\n`);
  }

  // ── Main-agent burden (always shown separately — it is the thesis metric) ──
  process.stdout.write(`\n${hr}\nWORKFLOW BOUNDARY  (main-agent context burden)\n${hr}\n`);
  printBurdenTable(Object.values(result.traces));

  // ── Cost metrics ─────────────────────────────────────────────────────────
  process.stdout.write(`\n${hr}\nCOST METRICS\n${hr}\n`);
  printCostTable(Object.values(result.traces));

  process.stdout.write(`\nTotal duration: ${(result.durationMs / 1000).toFixed(1)}s\n`);
  process.stdout.write(`${HR}\n`);
}

function printAgentScore(s: AgentScore): void {
  process.stdout.write(`\n  ${s.agentId}  (overall ${s.overall.toFixed(1)}/10)\n`);
  for (const [dim, score] of Object.entries(s.dimensions)) {
    const filled = Math.max(0, Math.min(10, Math.round(score.score)));
    const bar = '█'.repeat(filled) + '░'.repeat(10 - filled);
    const label = dim.padEnd(20);
    process.stdout.write(`    ${label} ${bar} ${String(score.score).padStart(2)}/10  ${score.rationale}\n`);
  }
}

function printBurdenTable(traces: AgentTrace[]): void {
  // Header
  const col = (s: string, w: number) => s.slice(0, w).padEnd(w);
  process.stdout.write(`\n  ${col('Agent', 20)}${col('MainCtx KB', 12)}${col('SidecarInternal KB', 20)}${col('FinalReport KB', 16)}${col('Total KB', 12)}Compression\n`);
  process.stdout.write(`  ${'─'.repeat(82)}\n`);

  for (const t of traces) {
    const b = t.mainAgentBurden;
    const mainKB   = (b.mainAgentContextCharsApprox / 1024).toFixed(1);
    const scKB     = (b.sidecarInternalTraceCharsApprox / 1024).toFixed(1);
    const reportKB = (b.finalReportCharsApprox / 1024).toFixed(1);
    const totalKB  = (b.totalWorkflowCharsApprox / 1024).toFixed(1);
    const ratio    = b.totalWorkflowCharsApprox > 0
      ? ((1 - b.mainAgentContextCharsApprox / b.totalWorkflowCharsApprox) * 100).toFixed(0)
      : '0';
    process.stdout.write(
      `  ${col(t.agentId, 20)}${col(mainKB, 12)}${col(scKB, 20)}${col(reportKB, 16)}${col(totalKB, 12)}${ratio}% compressed\n`,
    );
  }

  // Per-agent compression summary — compare by ratio, not absolute KB
  process.stdout.write('\n');
  for (const t of traces) {
    const b = t.mainAgentBurden;
    const mainKB  = (b.mainAgentContextCharsApprox / 1024).toFixed(1);
    const totalKB = (b.totalWorkflowCharsApprox / 1024).toFixed(1);
    const ratio   = b.totalWorkflowCharsApprox > 0
      ? ((1 - b.mainAgentContextCharsApprox / b.totalWorkflowCharsApprox) * 100).toFixed(0)
      : '0';
    const tag = b.sidecarInternalTraceCharsApprox > 0 ? ' [sidecar]' : ' [direct]';
    process.stdout.write(`  ${t.agentId.padEnd(20)}${mainKB}KB main-agent / ${totalKB}KB total workflow, ${ratio}% compressed${tag}\n`);
  }
}

function printCostTable(traces: AgentTrace[]): void {
  process.stdout.write(`\n  ${'Agent'.padEnd(20)}${'Turns'.padEnd(8)}${'ToolCalls'.padEnd(12)}${'FilesRead'.padEnd(14)}${'Dups'.padEnd(8)}${'OutputKB'.padEnd(10)}Time\n`);
  process.stdout.write(`  ${'─'.repeat(74)}\n`);
  for (const t of traces) {
    const c = t.costs;
    const row =
      t.agentId.padEnd(20) +
      String(c.turnCount).padEnd(8) +
      String(c.toolCallCount).padEnd(12) +
      `${c.uniqueFilesRead}u/${c.uniqueFilesRead + c.duplicateFilesRead}t`.padEnd(14) +
      String(c.duplicateFilesRead).padEnd(8) +
      `${(c.totalOutputChars / 1024).toFixed(1)}`.padEnd(10) +
      `${(c.wallTimeMs / 1000).toFixed(1)}s`;
    process.stdout.write(`  ${row}\n`);
  }
}

export function printExperimentSummary(result: ExperimentResult): void {
  process.stdout.write(`\n${HR}\nEXPERIMENT SUMMARY  ${result.experimentId}\n${HR}\n`);
  process.stdout.write(`Tasks:    ${result.taskIds.join(', ')}\n`);
  process.stdout.write(`Agents:   ${result.agentIds.join(', ')}\n`);
  process.stdout.write(`Trials:   ${result.trials.length}\n`);
  process.stdout.write(`${hr}\n`);

  for (const [agentId, agg] of Object.entries(result.aggregate)) {
    process.stdout.write(`\n${agentId}\n`);
    process.stdout.write(`  Overall:   ${agg.meanOverall.toFixed(2)}/10\n`);
    process.stdout.write(`  Win rate:  ${(agg.winRate * 100).toFixed(0)}%  (${agg.wins}/${agg.trials})\n`);

    if (Object.keys(agg.byDimension).length > 0) {
      process.stdout.write(`  Dimensions:\n`);
      for (const [dim, stats] of Object.entries(agg.byDimension)) {
        process.stdout.write(`    ${dim.padEnd(22)}  μ=${stats.mean.toFixed(2)}  σ=${stats.stddev.toFixed(2)}\n`);
      }
    }

    if (agg.meanBurden.mainAgentContextCharsApprox !== undefined) {
      const mainKB  = (agg.meanBurden.mainAgentContextCharsApprox / 1024).toFixed(1);
      const totalKB = ((agg.meanBurden.totalWorkflowCharsApprox ?? 0) / 1024).toFixed(1);
      process.stdout.write(`  Context burden (avg): ${mainKB}KB main-agent / ${totalKB}KB total\n`);
    }
    if (agg.meanCosts.toolCallCount !== undefined) {
      process.stdout.write(`  Tool calls (avg):     ${agg.meanCosts.toolCallCount.toFixed(1)}\n`);
    }
  }

  process.stdout.write(`${HR}\n`);
}
