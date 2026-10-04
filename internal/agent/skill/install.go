package skill

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/spf13/cobra"
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

func Install(projectRoot string, root *cobra.Command) (string, error) {
	files, err := Generate(root)
	if err != nil {
		return "", err
	}

	files["SKILL.md"] = []byte(skillTemplate)
	
	skillDir := filepath.Join(projectRoot, ".agents", "skills", "godot-cli")
	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, relativePath := range paths {
		path := filepath.Join(skillDir, filepath.FromSlash(relativePath))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return "", err
		}
		if err := os.WriteFile(path, files[relativePath], 0644); err != nil {
			return "", err
		}
	}
	return filepath.Join(skillDir, "SKILL.md"), nil
}
