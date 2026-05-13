/**
 * Agent runner — orchestrates ACP execution and trace metric computation.
 *
 * runAgent is the single entry point for running a task against an agent definition:
 *   1. Build per-turn prompts via AgentDef.buildPrompt
 *   2. Execute all turns through an ACP session (executeAgentTurns)
 *   3. Extract structured output via AgentDef.extractOutput
 *   4. Compute CostMetrics, MainAgentBurden, and FollowupMetrics from the raw turns
 */

import { join } from 'node:path';
import { homedir } from 'node:os';
import { executeAgentTurns } from './session.js';
import { computeCostMetrics, computeFollowupMetrics, defaultBurden } from './metrics.js';
import type { Task, AgentDef, AgentTrace } from '../types.js';

export { defaultBurden } from './metrics.js';

export interface RunOptions {
  acpAgent: string;
  acpStoreDir?: string;
}

const DEFAULT_ACP_STORE_DIR = join(homedir(), '.ghx-bench', 'acp-store');

/**
 * Runs a task through a single agent definition and returns the complete trace.
 *
 * The session key should be unique per (experiment, agent, task, trial) to
 * prevent cross-run context contamination in the ACP session store.
 */
export async function runAgent(
  task: Task,
  agentDef: AgentDef,
  sessionKey: string,
  options: RunOptions,
): Promise<AgentTrace> {
  const acpStoreDir = options.acpStoreDir ?? DEFAULT_ACP_STORE_DIR;

  const agentTurns = [...task.turns].map((question, i) => ({
    question,
    prompt: agentDef.buildPrompt(i, question, task, sessionKey),
  }));

  const start = Date.now();
  const rawTurns = await executeAgentTurns(sessionKey, agentTurns, {
    acpAgent: options.acpAgent,
    acpStoreDir,
  });
  const wallTimeMs = Date.now() - start;

  const output = agentDef.extractOutput(rawTurns);
  const costs = computeCostMetrics(rawTurns, wallTimeMs);
  const mainAgentBurden = agentDef.computeBurden !== undefined
    ? agentDef.computeBurden(rawTurns)
    : defaultBurden(rawTurns);
  const followupMetrics = rawTurns.length >= 2 ? computeFollowupMetrics(rawTurns) : undefined;

  const trace: AgentTrace = {
    agentId: agentDef.id,
    taskId: task.id,
    sessionKey,
    turns: rawTurns,
    costs,
    mainAgentBurden,
    durationMs: wallTimeMs,
    ...output,
  };
  if (followupMetrics !== undefined) trace.followupMetrics = followupMetrics;
  return trace;
}
