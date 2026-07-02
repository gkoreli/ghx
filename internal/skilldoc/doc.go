package skilldoc

import _ "embed"

//go:embed SKILL.md
var SkillMD string

// The canonical skill documents live under skills/ghx/ and skills/ghx-mcp/
// at the repo root and are the ones discovered by the skills CLI.  This
// package keeps identical copies because go:embed cannot reach outside the
// package directory.  doc_test.go enforces they stay in sync.

//go:embed MCP-SKILL.md
var MCPSkillMD string
