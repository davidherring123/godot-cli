package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/davidherring123/godot-cli/internal/core"
	"github.com/davidherring123/godot-cli/internal/core/artifact"
	"github.com/invopop/jsonschema"
	mcpgo "github.com/modelcontextprotocol/go-sdk/mcp"
)

var toolNamePattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)

// callSerializer guarantees that the editor handles one action at a time
// without making callers wait on an uncancellable mutex.
type callSerializer struct {
	semaphore chan struct{}
}

func newCallSerializer() *callSerializer {
	return &callSerializer{semaphore: make(chan struct{}, 1)}
}

func (serializer *callSerializer) acquire(ctx context.Context) error {
	select {
	case serializer.semaphore <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (serializer *callSerializer) release() {
	<-serializer.semaphore
}

// ToolName converts an action name into the underscore-separated form used by
// MCP tools.
func ToolName(action string) string {
	return strings.ReplaceAll(action, ".", "_")
}

func addAction(
	server *mcpgo.Server,
	serializer *callSerializer,
	projectRoot string,
	action core.ActionDefinition,
) error {
	if action.Name == "" {
		return errors.New("action has an empty name")
	}

	if action.Invoke == nil {
		return fmt.Errorf("action %q has no invocation function", action.Name)
	}

	if action.Input == nil || action.Output == nil {
		return fmt.Errorf("action %q is missing an input or output schema", action.Name)
	}

	toolName := ToolName(action.Name)

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
		&mcpgo.Tool{
			Name:         toolName,
			Description:  action.Description,
			InputSchema:  inputSchema,
			OutputSchema: json.RawMessage(outputSchema),
			Annotations: &mcpgo.ToolAnnotations{
				ReadOnlyHint:    action.ReadOnly,
				DestructiveHint: &destructive,
				IdempotentHint:  action.Idempotent,
				OpenWorldHint:   &openWorld,
			},
		},
		toolHandler(serializer, projectRoot, action),
	)

	return nil
}

func objectSchema(actionName string, schema *jsonschema.Schema) (json.RawMessage, error) {
	if schema.Type != "object" {
		return nil, fmt.Errorf(
			"input schema for %q must be an object, got %q",
			actionName,
			schema.Type,
		)
	}

	data, err := json.Marshal(schema)
	if err != nil {
		return nil, fmt.Errorf("encoding input schema for %q: %w", actionName, err)
	}

	return json.RawMessage(data), nil
}

func toolHandler(
	serializer *callSerializer,
	projectRoot string,
	action core.ActionDefinition,
) mcpgo.ToolHandler {
	return func(ctx context.Context, request *mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		if request == nil || request.Params == nil {
			return errorResult(errors.New("missing tool request parameters")), nil
		}

		// The MCP SDK dispatches requests concurrently, but the addon and the
		// editor undo history assume one operation at a time. Waiting for the
		// serializer must honor client cancellation.
		if err := serializer.acquire(ctx); err != nil {
			return nil, err
		}

		// Run the action separately so a cancelled request returns promptly.
		// The serializer stays held until the action really finishes, keeping
		// the editor's one-operation-at-a-time contract.
		type invocation struct {
			result any
			err    error
		}

		finished := make(chan invocation, 1)
		arguments := request.Params.Arguments

		go func() {
			result, err := action.Invoke(projectRoot, arguments)
			finished <- invocation{result: result, err: err}
		}()

		var outcome invocation

		select {
		case outcome = <-finished:
			serializer.release()
		case <-ctx.Done():
			go func() {
				<-finished
				serializer.release()
			}()

			return nil, ctx.Err()
		}

		if outcome.err != nil {
			return errorResult(outcome.err), nil
		}

		text, err := json.MarshalIndent(outcome.result, "", "  ")
		if err != nil {
			return errorResult(fmt.Errorf("encoding %s result: %w", action.Name, err)), nil
		}

		content := []mcpgo.Content{&mcpgo.TextContent{Text: string(text)}}

		if source, ok := outcome.result.(artifact.ImageSource); ok {
			for _, image := range source.Images() {
				if len(image.Data) == 0 || image.MIMEType == "" {
					continue
				}

				content = append(content, &mcpgo.ImageContent{
					MIMEType: image.MIMEType,
					Data:     image.Data,
				})
			}
		}

		return &mcpgo.CallToolResult{
			Content:           content,
			StructuredContent: outcome.result,
		}, nil
	}
}
