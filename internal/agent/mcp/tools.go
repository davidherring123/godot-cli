package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/davidherring123/godot-mcp/internal/core"
	"github.com/davidherring123/godot-mcp/internal/protocol/artifact"
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
	tracer *callTracer,
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
		toolHandler(serializer, tracer, projectRoot, action),
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
	tracer *callTracer,
	projectRoot string,
	action core.ActionDefinition,
) mcpgo.ToolHandler {
	return func(ctx context.Context, request *mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
		started := time.Now()

		if err := ctx.Err(); err != nil {
			return nil, err
		}

		if request == nil || request.Params == nil {
			return errorResult(errors.New("missing tool request parameters")), nil
		}

		toolName := request.Params.Name
		if toolName == "" {
			toolName = ToolName(action.Name)
		}

		arguments := request.Params.Arguments

		recordTrace := func(isError bool) {
			tracer.record(toolName, arguments, isError, started)
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

		go func() {
			result, err := action.Invoke(projectRoot, arguments)
			finished <- invocation{result: result, err: err}
		}()

		var outcome invocation

		select {
		case outcome = <-finished:
			result := callResult(action, outcome.result, outcome.err)

			// Record before releasing the serializer so trace line order follows
			// editor execution order. The trace write never changes the result
			// or retries an already-applied mutation.
			recordTrace(result.IsError)
			serializer.release()

			return result, nil
		case <-ctx.Done():
			// The client stopped waiting, but the action continues to keep the
			// editor contract intact. Trace its real outcome when it finishes.
			go func() {
				outcome := <-finished
				recordTrace(callResult(action, outcome.result, outcome.err).IsError)
				serializer.release()
			}()

			return nil, ctx.Err()
		}
	}
}

func callResult(action core.ActionDefinition, result any, callErr error) *mcpgo.CallToolResult {
	if callErr != nil {
		return errorResult(callErr)
	}

	text, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return errorResult(fmt.Errorf("encoding %s result: %w", action.Name, err))
	}

	content := []mcpgo.Content{&mcpgo.TextContent{Text: string(text)}}
	if source, ok := result.(artifact.ImageSource); ok {
		for _, image := range source.Images() {
			if len(image.Data) > 0 && image.MIMEType != "" {
				content = append(content, &mcpgo.ImageContent{MIMEType: image.MIMEType, Data: image.Data})
			}
		}
	}

	return &mcpgo.CallToolResult{
		Content:           content,
		StructuredContent: result,
	}
}
