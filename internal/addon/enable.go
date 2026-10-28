package addon

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	projectFileName      = "project.godot"
	pluginResourcePath   = "res://" + addonPath + "/plugin.cfg"
	editorPluginsSection = "[editor_plugins]"
)

// Enabled reports whether project.godot enables the godot-mcp editor plugin.
func Enabled(projectRoot string) (bool, error) {
	data, err := os.ReadFile(filepath.Join(projectRoot, projectFileName))
	if err != nil {
		return false, err
	}

	_, enabled, err := updateEnabledPlugins(string(data), false)
	if err != nil {
		return false, err
	}

	return enabled, nil
}

// Enable adds the godot-mcp plugin to project.godot's enabled editor plugins.
// It reports whether the project file changed.
func Enable(projectRoot string) (bool, error) {
	path := filepath.Join(projectRoot, projectFileName)

	info, err := os.Stat(path)
	if err != nil {
		return false, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}

	updated, changed, err := updateEnabledPlugins(string(data), true)
	if err != nil {
		return false, err
	}

	if !changed {
		return false, nil
	}

	temp, err := os.CreateTemp(projectRoot, ".project.godot-*")
	if err != nil {
		return false, err
	}

	tempPath := temp.Name()

	defer func() {
		_ = os.Remove(tempPath)
	}()

	if _, err := temp.WriteString(updated); err != nil {
		_ = temp.Close()
		return false, err
	}

	if err := temp.Chmod(info.Mode().Perm()); err != nil {
		_ = temp.Close()
		return false, err
	}

	if err := temp.Close(); err != nil {
		return false, err
	}

	if err := os.Rename(tempPath, path); err != nil {
		return false, err
	}

	return true, nil
}

// updateEnabledPlugins inspects or updates the [editor_plugins] enabled list.
// When mutate is false it reports whether the plugin is already enabled; when
// mutate is true it reports whether the content changed.
func updateEnabledPlugins(content string, mutate bool) (string, bool, error) {
	lines := strings.Split(content, "\n")

	sectionStart, sectionEnd := findSection(lines)

	if sectionStart < 0 {
		if !mutate {
			return content, false, nil
		}

		return addSection(content), true, nil
	}

	index := findEnabledLine(lines, sectionStart, sectionEnd)

	if index < 0 {
		if !mutate {
			return content, false, nil
		}

		lines = insertLine(lines, sectionStart+1, enabledLine())

		return strings.Join(lines, "\n"), true, nil
	}

	line := lines[index]
	_, value, _ := splitAssignment(line)

	plugins, err := parsePluginArray(value)
	if err != nil {
		return content, false, fmt.Errorf("reading %s in project.godot: %w", editorPluginsSection, err)
	}

	for _, plugin := range plugins {
		if plugin == pluginResourcePath {
			return content, !mutate, nil
		}
	}

	if !mutate {
		return content, false, nil
	}

	lines[index] = replaceEnabled(line, append(plugins, pluginResourcePath))

	return strings.Join(lines, "\n"), true, nil
}

func findSection(lines []string) (int, int) {
	start := -1

	for index, line := range lines {
		trimmed := strings.TrimSpace(line)

		if !strings.HasPrefix(trimmed, "[") || !strings.HasSuffix(trimmed, "]") {
			continue
		}

		if start >= 0 {
			return start, index
		}

		if trimmed == editorPluginsSection {
			start = index
		}
	}

	if start < 0 {
		return -1, len(lines)
	}

	return start, len(lines)
}

func findEnabledLine(lines []string, start int, end int) int {
	for index := start + 1; index < end; index++ {
		if key, _, ok := splitAssignment(lines[index]); ok && key == "enabled" {
			return index
		}
	}

	return -1
}

// splitAssignment splits a config line on its first "=" and trims both sides.
func splitAssignment(line string) (string, string, bool) {
	equals := strings.Index(line, "=")

	if equals < 0 {
		return "", "", false
	}

	return strings.TrimSpace(line[:equals]), strings.TrimSpace(line[equals+1:]), true
}

func addSection(content string) string {
	if content != "" && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}

	if content != "" {
		content += "\n"
	}

	return content + editorPluginsSection + "\n" + enabledLine() + "\n"
}

func enabledLine() string {
	return "enabled=PackedStringArray(" + strconv.Quote(pluginResourcePath) + ")"
}

func insertLine(lines []string, index int, line string) []string {
	lines = append(lines, "")
	copy(lines[index+1:], lines[index:])
	lines[index] = line

	return lines
}

func replaceEnabled(line string, plugins []string) string {
	quoted := make([]string, len(plugins))
	for index, plugin := range plugins {
		quoted[index] = strconv.Quote(plugin)
	}

	suffix := ""
	if strings.HasSuffix(line, "\r") {
		suffix = "\r"
		line = strings.TrimSuffix(line, "\r")
	}

	prefix := line[:strings.Index(line, "=")+1]

	rest := line[len(prefix):]
	spacing := rest[:len(rest)-len(strings.TrimLeft(rest, " \t"))]

	return prefix + spacing + "PackedStringArray(" + strings.Join(quoted, ", ") + ")" + suffix
}

func parsePluginArray(value string) ([]string, error) {
	const prefix = "PackedStringArray("

	if !strings.HasPrefix(value, prefix) || !strings.HasSuffix(value, ")") {
		return nil, fmt.Errorf("unsupported enabled value %q", value)
	}

	inner := strings.TrimSpace(value[len(prefix) : len(value)-1])

	if inner == "" {
		return nil, nil
	}

	parts := strings.Split(inner, ",")
	plugins := make([]string, 0, len(parts))

	for _, part := range parts {
		part = strings.TrimSpace(part)

		if part == "" {
			continue
		}

		plugin, err := strconv.Unquote(part)
		if err != nil {
			return nil, fmt.Errorf("unsupported plugin entry %q: %w", part, err)
		}

		plugins = append(plugins, plugin)
	}

	return plugins, nil
}
