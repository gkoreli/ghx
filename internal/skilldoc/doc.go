package skilldoc

import _ "embed"

//go:embed SKILL.md
var SkillMD string

// NB: skills/ghx/SKILL.md at the repo root is the same document, kept for the
// skills CLI.  Keep internal/skilldoc/SKILL.md as the edit target and copy it
// to skills/ghx/SKILL.md after changes.  doc_test.go enforces this.

//go:embed MCP-SKILL.md
var MCPSkillMD string
