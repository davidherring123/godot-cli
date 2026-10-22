package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/davidherring123/godot-cli/internal/cli"
)

func TestInitEnablesAddonWithoutAgent(t *testing.T) {
	root := testProject(t)

	output := runInit(t, root, "init")

	if strings.Contains(output, "opencode mcp add") {
		t.Fatalf("init without an agent printed OpenCode instructions:\n%s", output)
	}

	assertAddonEnabled(t, root)
	assertNoOpenCodeConfig(t, root)
}

func TestInitPrintsOpenCodeMCPCommand(t *testing.T) {
	root := testProject(t)

	output := runInit(t, root, "init", "--agent", "opencode")

	if !strings.Contains(output, "opencode mcp add godot -- godot-cli mcp") {
		t.Fatalf("init did not print the OpenCode command:\n%s", output)
	}

	assertAddonEnabled(t, root)
	assertNoOpenCodeConfig(t, root)
}

func TestInitIsIdempotent(t *testing.T) {
	root := testProject(t)

	runInit(t, root, "init", "--agent", "opencode")

	first := readProject(t, root)

	output := runInit(t, root, "init", "--agent", "opencode")

	if !strings.Contains(output, "already enabled") {
		t.Fatalf("second init did not report an enabled addon:\n%s", output)
	}

	if second := readProject(t, root); second != first {
		t.Fatalf("project.godot changed on the second init:\n%s", second)
	}
}

func runInit(t *testing.T, root string, args ...string) string {
	t.Helper()

	t.Chdir(root)

	command := cli.NewRootCommand("test")

	var output bytes.Buffer

	command.SetOut(&output)
	command.SetErr(&output)
	command.SetArgs(args)

	if err := command.Execute(); err != nil {
		t.Fatalf("running %v: %v\n%s", args, err, output.String())
	}

	return output.String()
}

func testProject(t *testing.T) string {
	t.Helper()

	root := t.TempDir()

	if err := os.WriteFile(
		filepath.Join(root, "project.godot"),
		[]byte("[application]\n\nconfig/name=\"Test\"\n"),
		0644,
	); err != nil {
		t.Fatal(err)
	}

	return root
}

func assertAddonEnabled(t *testing.T, root string) {
	t.Helper()

	content := readProject(t, root)

	if !strings.Contains(content, "[editor_plugins]") {
		t.Fatalf("project.godot has no editor_plugins section:\n%s", content)
	}

	if !strings.Contains(content, "res://addons/godot_cli/plugin.cfg") {
		t.Fatalf("project.godot does not enable the addon:\n%s", content)
	}
}

func assertNoOpenCodeConfig(t *testing.T, root string) {
	t.Helper()

	for _, name := range []string{
		"opencode.json",
		"opencode.jsonc",
		filepath.Join(".opencode", "opencode.json"),
		filepath.Join(".opencode", "opencode.jsonc"),
	} {
		if _, err := os.Stat(filepath.Join(root, name)); err == nil {
			t.Errorf("init created OpenCode configuration %s", name)
		}
	}
}

func readProject(t *testing.T, root string) string {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(root, "project.godot"))
	if err != nil {
		t.Fatal(err)
	}

	return string(data)
}
