/**
 * AgentDef implementations for the ghx-sidecar A/B/C benchmark.
 *
 * sidecarAgent  — full sidecar persona + ghx grounding + evidence schema.
 *                 computeBurden() models the workflow boundary correctly:
 *                 main agent only receives the <ghx-report> block.
 *
 * ghxSkillAgent — ghx command knowledge injected on turn 0, no sidecar
 *                 structure. The main agent IS doing the research.
 *                 computeBurden() defaults to: all output = main-agent context.
 *
 * plainAgent    — no augmentation. Baseline.
 */

import { createSidecarPrompt } from '../src/core/prompt.js';
import { defaultBurden } from '@gkoreli/ghx-bench';
import type { AgentDef, Task, RawTurn, OutputSummary, MainAgentBurden } from '@gkoreli/ghx-bench';

// ── Shared helpers ────────────────────────────────────────────────────────────

const FILE_PATH_RE = /\b[\w./-]+\.(?:ts|tsx|js|go|py|rs|rb|java|c|cpp|h)\b/g;

function extractFilePaths(text: string): string[] {
  const matches = text.match(FILE_PATH_RE) ?? [];
  return [...new Set(matches)].filter(f => f.includes('/') && !f.startsWith('.'));
}

function textOutput(turns: RawTurn[]): OutputSummary {
  return {
    filesFound: extractFilePaths(turns.map(t => t.text).join('\n')),
    commandsRun: turns.flatMap(t => t.toolCalls),
    finalAnswer: turns.at(-1)?.text.slice(0, 800) ?? '',
  };
}

// ── Sidecar agent ─────────────────────────────────────────────────────────────

const GHX_REPORT_RE = /<ghx-report>([\s\S]*?)<\/ghx-report>/;

export const sidecarAgent: AgentDef = {
  id: 'sidecar',
  label: 'WITH sidecar (ghx-grounded + evidence schema)',

  buildPrompt(turn: number, question: string, task: Task, sessionKey: string): string {
    if (turn === 0) {
      const sys = createSidecarPrompt({
        session: sessionKey,
        repo: task.repo,
        question,
        depth: 'normal',
        output: 'evidence',
        allowedBackends: ['remote'],
      });
      return `${sys}\n\n---\n\nQuestion: ${question}`;
    }
    return question;
  },

  extractOutput(turns: RawTurn[]): OutputSummary {
    const fullText = turns.map(t => t.text).join('\n');
    const match = GHX_REPORT_RE.exec(fullText);
    if (match?.[1] !== undefined) {
      try {
        const obj = JSON.parse(match[1].trim()) as Record<string, unknown>;
        const filesFound = Array.isArray(obj['relevantFiles'])
          ? (obj['relevantFiles'] as Array<{ path: string }>).map(f => f.path)
          : [];
        const commandsRun = Array.isArray(obj['commandsRun'])
          ? (obj['commandsRun'] as string[])
          : turns.flatMap(t => t.toolCalls);
        const finalAnswer = typeof obj['answer'] === 'string' ? obj['answer'] : '';
        return { filesFound, commandsRun, finalAnswer };
      } catch { /* fall through */ }
    }
    return textOutput(turns);
  },

  /**
   * Sidecar workflow boundary: the main agent only sees the compressed
   * <ghx-report> block. All internal exploration is sidecar-internal.
   *
   * This is the architectural distinction vs ghxSkillAgent, where the main
   * agent processes all exploration output directly.
   */
  computeBurden(turns: RawTurn[]): MainAgentBurden {
    const fullText = turns.map(t => t.text).join('\n');
    const totalChars = turns.reduce((sum, t) => sum + t.text.length, 0);

    const match = GHX_REPORT_RE.exec(fullText);
    const reportChars = match?.[0].length ?? 0;

    // No report means the sidecar boundary failed — the main agent received no
    // compressed artifact. Fall back to default (all output = main-agent context)
    // rather than reporting 0, which would falsely look like perfect compression.
    if (reportChars === 0) {
      return defaultBurden(turns);
    }

    return {
      mainAgentContextCharsApprox: reportChars,
      mainAgentRepoEvidenceCharsApprox: reportChars,
      sidecarInternalTraceCharsApprox: Math.max(0, totalChars - reportChars),
      finalReportCharsApprox: reportChars,
      totalWorkflowCharsApprox: totalChars,
    };
  },
};

// ── ghx skill agent ───────────────────────────────────────────────────────────
//
// The decisive baseline: if sidecar beats ghxSkillAgent, the structured
// persona + evidence schema + workflow boundary is doing real work beyond
// just knowing ghx syntax.

const GHX_SKILL_PROMPT = `\
You are a code exploration agent with access to the ghx CLI for GitHub repo investigation.

## ghx command reference

\`\`\`bash
ghx explore <owner/repo>                    # Branch + tree + README in 1 API call
ghx read <owner/repo> <f1> [f2] [f3]       # Read 1-10 files in 1 API call
ghx read <owner/repo> "src/**/*.ts" --map   # Glob + parser-backed structural map
ghx read <owner/repo> --map <f1>            # Signatures only (~92% token reduction)
ghx read <owner/repo> --grep "pat" <f>      # Read file, show only matching lines
ghx read <owner/repo> --lines 42-80 <f>     # Read specific line range
ghx search "<query>"                        # Code search (AND matching)
ghx search --full "<query>"                 # Code search without line truncation
ghx tree <owner/repo> [path] --depth N      # Tree limited to N levels
\`\`\`

## Exploration discipline

1. **Start with search or direct read** if you know what to look for.
   Use \`ghx explore\` for orientation on an unfamiliar repo.
2. **Map before reading** — \`--map\` shows signatures of many files cheaply.
   Only read full files when you need the implementation detail.
3. **Search syntax** — every word is AND'd. Use \`repo:\` to scope.
   Do NOT use \`OR\`, \`NOT\`, \`symbol:\` — web-only, silently wrong.
4. **Batch reads** — \`ghx read repo f1 f2 f3\` is 1 API call.
5. **Avoid over-reading** — use \`--grep\` or \`--lines\` before reading entire files.
6. **Do not read test files** unless the question is specifically about tests.

## Anti-patterns to avoid

- Reading large files in full when \`--grep\` would find the relevant lines
- Using web-only search qualifiers (\`OR\`, \`NOT\`, \`symbol:\`)
- Multiple sequential reads of the same file
- Reading examples/, docs/, or test/ when answering implementation questions
`;

export const ghxSkillAgent: AgentDef = {
  id: 'ghxSkill',
  label: 'ghx skill (knows tool, no sidecar structure)',

  buildPrompt(turn: number, question: string, task: Task): string {
    if (turn === 0) {
      return `${GHX_SKILL_PROMPT}\n## Task\n\nRepo: ${task.repo}\n\nQuestion: ${question}`;
    }
    return question;
  },

  extractOutput: textOutput,

  // No computeBurden — defaults to: all exploration output = main-agent context.
  // This is correct: ghxSkillAgent IS the main agent doing research directly.
};

// ── Plain agent ───────────────────────────────────────────────────────────────

export const plainAgent: AgentDef = {
  id: 'plain',
  label: 'plain (no augmentation)',

  buildPrompt(turn: number, question: string, task: Task): string {
    return turn === 0 ? `Repo: ${task.repo}\n\n${question}` : question;
  },

  extractOutput: textOutput,

  // No computeBurden — same as ghxSkillAgent: all output = main-agent context.
};

// Re-export defaultBurden for reference in tests
export { defaultBurden };
