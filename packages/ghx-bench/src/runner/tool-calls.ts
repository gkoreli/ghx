/**
 * Tool call parsing — clean ACP event text and extract ghx read file arguments.
 *
 * ACP emits two events per tool invocation:
 *   (1) Command text with a trailing status suffix:
 *       "/path/to/ghx read owner/repo src/file.ts (in_progress)"
 *   (2) A status-only completion marker: "tool call" or "tool call (completed)"
 *
 * Responsibilities:
 *   cleanToolCall    — strip status noise; return null for completion-marker events
 *   extractReadFiles — capture all file path arguments from ghx read commands,
 *                      handling multi-file reads and flag arguments correctly
 */

/** Trailing status suffix appended by ACP to every tool invocation event. */
const STATUS_SUFFIX_RE = /\s*\([^)]+\)\s*$/;

/**
 * Cleans a raw ACP tool_call event string.
 *
 * Returns the command with status suffix removed, or null when the event is
 * a status-only completion marker ("tool call", "tool call (completed)") that
 * carries no command content.
 */
export function cleanToolCall(raw: string): string | null {
  if (raw.startsWith('tool call')) return null;
  return raw.replace(STATUS_SUFFIX_RE, '').trim() || null;
}

/**
 * Flags that consume the next whitespace-separated token as a value argument.
 * These must be skipped along with their value when parsing file arguments.
 */
const FLAGS_WITH_VALUE = new Set(['--grep', '--lines', '--level', '--limit', '--depth', '--kind']);

/**
 * Extracts all file path arguments from a list of ghx read command strings.
 *
 * Handles:
 *   Multi-file reads:  `ghx read owner/repo f1.ts f2.ts f3.ts`
 *   Flags with values: `--grep "pattern"`, `--lines 42-80`, `--kind func`
 *   Boolean flags:     `--map`, `--full`, `--remote`
 *   Glob patterns:     `"src/**\/*.ts"` (skipped — not a concrete path)
 *
 * Returns a deduplicated list of file paths in the order first encountered.
 */
export function extractReadFiles(toolCalls: string[]): string[] {
  const files: string[] = [];
  for (const tc of toolCalls) {
    // Repo always contains a slash (owner/repo), so match that shape explicitly.
    const m = /ghx read [^/\s]+\/[^\s]+ (.+)$/.exec(tc);
    if (m === null || m[1] === undefined) continue;
    files.push(...parseFileArguments(m[1]));
  }
  return [...new Set(files)];
}

/**
 * Parses the argument tail of a ghx read command into concrete file path strings.
 *
 * Tokens are processed left-to-right. Each token is classified and either
 * collected as a file path or skipped (with its value token when applicable).
 */
function parseFileArguments(argTail: string): string[] {
  const files: string[] = [];
  const tokens = argTail.trim().split(/\s+/);
  let i = 0;
  while (i < tokens.length) {
    const tok = tokens[i]!;
    if (tok.startsWith('--')) {
      // Skip flag; also skip its value token for flags that take one.
      i += FLAGS_WITH_VALUE.has(tok) ? 2 : 1;
    } else if (tok.startsWith('"') || tok.startsWith("'")) {
      i += 1; // Quoted string value — grep pattern or quoted glob.
    } else if (/^\d/.test(tok)) {
      i += 1; // Line range token: "42-80" or bare integer.
    } else if (tok.includes('*') || tok.includes('{')) {
      i += 1; // Glob pattern — not a concrete file path.
    } else if (/\.\w{1,5}$/.test(tok) || tok.includes('/')) {
      // Has a file extension or a path separator — treat as a file path.
      files.push(tok);
      i += 1;
    } else {
      i += 1;
    }
  }
  return files;
}
