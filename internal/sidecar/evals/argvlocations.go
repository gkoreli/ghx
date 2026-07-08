package evals

import (
	"slices"
	"strings"

	"github.com/gkoreli/ghx/v2/internal/sidecar"
	"github.com/gkoreli/ghx/v2/internal/sidecar/evals/hosttask"
)

// ADR-0032.2 D4 — argv-derived path scope.
//
// The ghx-sidecar profile's recon is ~100% kind=execute tool calls: the agent
// shells out to `ghx read/explore/inspect/grep/...` over the ACP execute tool,
// and ACP execute notifications carry no file `locations`. So D2 (stop dropping
// Locations) alone still leaves `Locations` empty at the SOURCE for the sidecar
// profile. This file closes that gap: when a captured trace has no ACP-reported
// locations, it parses the `ghx` command line already recorded in the trace
// (RawInput "command" / Title) and synthesizes the repo-relative file paths the
// recon touched, so path-scope decisions have real data for execute-driven
// recon.
//
// Two invariants make this safe and honest (visibility/truthfulness tenet):
//
//   - Additive. Derivation fires ONLY when the trace carries no ACP locations,
//     so it never overrides paths the adapter actually reported.
//   - No invented paths. It emits a path only for recognized ghx reconnaissance
//     subcommands, and only from tokens that are genuinely path arguments (read
//     positionals, tree/explore positionals, `--path`/`--glob` scope flags) —
//     never from a natural-language query/pattern positional (inspect's concern,
//     grep's regex), which are not file paths. Anything it cannot attribute to a
//     path stays empty, which is the honest answer.
//
// Blast radius: synthesized locations land only on execute-kind ghx traces. The
// host-task attribution R6 path-scope rule (hosttask.Classifier.classifyLocations)
// reads Locations only for read/edit/delete/move/search-kind calls, so it never
// consumes these — host-task grader verdicts cannot change from this. The value
// is a trustworthy, profile-consistent Locations field in committed artifacts.

// fillArgvLocations enriches a completed turn record's tool traces with
// argv-derived path scope (ADR-0032.2 D4). Applied at turn finalization by the
// runner (both the sidecar and the direct capture paths) so it operates on the
// complete trace, after every ACP update has been folded in.
func fillArgvLocations(rec *TurnRecord) {
	for i := range rec.ToolTraces {
		deriveArgvLocations(&rec.ToolTraces[i])
	}
	for i := range rec.ReplayedToolTraces {
		deriveArgvLocations(&rec.ReplayedToolTraces[i])
	}
}

// deriveArgvLocations fills one trace's Locations from its ghx command line
// when — and only when — ACP reported none. See the file header for the
// additive/no-invented-paths invariants.
func deriveArgvLocations(tr *sidecar.ToolCallTrace) {
	if len(tr.Locations) > 0 {
		return // ACP already reported locations; never override them.
	}
	// hosttask.ExecuteCommandLine is the one authority for resolving an execute
	// trace's command line from RawInput "command" (falling back to Title,
	// except the adapter's bare "Terminal" label): reuse it so live and
	// JSON-reloaded traces resolve identically.
	if paths := ghxArgvLocations(hosttask.ExecuteCommandLine(tr.RawInput, tr.Title)); len(paths) > 0 {
		tr.Locations = paths
	}
}

// ghxArgvLocations parses a shell command line and returns the repo-relative
// file paths that its `ghx` reconnaissance invocations target, deduped and in
// first-seen order. It returns nil when the command line invokes no ghx recon
// subcommand or names no path. It handles compound commands (a chain joined by
// `&&`, `||`, `;`, `|`, or newlines) by parsing each segment independently and
// unioning the ghx segments' paths — the sidecar routinely chains reads and
// pipes ghx into shell filters.
func ghxArgvLocations(cmd string) []string {
	var out []string
	for _, seg := range shellSegments(cmd) {
		args, ok := ghxSubcommandArgs(shellTokens(seg))
		if !ok {
			continue
		}
		for _, p := range subcommandPaths(args) {
			if p != "" && !slices.Contains(out, p) {
				out = append(out, p)
			}
		}
	}
	return out
}

// ghxSubcommandArgs reports whether a token list invokes ghx (directly or via
// `npx`) and, if so, returns the arguments AFTER the ghx executable token —
// i.e. starting at the subcommand. Mirrors the ghx-identity rules already used
// by the trace summarizer (rawTokenInvokesGhx / rawNpxPackageInvokesGhx).
func ghxSubcommandArgs(tokens []string) ([]string, bool) {
	if len(tokens) == 0 {
		return nil, false
	}
	if rawTokenInvokesGhx(tokens[0]) {
		return tokens[1:], true
	}
	if strings.Trim(strings.ToLower(tokens[0]), `"'()[],:;`) != "npx" {
		return nil, false
	}
	// npx form: skip npx's own flags, then the first bare token must be the ghx
	// package for this to be a ghx invocation.
	for i := 1; i < len(tokens); i++ {
		t := strings.Trim(strings.ToLower(tokens[i]), `"'()[],:;`)
		if t == "" || strings.HasPrefix(t, "-") {
			continue
		}
		if rawNpxPackageInvokesGhx(tokens[i]) {
			return tokens[i+1:], true
		}
		return nil, false
	}
	return nil, false
}

// subcommandPaths returns the file paths named by one ghx invocation's
// arguments (subcommand first). The per-subcommand rules are deliberately
// conservative: positional file arguments for the path-first read family,
// explicit `--path`/`--glob` scope flags for the query-first search family, and
// nothing for subcommands that name no path.
func subcommandPaths(args []string) []string {
	if len(args) == 0 {
		return nil
	}
	sub := strings.ToLower(strings.Trim(args[0], `"'`))
	rest := args[1:]
	switch sub {
	case "read", "explore", "tree", "astgrep":
		// `ghx <sub> <owner/repo> [path...]` — positionals after owner/repo are
		// file paths or globs (read/astgrep take many; tree/explore an optional
		// one). The owner/repo positional is never a file path, so it is dropped.
		return positionalPaths(rest)
	case "inspect", "grep", "search":
		// The second positional is a concern/regex/query, NOT a file path, so it
		// must not be synthesized as one. Only the explicit path-scope flags
		// (`--path`, `--glob`) name a real subtree.
		return flagScopePaths(rest)
	case "tier2":
		// `ghx tier2 <sub> ...` — recurse into the tier-2 subcommand (astgrep).
		return subcommandPaths(rest)
	}
	return nil
}

// positionalPaths returns the positional file arguments after the leading
// owner/repo positional, skipping flags, their values, and shell redirects. It
// biases toward under-extraction: an unrecognized flag is assumed to consume a
// following value, so a flag's argument is never mistaken for a path (a missed
// path is honest; an invented one is not).
func positionalPaths(rest []string) []string {
	var positionals []string
	for i := 0; i < len(rest); i++ {
		tok := rest[i]
		switch {
		case isRedirect(tok):
			if isBareRedirectOp(tok) {
				i++ // also skip the redirect target
			}
		case strings.HasPrefix(tok, "-"):
			if scopeFlagConsumesValue(tok) && i+1 < len(rest) && !strings.HasPrefix(rest[i+1], "-") {
				i++ // skip the flag's value
			}
		default:
			positionals = append(positionals, tok)
		}
	}
	if len(positionals) <= 1 {
		return nil // only owner/repo (or nothing) — no file path named
	}
	return positionals[1:] // drop the owner/repo positional
}

// flagScopePaths returns the values of the `--path` and `--glob` scope flags
// (both `--flag value` and `--flag=value` forms). These are the only path-shaped
// arguments of the query-first search subcommands.
func flagScopePaths(rest []string) []string {
	var out []string
	for i := 0; i < len(rest); i++ {
		tok := rest[i]
		if !strings.HasPrefix(tok, "-") {
			continue
		}
		name, val, hasEq := strings.Cut(strings.TrimLeft(tok, "-"), "=")
		switch strings.ToLower(name) {
		case "path", "glob":
			if hasEq {
				if val != "" {
					out = append(out, val)
				}
			} else if i+1 < len(rest) && !strings.HasPrefix(rest[i+1], "-") {
				out = append(out, rest[i+1])
				i++
			}
		}
	}
	return out
}

// ghxBooleanFlags are the ghx recon flags that take NO value; every other flag
// is assumed to consume the following token (see positionalPaths). Kept small
// and recon-scoped on purpose — these are the booleans that actually appear on
// path-first subcommands, plus the global output toggles.
var ghxBooleanFlags = map[string]bool{
	"map": true, "full": true, "json": true, "no-color": true,
	"help": true, "h": true, "version": true,
}

// scopeFlagConsumesValue reports whether a `-`/`--` flag token consumes the
// next token as its value (so positionalPaths does not mistake that value for a
// path). `--flag=value` is self-contained; a known boolean consumes nothing;
// anything else is assumed to take a value (bias toward under-extraction).
//
// Distinct from anticipation_predictor.go's frozen whitelist flagTakesValue,
// which is a measurement input for a different consumer: this one defaults the
// OTHER way (unknown ⇒ consumes) precisely so an unrecognized flag's argument
// is never emitted as a synthesized location.
func scopeFlagConsumesValue(tok string) bool {
	name := strings.TrimLeft(tok, "-")
	if strings.Contains(name, "=") {
		return false
	}
	return !ghxBooleanFlags[strings.ToLower(name)]
}

// isRedirect reports whether a token is (or carries) a shell redirect such as
// `>out`, `2>/dev/null`, or a bare `>`.
func isRedirect(tok string) bool {
	return strings.ContainsAny(tok, "<>")
}

// isBareRedirectOp reports whether a token is a redirect operator with its
// target in the NEXT token (`ghx ... > out.txt`), so the target is skipped too.
// An attached form (`2>/dev/null`, `>out.txt`) carries its own target.
func isBareRedirectOp(tok string) bool {
	switch tok {
	case ">", ">>", "<", "2>", "1>", "&>", "2>>", "1>>":
		return true
	}
	return false
}

// shellSegments splits a command line into its top-level command segments at
// `&&`, `||`, `;`, `|`, and newline boundaries, honoring single/double quotes
// so an operator inside a quoted argument does not split. A single `&`
// (background) and redirect operators are left inside the segment. Command
// substitution ($(...) / backticks) is not expanded — a ghx call nested inside
// one is simply not attributed (conservative), which is rare for recon.
func shellSegments(s string) []string {
	var segs []string
	start := 0
	var quote byte
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if quote != 0 {
			if ch == quote {
				quote = 0
			}
			continue
		}
		switch ch {
		case '\'', '"':
			quote = ch
		case '\n', ';':
			segs = append(segs, s[start:i])
			start = i + 1
		case '|':
			segs = append(segs, s[start:i])
			if i+1 < len(s) && s[i+1] == '|' {
				i++
			}
			start = i + 1
		case '&':
			if i+1 < len(s) && s[i+1] == '&' {
				segs = append(segs, s[start:i])
				i++
				start = i + 1
			}
		}
	}
	segs = append(segs, s[start:])
	return segs
}

// shellTokens splits one command segment into whitespace-separated tokens,
// honoring single/double quotes so a quoted glob or pattern with spaces stays
// one token; the surrounding quotes are stripped. Backslashes are kept literal
// (they appear only inside quoted --grep/--pattern values, never in a path).
func shellTokens(s string) []string {
	var tokens []string
	var cur strings.Builder
	var quote byte
	has := false
	flush := func() {
		if has {
			tokens = append(tokens, cur.String())
			cur.Reset()
			has = false
		}
	}
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if quote != 0 {
			if ch == quote {
				quote = 0
			} else {
				cur.WriteByte(ch)
			}
			continue
		}
		switch ch {
		case '\'', '"':
			quote = ch
			has = true // a quote opens a token even if the quoted text is empty
		case ' ', '\t':
			flush()
		default:
			cur.WriteByte(ch)
			has = true
		}
	}
	flush()
	return tokens
}
