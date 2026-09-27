package main

import (
	"fmt"
	"os"

	"github.com/davidherring123/godot-cli/internal/commands"
	"github.com/davidherring123/godot-cli/internal/commands/node"
	"github.com/davidherring123/godot-cli/internal/commands/project"
	"github.com/davidherring123/godot-cli/internal/commands/scene"
)

const VERSION = "dev"

func main() {
	root := commands.NewRootCommand(VERSION)

	root.AddCommand(
		project.NewInitCommand(),
		project.NewStatusCommand(),
		scene.NewCommand(),
		node.NewCommand(),
	)

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
