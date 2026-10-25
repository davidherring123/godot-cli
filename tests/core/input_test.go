package core_test

import (
	"testing"

	"github.com/davidherring123/godot-cli/internal/core/bridge"
	"github.com/davidherring123/godot-cli/internal/core/project/input"
	"github.com/davidherring123/godot-cli/tests/harness"
)

func TestInputListSucceeds(t *testing.T) {
	session := requireSuite(t)

	result := listInputActions(t, session)

	if len(result.Actions) == 0 {
		t.Fatal("expected built-in actions")
	}

	found := false

	for _, action := range result.Actions {
		if action.Name == "ui_accept" {
			found = true

			if len(action.Events) == 0 {
				t.Fatal("expected ui_accept to have bindings")
			}
		}
	}

	if !found {
		t.Fatal("expected input list to include ui_accept")
	}

	if err := session.LogJSON(input.ListName, result); err != nil {
		t.Fatal(err)
	}
}

func TestInputAddInspectDeleteRoundTrip(t *testing.T) {
	session := requireSuite(t)
	name := "cli_test_round_trip"

	deleteAction(t, session, name)
	t.Cleanup(func() { deleteAction(t, session, name) })

	added := addInputAction(t, session, name, floatPtr(0.25))

	if !added.Changed {
		t.Fatal("expected input.add to report a change")
	}

	if added.Deadzone != 0.25 {
		t.Fatalf("expected deadzone 0.25, got %v", added.Deadzone)
	}

	if len(added.Events) != 0 {
		t.Fatalf("expected no bindings, got %d", len(added.Events))
	}

	inspected := inspectInputAction(t, session, name)

	if inspected.Deadzone != 0.25 {
		t.Fatalf("expected deadzone 0.25, got %v", inspected.Deadzone)
	}

	deleted := deleteInputAction(t, session, name)

	if !deleted.Deleted {
		t.Fatal("expected input.delete to report a deletion")
	}

	_, err := bridge.Call[input.InspectParams, input.InspectResult](
		session.Client,
		input.InspectName,
		input.InspectParams{Action: name},
	)

	assertAPIErrorCode(t, err, bridge.ErrorCodeNotFound)
}

func TestInputAddDefaultDeadzone(t *testing.T) {
	session := requireSuite(t)
	name := "cli_test_default_deadzone"

	deleteAction(t, session, name)
	t.Cleanup(func() { deleteAction(t, session, name) })

	added := addInputAction(t, session, name, nil)

	if added.Deadzone != 0.5 {
		t.Fatalf("expected default deadzone 0.5, got %v", added.Deadzone)
	}
}

func TestInputAddDuplicateConflict(t *testing.T) {
	session := requireSuite(t)

	_, err := bridge.Call[input.AddParams, input.AddResult](
		session.Client,
		input.AddName,
		input.AddParams{Action: "ui_accept"},
	)

	assertAPIErrorCode(t, err, bridge.ErrorCodeConflict)
}

func TestInputAddInvalidDeadzoneRejected(t *testing.T) {
	session := requireSuite(t)

	_, err := bridge.Call[input.AddParams, input.AddResult](
		session.Client,
		input.AddName,
		input.AddParams{Action: "cli_test_bad_deadzone", Deadzone: floatPtr(2)},
	)

	assertAPIErrorCode(t, err, bridge.ErrorCodeInvalidArgument)
}

func TestInputSetDeadzone(t *testing.T) {
	session := requireSuite(t)
	name := "cli_test_set_deadzone"

	deleteAction(t, session, name)
	t.Cleanup(func() { deleteAction(t, session, name) })

	addInputAction(t, session, name, nil)

	updated := setInputAction(t, session, name, 0.75)

	if !updated.Changed {
		t.Fatal("expected input.set to report a change")
	}

	if updated.Deadzone != 0.75 {
		t.Fatalf("expected deadzone 0.75, got %v", updated.Deadzone)
	}

	noop := setInputAction(t, session, name, 0.75)

	if noop.Changed {
		t.Fatal("expected setting the current deadzone to report no change")
	}
}

func TestInputRenameSucceeds(t *testing.T) {
	session := requireSuite(t)
	oldName := "cli_test_rename_old"
	newName := "cli_test_rename_new"

	deleteAction(t, session, oldName)
	deleteAction(t, session, newName)
	t.Cleanup(func() { deleteAction(t, session, newName) })

	addInputAction(t, session, oldName, floatPtr(0.4))

	result := renameInputAction(t, session, oldName, newName)

	if !result.Changed {
		t.Fatal("expected input.rename to report a change")
	}

	inspected := inspectInputAction(t, session, newName)

	if inspected.Deadzone != 0.4 {
		t.Fatalf("expected deadzone 0.4, got %v", inspected.Deadzone)
	}

	_, err := bridge.Call[input.InspectParams, input.InspectResult](
		session.Client,
		input.InspectName,
		input.InspectParams{Action: oldName},
	)

	assertAPIErrorCode(t, err, bridge.ErrorCodeNotFound)
}

func TestInputRenameConflict(t *testing.T) {
	session := requireSuite(t)
	firstName := "cli_test_rename_conflict_a"
	secondName := "cli_test_rename_conflict_b"

	deleteAction(t, session, firstName)
	deleteAction(t, session, secondName)
	t.Cleanup(func() {
		deleteAction(t, session, firstName)
		deleteAction(t, session, secondName)
	})

	addInputAction(t, session, firstName, nil)
	addInputAction(t, session, secondName, nil)

	_, err := bridge.Call[input.RenameParams, input.RenameResult](
		session.Client,
		input.RenameName,
		input.RenameParams{Action: firstName, NewName: secondName},
	)

	assertAPIErrorCode(t, err, bridge.ErrorCodeConflict)
}

func TestInputRenameBuiltInRejected(t *testing.T) {
	session := requireSuite(t)

	_, err := bridge.Call[input.RenameParams, input.RenameResult](
		session.Client,
		input.RenameName,
		input.RenameParams{Action: "ui_accept", NewName: "accept_renamed"},
	)

	assertAPIErrorCode(t, err, bridge.ErrorCodeFailedPrecondition)
}

func TestInputDeleteBuiltInRejected(t *testing.T) {
	session := requireSuite(t)

	_, err := bridge.Call[input.DeleteParams, input.DeleteResult](
		session.Client,
		input.DeleteName,
		input.DeleteParams{Action: "ui_accept"},
	)

	assertAPIErrorCode(t, err, bridge.ErrorCodeFailedPrecondition)
}

func TestInputEventAddKeySucceeds(t *testing.T) {
	session := requireSuite(t)
	name := "cli_test_event_key"

	deleteAction(t, session, name)
	t.Cleanup(func() { deleteAction(t, session, name) })

	addInputAction(t, session, name, nil)

	result := addInputEvent(t, session, name, input.Event{
		Type:            input.EventTypeKey,
		PhysicalKeycode: intPtr(32),
		Shift:           true,
	})

	if !result.Changed {
		t.Fatal("expected input.event_add to report a change")
	}

	if result.Index != 0 {
		t.Fatalf("expected index 0, got %d", result.Index)
	}

	if result.Event.Type != input.EventTypeKey {
		t.Fatalf("expected key event, got %s", result.Event.Type)
	}

	inspected := inspectInputAction(t, session, name)

	if len(inspected.Events) != 1 {
		t.Fatalf("expected one binding, got %d", len(inspected.Events))
	}

	event := inspected.Events[0]

	if event.PhysicalKeycode == nil || *event.PhysicalKeycode != 32 {
		t.Fatalf("expected physical keycode 32, got %+v", event.PhysicalKeycode)
	}

	if !event.Shift {
		t.Fatal("expected the shift modifier to be preserved")
	}
}

func TestInputEventAddJoypadMotionSucceeds(t *testing.T) {
	session := requireSuite(t)
	name := "cli_test_event_motion"

	deleteAction(t, session, name)
	t.Cleanup(func() { deleteAction(t, session, name) })

	addInputAction(t, session, name, nil)

	addInputEvent(t, session, name, input.Event{
		Type:      input.EventTypeJoypadMotion,
		Axis:      intPtr(0),
		AxisValue: floatPtr(-1),
	})

	inspected := inspectInputAction(t, session, name)

	if len(inspected.Events) != 1 {
		t.Fatalf("expected one binding, got %d", len(inspected.Events))
	}

	event := inspected.Events[0]

	if event.Type != input.EventTypeJoypadMotion {
		t.Fatalf("expected joypad_motion, got %s", event.Type)
	}

	if event.AxisValue == nil || *event.AxisValue != -1 {
		t.Fatalf("expected axis_value -1, got %+v", event.AxisValue)
	}
}

func TestInputEventAddUnsupportedRejected(t *testing.T) {
	session := requireSuite(t)
	name := "cli_test_event_bad"

	deleteAction(t, session, name)
	t.Cleanup(func() { deleteAction(t, session, name) })

	addInputAction(t, session, name, nil)

	_, err := bridge.Call[input.EventAddParams, input.EventAddResult](
		session.Client,
		input.EventAddName,
		input.EventAddParams{
			Action: name,
			Event:  input.Event{Type: "touch"},
		},
	)

	assertAPIErrorCode(t, err, bridge.ErrorCodeInvalidArgument)
}

func TestInputEventAddMalformedFieldRejected(t *testing.T) {
	session := requireSuite(t)
	name := "cli_test_event_malformed"

	deleteAction(t, session, name)
	t.Cleanup(func() { deleteAction(t, session, name) })

	addInputAction(t, session, name, nil)

	type malformedEventParams struct {
		Action string         `json:"action"`
		Event  map[string]any `json:"event"`
	}

	_, err := bridge.Call[malformedEventParams, input.EventAddResult](
		session.Client,
		input.EventAddName,
		malformedEventParams{
			Action: name,
			Event: map[string]any{
				"type":    input.EventTypeKey,
				"keycode": 32,
				"shift":   "yes",
			},
		},
	)

	assertAPIErrorCode(t, err, bridge.ErrorCodeInvalidArgument)
}

func TestInputEventDeleteSucceeds(t *testing.T) {
	session := requireSuite(t)
	name := "cli_test_event_delete"

	deleteAction(t, session, name)
	t.Cleanup(func() { deleteAction(t, session, name) })

	addInputAction(t, session, name, nil)

	addInputEvent(t, session, name, input.Event{
		Type:        input.EventTypeMouseButton,
		ButtonIndex: intPtr(1),
	})
	addInputEvent(t, session, name, input.Event{
		Type:        input.EventTypeJoypadButton,
		ButtonIndex: intPtr(0),
	})

	result, err := bridge.Call[input.EventDeleteParams, input.EventDeleteResult](
		session.Client,
		input.EventDeleteName,
		input.EventDeleteParams{Action: name, Index: 0},
	)
	if err != nil {
		t.Fatalf("expected input.event_delete to succeed: %v", err)
	}

	if !result.Changed {
		t.Fatal("expected input.event_delete to report a change")
	}

	if len(result.Events) != 1 {
		t.Fatalf("expected one remaining binding, got %d", len(result.Events))
	}

	if result.Events[0].Type != input.EventTypeJoypadButton {
		t.Fatalf("expected the joypad binding to remain, got %s", result.Events[0].Type)
	}
}

func TestInputEventDeleteOutOfRangeReturnsNotFound(t *testing.T) {
	session := requireSuite(t)
	name := "cli_test_event_range"

	deleteAction(t, session, name)
	t.Cleanup(func() { deleteAction(t, session, name) })

	addInputAction(t, session, name, nil)

	_, err := bridge.Call[input.EventDeleteParams, input.EventDeleteResult](
		session.Client,
		input.EventDeleteName,
		input.EventDeleteParams{Action: name, Index: 3},
	)

	assertAPIErrorCode(t, err, bridge.ErrorCodeNotFound)
}

func TestInputInspectMissingReturnsNotFound(t *testing.T) {
	session := requireSuite(t)

	_, err := bridge.Call[input.InspectParams, input.InspectResult](
		session.Client,
		input.InspectName,
		input.InspectParams{Action: "cli_test_missing"},
	)

	assertAPIErrorCode(t, err, bridge.ErrorCodeNotFound)
}

func listInputActions(t *testing.T, session *harness.Session) input.ListResult {
	t.Helper()

	result, err := bridge.Call[input.ListParams, input.ListResult](
		session.Client,
		input.ListName,
		input.ListParams{},
	)
	if err != nil {
		t.Fatalf("expected input.list to succeed: %v", err)
	}

	return result
}

func addInputAction(
	t *testing.T,
	session *harness.Session,
	name string,
	deadzone *float64,
) input.AddResult {
	t.Helper()

	result, err := bridge.Call[input.AddParams, input.AddResult](
		session.Client,
		input.AddName,
		input.AddParams{Action: name, Deadzone: deadzone},
	)
	if err != nil {
		t.Fatalf("expected input.add to succeed: %v", err)
	}

	return result
}

func setInputAction(
	t *testing.T,
	session *harness.Session,
	name string,
	deadzone float64,
) input.SetResult {
	t.Helper()

	result, err := bridge.Call[input.SetParams, input.SetResult](
		session.Client,
		input.SetName,
		input.SetParams{Action: name, Deadzone: deadzone},
	)
	if err != nil {
		t.Fatalf("expected input.set to succeed: %v", err)
	}

	return result
}

func inspectInputAction(
	t *testing.T,
	session *harness.Session,
	name string,
) input.InspectResult {
	t.Helper()

	result, err := bridge.Call[input.InspectParams, input.InspectResult](
		session.Client,
		input.InspectName,
		input.InspectParams{Action: name},
	)
	if err != nil {
		t.Fatalf("expected input.inspect to succeed: %v", err)
	}

	return result
}

func renameInputAction(
	t *testing.T,
	session *harness.Session,
	name string,
	newName string,
) input.RenameResult {
	t.Helper()

	result, err := bridge.Call[input.RenameParams, input.RenameResult](
		session.Client,
		input.RenameName,
		input.RenameParams{Action: name, NewName: newName},
	)
	if err != nil {
		t.Fatalf("expected input.rename to succeed: %v", err)
	}

	return result
}

func deleteInputAction(
	t *testing.T,
	session *harness.Session,
	name string,
) input.DeleteResult {
	t.Helper()

	result, err := bridge.Call[input.DeleteParams, input.DeleteResult](
		session.Client,
		input.DeleteName,
		input.DeleteParams{Action: name},
	)
	if err != nil {
		t.Fatalf("expected input.delete to succeed: %v", err)
	}

	return result
}

func addInputEvent(
	t *testing.T,
	session *harness.Session,
	name string,
	event input.Event,
) input.EventAddResult {
	t.Helper()

	result, err := bridge.Call[input.EventAddParams, input.EventAddResult](
		session.Client,
		input.EventAddName,
		input.EventAddParams{Action: name, Event: event},
	)
	if err != nil {
		t.Fatalf("expected input.event_add to succeed: %v", err)
	}

	return result
}

func deleteAction(t *testing.T, session *harness.Session, name string) {
	t.Helper()

	_, _ = bridge.Call[input.DeleteParams, input.DeleteResult](
		session.Client,
		input.DeleteName,
		input.DeleteParams{Action: name},
	)
}

func intPtr(value int) *int {
	return &value
}

func floatPtr(value float64) *float64 {
	return &value
}
