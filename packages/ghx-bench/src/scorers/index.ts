/**
 * Scorer implementations.
 *
 * LLMJudgeScorer      — blind LLM evaluation: agents are anonymized and shuffled,
 *                       scored on five dimensions by a deny-all ACP judge session.
 *
 * DeterministicScorer — hard correctness checks: file recall, symbol recall,
 *                       claim coverage, bad-claim penalty, path discipline,
 *                       main-agent burden compression, follow-up continuity.
 */

export { LLMJudgeScorer } from './llm-judge.js';
export type { LLMJudgeScorerOptions } from './llm-judge.js';

export { DeterministicScorer } from './deterministic.js';
