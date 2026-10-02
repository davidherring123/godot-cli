package agent

import (
	"bytes"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"github.com/spf13/cobra"
)

//go:embed skill_template.md
var skillTemplate string

type skillTemplateData struct {
	Commands string
}

func Install(
	projectRoot string,
	target string,
	root *cobra.Command,
) (string, error) {
	skillDir, err := fetchSkillDirectory(projectRoot, target)
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(skillDir, 0755); err != nil {
		return "", err
	}

	content, err := renderSkill(root)
	if err != nil {
		return "", err
	}

	path := filepath.Join(skillDir, "skill.md")

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", err
	}

	return path, nil
}

func renderSkill(root *cobra.Command) (string, error) {
	tmpl, err := template.New("skill").Parse(skillTemplate)
	if err != nil {
		return "", err
	}

	data := skillTemplateData{
		Commands: renderCommmands(root),
	}

	var output bytes.Buffer

	if err := tmpl.Execute(&output, data); err != nil {
		return "", err
	}

	return output.String(), nil
}

// func renderCommands(root *)

// TODO: Make more graceful default case.
func fetchSkillDirectory(projectRoot string, target string) (string, error) {
	switch target {
	case "codex":
		return filepath.Join(
			projectRoot,
			".codex",
			"skills",
			"godot-cli",
		), nil
	default:
		return "", fmt.Errorf(
			"unsupported agent %q",
			target,
		)
	}
}