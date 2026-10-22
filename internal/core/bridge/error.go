package bridge

import "fmt"

type ErrorCode string

const (
	ErrorCodeInvalidRequest     ErrorCode = "invalid_request"
	ErrorCodeInvalidArgument    ErrorCode = "invalid_argument"
	ErrorCodeNotFound           ErrorCode = "not_found"
	ErrorCodeFailedPrecondition ErrorCode = "failed_precondition"
	ErrorCodeConflict           ErrorCode = "conflict"
	ErrorCodeBusy               ErrorCode = "busy"
	ErrorCodeUnsupported        ErrorCode = "unsupported"
	ErrorCodeActionFailed       ErrorCode = "action_failed"
	ErrorCodeInternalError      ErrorCode = "internal_error"
)

type APIError struct {
	Code    ErrorCode
	Message string
}

func (err *APIError) Error() string {
	return fmt.Sprintf("%s: %s", err.Code, err.Message)
}
