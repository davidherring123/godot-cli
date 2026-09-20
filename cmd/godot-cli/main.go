package main

import (
	"fmt"
	"os"
)

const VERSION = "dev"

const USAGE_TEXT = "Usage: godot-cli <command>"

const HELP_TEXT = `godot-cli — CLI tooling for Godot projects

Usage:
  godot-cli <command> [options]

Options:
  -h, --help    Show help
  -v, --version Show version
`

func main() {
	args, err := parseArgs(os.Args)

	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}

	if len(args.Positionals) < 1 {
		fmt.Println(USAGE_TEXT)
		os.Exit(1)
	}

	switch args.Positionals[0] {
	case "--version", "-v":
		fmt.Printf("godot-cli %s\n", VERSION)
	case "--help", "-h":
		fmt.Printf(HELP_TEXT)
	case "status":
		runStatus()
	case "scene":
		runScene(args)
	case "init":
		runInit(args)

	default:
		fmt.Printf("Unkown command: %s\n", os.Args[1])
		os.Exit(1)
	}

	os.Exit(0)
}
