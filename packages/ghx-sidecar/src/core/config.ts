/**
 * ghx-sidecar configuration.
 *
 * Config file location: `~/.ghx-sidecar/config.json`
 *
 * The most important field is `agent` — the name of the acpx-registered agent
 * (e.g. `"codex"`, `"claude"`, `"kiro"`) or a full command string for a custom
 * ACP-compatible harness.
 *
 * Run `ghx-sidecar config init` to auto-detect and write the initial config.
 */

import { existsSync, mkdirSync, readFileSync, writeFileSync } from 'node:fs';
import { homedir } from 'node:os';
import { join } from 'node:path';
import { exec } from 'node:child_process';
import { promisify } from 'node:util';

const execAsync = promisify(exec);

/** Path to the ghx-sidecar config directory. */
export const CONFIG_DIR = join(homedir(), '.ghx-sidecar');

/** Path to the config file. */
export const CONFIG_FILE = join(CONFIG_DIR, 'config.json');

/** Path to the sessions directory. */
export const SESSIONS_DIR = join(CONFIG_DIR, 'sessions');

/** ghx-sidecar configuration shape. */
export interface GhxSidecarConfig {
  /**
   * acpx agent name.
   * Known values: `"codex"`, `"claude"`, `"kiro"`.
   * @example "codex"
   * @example "claude"
   */
  agent: string;
  /**
   * Directory where ghx-sidecar session artifacts are stored.
   * @default "~/.ghx-sidecar/sessions"
   */
  sessionsDir: string;
}

const DEFAULTS: GhxSidecarConfig = {
  agent: 'codex',
  sessionsDir: SESSIONS_DIR,
};

/** Load config from disk, falling back to defaults if the file does not exist. */
export function loadConfig(): GhxSidecarConfig {
  if (!existsSync(CONFIG_FILE)) return { ...DEFAULTS };
  try {
    const raw = JSON.parse(readFileSync(CONFIG_FILE, 'utf8')) as Partial<GhxSidecarConfig>;
    return { ...DEFAULTS, ...raw };
  } catch {
    return { ...DEFAULTS };
  }
}

/** Write config to disk, creating the config directory if needed. */
export function saveConfig(config: GhxSidecarConfig): void {
  mkdirSync(CONFIG_DIR, { recursive: true });
  writeFileSync(CONFIG_FILE, JSON.stringify(config, null, 2) + '\n', 'utf8');
}

/** Known acpx-registered agent names and how to detect them. */
const KNOWN_AGENTS: Array<{ name: string; detectCmd: string }> = [
  { name: 'codex',  detectCmd: 'codex --version' },
  { name: 'claude', detectCmd: 'claude --version' },
  { name: 'kiro',   detectCmd: 'kiro --version' },
];

/**
 * Detect which ACP-compatible agents are available on PATH.
 * Returns the list of agent names acpx knows about that are currently installed.
 */
export async function detectAgents(): Promise<string[]> {
  const results = await Promise.all(
    KNOWN_AGENTS.map(async ({ name, detectCmd }) => {
      try {
        await execAsync(detectCmd, { timeout: 3000 });
        return name;
      } catch {
        return null;
      }
    }),
  );
  return results.filter((n): n is string => n !== null);
}

/** Format the config as a human-readable summary. */
export function formatConfig(config: GhxSidecarConfig): string {
  return [
    `agent:       ${config.agent}`,
    `sessionsDir: ${config.sessionsDir}`,
    `configFile:  ${CONFIG_FILE}`,
  ].join('\n');
}
