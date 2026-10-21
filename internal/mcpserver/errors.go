package mcpserver

import (
	"encoding/json"
	"errors"

	"github.com/davidherring123/godot-cli/internal/bridge"
	"github.com/davidherring123/godot-cli/internal/core"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type errorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func errorResult(err error) *mcp.CallToolResult {
	payload := errorPayload{
		Code:    string(bridge.ErrorCodeInternalError),
		Message: err.Error(),
	}

	var argumentError *core.ArgumentError
	if errors.As(err, &argumentError) {
		payload.Code = string(bridge.ErrorCodeInvalidArgument)
		payload.Message = argumentError.Error()
	}

	var apiError *bridge.APIError
	if errors.As(err, &apiError) {
		payload.Code = string(apiError.Code)
		payload.Message = apiError.Message
	}

	data, marshalErr := json.Marshal(payload)
	if marshalErr != nil {
		data = []byte(err.Error())
	}

	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{&mcp.TextContent{Text: string(data)}},
	}
}
