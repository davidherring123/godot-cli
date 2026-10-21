package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"

	"github.com/davidherring123/godot-cli/internal/core"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var toolNamePattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)

func addAction(server *mcp.Server, projectRoot string, action core.ActionDefinition) error {
	if action.Name == "" {
		return errors.New("action has an empty name")
	}

	if action.Invoke == nil {
		return fmt.Errorf("action %q has no invocation function", action.Name)
	}

	if action.Input == nil || action.Output == nil {
		return fmt.Errorf("action %q is missing an input or output schema", action.Name)
	}

	toolName := core.ToolName(action.Name)

	if !toolNamePattern.MatchString(toolName) {
		return fmt.Errorf("action %q maps to invalid MCP tool name %q", action.Name, toolName)
	}

	inputSchema, err := objectSchema(action.Name, action.Input)
	if err != nil {
		return err
	}

	outputSchema, err := json.Marshal(action.Output)
	if err != nil {
		return fmt.Errorf("encoding output schema for %q: %w", action.Name, err)
	}

	destructive := action.Destructive
	openWorld := false

	server.AddTool(
		&mcp.Tool{
			Name:         toolName,
			Description:  action.Description,
			InputSchema:  inputSchema,
			OutputSchema: json.RawMessage(outputSchema),
			Annotations: &mcp.ToolAnnotations{
				ReadOnlyHint:    action.ReadOnly,
				DestructiveHint: &destructive,
				IdempotentHint:  action.ReadOnly,
				OpenWorldHint:   &openWorld,
			},
		},
		toolHandler(projectRoot, action),
	)

	return nil
}

func objectSchema(actionName string, schema any) (json.RawMessage, error) {
	data, err := json.Marshal(schema)
	if err != nil {
		return nil, fmt.Errorf("encoding input schema for %q: %w", actionName, err)
	}

	var document struct {
		Type string `json:"type"`
	}

	if err := json.Unmarshal(data, &document); err != nil {
		return nil, fmt.Errorf("reading input schema for %q: %w", actionName, err)
	}

	if document.Type != "object" {
		return nil, fmt.Errorf(
			"input schema for %q must be an object, got %q",
			actionName,
			document.Type,
		)
	}

	return json.RawMessage(data), nil
}

func toolHandler(projectRoot string, action core.ActionDefinition) mcp.ToolHandler {
	return func(ctx context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		if request == nil || request.Params == nil {
			return errorResult(errors.New("missing tool request parameters")), nil
		}

		result, err := action.Invoke(projectRoot, request.Params.Arguments)
		if err != nil {
			return errorResult(err), nil
		}

		text, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return errorResult(fmt.Errorf("encoding %s result: %w", action.Name, err)), nil
		}

		return &mcp.CallToolResult{
			Content:           []mcp.Content{&mcp.TextContent{Text: string(text)}},
			StructuredContent: result,
		}, nil
	}
}
