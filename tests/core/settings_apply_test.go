package core_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/davidherring123/godot-cli/internal/core/batch"
	"github.com/davidherring123/godot-cli/internal/core/bridge"
	"github.com/davidherring123/godot-cli/internal/core/project/settings"
	"github.com/davidherring123/godot-cli/tests/harness"
)

func TestProjectSettingsApplyDryRunAndApply(t *testing.T) {
	session := requireSuite(t)

	restoreSetting(t, session, viewportWidthSetting)
	restoreSetting(t, session, projectNameSetting)

	dryRun, err := settings.Apply(session.Root(), settings.ApplyParams{
		Operations: []settings.ApplyOperation{
			{
				Operation: "set",
				Setting:   viewportWidthSetting,
				Value:     json.RawMessage(`1024`),
			},
			{
				Operation: "unset",
				Setting:   projectNameSetting,
			},
		},
		DryRun: true,
	})
	if err != nil {
		t.Fatalf("expected project.settings.apply dry run to succeed: %v", err)
	}

	if !dryRun.DryRun || dryRun.Applied {
		t.Fatalf("expected an unapplied dry run, got %+v", dryRun)
	}

	if dryRun.OperationCount != 2 || dryRun.Changed != 2 || dryRun.Unchanged != 0 {
		t.Fatalf("unexpected dry-run summary: %+v", dryRun)
	}

	if dryRun.Operations[0].Status != "updated" ||
		dryRun.Operations[1].Status != "unset" {
		t.Fatalf("unexpected dry-run statuses: %+v", dryRun.Operations)
	}

	if value := decodeNumber(
		t,
		getProjectSetting(t, session, viewportWidthSetting).Value,
	); value != 800 {
		t.Fatalf("dry run changed viewport width to %v", value)
	}

	if !getProjectSetting(t, session, projectNameSetting).Overridden {
		t.Fatal("dry run removed the project name override")
	}

	applied, err := settings.Apply(session.Root(), settings.ApplyParams{
		Operations: []settings.ApplyOperation{
			{
				Operation: "set",
				Setting:   viewportWidthSetting,
				Value:     json.RawMessage(`1024`),
			},
			{
				Operation: "unset",
				Setting:   projectNameSetting,
			},
		},
	})
	if err != nil {
		t.Fatalf("expected project.settings.apply to succeed: %v", err)
	}

	if applied.DryRun || !applied.Applied || applied.Changed != 2 {
		t.Fatalf("unexpected apply summary: %+v", applied)
	}

	if value := decodeNumber(
		t,
		getProjectSetting(t, session, viewportWidthSetting).Value,
	); value != 1024 {
		t.Fatalf("expected viewport width 1024, got %v", value)
	}

	if getProjectSetting(t, session, projectNameSetting).Overridden {
		t.Fatal("expected the project name override to be removed")
	}
}

func TestProjectSettingsApplyFailureRollsBackWithoutSaving(t *testing.T) {
	session := requireSuite(t)

	restoreSetting(t, session, viewportWidthSetting)

	before := readProjectFile(t, session)
	missing := "godot_cli/apply_missing"

	_, err := settings.Apply(session.Root(), settings.ApplyParams{
		Operations: []settings.ApplyOperation{
			{
				Operation: "set",
				Setting:   viewportWidthSetting,
				Value:     json.RawMessage(`1024`),
			},
			{
				Operation: "set",
				Setting:   missing,
				Value:     json.RawMessage(`1`),
			},
		},
	})

	assertAPIErrorCode(t, err, bridge.ErrorCodeNotFound)

	var apiError *bridge.APIError

	if !errors.As(err, &apiError) {
		t.Fatalf("expected an API error, got %v", err)
	}

	for _, expected := range []string{"operation 1", "set", missing, "no changes were saved"} {
		if !strings.Contains(apiError.Message, expected) {
			t.Fatalf("expected %q in %q", expected, apiError.Message)
		}
	}

	if value := decodeNumber(
		t,
		getProjectSetting(t, session, viewportWidthSetting).Value,
	); value != 800 {
		t.Fatalf("expected the first operation to be rolled back, got %v", value)
	}

	if after := readProjectFile(t, session); after != before {
		t.Fatalf("expected project.godot to be untouched")
	}
}

func TestProjectSettingsApplyNoOpReportsUnchanged(t *testing.T) {
	session := requireSuite(t)

	restoreSetting(t, session, viewportWidthSetting)

	result, err := settings.Apply(session.Root(), settings.ApplyParams{
		Operations: []settings.ApplyOperation{
			{
				Operation: "set",
				Setting:   viewportWidthSetting,
				Value:     json.RawMessage(`800`),
			},
		},
	})
	if err != nil {
		t.Fatalf("expected project.settings.apply to succeed: %v", err)
	}

	if !result.Applied || result.Changed != 0 || result.Unchanged != 1 {
		t.Fatalf("expected one unchanged operation, got %+v", result)
	}

	if result.Operations[0].Status != "unchanged" {
		t.Fatalf("unexpected status: %+v", result.Operations[0])
	}
}

func TestProjectSettingsApplyRejectsOversizedBatch(t *testing.T) {
	operations := make([]settings.ApplyOperation, batch.MaxOperations+1)

	for index := range operations {
		operations[index] = settings.ApplyOperation{
			Operation: "unset",
			Setting:   viewportWidthSetting,
		}
	}

	_, err := settings.Apply("", settings.ApplyParams{Operations: operations})

	assertAPIErrorCode(t, err, bridge.ErrorCodeInvalidArgument)

	var apiError *bridge.APIError

	if !errors.As(err, &apiError) || !strings.Contains(apiError.Message, "At most 100") {
		t.Fatalf("expected an operation limit error, got %v", err)
	}
}

func restoreSetting(t *testing.T, session *harness.Session, name string) {
	t.Helper()

	original := getProjectSetting(t, session, name)

	t.Cleanup(func() {
		if original.Overridden {
			_, _ = bridge.Call[
				settings.SetParams,
				settings.SetResult,
			](
				session.Client,
				settings.SetName,
				settings.SetParams{
					Setting: name,
					Value:   original.Value,
				},
			)

			return
		}

		_, _ = bridge.Call[
			settings.UnsetParams,
			settings.UnsetResult,
		](
			session.Client,
			settings.UnsetName,
			settings.UnsetParams{Setting: name},
		)
	})
}

func readProjectFile(t *testing.T, session *harness.Session) string {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(session.Root(), "project.godot"))
	if err != nil {
		t.Fatalf("reading project.godot: %v", err)
	}

	return string(data)
}
