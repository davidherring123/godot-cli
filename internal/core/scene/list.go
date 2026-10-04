package scene

import (
	"io/fs"
	"path/filepath"
	"strings"
)

const ListName = "scene.list"

func List(projectRoot string, params ListParams) (ListResult, error) {
	var scenes []string

	err := filepath.WalkDir(
		projectRoot,
		func(
			path string,
			entry fs.DirEntry,
			err error,
		) error {
			if err != nil {
				return err
			}

			if entry.IsDir() && entry.Name() == ".godot" {
				return filepath.SkipDir
			}

			if entry.IsDir() {
				return nil
			}

			if strings.ToLower(filepath.Ext(path)) != ".tscn" {
				return nil
			}

			relativePath, err := filepath.Rel(projectRoot, path)

			if err != nil {
				return err
			}

			scenes = append(
				scenes,
				"res://"+filepath.ToSlash(relativePath),
			)

			return nil
		})

	if err != nil {
		return ListResult{}, err
	}

	return ListResult{Scenes: scenes}, nil
}
