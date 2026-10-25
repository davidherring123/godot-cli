// Package apierr defines transport-neutral error codes and errors for actions.
//
// The codes are part of the CLI, MCP, and addon wire contract. They are kept
// free of transport details so domain packages can validate arguments and
// report failures without importing an adapter.
package apierr

import "fmt"

// Code is a stable machine-readable action error code.
type Code string

const (
	CodeInvalidRequest     Code = "invalid_request"
	CodeInvalidArgument    Code = "invalid_argument"
	CodeNotFound           Code = "not_found"
	CodeFailedPrecondition Code = "failed_precondition"
	CodeConflict           Code = "conflict"
	CodeBusy               Code = "busy"
	CodeUnsupported        Code = "unsupported"
	CodeActionFailed       Code = "action_failed"
	CodeInternalError      Code = "internal_error"
)

// Error reports an action failure with a stable code and human-readable
// message.
type Error struct {
	Code    Code
	Message string
}

func (err *Error) Error() string {
	return fmt.Sprintf("%s: %s", err.Code, err.Message)
}
