/**
 * ACP runtime bridge for ghx-sidecar.
 *
 * Uses the acpx/runtime SDK directly instead of spawning CLI subprocesses.
 * This gives typed ACP events, proper persistent session management, and
 * no subprocess quoting/escaping issues.
 */

import {
  createAcpRuntime,
  createFileSessionStore,
  createAgentRegistry,
  isAcpRuntimeError,
} from 'acpx/runtime';
import type { AcpRuntimeEvent } from 'acpx/runtime';
import { randomUUID } from 'node:crypto';
import { join } from 'node:path';
import { homedir } from 'node:os';
import { mkdirSync } from 'node:fs';
import type { SidecarReport } from './schema.js';

const DEFAULT_ACP_STORE_DIR = join(homedir(), '.ghx-sidecar', 'acp-store');

/** Options for a single acpx send invocation. */
export interface AcpxOptions {
  /** acpx-registered agent name (e.g. `"codex"`, `"claude"`, `"kiro"`). */
  agent: string;
  /**
   * Named session identifier.
   * ghx-sidecar uses the `ghx-sidecar:<name>` convention.
   */
  session: string;
  /** Prompt text to send. */
  prompt: string;
  /**
   * Directory where the acpx runtime stores its own ACP session records.
   * Separate from the ghx-sidecar metadata sessions dir.
   * @default "~/.ghx-sidecar/acp-store"
   */
  acpStoreDir?: string;
  /** If true, close the event stream immediately after starting the turn. */
  noWait?: boolean;
  /** Optional hook called for every streamed ACP event. */
  onEvent?: (event: AcpRuntimeEvent) => void;
  /** Called with the parsed SidecarReport if the agent emits a <ghx-report> block. */
  onReport?: (report: SidecarReport) => void;
}

/**
 * Send a prompt to an ACP agent session using the acpx/runtime SDK.
 *
 * Streams text_delta events to stdout and tool_call events to stderr so the
 * engineer sees the agent's live output. The turn result is awaited unless
 * noWait is true.
 *
 * @throws {Error} if the ACP turn fails.
 */
export async function acpxSend(options: AcpxOptions): Promise<void> {
  const {
    agent,
    session,
    prompt,
    acpStoreDir = DEFAULT_ACP_STORE_DIR,
    noWait = false,
    onEvent,
    onReport,
  } = options;

  mkdirSync(acpStoreDir, { recursive: true });

  const runtime = createAcpRuntime({
    cwd: process.cwd(),
    sessionStore: createFileSessionStore({ stateDir: acpStoreDir }),
    agentRegistry: createAgentRegistry(),
    permissionMode: 'deny-all',
    nonInteractivePermissions: 'deny',
  });

  const handle = await runtime.ensureSession({
    sessionKey: session,
    agent,
    mode: 'persistent',
  });

  const turn = runtime.startTurn({
    handle,
    text: prompt,
    mode: 'prompt',
    requestId: randomUUID(),
  });

  if (noWait) {
    await turn.closeStream({ reason: 'no-wait' });
    return;
  }

  let fullText = '';
  for await (const event of turn.events) {
    if (event.type === 'text_delta') {
      process.stdout.write(event.text);
      fullText += event.text;
    } else if (event.type === 'tool_call') {
      const status = event.status !== undefined ? ` [${event.status}]` : '';
      process.stderr.write(`  ▶${status} ${event.text}\n`);
    }
    onEvent?.(event);
  }
  process.stdout.write('\n');

  const result = await turn.result;
  if (result.status === 'failed') {
    throw new Error(`acpx turn failed: ${result.error.message}`);
  }

  if (onReport !== undefined) {
    const report = extractReport(fullText);
    if (report !== undefined) onReport(report);
  }
}

const GHX_REPORT_RE = /<ghx-report>([\s\S]*?)<\/ghx-report>/;

function extractReport(text: string): SidecarReport | undefined {
  const match = GHX_REPORT_RE.exec(text);
  if (match?.[1] === undefined) return undefined;
  try {
    const obj = JSON.parse(match[1].trim()) as unknown;
    if (isValidReport(obj)) return obj;
  } catch { /* malformed JSON — ignore */ }
  return undefined;
}

function isValidReport(obj: unknown): obj is SidecarReport {
  if (typeof obj !== 'object' || obj === null) return false;
  const r = obj as Record<string, unknown>;
  return typeof r['answer'] === 'string' && Array.isArray(r['relevantFiles']);
}

/**
 * Probe whether the configured agent is available via the acpx runtime.
 * Returns a diagnostic message.
 */
export async function acpxDoctor(
  agent: string,
  acpStoreDir = DEFAULT_ACP_STORE_DIR,
): Promise<{ ok: boolean; message: string }> {
  try {
    mkdirSync(acpStoreDir, { recursive: true });
    const runtime = createAcpRuntime({
      cwd: process.cwd(),
      sessionStore: createFileSessionStore({ stateDir: acpStoreDir }),
      agentRegistry: createAgentRegistry(),
      permissionMode: 'deny-all',
      nonInteractivePermissions: 'deny',
    });
    const report = await runtime.doctor();
    return { ok: report.ok, message: report.message };
  } catch (err) {
    if (isAcpRuntimeError(err)) {
      return { ok: false, message: `${err.code}: ${err.message}` };
    }
    return { ok: false, message: err instanceof Error ? err.message : String(err) };
  }
}
