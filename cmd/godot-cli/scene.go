package main

import (
	"fmt"
	"os"

	"github.com/davidherring123/godot-cli/internal/project"
	"github.com/davidherring123/godot-cli/internal/scene"
)

type SceneListOutput struct {
	Scenes []string `json:"scenes"`
}

func runScene(args ParsedArgs) {
	if len(args.Positionals) < 2 {
		fmt.Println("Usage: godot-cli scene <command>")
		os.Exit(1)
	}

	switch args.Positionals[1] {
	case "list":
		runSceneList(args.Flags)
	default:
		fmt.Printf("Unkown scene command: %s\n", args.Positionals[1])
		os.Exit(1)
	}
}

func runSceneList(flags Flags) {
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

	scenes, err := scene.List(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}

	if flags.JSON {
		printJSON(SceneListOutput{
			Scenes: scenes,
		})
		return
	}

	for _, scenePath := range scenes {
		fmt.Println(scenePath)
	}
}
