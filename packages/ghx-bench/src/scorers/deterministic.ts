/**
 * Deterministic scorer — hard correctness and architectural boundary checks.
 *
 * Dimensions:
 *   fileRecall        — expected files found
 *   filePrecision     — found files that are expected (hallucination penalty)
 *   symbolRecall      — expected symbols mentioned in answer
 *   claimCoverage     — required mechanism claims present (substring match)
 *   badClaimPenalty   — unacceptable claims absent (penalty when found)
 *   pathDiscipline    — avoided reading discouraged paths (e.g. tests)
 *   mainAgentBurden   — compression ratio: how much research entered main context?
 *   followupContinuity— session reuse: avoided re-reading, referenced prior findings
 *
 * All scores are 1–10 mapped from 0–1 ratios for comparability with LLM judge.
 * Tasks without checks return a no-op result.
 */

import type { Scorer, Task, AgentTrace, ScorerResult, AgentScore, DimensionScore } from '../types.js';

export class DeterministicScorer implements Scorer {
  readonly id = 'deterministic';

  async score(task: Task, traces: Record<string, AgentTrace>): Promise<ScorerResult> {
    const checks = task.checks;
    if (checks === undefined || Object.keys(checks).length === 0) {
      return { scorerId: this.id, scores: [], verdict: 'n/a', summary: 'No deterministic checks defined.' };
    }

    const scores: AgentScore[] = Object.values(traces).map(t => scoreTrace(t, checks));

    const winner = scores.reduce<AgentScore | undefined>((best, s) =>
      best === undefined || s.overall > best.overall ? s : best,
    undefined);

    const topScore = winner?.overall ?? 0;
    const topAgents = scores.filter(s => s.overall === topScore).map(s => s.agentId);
    const verdict = topAgents.length === 1 ? topAgents[0]! : 'tie';
    const summary = scores.map(s => `${s.agentId}: ${s.overall.toFixed(1)}/10`).join(', ');

    return { scorerId: this.id, scores, verdict, summary };
  }
}

// ── Per-trace scoring ─────────────────────────────────────────────────────────

function scoreTrace(trace: AgentTrace, checks: NonNullable<Task['checks']>): AgentScore {
  const dims: Record<string, DimensionScore> = {};
  const allText = [trace.finalAnswer, ...trace.turns.map(t => t.text)].join('\n');
  const allTextLower = allText.toLowerCase();
  const allToolCalls = trace.turns.flatMap(t => t.toolCalls).join('\n').toLowerCase();

  // ── File metrics ──────────────────────────────────────────────────────────

  if (checks.expectedFiles !== undefined && checks.expectedFiles.length > 0) {
    const hits = checks.expectedFiles.filter(f => fileMatch(f, trace.filesFound)).length;
    dims['fileRecall'] = {
      score: ratioToScore(hits / checks.expectedFiles.length),
      rationale: `${hits}/${checks.expectedFiles.length} expected files found`,
    };

    if (trace.filesFound.length > 0) {
      const precision = hits / trace.filesFound.length;
      dims['filePrecision'] = {
        score: ratioToScore(precision),
        rationale: `${hits}/${trace.filesFound.length} found files were expected`,
      };
    }
  }

  // ── Symbol recall ─────────────────────────────────────────────────────────

  if (checks.expectedSymbols !== undefined && checks.expectedSymbols.length > 0) {
    const hits = checks.expectedSymbols.filter(s => allTextLower.includes(s.toLowerCase())).length;
    dims['symbolRecall'] = {
      score: ratioToScore(hits / checks.expectedSymbols.length),
      rationale: `${hits}/${checks.expectedSymbols.length} expected symbols mentioned`,
    };
  }

  // ── Claim coverage (mechanism understanding) ──────────────────────────────

  if (checks.requiredClaims !== undefined && checks.requiredClaims.length > 0) {
    const hits = checks.requiredClaims.filter(claim =>
      claimMatch(claim, allTextLower),
    ).length;
    dims['claimCoverage'] = {
      score: ratioToScore(hits / checks.requiredClaims.length),
      rationale: `${hits}/${checks.requiredClaims.length} required claims present`,
    };
  }

  // ── Bad claim penalty ─────────────────────────────────────────────────────

  if (checks.unacceptableClaims !== undefined && checks.unacceptableClaims.length > 0) {
    const violations = checks.unacceptableClaims.filter(claim =>
      claimMatch(claim, allTextLower),
    ).length;
    const ratio = 1 - violations / checks.unacceptableClaims.length;
    dims['badClaimPenalty'] = {
      score: ratioToScore(ratio),
      rationale: violations === 0
        ? 'No unacceptable claims found'
        : `${violations} unacceptable claim(s) present`,
    };
  }

  // ── Path discipline ───────────────────────────────────────────────────────

  if (checks.avoidPaths !== undefined && checks.avoidPaths.length > 0) {
    const violations = checks.avoidPaths.filter(p =>
      allToolCalls.includes(p.toLowerCase()),
    ).length;
    dims['pathDiscipline'] = {
      score: ratioToScore(1 - violations / checks.avoidPaths.length),
      rationale: violations === 0
        ? 'Avoided all discouraged paths'
        : `Read ${violations} discouraged path(s)`,
    };
  }

  // ── Main-agent burden (workflow boundary metric) ──────────────────────────
  //
  // Compression ratio: what fraction of total workflow chars entered the
  // main agent's context? Lower ratio = cleaner boundary = higher score.
  //
  // sidecar: mainAgentContext = report block only → ratio ≈ 0.1 → score ≈ 9
  // plain:   mainAgentContext = all output → ratio ≈ 1.0 → score ≈ 1

  const burden = trace.mainAgentBurden;
  if (burden.totalWorkflowCharsApprox > 0) {
    const ratio = burden.mainAgentContextCharsApprox / burden.totalWorkflowCharsApprox;
    const compressionScore = ratioToScore(1 - ratio);
    const kbMain = (burden.mainAgentContextCharsApprox / 1024).toFixed(1);
    const kbTotal = (burden.totalWorkflowCharsApprox / 1024).toFixed(1);
    dims['mainAgentBurden'] = {
      score: compressionScore,
      rationale: `Main-agent context: ${kbMain}KB / ${kbTotal}KB total (${(ratio * 100).toFixed(0)}% ratio)`,
    };
  }

  // ── Follow-up continuity ──────────────────────────────────────────────────

  const fm = trace.followupMetrics;
  if (fm !== undefined) {
    const totalReads = trace.costs.uniqueFilesRead + fm.repeatReadsAcrossTurns;
    const repeatRate = totalReads > 0 ? fm.repeatReadsAcrossTurns / totalReads : 0;
    const continuityBase = 1 - repeatRate;
    const bonus = fm.priorFindingsReferenced ? 0.15 : 0;
    dims['followupContinuity'] = {
      score: ratioToScore(Math.min(1, continuityBase + bonus)),
      rationale: `${fm.repeatReadsAcrossTurns} repeat reads, ${fm.newFilesReadOnFollowup} new; prior findings ${fm.priorFindingsReferenced ? 'referenced' : 'not referenced'}`,
    };
  }

  const dim_scores = Object.values(dims).map(d => d.score);
  const overall = dim_scores.length > 0
    ? dim_scores.reduce((a, b) => a + b, 0) / dim_scores.length
    : 0;

  return { agentId: trace.agentId, dimensions: dims, overall };
}

// ── Helpers ───────────────────────────────────────────────────────────────────

function fileMatch(expected: string, found: string[]): boolean {
  const expectedBase = expected.split('/').pop()!.toLowerCase();
  return found.some(f => {
    const base = f.split('/').pop()!.toLowerCase();
    return base === expectedBase || f.toLowerCase().endsWith(expected.toLowerCase());
  });
}

/**
 * Claim matching: check if key terms from the claim appear together in text.
 * Splits the claim into significant words (>3 chars) and requires >60% to match.
 */
function claimMatch(claim: string, textLower: string): boolean {
  const words = claim.toLowerCase().split(/\s+/).filter(w => w.length > 3);
  if (words.length === 0) return textLower.includes(claim.toLowerCase());
  const hits = words.filter(w => textLower.includes(w)).length;
  return hits / words.length >= 0.6;
}

function ratioToScore(ratio: number): number {
  return Math.round(Math.max(1, Math.min(10, ratio * 9 + 1)));
}
