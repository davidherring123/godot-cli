// Package cli implements the godot-cli command tree.
package cli

import (
	"os"

	"github.com/davidherring123/godot-cli/internal/core/project"
)

func FetchProjectContext() (*project.Project, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	root, err := project.FindProjectRoot(cwd)
	if err != nil {
		return nil, err
	}
	return &project.Project{Root: root}, nil
}
