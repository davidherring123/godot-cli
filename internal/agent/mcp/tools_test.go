package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/davidherring123/godot-cli/internal/core"
	mcpgo "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestToolHandlerCancelsWhileWaitingForSerializer(t *testing.T) {
	serializer := newCallSerializer()

	if err := serializer.acquire(context.Background()); err != nil {
		t.Fatal(err)
	}

	defer serializer.release()

	invoked := make(chan struct{}, 1)
	handler := toolHandler(serializer, "", core.ActionDefinition{
		Name: "test.action",
		Invoke: func(string, json.RawMessage) (any, error) {
			invoked <- struct{}{}
			return struct{}{}, nil
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	started := time.Now()

	_, err := handler(ctx, toolRequest())
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("handler error = %v; want context deadline exceeded", err)
	}

	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("handler waited %s for an uncancellable serializer", elapsed)
	}

	select {
	case <-invoked:
		t.Fatal("handler invoked the action while the serializer was held")
	default:
	}
}

func TestToolHandlerCancelsDuringInvocation(t *testing.T) {
	serializer := newCallSerializer()
	entered := make(chan struct{})
	finish := make(chan struct{})

	handler := toolHandler(serializer, "", core.ActionDefinition{
		Name: "test.action",
		Invoke: func(string, json.RawMessage) (any, error) {
			close(entered)
			<-finish
			return struct{}{}, nil
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)

	go func() {
		_, err := handler(ctx, toolRequest())
		done <- err
	}()

	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("handler never invoked the action")
	}

	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("handler error = %v; want context canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("handler did not return after cancellation")
	}

	close(finish)

	// The serializer must remain held until the abandoned invocation really
	// finishes, then become available to the next request.
	waitCtx, waitCancel := context.WithTimeout(context.Background(), time.Second)
	defer waitCancel()

	if err := serializer.acquire(waitCtx); err != nil {
		t.Fatalf("serializer was not released after the invocation finished: %v", err)
	}

	serializer.release()
}

func toolRequest() *mcpgo.CallToolRequest {
	return &mcpgo.CallToolRequest{
		Params: &mcpgo.CallToolParamsRaw{
			Name:      "test_action",
			Arguments: json.RawMessage(`{}`),
		},
	}
}
