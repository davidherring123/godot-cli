package core_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/davidherring123/godot-cli/internal/core/batch"
	"github.com/davidherring123/godot-cli/internal/core/bridge"
	"github.com/davidherring123/godot-cli/internal/core/input"
	"github.com/davidherring123/godot-cli/tests/harness"
)

func TestInputMapApplyDryRunAndApplyOrdered(t *testing.T) {
	session := requireSuite(t)
	name := "cli_test_batch"
	renamed := "cli_test_batch_renamed"

	deleteAction(t, session, name)
	deleteAction(t, session, renamed)
	t.Cleanup(func() {
		deleteAction(t, session, name)
		deleteAction(t, session, renamed)
	})

	operations := []input.ApplyOperation{
		{
			Operation: "add",
			Action:    name,
			Deadzone:  floatPtr(0.25),
		},
		{
			Operation: "event_add",
			Action:    name,
			Event: &input.Event{
				Type:            input.EventTypeKey,
				PhysicalKeycode: intPtr(32),
			},
		},
		{
			Operation: "rename",
			Action:    name,
			NewName:   renamed,
		},
	}

	dryRun, err := input.Apply(session.Root(), input.ApplyParams{
		Operations: operations,
		DryRun:     true,
	})
	if err != nil {
		t.Fatalf("expected input.map_apply dry run to succeed: %v", err)
	}

	if !dryRun.DryRun || dryRun.Applied {
		t.Fatalf("expected an unapplied dry run, got %+v", dryRun)
	}

	if dryRun.OperationCount != 3 || dryRun.Changed != 3 || dryRun.Unchanged != 0 {
		t.Fatalf("unexpected dry-run summary: %+v", dryRun)
	}

	assertOperationStatuses(t, dryRun.Operations, []string{
		"added",
		"binding_added",
		"renamed",
	})

	assertInputMissing(t, session, name)
	assertInputMissing(t, session, renamed)

	applied, err := input.Apply(session.Root(), input.ApplyParams{
		Operations: operations,
	})
	if err != nil {
		t.Fatalf("expected input.map_apply to succeed: %v", err)
	}

	if applied.DryRun || !applied.Applied || applied.Changed != 3 {
		t.Fatalf("unexpected apply summary: %+v", applied)
	}

	inspected := inspectInputAction(t, session, renamed)

	if inspected.Deadzone != 0.25 {
		t.Fatalf("expected deadzone 0.25, got %v", inspected.Deadzone)
	}

	if len(inspected.Events) != 1 {
		t.Fatalf("expected one binding, got %d", len(inspected.Events))
	}

	if inspected.Events[0].PhysicalKeycode == nil ||
		*inspected.Events[0].PhysicalKeycode != 32 {
		t.Fatalf("unexpected binding: %+v", inspected.Events[0])
	}
}

func TestInputMapApplyEventDeleteAndDelete(t *testing.T) {
	session := requireSuite(t)
	name := "cli_test_batch_remove"

	deleteAction(t, session, name)
	t.Cleanup(func() { deleteAction(t, session, name) })

	_, err := input.Apply(session.Root(), input.ApplyParams{
		Operations: []input.ApplyOperation{
			{
				Operation: "add",
				Action:    name,
			},
			{
				Operation: "event_add",
				Action:    name,
				Event: &input.Event{
					Type:        input.EventTypeMouseButton,
					ButtonIndex: intPtr(1),
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("seeding input.map_apply batch: %v", err)
	}

	removed, err := input.Apply(session.Root(), input.ApplyParams{
		Operations: []input.ApplyOperation{
			{
				Operation: "event_delete",
				Action:    name,
				Index:     intPtr(0),
			},
			{
				Operation: "delete",
				Action:    name,
			},
		},
	})
	if err != nil {
		t.Fatalf("expected input.map_apply removal to succeed: %v", err)
	}

	if !removed.Applied || removed.Changed != 2 {
		t.Fatalf("unexpected removal summary: %+v", removed)
	}

	assertOperationStatuses(t, removed.Operations, []string{
		"binding_deleted",
		"deleted",
	})

	if removed.Operations[0].BindingIndex == nil ||
		*removed.Operations[0].BindingIndex != 0 {
		t.Fatalf("expected binding index 0, got %+v", removed.Operations[0])
	}

	assertInputMissing(t, session, name)
}

func TestInputMapApplyFailureRestoresInMemoryState(t *testing.T) {
	session := requireSuite(t)
	name := "cli_test_batch_rollback"

	deleteAction(t, session, name)
	t.Cleanup(func() { deleteAction(t, session, name) })

	before := readProjectFile(t, session)

	_, err := input.Apply(session.Root(), input.ApplyParams{
		Operations: []input.ApplyOperation{
			{
				Operation: "add",
				Action:    name,
			},
			{
				Operation: "event_delete",
				Action:    name,
				Index:     intPtr(5),
			},
		},
	})

	assertAPIErrorCode(t, err, bridge.ErrorCodeNotFound)

	var apiError *bridge.APIError

	if !errors.As(err, &apiError) {
		t.Fatalf("expected an API error, got %v", err)
	}

	for _, expected := range []string{
		"operation 1",
		"event_delete",
		name,
		"Binding index out of range",
		"no changes were saved",
	} {
		if !strings.Contains(apiError.Message, expected) {
			t.Fatalf("expected %q in %q", expected, apiError.Message)
		}
	}

	assertInputMissing(t, session, name)

	if after := readProjectFile(t, session); after != before {
		t.Fatal("expected project.godot to be untouched")
	}
}

func TestInputMapApplyRejectsOversizedBatch(t *testing.T) {
	operations := make([]input.ApplyOperation, batch.MaxOperations+1)

	for index := range operations {
		operations[index] = input.ApplyOperation{
			Operation: "delete",
			Action:    "cli_test_batch_limit",
		}
	}

	_, err := input.Apply("", input.ApplyParams{Operations: operations})

	assertAPIErrorCode(t, err, bridge.ErrorCodeInvalidArgument)

	var apiError *bridge.APIError

	if !errors.As(err, &apiError) || !strings.Contains(apiError.Message, "At most 100") {
		t.Fatalf("expected an operation limit error, got %v", err)
	}
}

func TestInputMapApplyReportsInvalidOperation(t *testing.T) {
	_, err := input.Apply("", input.ApplyParams{
		Operations: []input.ApplyOperation{
			{
				Operation: "set",
				Action:    "jump",
			},
		},
	})

	assertAPIErrorCode(t, err, bridge.ErrorCodeInvalidArgument)

	var apiError *bridge.APIError

	if !errors.As(err, &apiError) {
		t.Fatalf("expected an API error, got %v", err)
	}

	for _, expected := range []string{"operation 0", "set", "jump", "deadzone"} {
		if !strings.Contains(apiError.Message, expected) {
			t.Fatalf("expected %q in %q", expected, apiError.Message)
		}
	}
}

func TestInputMapApplyRejectsFieldsFromOtherOperations(t *testing.T) {
	deadzone := 0.2
	event := input.Event{
		Type:            input.EventTypeKey,
		PhysicalKeycode: intPtr(32),
	}
	index := 0

	cases := []struct {
		name      string
		operation input.ApplyOperation
		field     string
	}{
		{
			name: "add with new_name",
			operation: input.ApplyOperation{
				Operation: "add",
				Action:    "jump",
				NewName:   "leap",
			},
			field: "new_name",
		},
		{
			name: "set with event",
			operation: input.ApplyOperation{
				Operation: "set",
				Action:    "jump",
				Deadzone:  &deadzone,
				Event:     &event,
			},
			field: "event",
		},
		{
			name: "rename with deadzone",
			operation: input.ApplyOperation{
				Operation: "rename",
				Action:    "jump",
				NewName:   "leap",
				Deadzone:  &deadzone,
			},
			field: "deadzone",
		},
		{
			name: "delete with index",
			operation: input.ApplyOperation{
				Operation: "delete",
				Action:    "jump",
				Index:     &index,
			},
			field: "index",
		},
		{
			name: "event_add with deadzone",
			operation: input.ApplyOperation{
				Operation: "event_add",
				Action:    "jump",
				Deadzone:  &deadzone,
				Event:     &event,
			},
			field: "deadzone",
		},
		{
			name: "event_delete with event",
			operation: input.ApplyOperation{
				Operation: "event_delete",
				Action:    "jump",
				Index:     &index,
				Event:     &event,
			},
			field: "event",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := input.Apply("", input.ApplyParams{
				Operations: []input.ApplyOperation{testCase.operation},
			})

			assertAPIErrorCode(t, err, bridge.ErrorCodeInvalidArgument)

			var apiError *bridge.APIError

			if !errors.As(err, &apiError) {
				t.Fatalf("expected an API error, got %v", err)
			}

			if !strings.Contains(apiError.Message, "does not accept") ||
				!strings.Contains(apiError.Message, testCase.field) {
				t.Fatalf(
					"expected a field exclusivity error for %s, got %q",
					testCase.field,
					apiError.Message,
				)
			}
		})
	}
}

func TestInputMapApplyAddonRejectsExclusiveFields(t *testing.T) {
	session := requireSuite(t)

	_, err := bridge.Call[input.ApplyParams, input.ApplyResult](
		session.Client,
		input.ApplyName,
		input.ApplyParams{
			Operations: []input.ApplyOperation{
				{
					Operation: "add",
					Action:    "cli_test_batch_addon_fields",
					NewName:   "leap",
				},
			},
		},
	)

	assertAPIErrorCode(t, err, bridge.ErrorCodeInvalidArgument)

	var apiError *bridge.APIError

	if !errors.As(err, &apiError) ||
		!strings.Contains(apiError.Message, "does not accept new_name") {
		t.Fatalf("expected an addon field exclusivity error, got %v", err)
	}
}

func TestInputMapApplyNoOpDoesNotRewriteProjectFile(t *testing.T) {
	session := requireSuite(t)
	name := "cli_test_batch_noop"

	deleteAction(t, session, name)
	t.Cleanup(func() { deleteAction(t, session, name) })

	if _, err := input.Add(session.Root(), input.AddParams{
		Action:   name,
		Deadzone: floatPtr(0.3),
	}); err != nil {
		t.Fatalf("seeding the no-op action: %v", err)
	}

	before := projectFileModTime(t, session)

	time.Sleep(20 * time.Millisecond)

	result, err := input.Apply(session.Root(), input.ApplyParams{
		Operations: []input.ApplyOperation{
			{
				Operation: "set",
				Action:    name,
				Deadzone:  floatPtr(0.3),
			},
		},
	})
	if err != nil {
		t.Fatalf("expected the no-op batch to succeed: %v", err)
	}

	if result.Changed != 0 || result.Unchanged != 1 {
		t.Fatalf("unexpected no-op summary: %+v", result)
	}

	if after := projectFileModTime(t, session); !after.Equal(before) {
		t.Fatalf(
			"all-no-op batch rewrote project.godot: before %s, after %s",
			before,
			after,
		)
	}
}

func assertOperationStatuses(
	t *testing.T,
	operations []input.ApplyOperationSummary,
	statuses []string,
) {
	t.Helper()

	if len(operations) != len(statuses) {
		t.Fatalf(
			"got %d operation summaries; want %d",
			len(operations),
			len(statuses),
		)
	}

	for index, status := range statuses {
		if operations[index].Status != status {
			t.Fatalf(
				"operation %d status = %s; want %s",
				index,
				operations[index].Status,
				status,
			)
		}
	}
}

func assertInputMissing(
	t *testing.T,
	session *harness.Session,
	name string,
) {
	t.Helper()

	_, err := bridge.Call[input.InspectParams, input.InspectResult](
		session.Client,
		input.InspectName,
		input.InspectParams{Action: name},
	)

	assertAPIErrorCode(t, err, bridge.ErrorCodeNotFound)
}
