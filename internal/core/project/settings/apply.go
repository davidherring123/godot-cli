package settings

import (
	"fmt"

	"github.com/davidherring123/godot-cli/internal/core/batch"
	"github.com/davidherring123/godot-cli/internal/core/bridge"
)

// ApplyName is the ordered project-settings batch action. MCP exposes it as
// project_settings_apply.
const ApplyName = "project.settings.apply"

// Apply runs ordered project-setting mutations. The addon validates every
// operation, applies them in memory, and saves project.godot once.
func Apply(projectRoot string, params ApplyParams) (ApplyResult, error) {
	if err := validateApply(params); err != nil {
		return ApplyResult{}, err
	}

	client, err := bridge.Connect(projectRoot)
	if err != nil {
		return ApplyResult{}, err
	}
	defer func() { _ = client.Close() }()

	return bridge.Call[ApplyParams, ApplyResult](client, ApplyName, params)
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
