package input

import (
	"fmt"

	"github.com/davidherring123/godot-cli/internal/core/batch"
)

// ApplyName is the ordered Input Map batch action. MCP exposes it as
// input_map_apply.
const ApplyName = "input.map_apply"

// Apply runs ordered Input Map mutations. The addon validates every operation,
// applies them in memory, and saves project.godot once.
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
