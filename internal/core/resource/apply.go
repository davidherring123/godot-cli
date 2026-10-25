package resource

import (
	"fmt"

	"github.com/davidherring123/godot-cli/internal/port/invocation"
	"github.com/davidherring123/godot-cli/internal/support/batch"
)

// ApplyName is the ordered resource batch action. MCP exposes it as
// resource_apply.
const ApplyName = "resource.apply"

// Apply runs ordered resource mutations. The addon validates operations in
// order, resolves references to resources created earlier in the batch, and
// applies writes only after a no-write preview pass. If a later write fails,
// every affected file is restored on a best-effort basis and a rollback failure
// is reported as internal_error.
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

			if operation.Type != "" {
				return batch.InvalidOperation(
					index,
					operation.Operation,
					target,
					"set does not accept a type",
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
