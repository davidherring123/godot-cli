package main

import (
	"fmt"
	"os"

	"github.com/davidherring123/godot-cli/internal/bridge"
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
	case "tree":
		runSceneTree(args)
	case "list":
		runSceneList(args)
	default:
		fmt.Printf("Unkown scene command: %s\n", args.Positionals[1])
		os.Exit(1)
	}
}

func runSceneList(args ParsedArgs) {
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

	printJSON(SceneListOutput{
		Scenes: scenes,
	})
}

func runSceneTree(args ParsedArgs) {
	if len(args.Positionals) < 3 {
		fmt.Fprintln(os.Stderr, "Usage: godot-cli scene tree <scene>")
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

	tree, err := scene.Tree(
		client,
		args.Positionals[2],
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}

	printJSON(tree)
}
