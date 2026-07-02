# AGENTS.md

## Project

ghx — GitHub code exploration for AI agents. Go binary distributed via npm, Homebrew, and `go install`.

## Architecture

```
cmd/ghx/             — Go binary entrypoint
internal/cli/        — CLI commands (cobra)
internal/ghx/        — core library (explore, read, search, repos, tree, glob)
internal/codemode/   — JS executor (goja sandbox, esbuild transpilation, type generation)
internal/mapengine/  — parser-backed structural map engine
internal/sidecar/    — sidecar runtime, sessions, reports, ACP integration
internal/skilldoc/   — CLI and MCP agent skills embedded via go:embed
npm/             — platform-specific npm packages (one per OS/arch, contains Go binary)
scripts/         — CI and release scripts
```

## Version Bumping

**Always use the bump script.** Never edit `package.json` version manually.

```bash
node scripts/bump.js          # patch (default)
node scripts/bump.js minor
node scripts/bump.js major
```

This updates both `version` and all `optionalDependencies` in `package.json`. The CI pipeline triggers on `package.json` changes to `mainline`: auto-tags → GoReleaser builds 6 platform binaries → npm publishes all 7 packages with OIDC provenance.

## Release Flow

1. `node scripts/bump.js` — bump version
2. `git commit` + `git push` — triggers CI
3. CI: tag → GoReleaser (GitHub release + Homebrew tap) → npm publish (OIDC, no token needed)

## SKILL.md Files

`internal/skilldoc/SKILL.md` and `internal/skilldoc/MCP-SKILL.md` are embedded into the binary via `go:embed`. If you modify them, the binary must be rebuilt for changes to take effect. The `ghx skill` and `ghx skill --mcp` commands print the embedded content.

## Build

```bash
go build -o ghx ./cmd/ghx
```

## Test

```bash
go test ./...
```
