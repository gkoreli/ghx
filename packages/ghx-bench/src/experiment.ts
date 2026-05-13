/**
 * Experiment orchestrator.
 *
 * For each task × each trial:
 *   1. Run all agents in sequence, capturing full traces.
 *   2. Pass all traces to each scorer.
 *   3. Persist the TrialResult to disk.
 *
 * Agents run sequentially (not in parallel) to avoid ACP session contention
 * and to keep stdout output readable during a run.
 */

import { join } from 'node:path';
import { homedir } from 'node:os';
import { runAgent } from './runner/index.js';
import { saveTrialResult } from './store.js';
import { printTrialResult, printExperimentSummary } from './report.js';
import type {
  ExperimentConfig,
  ExperimentResult,
  TrialResult,
  AgentTrace,
  AggregateScore,
  CostMetrics,
  MainAgentBurden,
  DimensionScore,
} from './types.js';

const DEFAULT_STORE_DIR = join(homedir(), '.ghx-bench', 'results');

export async function runExperiment(config: ExperimentConfig): Promise<ExperimentResult> {
  const trials = config.trials ?? 1;
  const storeDir = config.storeDir ?? DEFAULT_STORE_DIR;
  const acpStoreDir = config.acpStoreDir ?? join(homedir(), '.ghx-bench', 'acp-store');
  const allTrials: TrialResult[] = [];

  const runOpts = { acpAgent: config.acpAgent, acpStoreDir };

  for (const task of config.tasks) {
    for (let t = 0; t < trials; t++) {
      const trialStart = Date.now();
      const runId = Date.now().toString(36);
      const trialId = `${config.id}-${task.id}-${t}-${runId}`;

      process.stdout.write(`\n${'═'.repeat(62)}\n`);
      process.stdout.write(`Experiment: ${config.id}  Task: ${task.id}  Trial: ${t + 1}/${trials}\n`);
      process.stdout.write(`${'═'.repeat(62)}\n`);

      const traces: Record<string, AgentTrace> = {};

      for (const agentDef of config.agents) {
        const sessionKey = `bench:${config.id}:${agentDef.id}:${task.id}:${t}:${runId}`;
        process.stdout.write(`\n[ ${agentDef.label} ]\n\n`);
        traces[agentDef.id] = await runAgent(task, agentDef, sessionKey, runOpts);
      }

      const scorerResults = await Promise.all(
        config.scorers.map(scorer => scorer.score(task, traces)),
      );

      const trial: TrialResult = {
        id: trialId,
        experimentId: config.id,
        taskId: task.id,
        trialIndex: t,
        timestamp: new Date().toISOString(),
        traces,
        scores: scorerResults,
        durationMs: Date.now() - trialStart,
      };

      printTrialResult(trial);
      saveTrialResult(trial, storeDir);
      allTrials.push(trial);
    }
  }

  const result: ExperimentResult = {
    experimentId: config.id,
    taskIds: config.tasks.map(t => t.id),
    agentIds: config.agents.map(a => a.id),
    scorerIds: config.scorers.map(s => s.id),
    trials: allTrials,
    aggregate: computeAggregate(config.agents.map(a => a.id), allTrials),
  };

  printExperimentSummary(result);
  return result;
}

function computeAggregate(agentIds: string[], trials: TrialResult[]): Record<string, AggregateScore> {
  const agg: Record<string, AggregateScore> = {};

  for (const agentId of agentIds) {
    const llmScores = trials.flatMap(t =>
      t.scores
        .filter(s => s.scorerId === 'llm-judge')
        .flatMap(s => s.scores.filter(a => a.agentId === agentId)),
    );

    const wins = trials.filter(t =>
      t.scores.some(s => s.scorerId === 'llm-judge' && s.verdict === agentId),
    ).length;

    const overalls = llmScores.map(s => s.overall);
    const meanOverall = mean(overalls);

    const dimNames = llmScores.length > 0
      ? Object.keys(llmScores[0]!.dimensions)
      : [];

    const byDimension: Record<string, { mean: number; stddev: number }> = {};
    for (const dim of dimNames) {
      const vals = llmScores.map(s => {
        const d = s.dimensions[dim] as DimensionScore | undefined;
        return d?.score ?? 0;
      });
      byDimension[dim] = { mean: mean(vals), stddev: stddev(vals) };
    }

    const costTraces = trials
      .map(t => t.traces[agentId])
      .filter((t): t is AgentTrace => t !== undefined);

    agg[agentId] = {
      wins,
      trials: trials.length,
      winRate: trials.length > 0 ? wins / trials.length : 0,
      meanOverall,
      byDimension,
      meanCosts: meanCosts(costTraces),
      meanBurden: meanBurdens(costTraces),
    };
  }

  return agg;
}

function meanBurdens(traces: AgentTrace[]): Partial<MainAgentBurden> {
  if (traces.length === 0) return {};
  const keys: (keyof MainAgentBurden)[] = [
    'mainAgentContextCharsApprox',
    'mainAgentRepoEvidenceCharsApprox',
    'sidecarInternalTraceCharsApprox',
    'finalReportCharsApprox',
    'totalWorkflowCharsApprox',
  ];
  const result: Partial<MainAgentBurden> = {};
  for (const k of keys) {
    (result as Record<string, number>)[k] = mean(traces.map(t => t.mainAgentBurden[k]));
  }
  return result;
}

function meanCosts(traces: AgentTrace[]): Partial<CostMetrics> {
  if (traces.length === 0) return {};
  const keys: (keyof CostMetrics)[] = ['turnCount', 'toolCallCount', 'uniqueFilesRead', 'duplicateFilesRead', 'totalOutputChars', 'wallTimeMs'];
  const result: Partial<CostMetrics> = {};
  for (const k of keys) {
    (result as Record<string, number>)[k] = mean(traces.map(t => t.costs[k]));
  }
  return result;
}

function mean(vals: number[]): number {
  if (vals.length === 0) return 0;
  return vals.reduce((a, b) => a + b, 0) / vals.length;
}

function stddev(vals: number[]): number {
  if (vals.length < 2) return 0;
  const m = mean(vals);
  return Math.sqrt(vals.reduce((sum, v) => sum + (v - m) ** 2, 0) / vals.length);
}
