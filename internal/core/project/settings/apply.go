package settings

import (
	"fmt"

	"github.com/davidherring123/godot-cli/internal/port/invocation"
	"github.com/davidherring123/godot-cli/internal/support/batch"
)

// ApplyName is the ordered project-settings batch action. MCP exposes it as
// project_settings_apply.
const ApplyName = "project.settings.apply"

// Apply runs ordered project-setting mutations. The addon validates and
// applies operations in order against batch-local override state, saves
// project.godot once, and skips the save when every operation is a no-op. A
// save failure restores in-memory state and attempts to restore the file; a
// rollback failure is reported as internal_error.
func Apply(projectRoot string, params ApplyParams) (ApplyResult, error) {
	if err := validateApply(params); err != nil {
		return ApplyResult{}, err
	}

	return invocation.Call[ApplyParams, ApplyResult](projectRoot, ApplyName, params)
}

func validateApply(params ApplyParams) error {
	if err := batch.ValidateOperationCount(len(params.Operations)); err != nil {
		return err
	}

	for index, operation := range params.Operations {
		target := operation.Setting

		switch operation.Operation {
		case "set":
			if operation.Setting == "" {
				return batch.InvalidOperation(
					index,
					operation.Operation,
					target,
					"A project setting name is required",
				)
			}

			if len(operation.Value) == 0 {
				return batch.InvalidOperation(
					index,
					operation.Operation,
					target,
					"A value is required",
				)
			}
		case "unset":
			if operation.Setting == "" {
				return batch.InvalidOperation(
					index,
					operation.Operation,
					target,
					"A project setting name is required",
				)
			}

			if len(operation.Value) > 0 {
				return batch.InvalidOperation(
					index,
					operation.Operation,
					target,
					"unset does not accept a value",
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
	}

	return nil
}
