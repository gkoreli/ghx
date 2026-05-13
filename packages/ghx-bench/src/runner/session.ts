/**
 * ACP session execution — creates a persistent agent session and runs turns.
 *
 * Each turn sends a prompt to the configured ACP agent (Codex, Claude, Kiro)
 * and captures the event stream: text deltas accumulate into the response text,
 * tool_call events are cleaned and stored as the command list.
 *
 * deny-all permission mode is enforced at the runtime layer so no write actions
 * can reach the filesystem, git, or any external service during evaluation.
 */

import { createAcpRuntime, createFileSessionStore, createAgentRegistry } from 'acpx/runtime';
import { randomUUID } from 'node:crypto';
import { mkdirSync } from 'node:fs';
import { cleanToolCall } from './tool-calls.js';
import type { RawTurn } from '../types.js';

export interface SessionOptions {
  acpAgent: string;
  acpStoreDir: string;
}

/**
 * One agent turn: the question stored for display and analysis, and the
 * full prompt actually sent to the ACP agent (may include persona on turn 0).
 */
export interface AgentTurn {
  question: string;
  prompt: string;
}

/**
 * Executes a sequence of turns in a persistent ACP session.
 *
 * Text delta events stream to stdout. Cleaned tool call commands stream to
 * stderr and are captured in the returned RawTurn data. A turn failure throws
 * immediately — the experiment orchestrator decides whether to retry or abort.
 */
export async function executeAgentTurns(
  sessionKey: string,
  turns: AgentTurn[],
  options: SessionOptions,
): Promise<RawTurn[]> {
  mkdirSync(options.acpStoreDir, { recursive: true });

  const runtime = createAcpRuntime({
    cwd: process.cwd(),
    sessionStore: createFileSessionStore({ stateDir: options.acpStoreDir }),
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

  for (let i = 0; i < turns.length; i++) {
    const { question, prompt } = turns[i]!;
    const turnStart = Date.now();
    const raw: RawTurn = { turn: i, prompt: question, text: '', toolCalls: [], durationMs: 0 };

    const turn = runtime.startTurn({ handle, text: prompt, mode: 'prompt', requestId: randomUUID() });

    for await (const event of turn.events) {
      if (event.type === 'text_delta') {
        process.stdout.write(event.text);
        raw.text += event.text;
      } else if (event.type === 'tool_call' && typeof event.text === 'string') {
        const cmd = cleanToolCall(event.text);
        if (cmd !== null) {
          process.stderr.write(`  ▶ ${cmd}\n`);
          raw.toolCalls.push(cmd);
        }
      }
    }
    process.stdout.write('\n');

    const result = await turn.result;
    if (result.status === 'failed') {
      throw new Error(`ACP turn ${i} failed for session "${sessionKey}": ${result.error.message}`);
    }

    raw.durationMs = Date.now() - turnStart;
    rawTurns.push(raw);
  }

  return rawTurns;
}
