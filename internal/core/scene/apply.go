package scene

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/davidherring123/godot-cli/internal/port/invocation"
	"github.com/davidherring123/godot-cli/internal/protocol/apierr"
	"github.com/davidherring123/godot-cli/internal/support/batch"
)

const ApplyName = "scene.apply"

var applyIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`)

// applyAllowedFields is the source of truth for which fields each operation
// accepts. The addon uses the matching table so bridge and MCP callers see the
// same diagnostics.
var applyAllowedFields = map[string]map[string]bool{
	"node.add": {
		"operation": true, "id": true, "type": true, "parent": true,
		"name": true, "properties": true,
	},
	"node.set": {
		"operation": true, "id": true, "node": true, "properties": true,
	},
	"node.rename": {
		"operation": true, "id": true, "node": true, "name": true,
	},
	"node.duplicate": {
		"operation": true, "id": true, "node": true, "name": true,
	},
	"node.move": {
		"operation": true, "id": true, "node": true, "parent": true,
		"index": true, "keep_global_transform": true,
	},
	"node.delete": {
		"operation": true, "id": true, "node": true,
	},
	"node.instantiate": {
		"operation": true, "id": true, "scene": true, "parent": true,
		"name": true, "properties": true,
	},
}

var applyFieldOrder = []string{
	"operation",
	"id",
	"node",
	"parent",
	"type",
	"name",
	"scene",
	"properties",
	"index",
	"keep_global_transform",
}

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

	if params.DryRun && params.Save {
		return &apierr.Error{
			Code:    apierr.CodeInvalidArgument,
			Message: "save cannot be combined with dry_run",
		}
	}

	defined := make(map[string]bool, len(params.Operations))

	for index, operation := range params.Operations {
		if err := validateApplyOperation(index, operation, defined); err != nil {
			return err
		}

		if operation.ID != "" {
			defined[operation.ID] = true
		}
	}

	return nil
}

func validateApplyOperation(
	index int,
	operation ApplyOperation,
	defined map[string]bool,
) error {
	op := operation.Operation
	target := applyOperationTarget(operation)

	if op == "" {
		return batch.InvalidOperation(
			index,
			"",
			target,
			"An operation is required",
		)
	}

	allowed, ok := applyAllowedFields[op]

	if !ok {
		return batch.InvalidOperation(
			index,
			op,
			target,
			fmt.Sprintf("Unsupported operation: %s", op),
		)
	}

	for _, field := range applyFieldOrder {
		if operation.HasField(field) && !allowed[field] {
			return batch.InvalidOperation(
				index,
				op,
				target,
				fmt.Sprintf("%s does not accept %s", op, field),
			)
		}
	}

	if err := validateOperationID(index, operation, defined); err != nil {
		return err
	}

	switch op {
	case "node.add":
		return validateAdd(index, operation, defined)
	case "node.set":
		return validateSet(index, operation, defined)
	case "node.rename":
		return validateRename(index, operation, defined)
	case "node.duplicate":
		return validateDuplicate(index, operation, defined)
	case "node.move":
		return validateMove(index, operation, defined)
	case "node.delete":
		return validateDelete(index, operation, defined)
	case "node.instantiate":
		return validateInstantiate(index, operation, defined)
	}

	return nil
}

func applyOperationTarget(operation ApplyOperation) string {
	if operation.Node != "" {
		return operation.Node
	}

	if operation.Parent != "" {
		return operation.Parent
	}

	return operation.ID
}

func validateAdd(index int, operation ApplyOperation, defined map[string]bool) error {
	if operation.Type == "" {
		return applyError(index, operation.Operation, applyOperationTarget(operation), "A type is required")
	}

	if operation.Parent == "" {
		return applyError(index, operation.Operation, applyOperationTarget(operation), "A parent is required")
	}

	return validateReference(index, operation.Operation, "parent", operation.Parent, defined)
}

func validateSet(index int, operation ApplyOperation, defined map[string]bool) error {
	if operation.Node == "" {
		return applyError(index, operation.Operation, applyOperationTarget(operation), "A node is required")
	}

	if len(operation.Properties) == 0 {
		return applyError(
			index,
			operation.Operation,
			applyOperationTarget(operation),
			"A nonempty properties object is required",
		)
	}

	return validateReference(index, operation.Operation, "node", operation.Node, defined)
}

func validateRename(index int, operation ApplyOperation, defined map[string]bool) error {
	if operation.Node == "" {
		return applyError(index, operation.Operation, applyOperationTarget(operation), "A node is required")
	}

	if operation.Name == "" {
		return applyError(index, operation.Operation, applyOperationTarget(operation), "A name is required")
	}

	return validateReference(index, operation.Operation, "node", operation.Node, defined)
}

func validateDuplicate(index int, operation ApplyOperation, defined map[string]bool) error {
	if operation.Node == "" {
		return applyError(index, operation.Operation, applyOperationTarget(operation), "A node is required")
	}

	return validateReference(index, operation.Operation, "node", operation.Node, defined)
}

func validateMove(index int, operation ApplyOperation, defined map[string]bool) error {
	if operation.Node == "" {
		return applyError(index, operation.Operation, applyOperationTarget(operation), "A node is required")
	}

	if operation.Parent == "" {
		return applyError(index, operation.Operation, applyOperationTarget(operation), "A parent is required")
	}

	if operation.Index != nil && *operation.Index < -1 {
		return applyError(
			index,
			operation.Operation,
			applyOperationTarget(operation),
			"index must be zero or greater; omit it to move the node to the end",
		)
	}

	if err := validateReference(index, operation.Operation, "node", operation.Node, defined); err != nil {
		return err
	}

	return validateReference(index, operation.Operation, "parent", operation.Parent, defined)
}

func validateDelete(index int, operation ApplyOperation, defined map[string]bool) error {
	if operation.Node == "" {
		return applyError(index, operation.Operation, applyOperationTarget(operation), "A node is required")
	}

	return validateReference(index, operation.Operation, "node", operation.Node, defined)
}

func validateInstantiate(index int, operation ApplyOperation, defined map[string]bool) error {
	if operation.Scene == "" {
		return applyError(index, operation.Operation, applyOperationTarget(operation), "A scene is required")
	}

	if operation.Parent == "" {
		return applyError(index, operation.Operation, applyOperationTarget(operation), "A parent is required")
	}

	return validateReference(index, operation.Operation, "parent", operation.Parent, defined)
}

func validateOperationID(
	index int,
	operation ApplyOperation,
	defined map[string]bool,
) error {
	if !operation.HasField("id") {
		return nil
	}

	if operation.ID == "" {
		return applyError(
			index,
			operation.Operation,
			applyOperationTarget(operation),
			"An id must be a nonempty string",
		)
	}

	if !applyIDPattern.MatchString(operation.ID) {
		return applyError(
			index,
			operation.Operation,
			applyOperationTarget(operation),
			"Invalid id: %s",
			operation.ID,
		)
	}

	if defined[operation.ID] {
		return applyError(
			index,
			operation.Operation,
			applyOperationTarget(operation),
			"Duplicate operation id: %s",
			operation.ID,
		)
	}

	return nil
}

func validateReference(
	index int,
	op string,
	field string,
	value string,
	defined map[string]bool,
) error {
	if !strings.HasPrefix(value, "@") {
		return nil
	}

	id := strings.TrimPrefix(value, "@")

	if id == "" {
		return applyError(
			index,
			op,
			"",
			"%s has an empty operation reference",
			field,
		)
	}

	if !defined[id] {
		return applyError(
			index,
			op,
			"",
			"%s refers to unknown or not-yet-defined operation id %s",
			field,
			value,
		)
	}

	return nil
}

func applyError(index int, op, target, format string, args ...any) error {
	return batch.InvalidOperation(
		index,
		op,
		target,
		fmt.Sprintf(format, args...),
	)
}
