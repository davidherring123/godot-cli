package validate

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveSceneConfiguredMainSceneUID(t *testing.T) {
	root := t.TempDir()
	project := `[application]
run/main_scene="uid://abc123"
`

	if err := os.WriteFile(
		filepath.Join(root, "project.godot"),
		[]byte(project),
		0644,
	); err != nil {
		t.Fatal(err)
	}

	scene, err := resolveScene(root, "")
	if err != nil {
		t.Fatalf("resolving configured main scene: %v", err)
	}

	if scene != "uid://abc123" {
		t.Fatalf("expected uid://abc123, got %s", scene)
	}
}

func TestResolveSceneExplicitUID(t *testing.T) {
	scene, err := resolveScene(t.TempDir(), "uid://abc123")
	if err != nil {
		t.Fatalf("resolving explicit UID: %v", err)
	}

	if scene != "uid://abc123" {
		t.Fatalf("expected uid://abc123, got %s", scene)
	}
}

func TestResolveSceneConfiguredMainSceneResourcePath(t *testing.T) {
	root := t.TempDir()
	project := `[application]
run/main_scene="res://main.tscn"
`

	if err := os.WriteFile(
		filepath.Join(root, "project.godot"),
		[]byte(project),
		0644,
	); err != nil {
		t.Fatal(err)
	}

	scene, err := resolveScene(root, "")
	if err != nil {
		t.Fatalf("resolving configured main scene: %v", err)
	}

	if scene != "res://main.tscn" {
		t.Fatalf("expected res://main.tscn, got %s", scene)
	}
}

func TestResolveSceneExplicitResourcePath(t *testing.T) {
	scene, err := resolveScene(t.TempDir(), "res://main.tscn")
	if err != nil {
		t.Fatalf("resolving explicit resource path: %v", err)
	}

	if scene != "res://main.tscn" {
		t.Fatalf("expected res://main.tscn, got %s", scene)
	}
}
