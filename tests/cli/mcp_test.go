package cli_test

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/davidherring123/godot-cli/internal/cli"
	"github.com/spf13/cobra"
)

func TestMCPCommandFlagDefaults(t *testing.T) {
	command := findMCPCLIFlagCommand(t, cli.NewRootCommand("test"))

	for name, want := range map[string]string{
		"transport":  "stdio",
		"host":       "127.0.0.1",
		"port":       "0",
		"state-file": "",
	} {
		flag := command.Flags().Lookup(name)
		if flag == nil {
			t.Fatalf("mcp command is missing --%s", name)
		}

		if flag.DefValue != want {
			t.Errorf("--%s default = %q, want %q", name, flag.DefValue, want)
		}
	}
}

func TestMCPCommandRejectsUnsupportedTransport(t *testing.T) {
	err := runMCPCLICommand(t, "mcp", "--transport", "websocket")
	if err == nil {
		t.Fatal("unsupported transport was accepted")
	}

	if !strings.Contains(err.Error(), "stdio") || !strings.Contains(err.Error(), "http") {
		t.Fatalf("error = %v, want the supported transports", err)
	}
}

func TestMCPCommandRejectsNonLoopbackHost(t *testing.T) {
	projectRoot, err := filepath.Abs(filepath.Join("..", "fixtures", "core"))
	if err != nil {
		t.Fatal(err)
	}

	err = runMCPCLICommand(
		t,
		"mcp",
		"--transport",
		"http",
		"--host",
		"0.0.0.0",
		"--project",
		projectRoot,
	)
	if err == nil {
		t.Fatal("non-loopback host was accepted")
	}

	if !strings.Contains(err.Error(), "loopback") {
		t.Fatalf("error = %v, want a loopback error", err)
	}
}

func runMCPCLICommand(t *testing.T, args ...string) error {
	t.Helper()

	command := cli.NewRootCommand("test")

	var output bytes.Buffer

	command.SetOut(&output)
	command.SetErr(&output)
	command.SetArgs(args)

	return command.Execute()
}

func findMCPCLIFlagCommand(t *testing.T, root *cobra.Command) *cobra.Command {
	t.Helper()

	for _, command := range root.Commands() {
		if command.Name() == "mcp" {
			return command
		}
	}

	t.Fatal("mcp command is not registered")

	return nil
}
