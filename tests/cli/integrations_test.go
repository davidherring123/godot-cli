package cli_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/davidherring123/godot-cli/internal/agent/integration"
	"github.com/davidherring123/godot-cli/internal/cli"
	"github.com/spf13/cobra"
)

func TestIntegrationCommandsAreRegistered(t *testing.T) {
	root := cli.NewRootCommand("test")

	connect := findCLICommand(t, root, "connect")
	if connect.Flags().Lookup("project") == nil {
		t.Error("connect is missing --project")
	}

	integrations := findCLICommand(t, root, "integrations")
	findCLICommand(t, integrations, "status")
	install := findCLICommand(t, integrations, "install")

	if !containsString(install.ValidArgs, "opencode") ||
		!containsString(install.ValidArgs, "claude") ||
		!containsString(install.ValidArgs, "codex") ||
		!containsString(install.ValidArgs, "all") {
		t.Errorf("install valid args = %v", install.ValidArgs)
	}
}

func TestConnectRejectsMissingProject(t *testing.T) {
	output, err := runIntegrationCLICommand(
		t,
		"connect",
		"--project",
		filepath.Join(t.TempDir(), "missing"),
	)
	if err == nil {
		t.Fatal("connect accepted a missing project")
	}

	if !strings.Contains(err.Error(), "project path") {
		t.Fatalf("error = %v, want a project path error\n%s", err, output)
	}
}

func TestIntegrationsInstallAndStatusPrintJSON(t *testing.T) {
	root := testProject(t)

	output, err := runIntegrationCLICommand(t, "integrations", "install", "all", "--project", root)
	if err != nil {
		t.Fatalf("installing integrations: %v\n%s", err, output)
	}

	var install integration.InstallReport
	if err := json.Unmarshal([]byte(output), &install); err != nil {
		t.Fatalf("install output is not JSON: %v\n%s", err, output)
	}

	if install.ProjectRoot != root {
		t.Errorf("install project root = %q, want %q", install.ProjectRoot, root)
	}

	if len(install.Clients) != 3 {
		t.Fatalf("install report has %d clients, want 3", len(install.Clients))
	}

	if !filepath.IsAbs(install.HostPath) {
		t.Errorf("host path is not absolute: %q", install.HostPath)
	}

	for _, path := range []string{
		filepath.Join(root, "opencode.json"),
		filepath.Join(root, ".mcp.json"),
		filepath.Join(root, ".codex", "config.toml"),
		filepath.Join(root, ".agents", "skills", "godot-cli", "SKILL.md"),
		filepath.Join(root, ".claude", "skills", "godot-cli", "SKILL.md"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("install did not create %s: %v", path, err)
		}
	}

	output, err = runIntegrationCLICommand(t, "integrations", "status", "--project", root)
	if err != nil {
		t.Fatalf("checking integration status: %v\n%s", err, output)
	}

	var status integration.StatusReport
	if err := json.Unmarshal([]byte(output), &status); err != nil {
		t.Fatalf("status output is not JSON: %v\n%s", err, output)
	}

	for _, client := range status.Clients {
		if !client.Configured || !client.UpToDate {
			t.Errorf("%s status = %+v", client.Client, client)
		}
	}

	nested := filepath.Join(root, "nested", "directory")
	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatal(err)
	}

	t.Chdir(nested)

	output, err = runIntegrationCLICommand(t, "integrations", "status")
	if err != nil {
		t.Fatalf("discovering project for integration status: %v\n%s", err, output)
	}

	if err := json.Unmarshal([]byte(output), &status); err != nil {
		t.Fatalf("discovered status output is not JSON: %v\n%s", err, output)
	}

	if status.ProjectRoot != root {
		t.Errorf("discovered project root = %q, want %q", status.ProjectRoot, root)
	}
}

func TestIntegrationsInstallRejectsUnknownClient(t *testing.T) {
	root := testProject(t)

	output, err := runIntegrationCLICommand(t, "integrations", "install", "unknown", "--project", root)
	if err == nil {
		t.Fatal("unknown client was accepted")
	}

	if !strings.Contains(err.Error(), "unknown") {
		t.Fatalf("error = %v, want an unknown-client error\n%s", err, output)
	}
}

func runIntegrationCLICommand(t *testing.T, args ...string) (string, error) {
	t.Helper()

	command := cli.NewRootCommand("test")

	var output bytes.Buffer

	command.SetOut(&output)
	command.SetErr(&output)
	command.SetArgs(args)

	err := command.Execute()

	return output.String(), err
}

func findCLICommand(t *testing.T, parent *cobra.Command, name string) *cobra.Command {
	t.Helper()

	for _, command := range parent.Commands() {
		if command.Name() == name {
			return command
		}
	}

	t.Fatalf("%s command is not registered under %s", name, parent.CommandPath())

	return nil
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}

	return false
}
