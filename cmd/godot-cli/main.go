package main

import (
	"fmt"
	"os"

	"github.com/davidherring123/godot-cli/internal/cli"
)

const VERSION = "dev"

func main() {
	root := cli.NewRootCommand(VERSION)

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
