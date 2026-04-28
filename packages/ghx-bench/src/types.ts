/**
 * Core types for the ghx-bench agentic eval engine.
 *
 * Separation of concerns:
 *   Task      — what to evaluate (scenario, judge context, deterministic checks)
 *   AgentDef  — how to run an agent (prompt builder, output extractor, burden model)
 *   Scorer    — how to evaluate traces (LLM judge, deterministic, burden)
 *   RunResult — what was captured (traces, burden, follow-up metrics, scores)
 */

// ── Task ──────────────────────────────────────────────────────────────────────

/** Deterministic correctness checks. Used by DeterministicScorer. */
export interface TaskChecks {
  /** File paths (basename match) the correct answer must identify. */
  expectedFiles?: string[];
  /** Symbol names (function/type/class) that must appear in the answer. */
  expectedSymbols?: string[];
  /** Concepts/keywords that must appear in the final answer. */
  expectedConcepts?: string[];
  /**
   * Claims that must be present (substring match against all output text).
   * Tests mechanism understanding, not just filename mention.
   */
  requiredClaims?: string[];
  /**
   * Claims that must NOT be present — hallucinated mechanisms or wrong files.
   * Penalizes confident wrong answers.
   */
  unacceptableClaims?: string[];
  /**
   * Path substrings the agent should NOT read (e.g. "test/" when question
   * is not about tests). Penalizes unnecessary scope expansion.
   */
  avoidPaths?: string[];
}

/** A multi-turn evaluation scenario. Pure data — no logic. */
export interface Task {
  id: string;
  repo: string;
  turns: readonly string[];
  /** Reference answer description — visible ONLY to the judge. */
  judgeContext: string;
  checks?: TaskChecks;
  tags?: string[];
}

// ── Agent ─────────────────────────────────────────────────────────────────────

export interface RawTurn {
  turn: number;
  prompt: string;
  text: string;
  toolCalls: string[];
  durationMs: number;
}

export interface OutputSummary {
  filesFound: string[];
  commandsRun: string[];
  finalAnswer: string;
}

/**
 * Workflow-boundary metric: how much repo-research material entered the
 * main coding agent's context?
 *
 * For `sidecarAgent`: the main agent only receives the compressed evidence
 * report — all internal exploration is sidecar-internal.
 *
 * For `ghxSkillAgent` / `plainAgent`: the main agent IS doing the research,
 * so all exploration output counts as main-agent context.
 *
 * This distinction is the core architectural thesis:
 *   sidecar may spend more total chars internally and still win
 *   because the main agent's context stays clean.
 */
export interface MainAgentBurden {
  /**
   * Characters the main coding agent had to process.
   * sidecar: report block only. others: all exploration output.
   */
  mainAgentContextCharsApprox: number;
  /**
   * Subset of main-agent context that is raw repo evidence
   * (file contents, search results, tool outputs).
   */
  mainAgentRepoEvidenceCharsApprox: number;
  /**
   * Sidecar's internal scratchpad (exploration trace, tool outputs) that
   * was NOT forwarded to the main agent.
   * Zero for non-sidecar agents.
   */
  sidecarInternalTraceCharsApprox: number;
  /** The final structured artifact handed to the main agent. */
  finalReportCharsApprox: number;
  /** Total chars across sidecar internal + main agent context. */
  totalWorkflowCharsApprox: number;
}

/**
 * Follow-up continuity metrics: does the agent reuse prior session state
 * or re-research from scratch on Q2/Q3?
 */
export interface FollowupMetrics {
  /** Files read in turn 1+ that were also read in turn 0 (re-read penalty). */
  repeatReadsAcrossTurns: number;
  /** Files first encountered in turn 1+ (new coverage). */
  newFilesReadOnFollowup: number;
  /**
   * Did later turns reference specific findings (file paths, symbol names)
   * from earlier turns? Indicates session memory reuse.
   */
  priorFindingsReferenced: boolean;
}

export interface CostMetrics {
  turnCount: number;
  toolCallCount: number;
  uniqueFilesRead: number;
  duplicateFilesRead: number;
  totalOutputChars: number;
  wallTimeMs: number;
}

/**
 * Declarative agent definition.
 *
 * Encapsulates all agent-specific behavior: prompt construction, output
 * interpretation, and how to model the workflow boundary (mainAgentBurden).
 *
 * The engine has no knowledge of sidecar prompts, ghx-report parsing, or
 * burden computation — that lives here.
 */
export interface AgentDef {
  id: string;
  label: string;
  buildPrompt(turn: number, question: string, task: Task, sessionKey: string): string;
  extractOutput(turns: RawTurn[]): OutputSummary;
  /**
   * Compute the main-agent burden model for this workflow.
   *
   * If undefined, the runner applies the default (all output chars enter
   * main-agent context), which is correct for non-sidecar agents.
   */
  computeBurden?(turns: RawTurn[]): MainAgentBurden;
}

// ── Trace ─────────────────────────────────────────────────────────────────────

export interface AgentTrace extends OutputSummary {
  agentId: string;
  taskId: string;
  sessionKey: string;
  turns: RawTurn[];
  costs: CostMetrics;
  /** Workflow-boundary metric — always present. */
  mainAgentBurden: MainAgentBurden;
  /** Only present for tasks with 2+ turns. */
  followupMetrics?: FollowupMetrics;
  durationMs: number;
}

// ── Scorer ────────────────────────────────────────────────────────────────────

export interface DimensionScore {
  score: number;
  rationale: string;
}

export interface AgentScore {
  agentId: string;
  dimensions: Record<string, DimensionScore>;
  overall: number;
}

export interface ScorerResult {
  scorerId: string;
  scores: AgentScore[];
  verdict: string;
  summary: string;
}

export interface Scorer {
  id: string;
  score(task: Task, traces: Record<string, AgentTrace>): Promise<ScorerResult>;
}

// ── Trial and Experiment ──────────────────────────────────────────────────────

export interface TrialResult {
  id: string;
  experimentId: string;
  taskId: string;
  trialIndex: number;
  timestamp: string;
  traces: Record<string, AgentTrace>;
  scores: ScorerResult[];
  durationMs: number;
}

export interface ExperimentConfig {
  id: string;
  tasks: Task[];
  agents: AgentDef[];
  scorers: Scorer[];
  trials?: number;
  acpAgent: string;
  acpStoreDir?: string;
  storeDir?: string;
}

export interface AggregateScore {
  wins: number;
  trials: number;
  winRate: number;
  meanOverall: number;
  byDimension: Record<string, { mean: number; stddev: number }>;
  meanCosts: Partial<CostMetrics>;
  meanBurden: Partial<MainAgentBurden>;
}

export interface ExperimentResult {
  experimentId: string;
  taskIds: string[];
  agentIds: string[];
  scorerIds: string[];
  trials: TrialResult[];
  aggregate: Record<string, AggregateScore>;
}
