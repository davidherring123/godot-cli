package release_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/davidherring123/godot-cli/internal/release"
)

func TestBuildUsesRootOutputAndControlledEnvironment(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake Go command uses a POSIX shell script")
	}

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.test/release\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	addonDir := filepath.Join(root, filepath.FromSlash(release.DefaultAddonDir))
	if err := os.MkdirAll(addonDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(addonDir, "plugin.cfg"),
		[]byte("[plugin]\nversion=\"dev\"\n"),
		0o644,
	); err != nil {
		t.Fatal(err)
	}

	environmentPath := filepath.Join(root, "build-environment.txt")
	argumentsPath := filepath.Join(root, "build-arguments.txt")
	fakeGo := filepath.Join(root, "fake-go")
	script := `#!/bin/sh
env | sort > "` + environmentPath + `"
printf '%s\n' "$@" > "` + argumentsPath + `"
while [ "$#" -gt 0 ]; do
    if [ "$1" = "-o" ]; then
        printf 'fake host binary' > "$2"
        exit 0
    fi
    shift
done
exit 1
`
	if err := os.WriteFile(fakeGo, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	t.Setenv("GOAMD64", "v3")
	t.Setenv("GOARM64", "v9.5")
	t.Setenv("GOEXPERIMENT", "fieldtrack")
	t.Setenv("GOFLAGS", "-tags=unexpected")

	artifacts, err := release.Build(release.Options{
		Root:      root,
		Version:   "v1.2.3",
		Targets:   []release.Target{{OS: "linux", Arch: "amd64"}},
		GoCommand: fakeGo,
	})
	if err != nil {
		t.Fatalf("building release: %v", err)
	}

	wantDirectory := filepath.Join(root, "dist") + string(filepath.Separator)
	for _, artifact := range artifacts {
		if !strings.HasPrefix(artifact, wantDirectory) {
			t.Fatalf("artifact %q is outside %q", artifact, wantDirectory)
		}
	}

	environment, err := os.ReadFile(environmentPath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(environment)
	for _, unwanted := range []string{"GOEXPERIMENT=", "GOFLAGS=", "GOARM64="} {
		if strings.Contains(text, unwanted) {
			t.Fatalf("build inherited %s:\n%s", unwanted, text)
		}
	}
	if !strings.Contains(text, "GOAMD64=v1\n") {
		t.Fatalf("build did not set the amd64 baseline:\n%s", text)
	}

	arguments, err := os.ReadFile(argumentsPath)
	if err != nil {
		t.Fatal(err)
	}

	commandLine := strings.Split(strings.TrimSpace(string(arguments)), "\n")
	if len(commandLine) == 0 || commandLine[len(commandLine)-1] != "./cmd/godot-mcp-host" {
		t.Fatalf("release build command = %v, want ./cmd/godot-mcp-host", commandLine)
	}

	if strings.Contains(string(arguments), "./cmd/godot-cli\n") {
		t.Fatalf("release build still packages the legacy CLI:\n%s", arguments)
	}
}
