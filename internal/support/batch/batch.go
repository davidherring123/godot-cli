// Package batch provides shared validation and diagnostics for ordered,
// domain-specific mutation batches.
package batch

import (
	"fmt"

	"github.com/davidherring123/godot-cli/internal/protocol/apierr"
)

// MaxOperations is the maximum number of operations accepted by one apply
// action. Keeping the limit modest bounds request size and rollback cost while
// still covering realistic multi-step edits.
const MaxOperations = 100

// ValidateOperationCount checks the batch size before any editor call.
func ValidateOperationCount(count int) error {
	if count == 0 {
		return &apierr.Error{
			Code:    apierr.CodeInvalidArgument,
			Message: "A nonempty operations array is required",
		}
	}

	if count > MaxOperations {
		return &apierr.Error{
			Code: apierr.CodeInvalidArgument,
			Message: fmt.Sprintf(
				"At most %d operations are allowed",
				MaxOperations,
			),
		}
	}

	return nil
}

// InvalidOperation reports which operation failed validation. The format
// mirrors the addon so direct bridge callers and MCP callers see identical
// diagnostics.
func InvalidOperation(index int, operation, target, message string) error {
	return &apierr.Error{
		Code:    apierr.CodeInvalidArgument,
		Message: OperationMessage(index, operation, target, message),
	}
}

// OperationMessage formats one failed operation without assuming that every
// field is present.
func OperationMessage(index int, operation, target, message string) string {
	label := operation

	if target != "" {
		if label == "" {
			label = target
		} else {
			label = operation + " " + target
		}
	}

	if label == "" {
		return fmt.Sprintf("operation %d: %s", index, message)
	}

	return fmt.Sprintf("operation %d (%s): %s", index, label, message)
}
