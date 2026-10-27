package addon_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/davidherring123/godot-cli/internal/addon"
)

const pluginPath = "res://addons/godot_cli/plugin.cfg"

func TestInstallRemovesObsoleteManagedPaths(t *testing.T) {
	root := writeProject(t, "[application]\n\nconfig/name=\"Test\"\n")

	if err := addon.Install(root); err != nil {
		t.Fatalf("installing addon: %v", err)
	}

	obsoletePath := filepath.Join(addon.Path(root), "core", "input")
	if err := os.MkdirAll(obsoletePath, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(obsoletePath, "input_service.gd"), []byte("class_name InputService\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := addon.Install(root); err != nil {
		t.Fatalf("updating addon: %v", err)
	}

	if _, err := os.Stat(obsoletePath); !os.IsNotExist(err) {
		t.Fatalf("obsolete managed path still exists: %v", err)
	}

	newPath := filepath.Join(addon.Path(root), "core", "project", "input", "input_service.gd")
	if _, err := os.Stat(newPath); err != nil {
		t.Fatalf("new input service is missing: %v", err)
	}
}

func TestInstallIncludesMCPPanel(t *testing.T) {
	root := writeProject(t, "[application]\n\nconfig/name=\"Test\"\n")

	if err := addon.Install(root); err != nil {
		t.Fatalf("installing addon: %v", err)
	}

	for _, relative := range []string{
		"mcp/backend_supervisor.gd",
		"mcp/integration_manager.gd",
		"ui/mcp_panel.gd",
	} {
		path := filepath.Join(addon.Path(root), filepath.FromSlash(relative))

		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("installed addon is missing %s: %v", relative, err)
		}

		if info.Size() == 0 {
			t.Fatalf("installed addon file %s is empty", relative)
		}
	}

	plugin, err := os.ReadFile(filepath.Join(addon.Path(root), "plugin.gd"))
	if err != nil {
		t.Fatalf("reading installed plugin: %v", err)
	}

	for _, expected := range []string{
		"mcp/backend_supervisor.gd",
		"ui/mcp_panel.gd",
		"add_control_to_bottom_panel(mcp_panel, \"Godot MCP\")",
		"mcp_backend.poll(delta)",
	} {
		if !strings.Contains(string(plugin), expected) {
			t.Fatalf("installed plugin is missing %q:\n%s", expected, plugin)
		}
	}
}

func TestEnableAddsSection(t *testing.T) {
	root := writeProject(t, "[application]\n\nconfig/name=\"Test\"\n")

	changed, err := addon.Enable(root)
	if err != nil {
		t.Fatalf("enabling addon: %v", err)
	}

	if !changed {
		t.Fatal("Enable reported no change; want the section to be added")
	}

	content := readProject(t, root)

	if !strings.Contains(content, "[editor_plugins]") {
		t.Fatalf("project.godot has no editor_plugins section:\n%s", content)
	}

	if !strings.Contains(content, pluginPath) {
		t.Fatalf("project.godot does not enable the plugin:\n%s", content)
	}

	enabled, err := addon.Enabled(root)
	if err != nil {
		t.Fatalf("checking enabled: %v", err)
	}

	if !enabled {
		t.Fatal("Enabled reported false after Enable")
	}
}

func TestEnableIsIdempotent(t *testing.T) {
	root := writeProject(t, "[application]\n\nconfig/name=\"Test\"\n")

	if _, err := addon.Enable(root); err != nil {
		t.Fatalf("enabling addon: %v", err)
	}

	first := readProject(t, root)

	changed, err := addon.Enable(root)
	if err != nil {
		t.Fatalf("enabling addon again: %v", err)
	}

	if changed {
		t.Fatal("Enable reported a change on the second call")
	}

	if second := readProject(t, root); second != first {
		t.Fatalf("project.godot changed on the second call:\n%s", second)
	}
}

func TestEnablePreservesExistingPlugins(t *testing.T) {
	root := writeProject(
		t,
		"[editor_plugins]\n\nenabled=PackedStringArray(\"res://addons/other/plugin.cfg\")\n",
	)

	if _, err := addon.Enable(root); err != nil {
		t.Fatalf("enabling addon: %v", err)
	}

	content := readProject(t, root)

	if !strings.Contains(content, "res://addons/other/plugin.cfg") {
		t.Fatalf("existing plugin was removed:\n%s", content)
	}

	if !strings.Contains(content, pluginPath) {
		t.Fatalf("godot-cli plugin was not added:\n%s", content)
	}
}

func TestEnableSpacedEnabledAssignment(t *testing.T) {
	root := writeProject(
		t,
		"[editor_plugins]\n\nenabled = PackedStringArray(\"res://addons/other/plugin.cfg\")\n\n[application]\n\nconfig/name=\"Test\"\n",
	)

	enabled, err := addon.Enabled(root)
	if err != nil {
		t.Fatalf("checking enabled: %v", err)
	}

	if enabled {
		t.Fatal("Enabled reported true before Enable")
	}

	changed, err := addon.Enable(root)
	if err != nil {
		t.Fatalf("enabling addon: %v", err)
	}

	if !changed {
		t.Fatal("Enable reported no change; want the plugin to be added")
	}

	content := readProject(t, root)

	if count := strings.Count(content, "enabled"); count != 1 {
		t.Fatalf("project.godot has %d enabled keys; want 1:\n%s", count, content)
	}

	if !strings.Contains(content, "enabled = PackedStringArray(") {
		t.Fatalf("whitespace around enabled assignment was not preserved:\n%s", content)
	}

	if !strings.Contains(content, "res://addons/other/plugin.cfg") {
		t.Fatalf("existing plugin was removed:\n%s", content)
	}

	if !strings.Contains(content, pluginPath) {
		t.Fatalf("godot-cli plugin was not added:\n%s", content)
	}

	if !strings.Contains(content, "config/name=\"Test\"") {
		t.Fatalf("settings outside editor_plugins were removed:\n%s", content)
	}

	enabled, err = addon.Enabled(root)
	if err != nil {
		t.Fatalf("checking enabled: %v", err)
	}

	if !enabled {
		t.Fatal("Enabled reported false after Enable")
	}

	changed, err = addon.Enable(root)
	if err != nil {
		t.Fatalf("enabling addon again: %v", err)
	}

	if changed {
		t.Fatal("Enable reported a change on the second call")
	}

	if second := readProject(t, root); second != content {
		t.Fatalf("project.godot changed on the second call:\n%s", second)
	}
}

func TestEnableEmptyArray(t *testing.T) {
	root := writeProject(t, "[editor_plugins]\n\nenabled=PackedStringArray()\n")

	if _, err := addon.Enable(root); err != nil {
		t.Fatalf("enabling addon: %v", err)
	}

	if content := readProject(t, root); !strings.Contains(content, pluginPath) {
		t.Fatalf("godot-cli plugin was not added:\n%s", content)
	}
}

func TestEnableMissingKey(t *testing.T) {
	root := writeProject(t, "[editor_plugins]\n\n[application]\n\nconfig/name=\"Test\"\n")

	if _, err := addon.Enable(root); err != nil {
		t.Fatalf("enabling addon: %v", err)
	}

	content := readProject(t, root)

	if !strings.Contains(content, pluginPath) {
		t.Fatalf("godot-cli plugin was not added:\n%s", content)
	}

	section := content[strings.Index(content, "[editor_plugins]"):]

	if application := strings.Index(section, "[application]"); application >= 0 {
		if enabled := strings.Index(section[:application], pluginPath); enabled < 0 {
			t.Fatalf("enabled key was not added to editor_plugins:\n%s", content)
		}
	}
}

func TestEnableRejectsUnsupportedValue(t *testing.T) {
	root := writeProject(t, "[editor_plugins]\n\nenabled=true\n")

	if _, err := addon.Enable(root); err == nil {
		t.Fatal("Enable succeeded with enabled=true; want an error")
	}
}

func TestEnabledMissingSection(t *testing.T) {
	root := writeProject(t, "[application]\n\nconfig/name=\"Test\"\n")

	enabled, err := addon.Enabled(root)
	if err != nil {
		t.Fatalf("checking enabled: %v", err)
	}

	if enabled {
		t.Fatal("Enabled reported true without an editor_plugins section")
	}
}

func writeProject(t *testing.T, content string) string {
	t.Helper()

	root := t.TempDir()

	if err := os.WriteFile(filepath.Join(root, "project.godot"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	return root
}

func readProject(t *testing.T, root string) string {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(root, "project.godot"))
	if err != nil {
		t.Fatal(err)
	}

	return string(data)
}
