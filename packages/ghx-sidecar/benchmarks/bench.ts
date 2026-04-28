#!/usr/bin/env node
/**
 * ghx-sidecar A/B/C benchmark.
 *
 * Three agents, two scorers, LLM judge is blind (randomized labels).
 *
 *   sidecar   — full ghx-sidecar persona + evidence schema
 *   ghxSkill  — knows ghx commands, no sidecar structure (key baseline)
 *   plain     — no augmentation
 *
 * Usage (from packages/ghx-sidecar/):
 *   npx tsx benchmarks/bench.ts [task-id] [--trials N]
 *
 * Tasks: hono-middleware, openai-streaming, express-routing
 */

import { runExperiment, LLMJudgeScorer, DeterministicScorer } from '@gkoreli/ghx-bench';
import { loadConfig } from '../src/core/config.js';
import { sidecarAgent, ghxSkillAgent, plainAgent } from './agents.js';
import { findTask, TASKS } from './tasks.js';

async function main(): Promise<void> {
  const config = loadConfig();

  const taskId = process.argv.find(
    a => !a.startsWith('--') && !a.includes('bench.ts') && !a.includes('tsx') && !a.includes('node'),
  );
  const trialsIdx = process.argv.indexOf('--trials');
  const trials = trialsIdx !== -1 ? parseInt(process.argv[trialsIdx + 1] ?? '1', 10) : 1;

  const task = findTask(taskId);

  process.stdout.write(`Tasks available: ${TASKS.map(t => t.id).join(', ')}\n`);
  process.stdout.write(`Running: ${task.id}  trials: ${trials}  agent: ${config.agent}\n\n`);

  await runExperiment({
    id: 'ghx-sidecar-abc',
    tasks: [task],
    agents: [sidecarAgent, ghxSkillAgent, plainAgent],
    scorers: [
      new LLMJudgeScorer({ acpAgent: config.agent }),
      new DeterministicScorer(),
    ],
    trials,
    acpAgent: config.agent,
  });
}

main().catch(err => {
  process.stderr.write(`bench: ${err instanceof Error ? err.message : String(err)}\n`);
  process.exit(1);
});
