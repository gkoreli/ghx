#!/usr/bin/env node

/**
 * @gkoreli/ghx-sidecar
 *
 * Named persistent ACP sidecar sessions for ghx repository reconnaissance.
 *
 * Usage:
 *   ghx-sidecar [--session <name>] [--repo <owner/repo>] <question>
 *   ghx-sidecar sessions list
 *   ghx-sidecar sessions show <name>
 *   ghx-sidecar sessions clear <name>
 *   ghx-sidecar config show
 *   ghx-sidecar config init
 *   ghx-sidecar config set <key> <value>
 *   ghx-sidecar doctor
 *
 * How it works:
 *   1. Resolves the named session (default: "default").
 *   2. On first use: injects the ghx-sidecar agent definition as the opening
 *      turn so the ACP agent takes on the sidecar role.
 *   3. Delegates to acpx, which manages the actual ACP session and agent
 *      process (codex-acp, claude, kiro, etc.).
 *   4. Session artifacts (metadata, evidence reports) are persisted under
 *      ~/.ghx-sidecar/sessions/<name>/.
 */

// ── Public library API ────────────────────────────────────────────────────────

export { createSidecarPrompt } from './core/prompt.js';
export { runPreflight, formatPreflight } from './core/preflight.js';
export type { PreflightResult, PreflightCheck } from './core/preflight.js';
export {
  isInitialized, initSession, readMeta, recordTurn, saveReport,
  listSessions, listReports,
} from './core/session.js';
export type { SessionMeta } from './core/session.js';
export { loadConfig, saveConfig, detectAgents, formatConfig, CONFIG_DIR, SESSIONS_DIR } from './core/config.js';
export type { GhxSidecarConfig } from './core/config.js';
export { acpxSend, acpxDoctor } from './acpx.js';
export type { AcpxOptions } from './acpx.js';
export type { SidecarRequest, SidecarReport, SidecarRuntime, Backend, Claim, RelevantFile, Evidence } from './schema.js';

// ── CLI entry ─────────────────────────────────────────────────────────────────

if (import.meta.url === `file://${process.argv[1]}`) {
  await cli(process.argv.slice(2)).catch(err => {
    process.stderr.write(`ghx-sidecar: ${err instanceof Error ? err.message : String(err)}\n`);
    process.exit(1);
  });
}

async function cli(argv: string[]): Promise<void> {
  const { loadConfig, saveConfig, detectAgents, formatConfig, CONFIG_DIR, SESSIONS_DIR } =
    await import('./core/config.js');
  const { runPreflight, formatPreflight } = await import('./core/preflight.js');
  const {
    isInitialized, initSession, readMeta, recordTurn, saveReport, listSessions, listReports,
  } = await import('./core/session.js');
  const { createSidecarPrompt } = await import('./core/prompt.js');
  const { acpxSend, acpxDoctor } = await import('./acpx.js');
  const { mkdirSync, existsSync } = await import('node:fs');
  const { readFileSync } = await import('node:fs');

  const cmd = argv[0];

  // ── doctor ──────────────────────────────────────────────────────────────────
  if (cmd === 'doctor') {
    const config = loadConfig();
    const result = await runPreflight({
      checkGhToken: true,
      checkNetwork: true,
      checkGhxBinary: true,
    });
    process.stdout.write(formatPreflight(result) + '\n');
    const acpxReport = await acpxDoctor(config.agent);
    process.stdout.write(`  ${acpxReport.ok ? '✓' : '✗'} acpx-runtime (${config.agent}): ${acpxReport.message}\n`);
    process.exit(result.passed && acpxReport.ok ? 0 : 1);
  }

  // ── config ───────────────────────────────────────────────────────────────────
  if (cmd === 'config') {
    const sub = argv[1];
    const config = loadConfig();

    if (sub === 'show' || sub === undefined) {
      process.stdout.write(formatConfig(config) + '\n');
      return;
    }

    if (sub === 'init') {
      process.stdout.write('Detecting available ACP agents...\n');
      const found = await detectAgents();
      if (found.length === 0) {
        process.stderr.write(
          'No known agents detected. Install codex, claude, or kiro, then run again.\n',
        );
        process.exit(1);
      }
      process.stdout.write(`Found: ${found.join(', ')}\n`);
      const chosen = found[0]!;
      config.agent = chosen;
      mkdirSync(CONFIG_DIR, { recursive: true });
      saveConfig(config);
      process.stdout.write(`Config written. Agent set to: ${chosen}\n`);
      process.stdout.write(formatConfig(config) + '\n');
      return;
    }

    if (sub === 'set') {
      const key = argv[2] as keyof typeof config | undefined;
      const value = argv[3];
      if (key === undefined || value === undefined) {
        process.stderr.write('Usage: ghx-sidecar config set <key> <value>\n');
        process.exit(1);
      }
      (config as unknown as Record<string, string>)[key] = value;
      saveConfig(config);
      process.stdout.write(`Set ${key} = ${value}\n`);
      return;
    }

    process.stderr.write(`Unknown config subcommand: ${sub ?? ''}\n`);
    process.exit(1);
  }

  // ── sessions ─────────────────────────────────────────────────────────────────
  if (cmd === 'sessions') {
    const sub = argv[1];
    const config = loadConfig();

    if (sub === 'list' || sub === undefined) {
      const sessions = listSessions(config.sessionsDir);
      if (sessions.length === 0) {
        process.stdout.write('No sessions yet.\n');
        return;
      }
      for (const s of sessions) {
        const initialized = isInitialized(config.sessionsDir, s.name) ? '✓' : '○';
        process.stdout.write(
          `${initialized} ghx-sidecar:${s.name.padEnd(32)} repo:${(s.repo || '?').padEnd(24)} turns:${s.turnCount}  ${s.updatedAt.slice(0, 16)}\n`,
        );
      }
      return;
    }

    if (sub === 'show') {
      const name = argv[2];
      if (name === undefined) {
        process.stderr.write('Usage: ghx-sidecar sessions show <name>\n');
        process.exit(1);
      }
      const meta = readMeta(config.sessionsDir, name);
      if (meta === undefined) {
        process.stderr.write(`Session "${name}" not found.\n`);
        process.exit(1);
      }
      process.stdout.write(JSON.stringify(meta, null, 2) + '\n');
      const reports = listReports(config.sessionsDir, name);
      if (reports.length > 0) {
        process.stdout.write(`\nReports (${reports.length}):\n`);
        for (const r of reports) process.stdout.write(`  ${r}\n`);
        // Print most recent report
        const latest = reports[0];
        if (latest !== undefined) {
          process.stdout.write('\nLatest report:\n');
          process.stdout.write(readFileSync(latest, 'utf8') + '\n');
        }
      }
      return;
    }

    if (sub === 'clear') {
      const { rmSync } = await import('node:fs');
      const name = argv[2];
      if (name === undefined) {
        process.stderr.write('Usage: ghx-sidecar sessions clear <name>\n');
        process.exit(1);
      }
      const { join } = await import('node:path');
      const dir = join(config.sessionsDir, name);
      if (existsSync(dir)) {
        rmSync(dir, { recursive: true });
        process.stdout.write(`Cleared session "${name}".\n`);
      } else {
        process.stdout.write(`Session "${name}" does not exist.\n`);
      }
      return;
    }

    process.stderr.write(`Unknown sessions subcommand: ${sub ?? ''}\n`);
    process.exit(1);
  }

  // ── ask (default) ─────────────────────────────────────────────────────────
  let sessionName = 'default';
  let repo = '';
  let noWait = false;
  const questionParts: string[] = [];

  for (let i = 0; i < argv.length; i++) {
    const arg = argv[i] ?? '';
    if ((arg === '--session' || arg === '-s') && argv[i + 1] !== undefined) {
      sessionName = argv[++i] ?? 'default';
    } else if ((arg === '--repo' || arg === '-r') && argv[i + 1] !== undefined) {
      repo = argv[++i] ?? '';
    } else if (arg === '--no-wait') {
      noWait = true;
    } else if (arg !== '') {
      questionParts.push(arg);
    }
  }

  const question = questionParts.join(' ').trim();

  if (question === '') {
    printUsage();
    process.exit(1);
  }

  const config = loadConfig();
  mkdirSync(config.sessionsDir, { recursive: true });

  const initialized = isInitialized(config.sessionsDir, sessionName);
  const inferredRepo = repo || inferRepo(question);

  let prompt: string;

  if (!initialized) {
    // First turn: inject the full sidecar agent definition so the ACP agent
    // takes on the ghx-sidecar role for this session.
    const request = {
      session: `ghx-sidecar:${sessionName}`,
      repo: inferredRepo,
      question,
      depth: 'normal' as const,
      output: 'evidence' as const,
      allowedBackends: ['remote' as const],
    };
    const agentDef = createSidecarPrompt(request);
    // Send agent definition first, then the actual question.
    prompt = agentDef + '\n\n---\n\nQuestion: ' + question;

    initSession(config.sessionsDir, sessionName, inferredRepo, question.slice(0, 60));
    process.stderr.write(
      `[ghx-sidecar] New session "${sessionName}" → acpx ${config.agent}\n`,
    );
  } else {
    // Follow-up turn: session already has the sidecar role, just ask.
    prompt = question;
    process.stderr.write(
      `[ghx-sidecar] Resuming session "${sessionName}" → acpx ${config.agent}\n`,
    );
  }

  await acpxSend({
    agent: config.agent,
    session: `ghx-sidecar:${sessionName}`,
    prompt,
    noWait,
    onReport: (report) => { saveReport(config.sessionsDir, sessionName, report); },
  });

  recordTurn(config.sessionsDir, sessionName);
}

// ── Helpers ───────────────────────────────────────────────────────────────────

function inferRepo(text: string): string {
  const match = /\b([a-zA-Z0-9_.-]+\/[a-zA-Z0-9_.-]+)\b/.exec(text);
  return match?.[1] ?? '';
}

function printUsage(): void {
  process.stdout.write(`\
Usage:
  ghx-sidecar [--session <name>] [--repo <owner/repo>] [--no-wait] <question>
  ghx-sidecar sessions list
  ghx-sidecar sessions show <name>
  ghx-sidecar sessions clear <name>
  ghx-sidecar config show
  ghx-sidecar config init
  ghx-sidecar config set <key> <value>
  ghx-sidecar doctor

Examples:
  ghx-sidecar "honojs/hono - Where is middleware composition implemented?"
  ghx-sidecar --session hono-mw --repo honojs/hono "Where is middleware composition?"
  ghx-sidecar --session hono-mw "How do errors propagate through that path?"
  ghx-sidecar config init
  ghx-sidecar doctor
`);
}
