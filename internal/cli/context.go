package cli

import (
	"os"

	"github.com/davidherring123/godot-cli/internal/core"
)

func FetchProjectContext() (*core.Project, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	root, err := core.FindProjectRoot(cwd)
	if err != nil {
		return nil, err
	}
	return &core.Project{Root: root}, nil
}
