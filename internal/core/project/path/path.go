// Package path resolves scene arguments to Godot resource paths.
package path

import (
	"errors"
	"fmt"
	"os"
	stdpath "path"
	"path/filepath"
	"strings"
)

// Resolve turns a scene argument into a resource path. An empty scene uses the
// project's configured main scene. Relative and absolute filesystem paths must
// stay inside the project; res:// paths must not escape it.
func Resolve(projectRoot string, scene string) (string, error) {
	scene = strings.TrimSpace(scene)

	if scene == "" {
		mainScene, err := mainScenePath(projectRoot)
		if err != nil {
			return "", err
		}

		if mainScene == "" {
			return "", errors.New(
				"no main scene configured; pass a scene path or set application/run/main_scene",
			)
		}

		if strings.HasPrefix(mainScene, "uid://") {
			return mainScene, nil
		}

		return normalizeResPath(mainScene)
	}

	if strings.HasPrefix(scene, "uid://") {
		return scene, nil
	}

	if strings.HasPrefix(scene, "res://") {
		return normalizeResPath(scene)
	}

	absolute := scene

	if !filepath.IsAbs(absolute) {
		absolute = filepath.Join(projectRoot, absolute)
	}

	relative, err := filepath.Rel(projectRoot, absolute)
	if err != nil {
		return "", fmt.Errorf("scene must be inside the project: %s", scene)
	}

	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("scene must be inside the project: %s", scene)
	}

	return "res://" + filepath.ToSlash(relative), nil
}

func normalizeResPath(scene string) (string, error) {
	relative := strings.TrimPrefix(scene, "res://")

	for _, segment := range strings.Split(relative, "/") {
		if segment == ".." {
			return "", fmt.Errorf("scene must be inside the project: %s", scene)
		}
	}

	relative = strings.TrimPrefix(stdpath.Clean(relative), "/")

	if relative == "" || relative == "." {
		return "", fmt.Errorf("invalid scene path: %s", scene)
	}

	return "res://" + relative, nil
}

func mainScenePath(projectRoot string) (string, error) {
	data, err := os.ReadFile(filepath.Join(projectRoot, "project.godot"))
	if err != nil {
		return "", fmt.Errorf("reading project.godot: %w", err)
	}

	section := ""

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)

		if line == "" || strings.HasPrefix(line, ";") {
			continue
		}

		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.Trim(line, "[]")
			continue
		}

		if section != "application" {
			continue
		}

		key, value, found := strings.Cut(line, "=")

		if !found || strings.TrimSpace(key) != "run/main_scene" {
			continue
		}

		return unquote(strings.TrimSpace(value)), nil
	}

	return "", nil
}

func unquote(value string) string {
	if len(value) >= 2 && strings.HasPrefix(value, `"`) && strings.HasSuffix(value, `"`) {
		return value[1 : len(value)-1]
	}

	return value
}
