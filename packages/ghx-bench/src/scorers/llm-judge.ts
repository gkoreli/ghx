/**
 * LLM-as-judge scorer — blind evaluation.
 *
 * Agent identities are anonymized ("Agent 1", "Agent 2", ...) and their order
 * is shuffled per trial before the judge sees them. This prevents the judge
 * from rewarding "sidecarAgent" because the label sounds more structured.
 *
 * The judge scores each agent on three dimensions:
 *   reasoning   — answer correctness and evidence quality
 *   judgment    — file selection and scope discipline
 *   trajectory  — tool call efficiency and path quality
 *
 * deny-all permission mode prevents the judge from calling ghx tools to verify
 * claims — it must evaluate based solely on the provided traces.
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
  Scorer,
  Task,
  AgentTrace,
  ScorerResult,
  AgentScore,
  DimensionScore,
} from '../types.js';

const DEFAULT_ACP_STORE_DIR = join(homedir(), '.ghx-bench', 'acp-store');

// ── Anonymization ─────────────────────────────────────────────────────────────

interface AnonymizedTrace {
  label: string;
  trace: AgentTrace;
}

function anonymize(traces: Record<string, AgentTrace>): {
  anonymized: AnonymizedTrace[];
  labelToId: Record<string, string>;
} {
  const entries = Object.entries(traces);
  // Fisher-Yates shuffle for random ordering
  for (let i = entries.length - 1; i > 0; i--) {
    const j = Math.floor(Math.random() * (i + 1));
    const tmp = entries[i]!;
    entries[i] = entries[j]!;
    entries[j] = tmp;
  }
  const labelToId: Record<string, string> = {};
  const anonymized: AnonymizedTrace[] = entries.map(([agentId, trace], idx) => {
    const label = `Agent ${idx + 1}`;
    labelToId[label] = agentId;
    return { label, trace };
  });
  return { anonymized, labelToId };
}

// ── Prompt factory ────────────────────────────────────────────────────────────

function buildJudgePrompt(task: Task, anonymized: AnonymizedTrace[]): string {
  const agentBlocks = anonymized.map(({ label, trace }) => formatTrace(label, trace)).join('\n\n---\n\n');
  const labelList = anonymized.map(a => `"${a.label}"`).join(', ');

  return `\
You are an expert evaluator for AI agents performing GitHub repository code exploration.
This is a pure text reasoning task. Do NOT call any tools or run any commands.
Evaluate based solely on the execution traces provided below.
The agent labels (${labelList}) are anonymous — you do not know which system produced each trace.

## Task

Repo: ${task.repo}
Questions (${task.turns.length} turns):
${task.turns.map((q, i) => `  ${i + 1}. ${q}`).join('\n')}

## What a correct answer looks like

${task.judgeContext}

## Agent traces

${agentBlocks}

## Scoring rubric

Score each agent on three dimensions (integer 1–10):

**reasoning** — Does the final answer correctly explain the implementation?
Are claims backed by evidence from actual code reads (cited line-level observations,
quoted function names, file contents), not plausible-sounding inference?
Is uncertainty acknowledged? Does context carry across turns?

**judgment** — Did the agent make good decisions about what to read and what to skip?
Avoid over-reading (tests, examples, config, unrelated paths). Identify the most
relevant files without hints. Stop when enough evidence is gathered.

**trajectory** — Did the agent follow an efficient path?
Reward: search before read, narrow before expand, find key file within few commands.
Penalize: --help calls when not needed, failed queries requiring retry, reading
irrelevant files, redundant commands, finding the right file only after many wrong attempts.

**evidenceCoverage** — Are major claims backed by file/line/command evidence?
Reward: specific file + line citations, quoted function names, commands that verify claims.
Penalize: confident assertions with no inspection trail, answers that sound plausible
but cite no specific code location or output.

**uncertaintyHonesty** — Does the agent correctly acknowledge what it did NOT inspect?
Reward: explicit list of uninspected areas (tests not checked, history not reviewed,
only remote backend used, etc.), and next-step recommendations.
Penalize: overclaiming certainty, no uncertainty section, suggesting the investigation
is complete when it obviously is not.

## Output

Respond with a single <eval-report> JSON block and nothing after the closing tag.
Use the exact agent labels: ${labelList}.

<eval-report>
{
  "scores": [
    {
      "agentId": "Agent 1",
      "dimensions": {
        "reasoning":        { "score": 8, "rationale": "one sentence" },
        "judgment":         { "score": 7, "rationale": "one sentence" },
        "trajectory":       { "score": 9, "rationale": "one sentence" },
        "evidenceCoverage": { "score": 8, "rationale": "one sentence" },
        "uncertaintyHonesty": { "score": 6, "rationale": "one sentence" }
      },
      "overall": 7.6
    }
  ],
  "verdict": "Agent 1",
  "summary": "one paragraph explaining the verdict"
}
</eval-report>
`;
}

function formatTrace(label: string, trace: AgentTrace): string {
  const lines: string[] = [`${label}  (${trace.costs.toolCallCount} tool calls, ${(trace.costs.wallTimeMs / 1000).toFixed(1)}s)`];

  for (const turn of trace.turns) {
    lines.push(`\nTurn ${turn.turn + 1} — ${turn.prompt}`);
    if (turn.toolCalls.length > 0) {
      lines.push(`Tool calls:`);
      for (const tc of turn.toolCalls) lines.push(`  • ${tc}`);
    }
    const snippet = turn.text.slice(0, 1200);
    const tail = turn.text.length > 1200 ? ' …[truncated]' : '';
    lines.push(`Response: ${snippet}${tail}`);
  }

  lines.push(`\nFiles identified: ${trace.filesFound.join(', ') || '(none)'}`);
  lines.push(`Final answer: ${trace.finalAnswer.slice(0, 600)}`);
  return lines.join('\n');
}

// ── Parser ────────────────────────────────────────────────────────────────────

const EVAL_REPORT_RE = /<eval-report>([\s\S]*?)<\/eval-report>/;

interface RawScore {
  agentId: string;
  dimensions: Record<string, { score: number; rationale: string }>;
  overall: number;
}

interface RawReport {
  scores: RawScore[];
  verdict: string;
  summary: string;
}

function parseEvalReport(text: string): RawReport | undefined {
  const match = EVAL_REPORT_RE.exec(text);
  if (match?.[1] === undefined) return undefined;
  try {
    const obj = JSON.parse(match[1].trim()) as Record<string, unknown>;
    if (Array.isArray(obj['scores']) && typeof obj['verdict'] === 'string' && typeof obj['summary'] === 'string') {
      return obj as unknown as RawReport;
    }
  } catch { /* malformed */ }
  return undefined;
}

// ── Scorer ────────────────────────────────────────────────────────────────────

export interface LLMJudgeScorerOptions {
  acpAgent: string;
  acpStoreDir?: string;
}

export class LLMJudgeScorer implements Scorer {
  readonly id = 'llm-judge';

  constructor(private readonly opts: LLMJudgeScorerOptions) {}

  async score(task: Task, traces: Record<string, AgentTrace>): Promise<ScorerResult> {
    const acpStoreDir = this.opts.acpStoreDir ?? DEFAULT_ACP_STORE_DIR;
    mkdirSync(acpStoreDir, { recursive: true });

    const { anonymized, labelToId } = anonymize(traces);

    const runtime = createAcpRuntime({
      cwd: process.cwd(),
      sessionStore: createFileSessionStore({ stateDir: acpStoreDir }),
      agentRegistry: createAgentRegistry(),
      permissionMode: 'deny-all',
      nonInteractivePermissions: 'deny',
    });

    const sessionKey = `bench:judge:${task.id}:${Date.now().toString(36)}`;
    const handle = await runtime.ensureSession({
      sessionKey,
      agent: this.opts.acpAgent,
      mode: 'persistent',
    });

    const prompt = buildJudgePrompt(task, anonymized);
    process.stdout.write('\n[ Judge — blind ]\n\n');

    let fullText = '';
    const turn = runtime.startTurn({
      handle,
      text: prompt,
      mode: 'prompt',
      requestId: randomUUID(),
    });

    for await (const event of turn.events) {
      if (event.type === 'text_delta') {
        process.stdout.write(event.text);
        fullText += event.text;
      }
    }
    process.stdout.write('\n');
    await turn.result;

    const raw = parseEvalReport(fullText);
    if (raw === undefined) {
      return { scorerId: this.id, scores: [], verdict: 'unknown', summary: 'Judge did not produce a valid <eval-report>.' };
    }

    // Re-map anonymous labels back to real agentIds
    const scores: AgentScore[] = raw.scores.map(s => ({
      agentId: labelToId[s.agentId] ?? s.agentId,
      dimensions: Object.fromEntries(
        Object.entries(s.dimensions).map(([k, v]) => [k, { score: v.score, rationale: v.rationale } satisfies DimensionScore]),
      ),
      overall: s.overall,
    }));

    const verdictId = labelToId[raw.verdict] ?? raw.verdict;

    return { scorerId: this.id, scores, verdict: verdictId, summary: raw.summary };
  }
}
