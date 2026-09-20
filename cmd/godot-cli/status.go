package main

import (
	"fmt"
	"os"

	"github.com/davidherring123/godot-cli/internal/project"
)

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
