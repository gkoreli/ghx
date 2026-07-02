package skilldoc

import (
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
		"SKILL.md": SkillMD,
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
