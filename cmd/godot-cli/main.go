package main

import (
	"fmt"
	"os"

	"github.com/davidherring123/godot-cli/internal/project"
)

const VERSION = "dev"

const HELP_TEXT = `godot-cli — CLI tooling for Godot projects

Usage:
  godot-cli <command> [options]

Commands:
  status        Show project and Godot information

Options:
  -h, --help    Show help
  -v, --version Show version
`

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: godot-cli <command>")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "--version", "-v":
		fmt.Printf("godot-cli %s\n", VERSION)
	case "--help", "-h":
		fmt.Printf(HELP_TEXT)
	case "status":
		runStatus()

	default:
		fmt.Printf("Unkown command: %s\n", os.Args[1])
		os.Exit(1)
	}

	os.Exit(0)
}

func runStatus() {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	root, err := project.FindProjectRoot(cwd)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}

	fmt.Println("Godot project:", root)
}