// Package cli implements the godot-cli command tree.
package cli

import "github.com/davidherring123/godot-cli/internal/core/project"

func FetchProjectContext() (*project.Project, error) {
	resolved, err := project.Resolve("")
	if err != nil {
		return nil, err
	}
	return &resolved, nil
}
