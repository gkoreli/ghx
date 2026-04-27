/**
 * Sidecar preflight / doctor checks.
 *
 * Run before accepting the first exploration request to surface auth,
 * network, and binary issues as clear diagnostics rather than confusing
 * mid-exploration failures.
 */

import { exec } from 'node:child_process';
import { promisify } from 'node:util';
/** Subset of config needed by preflight. */
export interface PreflightConfig {
  checkGhToken?: boolean;
  checkNetwork?: boolean;
  checkGhxBinary?: boolean;
  strict?: boolean;
}

const execAsync = promisify(exec);

/** Result of a single preflight check. */
export interface PreflightCheck {
  name: string;
  passed: boolean;
  message: string;
}

/** Aggregated result of all configured preflight checks. */
export interface PreflightResult {
  /** True only when every configured check passed. */
  passed: boolean;
  checks: PreflightCheck[];
}

async function checkGhToken(): Promise<PreflightCheck> {
  const token = process.env['GH_TOKEN'] ?? process.env['GITHUB_TOKEN'];
  if (token !== undefined && token.length > 0) {
    return { name: 'gh-token', passed: true, message: 'GH_TOKEN / GITHUB_TOKEN is present' };
  }
  return {
    name: 'gh-token',
    passed: false,
    message: 'Neither GH_TOKEN nor GITHUB_TOKEN is set — ghx calls will fail',
  };
}

async function checkNetwork(): Promise<PreflightCheck> {
  try {
    const res = await fetch('https://api.github.com', {
      method: 'HEAD',
      signal: AbortSignal.timeout(5000),
    });
    // 403 = rate limited but reachable; 200/204 = fine
    if (res.status < 500) {
      return { name: 'network', passed: true, message: `api.github.com reachable (HTTP ${res.status})` };
    }
    return { name: 'network', passed: false, message: `api.github.com returned HTTP ${res.status}` };
  } catch (err) {
    const msg = err instanceof Error ? err.message : String(err);
    return { name: 'network', passed: false, message: `api.github.com unreachable: ${msg}` };
  }
}

async function checkGhxBinary(): Promise<PreflightCheck> {
  try {
    const { stdout } = await execAsync('ghx version', { timeout: 5000 });
    return { name: 'ghx-binary', passed: true, message: `ghx found: ${stdout.trim()}` };
  } catch {
    return {
      name: 'ghx-binary',
      passed: false,
      message: 'ghx binary not found on PATH — add ghx to PATH or install it',
    };
  }
}

/**
 * Run all enabled preflight checks and return the aggregated result.
 * All checks run in parallel for speed.
 */
export async function runPreflight(config: PreflightConfig = {}): Promise<PreflightResult> {
  const tasks: Array<Promise<PreflightCheck>> = [];

  if (config.checkGhToken !== false) tasks.push(checkGhToken());
  if (config.checkNetwork !== false) tasks.push(checkNetwork());
  if (config.checkGhxBinary !== false) tasks.push(checkGhxBinary());

  const checks = await Promise.all(tasks);
  return { passed: checks.every(c => c.passed), checks };
}

/** Format a {@link PreflightResult} as a human-readable diagnostic block. */
export function formatPreflight(result: PreflightResult): string {
  const lines = result.checks.map(c => `  ${c.passed ? '✓' : '✗'} ${c.name}: ${c.message}`);
  return `Preflight ${result.passed ? 'PASS' : 'FAIL'}\n${lines.join('\n')}`;
}
