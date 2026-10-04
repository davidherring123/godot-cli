package core

import (
	"errors"
	"os"
	"path/filepath"
)

type Project struct {
	Root string
}

func FindProjectRoot(start string) (string, error) {
	current, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}

	for {
		projectFile := filepath.Join(current, "project.godot")

		if _, err := os.Stat(projectFile); err == nil {
			return current, nil
		}

		parent := filepath.Dir(current)

		if parent == current {
			return "", errors.New("no Godot project found")
		}

		current = parent
	}
}
