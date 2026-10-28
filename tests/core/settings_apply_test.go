package core_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/davidherring123/godot-mcp/internal/adapter/bridge"
	"github.com/davidherring123/godot-mcp/internal/core/project/settings"
	"github.com/davidherring123/godot-mcp/internal/protocol/apierr"
	"github.com/davidherring123/godot-mcp/internal/support/batch"
	"github.com/davidherring123/godot-mcp/tests/harness"
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
	missing := "godot_mcp/apply_missing"

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

	assertAPIErrorCode(t, err, apierr.CodeNotFound)

	var apiError *apierr.Error

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

func TestProjectSettingsApplySetThenUnsetUsesBatchState(t *testing.T) {
	session := requireSuite(t)

	restoreSetting(t, session, viewportWidthSetting)

	operations := []settings.ApplyOperation{
		{
			Operation: "set",
			Setting:   viewportWidthSetting,
			Value:     json.RawMessage(`1024`),
		},
		{
			Operation: "unset",
			Setting:   viewportWidthSetting,
		},
	}

	dryRun, err := settings.Apply(session.Root(), settings.ApplyParams{
		Operations: operations,
		DryRun:     true,
	})
	if err != nil {
		t.Fatalf("expected the set-then-unset dry run to succeed: %v", err)
	}

	if dryRun.Changed != 2 || dryRun.Unchanged != 0 {
		t.Fatalf("unexpected dry-run summary: %+v", dryRun)
	}

	if dryRun.Operations[0].Status != "updated" ||
		dryRun.Operations[1].Status != "unset" {
		t.Fatalf("unexpected dry-run statuses: %+v", dryRun.Operations)
	}

	// The dry run simulated an override that was removed again, so the
	// original override must still be on disk.
	if value := decodeNumber(
		t,
		getProjectSetting(t, session, viewportWidthSetting).Value,
	); value != 800 {
		t.Fatalf("dry run changed viewport width to %v", value)
	}

	if !getProjectSetting(t, session, viewportWidthSetting).Overridden {
		t.Fatal("dry run removed the original override")
	}

	applied, err := settings.Apply(session.Root(), settings.ApplyParams{
		Operations: operations,
	})
	if err != nil {
		t.Fatalf("expected the set-then-unset batch to succeed: %v", err)
	}

	if applied.Changed != 2 || applied.Unchanged != 0 {
		t.Fatalf("unexpected apply summary: %+v", applied)
	}

	if getProjectSetting(t, session, viewportWidthSetting).Overridden {
		t.Fatal("expected unset to remove the override set earlier in the batch")
	}

	if value := decodeNumber(
		t,
		getProjectSetting(t, session, viewportWidthSetting).Value,
	); value != 1152 {
		t.Fatalf("expected the engine default after unset, got %v", value)
	}
}

func TestProjectSettingsApplyDuplicateUnsetReportsUnchanged(t *testing.T) {
	session := requireSuite(t)

	restoreSetting(t, session, viewportWidthSetting)
	setProjectSetting(
		t,
		session,
		viewportWidthSetting,
		json.RawMessage(`1024`),
	)

	result, err := settings.Apply(session.Root(), settings.ApplyParams{
		Operations: []settings.ApplyOperation{
			{Operation: "unset", Setting: viewportWidthSetting},
			{Operation: "unset", Setting: viewportWidthSetting},
		},
	})
	if err != nil {
		t.Fatalf("expected duplicate unset to succeed: %v", err)
	}

	if result.Changed != 1 || result.Unchanged != 1 {
		t.Fatalf("unexpected duplicate-unset summary: %+v", result)
	}

	if result.Operations[0].Status != "unset" ||
		result.Operations[1].Status != "unchanged" {
		t.Fatalf("unexpected duplicate-unset statuses: %+v", result.Operations)
	}

	if getProjectSetting(t, session, viewportWidthSetting).Overridden {
		t.Fatal("expected the override to be removed")
	}
}

func TestProjectSettingsApplySetToDefaultClearsOverrideState(t *testing.T) {
	session := requireSuite(t)

	restoreSetting(t, session, viewportWidthSetting)
	setProjectSetting(
		t,
		session,
		viewportWidthSetting,
		json.RawMessage(`1024`),
	)

	result, err := settings.Apply(session.Root(), settings.ApplyParams{
		Operations: []settings.ApplyOperation{
			{
				Operation: "set",
				Setting:   viewportWidthSetting,
				Value:     json.RawMessage(`1152`),
			},
			{Operation: "unset", Setting: viewportWidthSetting},
		},
	})
	if err != nil {
		t.Fatalf("expected set-to-default then unset to succeed: %v", err)
	}

	if result.Changed != 1 || result.Unchanged != 1 {
		t.Fatalf("unexpected set-to-default summary: %+v", result)
	}

	if result.Operations[1].Status != "unchanged" {
		t.Fatalf(
			"expected unset after set-to-default to be unchanged: %+v",
			result.Operations,
		)
	}

	if getProjectSetting(t, session, viewportWidthSetting).Overridden {
		t.Fatal("expected setting the default value to clear the override")
	}
}

func TestProjectSettingsApplyAddonRejectsExclusiveFields(t *testing.T) {
	session := requireSuite(t)

	_, err := bridge.Call[settings.ApplyParams, settings.ApplyResult](
		session.Client,
		settings.ApplyName,
		settings.ApplyParams{
			Operations: []settings.ApplyOperation{
				{
					Operation: "unset",
					Setting:   viewportWidthSetting,
					Value:     json.RawMessage(`1`),
				},
			},
		},
	)

	assertAPIErrorCode(t, err, apierr.CodeInvalidArgument)

	var apiError *apierr.Error

	if !errors.As(err, &apiError) ||
		!strings.Contains(apiError.Message, "does not accept value") {
		t.Fatalf("expected an addon field exclusivity error, got %v", err)
	}
}

func TestProjectSettingsApplyNoOpDoesNotRewriteProjectFile(t *testing.T) {
	session := requireSuite(t)

	restoreSetting(t, session, viewportWidthSetting)

	before := projectFileModTime(t, session)

	time.Sleep(20 * time.Millisecond)

	if _, err := settings.Apply(session.Root(), settings.ApplyParams{
		Operations: []settings.ApplyOperation{
			{
				Operation: "set",
				Setting:   viewportWidthSetting,
				Value:     json.RawMessage(`800`),
			},
		},
	}); err != nil {
		t.Fatalf("expected the no-op batch to succeed: %v", err)
	}

	if after := projectFileModTime(t, session); !after.Equal(before) {
		t.Fatalf(
			"all-no-op batch rewrote project.godot: before %s, after %s",
			before,
			after,
		)
	}
}

func projectFileModTime(t *testing.T, session *harness.Session) time.Time {
	t.Helper()

	info, err := os.Stat(filepath.Join(session.Root(), "project.godot"))
	if err != nil {
		t.Fatalf("stating project.godot: %v", err)
	}

	return info.ModTime()
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

	assertAPIErrorCode(t, err, apierr.CodeInvalidArgument)

	var apiError *apierr.Error

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
