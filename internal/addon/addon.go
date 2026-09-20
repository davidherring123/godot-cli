package addon

import (
	"embed"
	"fmt"
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

func IsEnabled(projectRoot string) bool {
	projectFile := filepath.Join(projectRoot, "project.godot")

	data, err := os.ReadFile(projectFile)
	if err != nil {
		return false
	}

	const pluginPath = `"res://addons/godot_cli/plugin.cfg"`

	return strings.Contains(string(data), pluginPath)
}

func Enable(projectRoot string) error {
	projectFile := filepath.Join(projectRoot, "project.godot")

	data, err := os.ReadFile(projectFile)
	if err != nil {
		return err
	}

	content := string(data)

	const pluginPath = "res://addons/godot_cli/plugin.cfg"
	const pluginEntry = `"res://addons/godot_cli/plugin.cfg"`

	if strings.Contains(content, pluginPath) {
		return nil
	}

	const sectionName = "[editor_plugins]"

	sectionStart := strings.Index(content, sectionName)

	if sectionStart == -1 {
		content += fmt.Sprintf(
			"\n%s\n\nenabled=PackedStringArray(%s)\n",
			sectionName,
			pluginEntry,
		)

		return os.WriteFile(projectFile, []byte(content), 0644)
	}

	sectionBodyStart := sectionStart + len(sectionName)
	sectionEnd := len(content)

	if nextSection := strings.Index(content[sectionBodyStart:], "\n["); nextSection != -1 {
		sectionEnd = sectionBodyStart + nextSection
	}

	section := content[sectionStart:sectionEnd]

	const enabledPrefix = "enabled=PackedStringArray("

	enabledOffset := strings.Index(section, enabledPrefix)

	if enabledOffset == -1 {
		insertAt := sectionBodyStart

		content =
			content[:insertAt] +
				fmt.Sprintf("\n\nenabled=PackedStringArray(%s)", pluginEntry) +
				content[insertAt:]

		return os.WriteFile(projectFile, []byte(content), 0644)
	}

	enabledStart := sectionStart + enabledOffset
	arrayStart := enabledStart + len(enabledPrefix)

	arrayEndOffset := strings.Index(content[arrayStart:], ")")
	if arrayEndOffset == -1 {
		return fmt.Errorf("invalid editor_plugins enabled setting")
	}

	arrayEnd := arrayStart + arrayEndOffset

	existing := strings.TrimSpace(content[arrayStart:arrayEnd])

	if existing == "" {
		content =
			content[:arrayStart] +
				pluginEntry +
				content[arrayEnd:]
	} else {
		content =
			content[:arrayEnd] +
				", " + pluginEntry +
				content[arrayEnd:]
	}

	return os.WriteFile(projectFile, []byte(content), 0644)
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
