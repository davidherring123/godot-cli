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

// Content returns the canonical portable skill template.
func Content() []byte {
	return []byte(skillTemplate)
}

// PortablePath returns the project-scoped skill path shared by OpenCode and
// Codex.
func PortablePath(projectRoot string) string {
	return filepath.Join(projectRoot, ".agents", "skills", "godot-cli", "SKILL.md")
}

// ClaudePath returns the project-scoped skill path recognized by Claude Code.
func ClaudePath(projectRoot string) string {
	return filepath.Join(projectRoot, ".claude", "skills", "godot-cli", "SKILL.md")
}

// Install writes the portable skill into the project.
func Install(projectRoot string) (string, error) {
	skillPath := PortablePath(projectRoot)

	if _, err := InstallAt(skillPath); err != nil {
		return "", err
	}

	return skillPath, nil
}

// InstallAt writes the canonical skill to an explicit path. It reports whether
// the file content changed.
func InstallAt(skillPath string) (bool, error) {
	if err := os.MkdirAll(filepath.Dir(skillPath), 0755); err != nil {
		return false, err
	}

	content := Content()

	existing, err := os.ReadFile(skillPath)
	if err == nil && string(existing) == string(content) {
		return false, nil
	}

	if err != nil && !os.IsNotExist(err) {
		return false, err
	}

	if err := os.WriteFile(skillPath, content, 0644); err != nil {
		return false, err
	}

	return true, nil
}
