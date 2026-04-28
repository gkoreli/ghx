/**
 * Trial result persistence.
 *
 * Each trial is saved as a JSON file under:
 *   <storeDir>/<experimentId>/<trialId>.json
 *
 * This lets you compare results across multiple runs, track regressions after
 * sidecar prompt changes, and compute multi-trial aggregate statistics.
 */

import { mkdirSync, writeFileSync, readdirSync, readFileSync } from 'node:fs';
import { join } from 'node:path';
import { homedir } from 'node:os';
import type { TrialResult } from './types.js';

const DEFAULT_STORE_DIR = join(homedir(), '.ghx-bench', 'results');

export function saveTrialResult(result: TrialResult, storeDir = DEFAULT_STORE_DIR): string {
  const dir = join(storeDir, result.experimentId);
  mkdirSync(dir, { recursive: true });
  const path = join(dir, `${result.id}.json`);
  writeFileSync(path, JSON.stringify(result, null, 2), 'utf8');
  return path;
}

export function loadTrialResults(experimentId: string, storeDir = DEFAULT_STORE_DIR): TrialResult[] {
  const dir = join(storeDir, experimentId);
  let files: string[];
  try {
    files = readdirSync(dir).filter(f => f.endsWith('.json'));
  } catch {
    return [];
  }
  return files
    .map(f => {
      try {
        return JSON.parse(readFileSync(join(dir, f), 'utf8')) as TrialResult;
      } catch {
        return undefined;
      }
    })
    .filter((r): r is TrialResult => r !== undefined)
    .sort((a, b) => a.timestamp.localeCompare(b.timestamp));
}
