package main

import (
	"fmt"
	"os"

	"github.com/davidherring123/godot-cli/internal/bridge"
	"github.com/davidherring123/godot-cli/internal/node"
	"github.com/davidherring123/godot-cli/internal/project"
)

func runNode(args ParsedArgs) {
	if len(args.Positionals) < 2 {
		fmt.Fprintln(
			os.Stderr,
			"Usage: godot-cli node <command>",
		)
		os.Exit(1)
	}

	switch args.Positionals[1] {
	case "inspect":
		runNodeInspect(args)

	default:
		fmt.Fprintf(
			os.Stderr,
			"Unknown node command: %s\n",
			args.Positionals[1],
		)
		os.Exit(1)
	}
}

func runNodeInspect(args ParsedArgs) {
	if len(args.Positionals) < 4 {
		fmt.Fprintln(
			os.Stderr,
			"Usage: godot-cli node inspect <scene> <node>",
		)
		os.Exit(1)
	}

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}

	root, err := project.FindProjectRoot(cwd)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}

	client, err := bridge.Connect(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
	defer client.Close()

	result, err := node.Inspect(
		client,
		args.Positionals[2],
		args.Positionals[3],
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}

	printJSON(result)
}
