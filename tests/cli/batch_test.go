package cli_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/davidherring123/godot-cli/internal/cli"
)

func TestApplyCommandsRejectTrailingJSONGarbage(t *testing.T) {
	for _, command := range [][]string{
		{"project", "settings", "apply"},
		{"input", "apply"},
		{"resource", "apply"},
	} {
		for _, operations := range []string{
			`[{"operation":"set"}] trailing`,
			`[{"operation":"set"}] []`,
		} {
			err := runBatchCommand(t, append(command, "--operations", operations)...)

			if err == nil {
				t.Fatalf("%v accepted malformed operations %q", command, operations)
			}

			if !strings.Contains(err.Error(), "single JSON array") {
				t.Fatalf(
					"%v returned %q for %q; want a single JSON array error",
					command,
					err,
					operations,
				)
			}
		}
	}
}

func TestApplyCommandsRejectUnknownOperationFields(t *testing.T) {
	err := runBatchCommand(
		t,
		"project",
		"settings",
		"apply",
		"--operations",
		`[{"operation":"unset","setting":"application/config/name","typo":true}]`,
	)

	if err == nil {
		t.Fatal("expected an unknown-field error")
	}

	if !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("expected an unknown field error, got %v", err)
	}
}

func TestApplyCommandsRejectNonArrayOperations(t *testing.T) {
	err := runBatchCommand(
		t,
		"input",
		"apply",
		"--operations",
		`{"operation":"add"}`,
	)

	if err == nil {
		t.Fatal("expected a JSON array error")
	}

	if !strings.Contains(err.Error(), "must be a JSON array") {
		t.Fatalf("expected a JSON array error, got %v", err)
	}
}

func runBatchCommand(t *testing.T, args ...string) error {
	t.Helper()

	command := cli.NewRootCommand("test")

	var output bytes.Buffer

	command.SetOut(&output)
	command.SetErr(&output)
	command.SetArgs(args)

	return command.Execute()
}
