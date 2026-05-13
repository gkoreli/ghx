/**
 * Trace metric computation — derives structured metrics from raw turn data.
 *
 * Three metric families, each answering a distinct product question:
 *
 *   CostMetrics       "How much work did this agent do?"
 *                     Turns, tool calls, files read, output volume, wall time.
 *
 *   MainAgentBurden   "How much of that work landed in the expensive main agent's context?"
 *                     The sidecar thesis: internal exploration should stay cheap and internal;
 *                     only the compressed evidence report crosses the workflow boundary.
 *                     defaultBurden() is the baseline for non-sidecar agents where the
 *                     main agent IS the researcher and all output counts.
 *
 *   FollowupMetrics   "Did the agent carry session memory across turns?"
 *                     Re-reads indicate wasted work. Prior-finding references indicate
 *                     the agent is building on what it already knows rather than restarting.
 */

import type { RawTurn, CostMetrics, MainAgentBurden, FollowupMetrics } from '../types.js';
import { extractReadFiles } from './tool-calls.js';

/** Derives raw operational cost counts from completed turn data. */
export function computeCostMetrics(turns: RawTurn[], wallTimeMs: number): CostMetrics {
  const allToolCalls = turns.flatMap(t => t.toolCalls);
  const readFiles = extractReadFiles(allToolCalls);
  const fileCounts = new Map<string, number>();
  for (const f of readFiles) fileCounts.set(f, (fileCounts.get(f) ?? 0) + 1);

  return {
    turnCount: turns.length,
    toolCallCount: allToolCalls.length,
    uniqueFilesRead: fileCounts.size,
    duplicateFilesRead: [...fileCounts.values()].filter(c => c > 1).length,
    totalOutputChars: turns.reduce((sum, t) => sum + t.text.length, 0),
    wallTimeMs,
  };
}

/**
 * Default burden model for non-sidecar agents (ghxSkillAgent, plainAgent).
 *
 * When the main agent is doing the research directly, every exploration turn
 * counts as main-agent context — there is no sidecar boundary to absorb it.
 * sidecarAgent overrides this via AgentDef.computeBurden to report only the
 * compressed report size as the main-agent context cost.
 */
export function defaultBurden(turns: RawTurn[]): MainAgentBurden {
  const totalChars = turns.reduce((sum, t) => sum + t.text.length, 0);
  const lastChars = turns.at(-1)?.text.length ?? 0;
  return {
    mainAgentContextCharsApprox: totalChars,
    mainAgentRepoEvidenceCharsApprox: totalChars,
    sidecarInternalTraceCharsApprox: 0,
    finalReportCharsApprox: lastChars,
    totalWorkflowCharsApprox: totalChars,
  };
}

/**
 * Computes follow-up continuity metrics for tasks with 2+ turns.
 *
 * Repeat reads across turns indicate the agent is re-discovering what it already
 * found rather than building on prior session state. Prior-finding references
 * indicate the opposite: the agent is using accumulated context efficiently.
 */
export function computeFollowupMetrics(turns: RawTurn[]): FollowupMetrics {
  const turn0Files = new Set(extractReadFiles(turns[0]?.toolCalls ?? []));
  const laterFiles = new Set(turns.slice(1).flatMap(t => extractReadFiles(t.toolCalls)));

  const repeatReadsAcrossTurns = [...laterFiles].filter(f => turn0Files.has(f)).length;
  const newFilesReadOnFollowup = [...laterFiles].filter(f => !turn0Files.has(f)).length;

  // Heuristic: did later turns reference specific paths or symbols found in turn 0?
  const turn0Findings = [...turn0Files, ...extractMentionedPaths(turns[0]?.text ?? '')];
  const laterText = turns.slice(1).map(t => t.text).join('\n').toLowerCase();
  const priorFindingsReferenced = turn0Findings.some(
    f => f.length > 3 && laterText.includes(f.toLowerCase()),
  );

  return { repeatReadsAcrossTurns, newFilesReadOnFollowup, priorFindingsReferenced };
}

/** Extracts file paths mentioned in agent output text (for prior-finding detection). */
function extractMentionedPaths(text: string): string[] {
  const matches = text.match(/\b[\w./-]+\.(?:ts|tsx|js|go|py|rs|rb|java)\b/g) ?? [];
  return [...new Set(matches)].filter(f => f.includes('/'));
}
