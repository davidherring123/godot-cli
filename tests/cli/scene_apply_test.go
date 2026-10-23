package cli_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/davidherring123/godot-cli/internal/cli"
)

func TestSceneApplyRejectsMalformedOperations(t *testing.T) {
	root := testProject(t)

	output, err := runCLI(t, root, "scene", "apply", "--operations", "not json")

	if err == nil {
		t.Fatalf("expected malformed operations to fail:\n%s", output)
	}

	if !strings.Contains(err.Error(), "operations must be a JSON array") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSceneApplyRejectsInvalidBatchWithoutEditor(t *testing.T) {
	root := testProject(t)

	output, err := runCLI(
		t,
		root,
		"scene",
		"apply",
		"--operations",
		`[{"operation":"node.explode"}]`,
	)

	if err == nil {
		t.Fatalf("expected an invalid batch to fail:\n%s", output)
	}

	if !strings.Contains(err.Error(), "Unsupported operation") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSceneApplyRejectsTrailingJSON(t *testing.T) {
	root := testProject(t)

	output, err := runCLI(
		t,
		root,
		"scene",
		"apply",
		"--operations",
		`[{"operation":"node.delete","node":"."}] []`,
	)

	if err == nil {
		t.Fatalf("expected trailing JSON to fail:\n%s", output)
	}

	if !strings.Contains(err.Error(), "single JSON array") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSceneApplyRejectsOldOpField(t *testing.T) {
	root := testProject(t)

	output, err := runCLI(
		t,
		root,
		"scene",
		"apply",
		"--operations",
		`[{"op":"node.delete","node":"."}]`,
	)

	if err == nil {
		t.Fatalf("expected the old op field to fail:\n%s", output)
	}

	if !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSceneApplyRejectsEmptyID(t *testing.T) {
	root := testProject(t)

	output, err := runCLI(
		t,
		root,
		"scene",
		"apply",
		"--operations",
		`[{"operation":"node.delete","id":"","node":"."}]`,
	)

	if err == nil {
		t.Fatalf("expected an empty id to fail:\n%s", output)
	}

	if !strings.Contains(err.Error(), "nonempty string") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSceneApplyRejectsDryRunWithSave(t *testing.T) {
	root := testProject(t)

	output, err := runCLI(
		t,
		root,
		"scene",
		"apply",
		"--dry-run",
		"--save",
		"--operations",
		`[{"operation":"node.delete","node":"."}]`,
	)

	if err == nil {
		t.Fatalf("expected dry_run with save to fail:\n%s", output)
	}

	if !strings.Contains(err.Error(), "save cannot be combined with dry_run") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func runCLI(t *testing.T, root string, args ...string) (string, error) {
	t.Helper()

	t.Chdir(root)

	command := cli.NewRootCommand("test")

	var output bytes.Buffer

	command.SetOut(&output)
	command.SetErr(&output)
	command.SetArgs(args)

	err := command.Execute()

	return output.String(), err
}
