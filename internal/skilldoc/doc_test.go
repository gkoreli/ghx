package skilldoc

import (
	"os"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type skillFrontmatter struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Version     string `yaml:"version"`
	Author      string `yaml:"author"`
	License     string `yaml:"license"`
	Metadata    struct {
		Hermes struct {
			Tags          []string `yaml:"tags"`
			RelatedSkills []string `yaml:"related_skills"`
		} `yaml:"hermes"`
	} `yaml:"metadata"`
}

func TestEmbeddedSkillsHaveCompleteFrontmatter(t *testing.T) {
	docs := map[string]string{
		"SKILL.md":     SkillMD,
		"MCP-SKILL.md": MCPSkillMD,
	}

	for name, content := range docs {
		t.Run(name, func(t *testing.T) {
			fm := parseFrontmatter(t, content)

			if fm.Name == "" {
				t.Fatal("frontmatter name is required")
			}
			if fm.Description == "" {
				t.Fatal("frontmatter description is required")
			}
			if len(fm.Description) > 1024 {
				t.Fatalf("frontmatter description is too long: %d chars", len(fm.Description))
			}
			if fm.Version == "" {
				t.Fatal("frontmatter version is required")
			}
			if fm.Author == "" {
				t.Fatal("frontmatter author is required")
			}
			if fm.License == "" {
				t.Fatal("frontmatter license is required")
			}
			if len(fm.Metadata.Hermes.Tags) == 0 {
				t.Fatal("frontmatter metadata.hermes.tags must be non-empty")
			}
			if fm.Metadata.Hermes.RelatedSkills == nil {
				t.Fatal("frontmatter metadata.hermes.related_skills must be present")
			}
		})
	}
}

// TestSkillsRootMatchesEmbedded ensures the repo-root skill file
// matches the embedded one, so the skills CLI and the Go binary
// present the identical document.
func TestSkillsRootMatchesEmbedded(t *testing.T) {
	rootPath := "../../skills/ghx/SKILL.md"
	rootContent, err := os.ReadFile(rootPath)
	if err != nil {
		t.Fatalf("cannot read repo-root skill at %s — did the file move? %v", rootPath, err)
	}
	if strings.TrimSpace(string(rootContent)) != strings.TrimSpace(SkillMD) {
		t.Fatalf("skills/ghx/SKILL.md differs from internal/skilldoc/SKILL.md")
	}
}

func TestMCPSkillsRootMatchesEmbedded(t *testing.T) {
	var rootContent []byte
	rootContent, err := os.ReadFile("../../skills/ghx-mcp/SKILL.md")
	if err != nil {
		t.Fatalf("cannot read repo-root MCP skill: %v", err)
	}
	if strings.TrimSpace(string(rootContent)) != strings.TrimSpace(MCPSkillMD) {
		t.Fatalf("skills/ghx-mcp/SKILL.md differs from internal/skilldoc/MCP-SKILL.md")
	}
}

func parseFrontmatter(t *testing.T, content string) skillFrontmatter {
	t.Helper()
	if !strings.HasPrefix(content, "---\n") {
		t.Fatal("skill doc must start with frontmatter at byte 0")
	}

	end := strings.Index(content[len("---\n"):], "\n---\n")
	if end < 0 {
		t.Fatal("skill doc must close frontmatter with ---")
	}

	bodyStart := len("---\n") + end + len("\n---\n")
	if strings.TrimSpace(content[bodyStart:]) == "" {
		t.Fatal("skill doc body must be non-empty")
	}

	var fm skillFrontmatter
	if err := yaml.Unmarshal([]byte(content[len("---\n"):len("---\n")+end]), &fm); err != nil {
		t.Fatalf("frontmatter must parse as YAML: %v", err)
	}
	return fm
}
