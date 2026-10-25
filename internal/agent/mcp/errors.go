package mcp

import (
	"encoding/json"
	"errors"

	"github.com/davidherring123/godot-cli/internal/core"
	"github.com/davidherring123/godot-cli/internal/core/bridge"
	"github.com/davidherring123/godot-cli/internal/core/godot"
	"github.com/davidherring123/godot-cli/internal/core/runtime"
	mcpgo "github.com/modelcontextprotocol/go-sdk/mcp"
)

type errorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func errorResult(err error) *mcpgo.CallToolResult {
	payload := errorPayload{
		Code:    string(bridge.ErrorCodeInternalError),
		Message: err.Error(),
	}

	var argumentError *core.ArgumentError
	var apiError *bridge.APIError
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
		payload.Code = string(bridge.ErrorCodeInvalidArgument)
		payload.Message = argumentError.Error()
	case errors.Is(err, bridge.ErrNoEditor),
		errors.Is(err, bridge.ErrMultipleEditors),
		errors.Is(err, godot.ErrNotFound):
		payload.Code = string(bridge.ErrorCodeFailedPrecondition)
	}

	data, _ := json.Marshal(payload)

	return &mcpgo.CallToolResult{
		IsError: true,
		Content: []mcpgo.Content{&mcpgo.TextContent{Text: string(data)}},
	}
}
