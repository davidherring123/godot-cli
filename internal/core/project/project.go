// Package project locates and describes Godot project roots. Its subpackages
// cover project-scoped concerns: editor lifecycle, runtime sessions, the Input
// Map, project settings, and scene path resolution.
package project

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type Project struct {
	Root string
}

// Resolve locates the project containing path, or the project in the current
// working directory when path is empty. A path may point to a project
// directory, any directory inside a project, or a file inside a project.
func Resolve(path string) (Project, error) {
	if path == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return Project{}, fmt.Errorf("getting working directory: %w", err)
		}

		path = cwd
	}

	absolute, err := filepath.Abs(path)
	if err != nil {
		return Project{}, fmt.Errorf("resolving project path: %w", err)
	}

	info, err := os.Stat(absolute)
	if err != nil {
		return Project{}, fmt.Errorf("reading project path %s: %w", absolute, err)
	}

	if !info.IsDir() {
		absolute = filepath.Dir(absolute)
	}

	root, err := FindProjectRoot(absolute)
	if err != nil {
		return Project{}, fmt.Errorf("locating Godot project: %w", err)
	}

	return Project{Root: root}, nil
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
