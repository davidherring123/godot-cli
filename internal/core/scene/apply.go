package scene

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/davidherring123/godot-cli/internal/core/bridge"
)

const ApplyName = "scene.apply"

var applyIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`)

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
	if params.DryRun && params.Save {
		return applyError(-1, "", "save cannot be combined with dry_run")
	}

	if len(params.Operations) == 0 {
		return applyError(-1, "", "a nonempty operations array is required")
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
	switch operation.Op {
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
	case "":
		return applyError(index, "", "an op is required")
	default:
		return applyError(index, operation.Op, "unknown operation")
	}
}

func validateAdd(index int, operation ApplyOperation, defined map[string]bool) error {
	if err := validateOperationID(index, operation, defined); err != nil {
		return err
	}

	if err := rejectFields(index, operation.Op, map[string]bool{
		"node":                  operation.Node != "",
		"scene":                 operation.Scene != "",
		"index":                 operation.Index != nil,
		"keep_global_transform": operation.KeepGlobalTransform != nil,
	}); err != nil {
		return err
	}

	if operation.Type == "" {
		return applyError(index, operation.Op, "type is required")
	}

	if operation.Parent == "" {
		return applyError(index, operation.Op, "parent is required")
	}

	return validateReference(index, operation.Op, "parent", operation.Parent, defined)
}

func validateSet(index int, operation ApplyOperation, defined map[string]bool) error {
	if err := validateOperationID(index, operation, defined); err != nil {
		return err
	}

	if err := rejectFields(index, operation.Op, map[string]bool{
		"parent":                operation.Parent != "",
		"type":                  operation.Type != "",
		"name":                  operation.Name != "",
		"scene":                 operation.Scene != "",
		"index":                 operation.Index != nil,
		"keep_global_transform": operation.KeepGlobalTransform != nil,
	}); err != nil {
		return err
	}

	if operation.Node == "" {
		return applyError(index, operation.Op, "node is required")
	}

	if len(operation.Properties) == 0 {
		return applyError(index, operation.Op, "a nonempty properties object is required")
	}

	return validateReference(index, operation.Op, "node", operation.Node, defined)
}

func validateRename(index int, operation ApplyOperation, defined map[string]bool) error {
	if err := validateOperationID(index, operation, defined); err != nil {
		return err
	}

	if err := rejectFields(index, operation.Op, map[string]bool{
		"parent":                operation.Parent != "",
		"type":                  operation.Type != "",
		"scene":                 operation.Scene != "",
		"properties":            len(operation.Properties) > 0,
		"index":                 operation.Index != nil,
		"keep_global_transform": operation.KeepGlobalTransform != nil,
	}); err != nil {
		return err
	}

	if operation.Node == "" {
		return applyError(index, operation.Op, "node is required")
	}

	if operation.Name == "" {
		return applyError(index, operation.Op, "name is required")
	}

	return validateReference(index, operation.Op, "node", operation.Node, defined)
}

func validateDuplicate(index int, operation ApplyOperation, defined map[string]bool) error {
	if err := validateOperationID(index, operation, defined); err != nil {
		return err
	}

	if err := rejectFields(index, operation.Op, map[string]bool{
		"parent":                operation.Parent != "",
		"type":                  operation.Type != "",
		"scene":                 operation.Scene != "",
		"properties":            len(operation.Properties) > 0,
		"index":                 operation.Index != nil,
		"keep_global_transform": operation.KeepGlobalTransform != nil,
	}); err != nil {
		return err
	}

	if operation.Node == "" {
		return applyError(index, operation.Op, "node is required")
	}

	return validateReference(index, operation.Op, "node", operation.Node, defined)
}

func validateMove(index int, operation ApplyOperation, defined map[string]bool) error {
	if err := validateOperationID(index, operation, defined); err != nil {
		return err
	}

	if err := rejectFields(index, operation.Op, map[string]bool{
		"type":       operation.Type != "",
		"name":       operation.Name != "",
		"scene":      operation.Scene != "",
		"properties": len(operation.Properties) > 0,
	}); err != nil {
		return err
	}

	if operation.Node == "" {
		return applyError(index, operation.Op, "node is required")
	}

	if operation.Parent == "" {
		return applyError(index, operation.Op, "parent is required")
	}

	if operation.Index != nil && *operation.Index < -1 {
		return applyError(index, operation.Op, "index must be zero or greater")
	}

	if err := validateReference(index, operation.Op, "node", operation.Node, defined); err != nil {
		return err
	}

	return validateReference(index, operation.Op, "parent", operation.Parent, defined)
}

func validateDelete(index int, operation ApplyOperation, defined map[string]bool) error {
	if err := validateOperationID(index, operation, defined); err != nil {
		return err
	}

	if err := rejectFields(index, operation.Op, map[string]bool{
		"parent":                operation.Parent != "",
		"type":                  operation.Type != "",
		"name":                  operation.Name != "",
		"scene":                 operation.Scene != "",
		"properties":            len(operation.Properties) > 0,
		"index":                 operation.Index != nil,
		"keep_global_transform": operation.KeepGlobalTransform != nil,
	}); err != nil {
		return err
	}

	if operation.Node == "" {
		return applyError(index, operation.Op, "node is required")
	}

	return validateReference(index, operation.Op, "node", operation.Node, defined)
}

func validateInstantiate(index int, operation ApplyOperation, defined map[string]bool) error {
	if err := validateOperationID(index, operation, defined); err != nil {
		return err
	}

	if err := rejectFields(index, operation.Op, map[string]bool{
		"node":                  operation.Node != "",
		"type":                  operation.Type != "",
		"index":                 operation.Index != nil,
		"keep_global_transform": operation.KeepGlobalTransform != nil,
	}); err != nil {
		return err
	}

	if operation.Scene == "" {
		return applyError(index, operation.Op, "scene is required")
	}

	if operation.Parent == "" {
		return applyError(index, operation.Op, "parent is required")
	}

	return validateReference(index, operation.Op, "parent", operation.Parent, defined)
}

func validateOperationID(
	index int,
	operation ApplyOperation,
	defined map[string]bool,
) error {
	if operation.ID == "" {
		return nil
	}

	if !applyIDPattern.MatchString(operation.ID) {
		return applyError(index, operation.Op, "invalid id %q", operation.ID)
	}

	if defined[operation.ID] {
		return applyError(index, operation.Op, "duplicate id %q", operation.ID)
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
		return applyError(index, op, "%s has an empty operation reference", field)
	}

	if !defined[id] {
		return applyError(
			index,
			op,
			"%s refers to unknown or not-yet-defined operation id %q",
			field,
			value,
		)
	}

	return nil
}

// rejectFields reports the first field that is not valid for an operation.
func rejectFields(index int, op string, invalid map[string]bool) error {
	for _, field := range []string{
		"node",
		"parent",
		"type",
		"name",
		"scene",
		"properties",
		"index",
		"keep_global_transform",
	} {
		if invalid[field] {
			return applyError(index, op, "%s does not accept field %s", op, field)
		}
	}

	return nil
}

func applyError(index int, op string, format string, args ...any) error {
	label := "operations"

	if index >= 0 {
		label = fmt.Sprintf("operations[%d]", index)
	}

	if op != "" {
		label += " (" + op + ")"
	}

	return &bridge.APIError{
		Code:    bridge.ErrorCodeInvalidArgument,
		Message: label + ": " + fmt.Sprintf(format, args...),
	}
}
