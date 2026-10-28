package input

import (
	"fmt"

	"github.com/davidherring123/godot-mcp/internal/support/batch"
)

// ApplyName is the ordered Input Map batch action. The CLI exposes it as
// "input apply" and MCP exposes it as input_map_apply; the map_apply suffix
// keeps the Input Map domain visible in the action name.
const ApplyName = "input.map_apply"

// Apply runs ordered Input Map mutations. The addon validates and applies
// operations in order, saves project.godot once, and skips the save (and live
// Input Map reload) when every operation is a no-op. A save failure restores
// in-memory state and attempts to restore the file; a rollback failure is
// reported as internal_error.
func Apply(projectRoot string, params ApplyParams) (ApplyResult, error) {
	if err := validateApply(params); err != nil {
		return ApplyResult{}, err
	}

	return call[ApplyParams, ApplyResult](projectRoot, ApplyName, params)
}

func validateApply(params ApplyParams) error {
	if err := batch.ValidateOperationCount(len(params.Operations)); err != nil {
		return err
	}

	for index, operation := range params.Operations {
		if err := validateApplyOperation(index, operation); err != nil {
			return err
		}
	}

	return nil
}

func validateApplyOperation(index int, operation ApplyOperation) error {
	target := operation.Action

	if target == "" {
		return batch.InvalidOperation(
			index,
			operation.Operation,
			target,
			"An action name is required",
		)
	}

	if err := validateApplyFields(index, operation); err != nil {
		return err
	}

	switch operation.Operation {
	case "add":
		return validateApplyDeadzone(index, operation)
	case "set":
		if operation.Deadzone == nil {
			return batch.InvalidOperation(
				index,
				operation.Operation,
				target,
				"A deadzone is required",
			)
		}

		return validateApplyDeadzone(index, operation)
	case "rename":
		if operation.NewName == "" {
			return batch.InvalidOperation(
				index,
				operation.Operation,
				target,
				"A new action name is required",
			)
		}
	case "delete":
	case "event_add":
		if operation.Event == nil {
			return batch.InvalidOperation(
				index,
				operation.Operation,
				target,
				"An event is required",
			)
		}

		if err := validateEvent(*operation.Event); err != nil {
			return batch.InvalidOperation(
				index,
				operation.Operation,
				target,
				err.Error(),
			)
		}
	case "event_delete":
		if operation.Index == nil || *operation.Index < 0 {
			return batch.InvalidOperation(
				index,
				operation.Operation,
				target,
				"An event index is required",
			)
		}
	default:
		return batch.InvalidOperation(
			index,
			operation.Operation,
			target,
			fmt.Sprintf(
				"Unsupported operation: %s",
				operation.Operation,
			),
		)
	}

	return nil
}

// applyOperationForbiddenFields lists the fields owned by other operations.
// The addon performs the same check so direct bridge callers get the same
// errors.
var applyOperationForbiddenFields = map[string][]string{
	"add":          {"new_name", "event", "index"},
	"set":          {"new_name", "event", "index"},
	"rename":       {"deadzone", "event", "index"},
	"delete":       {"deadzone", "new_name", "event", "index"},
	"event_add":    {"deadzone", "new_name", "index"},
	"event_delete": {"deadzone", "new_name", "event"},
}

func validateApplyFields(index int, operation ApplyOperation) error {
	for _, field := range applyOperationForbiddenFields[operation.Operation] {
		if !applyFieldPresent(operation, field) {
			continue
		}

		return batch.InvalidOperation(
			index,
			operation.Operation,
			operation.Action,
			fmt.Sprintf(
				"%s does not accept %s",
				operation.Operation,
				field,
			),
		)
	}

	return nil
}

func applyFieldPresent(operation ApplyOperation, field string) bool {
	switch field {
	case "deadzone":
		return operation.Deadzone != nil
	case "new_name":
		return operation.NewName != ""
	case "event":
		return operation.Event != nil
	case "index":
		return operation.Index != nil
	default:
		return false
	}
}

func validateApplyDeadzone(index int, operation ApplyOperation) error {
	if operation.Deadzone == nil {
		return nil
	}

	if err := validateDeadzone(*operation.Deadzone); err != nil {
		return batch.InvalidOperation(
			index,
			operation.Operation,
			operation.Action,
			err.Error(),
		)
	}

	return nil
}
