// Package addon installs the embedded Godot editor addon into a project.
package addon

import (
	"embed"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const addonPath = "addons/godot_cli"

//go:embed godot_cli
var embeddedFiles embed.FS

func IsInstalled(projectRoot string) bool {
	pluginPath := filepath.Join(
		projectRoot,
		addonPath,
		"plugin.cfg",
	)

	_, err := os.Stat(pluginPath)
	return err == nil
}

// IsManaged reports whether the installed addon was installed by godot-cli.
func IsManaged(projectRoot string) bool {
	_, err := os.Stat(filepath.Join(Path(projectRoot), ".godot-cli-managed"))
	return err == nil
}

func Install(projectRoot string) error {
	source, err := fs.Sub(embeddedFiles, "godot_cli")
	if err != nil {
		return err
	}

	destination := filepath.Join(projectRoot, addonPath)

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
	return filepath.Join(projectRoot, addonPath)
}

func DisplayPath(projectRoot string) string {
	path := Path(projectRoot)

	relative, err := filepath.Rel(projectRoot, path)
	if err != nil {
		return path
	}

	return strings.ReplaceAll(relative, string(filepath.Separator), "/")
}
