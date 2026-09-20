package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/davidherring123/godot-cli/internal/addon"
	"github.com/davidherring123/godot-cli/internal/project"
)

func runInit(args ParsedArgs) {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}

	root, err := project.FindProjectRoot(cwd)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error: no Godot project found")
		os.Exit(1)
	}

	fmt.Println("Godot project found:", root)

	installed := addon.IsInstalled(root)

	if installed {
		fmt.Println("Godot CLI is already installed in this project.")
		return
	}

	projectName := filepath.Base(root)

	if !confirm(
		fmt.Sprintf(
			"Install Godot CLI addon into %s (%s)?",
			projectName,
			root,
		),
	) {
		return
	}

	runInstallAddon(root)
}

func runInstallAddon(projectRoot string) {

	if err := addon.Install(projectRoot); err != nil {
		fmt.Fprintln(os.Stderr, "Error installing addon:", err)
		os.Exit(1)
	}

	fmt.Println("✓ Installed addons/godot_cli\n\nEnable the addon in Godot:\nProject → Project Settings → Plugins → Godot CLI")
}
