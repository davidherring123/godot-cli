package mcp_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/davidherring123/godot-cli/internal/core"
	"github.com/davidherring123/godot-cli/internal/protocol/artifact"
	"github.com/invopop/jsonschema"
	mcpgo "github.com/modelcontextprotocol/go-sdk/mcp"
)

type imageResult struct {
	Label string `json:"label"`
	Image []byte `json:"-"`
}

func (result imageResult) Images() []artifact.Image {
	return []artifact.Image{{
		MIMEType: "image/png",
		Data:     result.Image,
	}}
}

func TestToolResultIncludesImageContent(t *testing.T) {
	image := []byte("\x89PNG\r\n\x1a\nfake")
	reflector := jsonschema.Reflector{Anonymous: true, ExpandedStruct: true}

	action := core.ActionDefinition{
		Name:        "test.capture",
		Description: "Return a fake screenshot.",
		Input:       reflector.ReflectFromType(reflect.TypeFor[struct{}]()),
		Output:      reflector.ReflectFromType(reflect.TypeFor[imageResult]()),
		ReadOnly:    true,
		Invoke: func(string, json.RawMessage) (any, error) {
			return imageResult{Label: "capture", Image: image}, nil
		},
	}

	session := connectMCP(t, "", []core.ActionDefinition{action})

	result, err := session.CallTool(context.Background(), &mcpgo.CallToolParams{
		Name:      "test_capture",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("calling tool: %v", err)
	}

	if result.IsError {
		t.Fatalf("tool returned an error: %s", mcpText(result))
	}

	if len(result.Content) != 2 {
		t.Fatalf("content blocks = %d; want text and image", len(result.Content))
	}

	imageContent, ok := result.Content[1].(*mcpgo.ImageContent)
	if !ok {
		t.Fatalf("second content block = %T; want image content", result.Content[1])
	}

	if imageContent.MIMEType != "image/png" || !bytes.Equal(imageContent.Data, image) {
		t.Fatalf("image content = %+v; want the capture bytes", imageContent)
	}

	structured, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatalf("marshaling structured content: %v", err)
	}

	if strings.Contains(string(structured), "Image") || !strings.Contains(string(structured), "capture") {
		t.Fatalf("structured content = %s; want metadata without raw image bytes", structured)
	}
}

func TestServerExposesRuntimeTools(t *testing.T) {
	root := t.TempDir()

	if err := os.WriteFile(
		filepath.Join(root, "project.godot"),
		[]byte("[application]\n"),
		0644,
	); err != nil {
		t.Fatal(err)
	}

	session := connectMCP(t, root, nil)

	tools, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("listing tools: %v", err)
	}

	found := make(map[string]bool, len(tools.Tools))

	for _, tool := range tools.Tools {
		found[tool.Name] = true
	}

	for _, name := range []string{
		"runtime_start",
		"runtime_status",
		"runtime_capture",
		"runtime_stop",
	} {
		if !found[name] {
			t.Errorf("tool %q is not exposed", name)
		}
	}
}

func TestRuntimeStopAdvertisesIdempotentAnnotations(t *testing.T) {
	root := t.TempDir()

	if err := os.WriteFile(
		filepath.Join(root, "project.godot"),
		[]byte("[application]\n"),
		0644,
	); err != nil {
		t.Fatal(err)
	}

	session := connectMCP(t, root, []core.ActionDefinition{
		core.RuntimeStart,
		core.RuntimeStatus,
		core.RuntimeStop,
	})

	tools, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("listing tools: %v", err)
	}

	byName := make(map[string]*mcpgo.Tool, len(tools.Tools))

	for _, tool := range tools.Tools {
		byName[tool.Name] = tool
	}

	stop := byName["runtime_stop"]
	if stop == nil || stop.Annotations == nil {
		t.Fatal("runtime_stop is missing tool annotations")
	}

	if !stop.Annotations.IdempotentHint {
		t.Fatal("runtime_stop should advertise idempotentHint=true")
	}

	if stop.Annotations.ReadOnlyHint {
		t.Fatal("runtime_stop should not advertise readOnlyHint=true")
	}

	if stop.Annotations.DestructiveHint == nil || !*stop.Annotations.DestructiveHint {
		t.Fatal("runtime_stop should advertise destructiveHint=true")
	}

	status := byName["runtime_status"]
	if status == nil || status.Annotations == nil {
		t.Fatal("runtime_status is missing tool annotations")
	}

	if !status.Annotations.ReadOnlyHint || !status.Annotations.IdempotentHint {
		t.Fatalf("runtime_status annotations = %+v; want read-only and idempotent", status.Annotations)
	}

	start := byName["runtime_start"]
	if start == nil || start.Annotations == nil {
		t.Fatal("runtime_start is missing tool annotations")
	}

	if start.Annotations.IdempotentHint {
		t.Fatal("runtime_start should not advertise idempotentHint=true")
	}
}

func TestRuntimeStartWithoutEditorIsActionable(t *testing.T) {
	root := t.TempDir()

	if err := os.WriteFile(
		filepath.Join(root, "project.godot"),
		[]byte("[application]\nrun/main_scene=\"res://main.tscn\"\n"),
		0644,
	); err != nil {
		t.Fatal(err)
	}

	session := connectMCP(t, root, []core.ActionDefinition{core.RuntimeStart})

	result, err := session.CallTool(context.Background(), &mcpgo.CallToolParams{
		Name:      "runtime_start",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("calling runtime_start: %v", err)
	}

	if !result.IsError {
		t.Fatal("runtime_start without an editor returned success")
	}

	text := mcpText(result)

	if !strings.Contains(text, `"code":"failed_precondition"`) {
		t.Fatalf("error result = %s; want failed_precondition", text)
	}

	if !strings.Contains(text, "running Godot editor") {
		t.Fatalf("error result = %s; want an editor recovery hint", text)
	}
}

func TestRuntimeCaptureWithoutSessionIsActionable(t *testing.T) {
	root := t.TempDir()

	if err := os.WriteFile(
		filepath.Join(root, "project.godot"),
		[]byte("[application]\n"),
		0644,
	); err != nil {
		t.Fatal(err)
	}

	session := connectMCP(t, root, []core.ActionDefinition{core.RuntimeCapture})

	result, err := session.CallTool(context.Background(), &mcpgo.CallToolParams{
		Name:      "runtime_capture",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("calling runtime_capture: %v", err)
	}

	if !result.IsError {
		t.Fatal("runtime_capture without a runtime returned success")
	}

	text := mcpText(result)

	if !strings.Contains(text, `"code":"failed_precondition"`) {
		t.Fatalf("error result = %s; want failed_precondition", text)
	}

	if !strings.Contains(text, "runtime.start") {
		t.Fatalf("error result = %s; want a recovery hint", text)
	}
}
