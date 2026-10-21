package skill

import (
	"encoding/json"
	"testing"

	"github.com/spf13/cobra"
)

func TestGenerateSkipsIgnoredCommands(t *testing.T) {
	root := &cobra.Command{Use: "godot-cli", Version: "test"}
	root.AddCommand(
		&cobra.Command{
			Use:         "mcp",
			Short:       "Run an MCP server",
			Annotations: map[string]string{IgnoreKey: "true"},
		},
		&cobra.Command{Use: "status", Short: "Show status"},
	)

	files, err := Generate(root)
	if err != nil {
		t.Fatalf("generating skill files: %v", err)
	}

	var reference Reference
	if err := json.Unmarshal(files["references/commands.json"], &reference); err != nil {
		t.Fatalf("reading commands.json: %v", err)
	}

	foundStatus := false

	for _, command := range reference.Commands {
		if command.Path == "godot-cli mcp" {
			t.Error("ignored mcp command was included in the skill reference")
		}

		if command.Path == "godot-cli status" {
			foundStatus = true
		}
	}

	if !foundStatus {
		t.Error("status command was not included in the skill reference")
	}
}
