package addon

import (
	"embed"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const ADDON_PATH = "addons/godot_cli"

//go:embed godot_cli
var embeddedFiles embed.FS

func IsInstalled(projectRoot string) bool {
	pluginPath := filepath.Join(
		projectRoot,
		ADDON_PATH,
		"plugin.cfg",
	)

	_, err := os.Stat(pluginPath)
	return err == nil
}

func Install(projectRoot string) error {
	source, err := fs.Sub(embeddedFiles, "godot_cli")
	if err != nil {
		return err
	}

	destination := filepath.Join(projectRoot, ADDON_PATH)

	err = fs.WalkDir(source, ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		target := filepath.Join(destination, filepath.FromSlash(path))

		if entry.IsDir() {
			return os.MkdirAll(target, 0755)
		}

		data, err := fs.ReadFile(source, path)
		if err != nil {
			return err
		}

		return os.WriteFile(target, data, 0644)
	})

	if err != nil {
		return err
	}

	return os.WriteFile(
		filepath.Join(destination, ".godot-cli-managed"),
		[]byte("managed by godot-cli\n"),
		0644,
	)
}

func Path(projectRoot string) string {
	return filepath.Join(projectRoot, ADDON_PATH)
}

func DisplayPath(projectRoot string) string {
	path := Path(projectRoot)

	relative, err := filepath.Rel(projectRoot, path)
	if err != nil {
		return path
	}

	return strings.ReplaceAll(relative, string(filepath.Separator), "/")
}
