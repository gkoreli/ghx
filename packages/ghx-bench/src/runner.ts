/**
 * Generic agent runner for the ghx-bench eval engine.
 *
 * Uses acpx/runtime directly. deny-all permission mode ensures no write actions.
 * Computes CostMetrics, MainAgentBurden, and FollowupMetrics from the trace.
 */

import {
  createAcpRuntime,
  createFileSessionStore,
  createAgentRegistry,
} from 'acpx/runtime';
import { randomUUID } from 'node:crypto';
import { mkdirSync } from 'node:fs';
import { join } from 'node:path';
import { homedir } from 'node:os';
import type {
  Task,
  AgentDef,
  AgentTrace,
  RawTurn,
  CostMetrics,
  MainAgentBurden,
  FollowupMetrics,
} from './types.js';

const DEFAULT_ACP_STORE_DIR = join(homedir(), '.ghx-bench', 'acp-store');

export interface RunOptions {
  acpAgent: string;
  acpStoreDir?: string;
}

export async function runAgent(
  task: Task,
  agentDef: AgentDef,
  sessionKey: string,
  options: RunOptions,
): Promise<AgentTrace> {
  const acpStoreDir = options.acpStoreDir ?? DEFAULT_ACP_STORE_DIR;
  mkdirSync(acpStoreDir, { recursive: true });

  const runtime = createAcpRuntime({
    cwd: process.cwd(),
    sessionStore: createFileSessionStore({ stateDir: acpStoreDir }),
    agentRegistry: createAgentRegistry(),
    permissionMode: 'deny-all',
    nonInteractivePermissions: 'deny',
  });

  const handle = await runtime.ensureSession({
    sessionKey,
    agent: options.acpAgent,
    mode: 'persistent',
  });

  const rawTurns: RawTurn[] = [];
  const start = Date.now();

  for (let i = 0; i < task.turns.length; i++) {
    const question = task.turns[i]!;
    const prompt = agentDef.buildPrompt(i, question, task, sessionKey);
    const turnStart = Date.now();
    const raw: RawTurn = { turn: i, prompt: question, text: '', toolCalls: [], durationMs: 0 };

    const turn = runtime.startTurn({
      handle,
      text: prompt,
      mode: 'prompt',
      requestId: randomUUID(),
    });

    for await (const event of turn.events) {
      if (event.type === 'text_delta') {
        process.stdout.write(event.text);
        raw.text += event.text;
      } else if (event.type === 'tool_call' && typeof event.text === 'string') {
        if (event.text !== 'tool call') {
          const statusTag = event.status !== undefined ? ` [${event.status}]` : '';
          process.stderr.write(`  ▶${statusTag} ${event.text}\n`);
          raw.toolCalls.push(event.text);
        }
      }
    }
    process.stdout.write('\n');

    const result = await turn.result;
    if (result.status === 'failed') {
      throw new Error(`ACP turn ${i} failed for ${agentDef.id}: ${result.error.message}`);
    }

    raw.durationMs = Date.now() - turnStart;
    rawTurns.push(raw);
  }

  const wallTimeMs = Date.now() - start;
  const output = agentDef.extractOutput(rawTurns);
  const costs = computeCosts(rawTurns, wallTimeMs);
  const mainAgentBurden = agentDef.computeBurden !== undefined
    ? agentDef.computeBurden(rawTurns)
    : defaultBurden(rawTurns);
  const followupMetrics = rawTurns.length >= 2 ? computeFollowup(rawTurns) : undefined;

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

// ── Cost computation ──────────────────────────────────────────────────────────

const READ_FILE_RE = /ghx read \S+ (\S+)/;

function extractReadFiles(toolCalls: string[]): string[] {
  return toolCalls.flatMap(tc => {
    const m = READ_FILE_RE.exec(tc);
    return m?.[1] !== undefined ? [m[1]] : [];
  });
}

function computeCosts(turns: RawTurn[], wallTimeMs: number): CostMetrics {
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

// ── Default burden (non-sidecar agents) ──────────────────────────────────────

/**
 * For agents where the main agent IS doing the research (ghxSkillAgent, plainAgent),
 * all exploration output counts as main-agent context — there is no sidecar boundary.
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

// ── Follow-up metrics ─────────────────────────────────────────────────────────

function computeFollowup(turns: RawTurn[]): FollowupMetrics {
  const turn0Files = new Set(extractReadFiles(turns[0]?.toolCalls ?? []));
  const laterFiles = new Set(
    turns.slice(1).flatMap(t => extractReadFiles(t.toolCalls)),
  );

  const repeatReadsAcrossTurns = [...laterFiles].filter(f => turn0Files.has(f)).length;
  const newFilesReadOnFollowup = [...laterFiles].filter(f => !turn0Files.has(f)).length;

  // Heuristic: do later turns reference specific paths/symbols found in turn 0?
  const turn0Findings = [...turn0Files, ...extractMentionedPaths(turns[0]?.text ?? '')];
  const laterText = turns.slice(1).map(t => t.text).join('\n').toLowerCase();
  const priorFindingsReferenced = turn0Findings.some(f =>
    f.length > 3 && laterText.includes(f.toLowerCase()),
  );

  return { repeatReadsAcrossTurns, newFilesReadOnFollowup, priorFindingsReferenced };
}

function extractMentionedPaths(text: string): string[] {
  const matches = text.match(/\b[\w./-]+\.(?:ts|tsx|js|go|py|rs|rb|java)\b/g) ?? [];
  return [...new Set(matches)].filter(f => f.includes('/'));
}
