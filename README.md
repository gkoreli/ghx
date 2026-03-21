# ghx — GitHub Code Exploration for AI Agents

One command does what takes 3-5 API calls. Batch file reads, code maps, search — all via `gh` CLI.

## v2 (current) — Go

Full rewrite in Go with multi-frontend architecture: CLI + MCP server + codemode (programmable JS sandbox).

```bash
cd v2 && go build -o ghx .
```

See [`v2/`](v2/) for source, [`v2/SKILL.md`](v2/SKILL.md) for CLI skill, [`v2/MCP-SKILL.md`](v2/MCP-SKILL.md) for MCP skill.

## v1 (historical) — Bash

Original bash implementation distributed via npm.

See [`v1/`](v1/) for source and [`v1/README.md`](v1/README.md) for docs.

## Architecture Decisions

See [`docs/adr/`](docs/adr/).

## License

MIT
