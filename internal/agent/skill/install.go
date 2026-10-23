package skill

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
)

//go:embed skill_template.md
var skillTemplate string

func ValidateNames(targets []string) error {
	for _, target := range targets {
		switch target {
		case "codex", "opencode":
		default:
			return fmt.Errorf("unsupported/unknown agent %q (supported: codex, opencode)", target)
		}
	}
	return nil
}

// Install writes the portable skill into the project.
func Install(projectRoot string) (string, error) {
	skillDir := filepath.Join(projectRoot, ".agents", "skills", "godot-cli")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		return "", err
	}

	skillPath := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(skillPath, []byte(skillTemplate), 0644); err != nil {
		return "", err
	}

	return skillPath, nil
}
