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

const GHX_REPORT_RE = /<ghx-report>[\s\S]*?<\/ghx-report>/;

function buildJudgePrompt(task: Task, anonymized: AnonymizedTrace[]): string {
  const agentBlocks = anonymized.map(({ label, trace }) => formatTrace(label, trace)).join('\n\n---\n\n');
  const labelList = anonymized.map(a => `"${a.label}"`).join(', ');

  return `\
You are an expert evaluator for AI coding assistant workflows performing GitHub repository
code exploration. This is a pure text reasoning task. Do NOT call any tools or run any commands.
Evaluate based solely on the execution traces provided below.
The agent labels (${labelList}) are anonymous — you do not know which system produced each trace.

## Architecture context — read before scoring

Two workflow architectures are being compared. Each trace is labeled with its architecture.

**SIDECAR WORKFLOW**: A cheap specialist model performs internal repo exploration (tool calls,
file reads, code searches). It compresses findings into a structured report artifact. The
expensive main coding agent only receives that compressed report — it never sees the internal
exploration trace. You will see the main-agent view (the report) and a summary of internal work.

Evaluate sidecar agents on: quality of the compressed report, evidence density in the report,
and how useful that artifact is for a coding agent to act on.

Do NOT penalize sidecar agents for internal tool call count — those calls run on a cheap model
that is invisible to the expensive main agent. DO penalize sidecar if:
- The final report is vague, missing evidence, or hard for a coding agent to act on
- Internal exploration was wildly wasteful without improving report quality
- The main agent received too much raw noise instead of structured findings
- Follow-up turns show no continuity (the sidecar re-researches what it already found)

**DIRECT WORKFLOW**: The expensive main model does all exploration itself. Every tool call,
file read, and reasoning step happens inside the main agent's context at full cost. You will
see the complete exploration transcript.

Evaluate direct agents on: efficiency of exploration, quality of the final answer, and whether
they avoided burning expensive main-agent context on irrelevant reads.

**What you are optimizing for**: Which workflow gives the expensive main coding agent the
cleanest, most useful, most evidence-grounded artifact for better coding judgment — at the
lowest main-agent context cost?

## Task

Repo: ${task.repo}
Questions (${task.turns.length} turns):
${task.turns.map((q, i) => `  ${i + 1}. ${q}`).join('\n')}

## What a correct answer looks like

${task.judgeContext}

## Agent traces

${agentBlocks}

## Scoring rubric

Score each agent on five dimensions (integer 1–10):

**answerCorrectness** — Is the final answer factually correct?
Are the key mechanisms, functions, and files correctly identified?
Is uncertainty acknowledged where appropriate? Does context carry across turns?

**evidenceQuality** — Are claims backed by actual code evidence?
Reward: specific file paths, line numbers, quoted function names, cited read output.
Penalize: confident assertions with no inspection trail, plausible-sounding but unverified claims,
answers that name the right file but don't explain the mechanism.

**reportUsefulness** — How actionable is what the main coding agent received?
For SIDECAR: is the compressed report structured, complete, and directly usable by a coding agent?
Does it surface the key facts without requiring the main agent to re-investigate?
For DIRECT: is the final response clear, well-organized, and grounded in actual findings?
Penalize for both: noise, over-qualification, missing critical files, unstructured dumps.

**explorationJudgment** — Were the right files identified and irrelevant ones skipped?
Did the agent find the key implementation files without unnecessary scope expansion?
Did follow-up turns build on prior findings rather than repeat them?
For SIDECAR: judge by what the report covers, not by internal tool call volume.
For DIRECT: also judge tool call efficiency since every read is expensive main-agent work.

**contextBurden** — How much did the main coding agent have to process?
For SIDECAR: reward a clean, compressed report that covers the answer without excess.
A dense, well-structured report that the main agent can act on directly scores highest.
For DIRECT: penalize large amounts of exploratory reasoning the main agent must filter through.
A direct agent that reached the answer efficiently with few reads scores higher than one that
read many files before finding the right one.

## Output

Respond with a single <eval-report> JSON block and nothing after the closing tag.
Use the exact agent labels: ${labelList}.

<eval-report>
{
  "scores": [
    {
      "agentId": "Agent 1",
      "dimensions": {
        "answerCorrectness": { "score": 8, "rationale": "one sentence" },
        "evidenceQuality":   { "score": 7, "rationale": "one sentence" },
        "reportUsefulness":  { "score": 9, "rationale": "one sentence" },
        "explorationJudgment": { "score": 8, "rationale": "one sentence" },
        "contextBurden":    { "score": 6, "rationale": "one sentence" }
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
  const isSidecar = trace.mainAgentBurden.sidecarInternalTraceCharsApprox > 0;
  return isSidecar ? formatSidecarTrace(label, trace) : formatDirectTrace(label, trace);
}

function formatSidecarTrace(label: string, trace: AgentTrace): string {
  const lines: string[] = [];
  const b = trace.mainAgentBurden;
  const internalKB = (b.sidecarInternalTraceCharsApprox / 1024).toFixed(1);
  const reportKB   = (b.finalReportCharsApprox / 1024).toFixed(1);
  const totalKB    = (b.totalWorkflowCharsApprox / 1024).toFixed(1);

  lines.push(`${label}  [SIDECAR WORKFLOW]  ${trace.costs.toolCallCount} internal tool calls, ${(trace.costs.wallTimeMs / 1000).toFixed(1)}s`);
  lines.push(`Cheap model: ${internalKB}KB internal exploration → ${reportKB}KB report  (${totalKB}KB total workflow)`);
  lines.push('');

  lines.push('── MAIN-AGENT VIEW (what the expensive main model received) ──');
  const fullText = trace.turns.map(t => t.text).join('\n');
  const reportMatch = GHX_REPORT_RE.exec(fullText);
  if (reportMatch !== null) {
    const block = reportMatch[0];
    const snippet = block.slice(0, 3000);
    lines.push(block.length > 3000 ? snippet + '\n…[report truncated]' : snippet);
  } else {
    lines.push('(no <ghx-report> found — sidecar boundary failed, main agent received raw output)');
    // Fall back: show final turn text so judge can still evaluate answer quality
    const last = trace.turns.at(-1);
    if (last !== undefined) {
      lines.push(last.text.slice(0, 1200));
    }
  }

  lines.push('');
  lines.push('── SIDECAR INTERNAL EXPLORATION (cheap model, NOT seen by main agent) ──');
  lines.push('Tool calls executed internally:');
  for (const turn of trace.turns) {
    if (turn.toolCalls.length > 0) {
      lines.push(`  Turn ${turn.turn + 1}:`);
      for (const tc of turn.toolCalls) lines.push(`    • ${tc}`);
    }
  }

  return lines.join('\n');
}

function formatDirectTrace(label: string, trace: AgentTrace): string {
  const lines: string[] = [];
  lines.push(`${label}  [DIRECT WORKFLOW]  ${trace.costs.toolCallCount} tool calls, ${(trace.costs.wallTimeMs / 1000).toFixed(1)}s`);
  lines.push('Expensive main model does all exploration directly — every tool call is main-agent context.');
  lines.push('');
  lines.push('── MAIN-AGENT VIEW (full exploration transcript) ──');

  for (const turn of trace.turns) {
    lines.push(`\nTurn ${turn.turn + 1} — ${turn.prompt}`);
    if (turn.toolCalls.length > 0) {
      lines.push('Tool calls:');
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
