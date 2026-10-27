package host_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/davidherring123/godot-cli/internal/agent/integration"
	"github.com/davidherring123/godot-cli/internal/hostcmd"
	"github.com/spf13/cobra"
)

func TestRootCommandSurfaceIsHostOnly(t *testing.T) {
	root := hostcmd.NewRootCommand("test")

	if root.Name() != "godot-mcp-host" {
		t.Errorf("root command name = %q", root.Name())
	}

	if root.Version != "test" {
		t.Errorf("root version = %q, want test", root.Version)
	}

	got := commandNames(root.Commands())

	want := []string{"connect", "integrations", "serve", "version"}
	if !slices.Equal(got, want) {
		t.Fatalf("host commands = %v, want %v", got, want)
	}

	for _, command := range root.Commands() {
		switch command.Name() {
		case "connect", "serve":
			assertOnlyProjectCommands(t, command)
		case "integrations":
			for _, child := range command.Commands() {
				assertOnlyProjectCommands(t, child)
			}
		}
	}
}

func TestRootHelpListsOnlyHostCommands(t *testing.T) {
	output := executeCommand(t, hostcmd.NewRootCommand("test"), "--help")

	for _, name := range []string{"serve", "connect", "integrations", "version"} {
		if !strings.Contains(output, "  "+name+" ") {
			t.Errorf("help does not list %s:\n%s", name, output)
		}
	}

	for _, name := range terminalCommandNames() {
		if strings.Contains(output, "  "+name+" ") {
			t.Errorf("help exposes terminal command %s:\n%s", name, output)
		}
	}
}

func TestHostCommandTreeHasNoTerminalDomains(t *testing.T) {
	seen := map[string]bool{}
	walkCommands(hostcmd.NewRootCommand("test"), func(command *cobra.Command) {
		seen[command.Name()] = true
	})

	for _, name := range terminalCommandNames() {
		if seen[name] {
			t.Errorf("host command tree exposes terminal command %q", name)
		}
	}

	for _, name := range []string{"serve", "connect", "integrations", "status", "install", "version"} {
		if !seen[name] {
			t.Errorf("host command tree is missing %q", name)
		}
	}
}

func TestServeCommandFlags(t *testing.T) {
	serve := findCommand(t, hostcmd.NewRootCommand("test"), "serve")

	defaults := map[string]string{
		"project":    "",
		"host":       "127.0.0.1",
		"port":       "0",
		"state-file": "",
	}

	for name, want := range defaults {
		flag := serve.Flags().Lookup(name)
		if flag == nil {
			t.Errorf("serve is missing --%s", name)
			continue
		}

		if flag.DefValue != want {
			t.Errorf("serve --%s default = %q, want %q", name, flag.DefValue, want)
		}
	}

	if serve.Flags().Lookup("transport") != nil {
		t.Error("serve exposes a terminal transport selector")
	}
}

func TestConnectCommandIsStableStdioEntrypoint(t *testing.T) {
	connect := findCommand(t, hostcmd.NewRootCommand("test"), "connect")

	if flag := connect.Flags().Lookup("project"); flag == nil {
		t.Error("connect is missing --project")
	}
}

func TestIntegrationsCommandSurface(t *testing.T) {
	integrations := findCommand(t, hostcmd.NewRootCommand("test"), "integrations")

	got := commandNames(integrations.Commands())
	want := []string{"install", "status"}
	if !slices.Equal(got, want) {
		t.Fatalf("integration commands = %v, want %v", got, want)
	}

	status := findCommand(t, integrations, "status")
	if flag := status.Flags().Lookup("project"); flag == nil {
		t.Error("integrations status is missing --project")
	}

	install := findCommand(t, integrations, "install")
	if flag := install.Flags().Lookup("project"); flag == nil {
		t.Error("integrations install is missing --project")
	}

	for _, client := range []string{"opencode", "claude", "codex", "all"} {
		if !slices.Contains(install.ValidArgs, client) {
			t.Errorf("install valid args = %v, missing %q", install.ValidArgs, client)
		}
	}
}

func TestVersionCommandsMatch(t *testing.T) {
	want := "godot-mcp-host v1.2.3\n"

	if got := executeCommand(t, hostcmd.NewRootCommand("v1.2.3"), "version"); got != want {
		t.Errorf("version output = %q, want %q", got, want)
	}

	if got := executeCommand(t, hostcmd.NewRootCommand("v1.2.3"), "--version"); got != want {
		t.Errorf("--version output = %q, want %q", got, want)
	}
}

func TestServeRejectsNonLoopbackHost(t *testing.T) {
	projectRoot := projectFixture(t)

	err := execute(hostcmd.NewRootCommand("test"), "serve", "--host", "0.0.0.0", "--project", projectRoot)
	if err == nil {
		t.Fatal("serve accepted a non-loopback host")
	}

	if !strings.Contains(err.Error(), "loopback") {
		t.Fatalf("error = %v, want a loopback error", err)
	}
}

func TestServeHasNoTransportSelector(t *testing.T) {
	projectRoot := projectFixture(t)

	err := execute(hostcmd.NewRootCommand("test"), "serve", "--transport", "http", "--project", projectRoot)
	if err == nil {
		t.Fatal("serve accepted --transport")
	}

	if !strings.Contains(err.Error(), "unknown flag") {
		t.Fatalf("error = %v, want an unknown flag error", err)
	}
}

func TestIntegrationCommandsPrintJSON(t *testing.T) {
	root := projectFixture(t)

	output := executeCommand(
		t,
		hostcmd.NewRootCommand("test"),
		"integrations",
		"install",
		"all",
		"--project",
		root,
	)

	var install integration.InstallReport
	if err := json.Unmarshal([]byte(output), &install); err != nil {
		t.Fatalf("install output is not JSON: %v\n%s", err, output)
	}

	if install.ProjectRoot != root || len(install.Clients) != 3 {
		t.Fatalf("install report = %+v", install)
	}

	output = executeCommand(
		t,
		hostcmd.NewRootCommand("test"),
		"integrations",
		"status",
		"--project",
		root,
	)

	var status integration.StatusReport
	if err := json.Unmarshal([]byte(output), &status); err != nil {
		t.Fatalf("status output is not JSON: %v\n%s", err, output)
	}

	for _, client := range status.Clients {
		if !client.Configured || !client.UpToDate {
			t.Errorf("%s status = %+v", client.Client, client)
		}
	}
}

func assertOnlyProjectCommands(t *testing.T, command *cobra.Command) {
	t.Helper()

	if flag := command.Flags().Lookup("project"); flag == nil {
		t.Errorf("%s is missing --project", command.CommandPath())
	}
}

func terminalCommandNames() []string {
	return []string{
		"batch",
		"editor",
		"init",
		"input",
		"mcp",
		"node",
		"project",
		"resource",
		"runtime",
		"scene",
		"validate",
	}
}

func commandNames(commands []*cobra.Command) []string {
	names := make([]string, 0, len(commands))
	for _, command := range commands {
		names = append(names, command.Name())
	}

	sort.Strings(names)

	return names
}

func walkCommands(command *cobra.Command, visit func(*cobra.Command)) {
	visit(command)

	for _, child := range command.Commands() {
		walkCommands(child, visit)
	}
}

func findCommand(t *testing.T, parent *cobra.Command, name string) *cobra.Command {
	t.Helper()

	for _, command := range parent.Commands() {
		if command.Name() == name {
			return command
		}
	}

	t.Fatalf("%s command is not registered under %s", name, parent.CommandPath())

	return nil
}

func executeCommand(t *testing.T, command *cobra.Command, args ...string) string {
	t.Helper()

	var output bytes.Buffer

	command.SetOut(&output)
	command.SetErr(&output)
	command.SetArgs(args)

	if err := command.Execute(); err != nil {
		t.Fatalf("executing %v: %v\n%s", args, err, output.String())
	}

	return output.String()
}

func execute(command *cobra.Command, args ...string) error {
	command.SetOut(&bytes.Buffer{})
	command.SetErr(&bytes.Buffer{})
	command.SetArgs(args)

	return command.Execute()
}

func projectFixture(t *testing.T) string {
	t.Helper()

	root := t.TempDir()

	if err := os.WriteFile(
		filepath.Join(root, "project.godot"),
		[]byte("[application]\n"),
		0o644,
	); err != nil {
		t.Fatal(err)
	}

	return root
}
