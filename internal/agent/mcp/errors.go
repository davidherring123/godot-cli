package mcp

import (
	"encoding/json"
	"errors"

	"github.com/davidherring123/godot-mcp/internal/core"
	"github.com/davidherring123/godot-mcp/internal/core/project/runtime"
	"github.com/davidherring123/godot-mcp/internal/port/editorinstance"
	"github.com/davidherring123/godot-mcp/internal/protocol/apierr"
	mcpgo "github.com/modelcontextprotocol/go-sdk/mcp"
)

type errorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func errorResult(err error) *mcpgo.CallToolResult {
	payload := errorPayload{
		Code:    string(apierr.CodeInternalError),
		Message: err.Error(),
	}

	var argumentError *core.ArgumentError
	var apiError *apierr.Error
	var runtimeError *runtime.Error

	switch {
	case errors.As(err, &apiError):
		payload.Code = string(apiError.Code)
		payload.Message = apiError.Message
	case errors.As(err, &runtimeError):
		if runtimeError.Code != "" {
			payload.Code = runtimeError.Code
		}

		payload.Message = runtimeError.Message
	case errors.As(err, &argumentError):
		payload.Code = string(apierr.CodeInvalidArgument)
		payload.Message = argumentError.Error()
	case errors.Is(err, editorinstance.ErrNoEditor),
		errors.Is(err, editorinstance.ErrMultipleEditors):
		payload.Code = string(apierr.CodeFailedPrecondition)
	}

	data, _ := json.Marshal(payload)

	return &mcpgo.CallToolResult{
		IsError: true,
		Content: []mcpgo.Content{&mcpgo.TextContent{Text: string(data)}},
	}
}
