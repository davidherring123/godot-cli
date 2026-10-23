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
		`[{"op":"node.explode"}]`,
	)

	if err == nil {
		t.Fatalf("expected an invalid batch to fail:\n%s", output)
	}

	if !strings.Contains(err.Error(), "unknown operation") {
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
		`[{"op":"node.delete","node":"."}]`,
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
