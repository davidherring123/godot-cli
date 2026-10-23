package resource

import (
	"fmt"

	"github.com/davidherring123/godot-cli/internal/core/batch"
	"github.com/davidherring123/godot-cli/internal/core/bridge"
)

// ApplyName is the ordered resource batch action. MCP exposes it as
// resource_apply.
const ApplyName = "resource.apply"

// Apply runs ordered resource mutations. The addon validates every operation
// before writing, then applies them in order and restores affected files when
// a later write fails.
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
		target := operation.Resource

		switch operation.Operation {
		case "create":
			if operation.Resource == "" {
				return batch.InvalidOperation(
					index,
					operation.Operation,
					target,
					"A resource path is required",
				)
			}

			if operation.Type == "" {
				return batch.InvalidOperation(
					index,
					operation.Operation,
					target,
					"A resource type is required",
				)
			}
		case "set":
			if operation.Resource == "" {
				return batch.InvalidOperation(
					index,
					operation.Operation,
					target,
					"A resource path is required",
				)
			}

			if len(operation.Properties) == 0 {
				return batch.InvalidOperation(
					index,
					operation.Operation,
					target,
					"A nonempty properties object is required",
				)
			}
		case "delete":
			if operation.Resource == "" {
				return batch.InvalidOperation(
					index,
					operation.Operation,
					target,
					"A resource path is required",
				)
			}

			if operation.Type != "" {
				return batch.InvalidOperation(
					index,
					operation.Operation,
					target,
					"delete does not accept a type",
				)
			}

			if len(operation.Properties) > 0 {
				return batch.InvalidOperation(
					index,
					operation.Operation,
					target,
					"delete does not accept properties",
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
