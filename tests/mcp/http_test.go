package mcp_test

import (
	"context"
	"encoding/json"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/davidherring123/godot-cli/internal/agent/mcp"
	"github.com/davidherring123/godot-cli/internal/core"
	mcpgo "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestMCPHTTPServesStateAndTools(t *testing.T) {
	root := t.TempDir()

	if err := os.WriteFile(
		filepath.Join(root, "project.godot"),
		[]byte("[application]\n"),
		0644,
	); err != nil {
		t.Fatal(err)
	}

	statePath := filepath.Join(t.TempDir(), "state", "mcp.json")
	ctx, cancel := context.WithCancel(context.Background())
	done := startMCPHTTPTestServer(ctx, t, mcp.Options{
		Version:     "test",
		ProjectRoot: root,
		Actions:     []core.ActionDefinition{core.EditorStatus},
		Transport:   mcp.TransportHTTP,
		StateFile:   statePath,
	})

	state := waitForMCPHTTPTestState(t, statePath)
	assertMCPHTTPTestState(t, state, root)

	session := connectMCPHTTPTestSession(ctx, t, state.URL)

	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		_ = session.Close()
		stopMCPHTTPTestServer(t, cancel, done)
		t.Fatalf("listing MCP tools: %v", err)
	}

	found := false

	for _, tool := range tools.Tools {
		if tool.Name == "editor_status" {
			found = true
		}
	}

	if !found {
		t.Errorf("editor_status tool is not exposed over HTTP")
	}

	result, err := session.CallTool(ctx, &mcpgo.CallToolParams{
		Name:      "editor_status",
		Arguments: map[string]any{},
	})
	if err != nil {
		_ = session.Close()
		stopMCPHTTPTestServer(t, cancel, done)
		t.Fatalf("calling editor_status over HTTP: %v", err)
	}

	if result.IsError {
		t.Errorf("editor_status returned an error: %s", mcpText(result))
	}

	// Cancel while the client still has a standalone SSE stream open. The
	// stateless HTTP handler must stop without waiting for the client.
	stopMCPHTTPTestServer(t, cancel, done)
	_ = session.Close()

	if _, err := os.Stat(statePath); !os.IsNotExist(err) {
		t.Fatalf("state file still exists after shutdown: %v", err)
	}
}

func TestMCPHTTPRejectsNonLoopbackHost(t *testing.T) {
	for _, host := range []string{"0.0.0.0", "::", "192.168.1.20", "example.com"} {
		t.Run(host, func(t *testing.T) {
			err := mcp.RunHTTP(context.Background(), mcp.Options{
				ProjectRoot: t.TempDir(),
				Transport:   mcp.TransportHTTP,
				Host:        host,
			})
			if err == nil {
				t.Fatalf("host %q was accepted", host)
			}

			if !strings.Contains(err.Error(), "loopback") {
				t.Fatalf("host %q error = %v, want a loopback error", host, err)
			}
		})
	}
}

func TestMCPHTTPBindsBeforeWritingState(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = listener.Close() }()

	port := listener.Addr().(*net.TCPAddr).Port
	statePath := filepath.Join(t.TempDir(), "mcp.json")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err = mcp.RunHTTP(ctx, mcp.Options{
		ProjectRoot: t.TempDir(),
		Transport:   mcp.TransportHTTP,
		Port:        port,
		StateFile:   statePath,
	})
	if err == nil {
		t.Fatal("starting on an occupied port succeeded")
	}

	if _, statErr := os.Stat(statePath); !os.IsNotExist(statErr) {
		t.Fatalf("state file was written before a successful bind: %v", statErr)
	}
}

func TestMCPHTTPLeavesStateOwnedByAnotherProcess(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "mcp.json")
	ctx, cancel := context.WithCancel(context.Background())
	done := startMCPHTTPTestServer(ctx, t, mcp.Options{
		ProjectRoot: t.TempDir(),
		Transport:   mcp.TransportHTTP,
		StateFile:   statePath,
	})

	state := waitForMCPHTTPTestState(t, statePath)
	foreign := state
	foreign.PID++
	foreign.StartedAt = foreign.StartedAt.Add(time.Second)

	data, err := json.Marshal(foreign)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(statePath, data, 0o600); err != nil {
		t.Fatal(err)
	}

	stopMCPHTTPTestServer(t, cancel, done)

	current := waitForMCPHTTPTestState(t, statePath)
	if current.PID != foreign.PID {
		t.Fatalf("state file was replaced or removed: pid = %d, want %d", current.PID, foreign.PID)
	}
}

func TestMCPHTTPReplacesExistingState(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "mcp.json")
	if err := os.WriteFile(statePath, []byte(`{"pid":1}`), 0o600); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := startMCPHTTPTestServer(ctx, t, mcp.Options{
		ProjectRoot: t.TempDir(),
		Transport:   mcp.TransportHTTP,
		StateFile:   statePath,
	})

	state := waitForMCPHTTPTestState(t, statePath)
	if state.PID != os.Getpid() {
		t.Fatalf("state pid = %d, want %d", state.PID, os.Getpid())
	}

	stopMCPHTTPTestServer(t, cancel, done)
}

func TestMCPHTTPSerializesStateFileOwnership(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "mcp.json")
	ctx, cancel := context.WithCancel(context.Background())
	done := startMCPHTTPTestServer(ctx, t, mcp.Options{
		ProjectRoot: t.TempDir(),
		Transport:   mcp.TransportHTTP,
		StateFile:   statePath,
	})
	waitForMCPHTTPTestState(t, statePath)

	secondCtx, secondCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer secondCancel()
	err := mcp.RunHTTP(secondCtx, mcp.Options{
		ProjectRoot: t.TempDir(),
		Transport:   mcp.TransportHTTP,
		StateFile:   statePath,
	})
	if err == nil || !strings.Contains(err.Error(), "already managed") {
		stopMCPHTTPTestServer(t, cancel, done)
		t.Fatalf("second server error = %v, want ownership conflict", err)
	}

	stopMCPHTTPTestServer(t, cancel, done)
}

func startMCPHTTPTestServer(ctx context.Context, t *testing.T, options mcp.Options) <-chan error {
	t.Helper()

	done := make(chan error, 1)

	go func() {
		done <- mcp.RunHTTP(ctx, options)
	}()

	return done
}

func stopMCPHTTPTestServer(t *testing.T, cancel context.CancelFunc, done <-chan error) {
	t.Helper()

	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("serving MCP over HTTP: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for the MCP HTTP server to stop")
	}
}

func connectMCPHTTPTestSession(ctx context.Context, t *testing.T, endpoint string) *mcpgo.ClientSession {
	t.Helper()

	client := mcpgo.NewClient(
		&mcpgo.Implementation{Name: "godot-mcp-test", Version: "test"},
		nil,
	)

	session, err := client.Connect(ctx, &mcpgo.StreamableClientTransport{
		Endpoint: endpoint,
	}, nil)
	if err != nil {
		t.Fatalf("connecting MCP HTTP client: %v", err)
	}

	return session
}

func assertMCPHTTPTestState(t *testing.T, state mcp.ServerState, projectRoot string) {
	t.Helper()

	if state.PID != os.Getpid() {
		t.Errorf("state pid = %d, want %d", state.PID, os.Getpid())
	}

	if state.Transport != mcp.TransportHTTP {
		t.Errorf("state transport = %q, want %q", state.Transport, mcp.TransportHTTP)
	}

	if state.Version != "test" {
		t.Errorf("state version = %q, want test", state.Version)
	}

	if state.Project != projectRoot {
		t.Errorf("state project = %q, want %q", state.Project, projectRoot)
	}

	if state.StartedAt.IsZero() {
		t.Error("state started_at is zero")
	}

	endpoint, err := url.Parse(state.URL)
	if err != nil {
		t.Fatalf("parsing state URL %q: %v", state.URL, err)
	}

	if endpoint.Scheme != "http" || endpoint.Path != mcp.HTTPPath {
		t.Errorf("state URL = %q, want an http URL ending in %s", state.URL, mcp.HTTPPath)
	}

	host, _, err := net.SplitHostPort(endpoint.Host)
	if err != nil {
		t.Fatalf("splitting state URL host %q: %v", endpoint.Host, err)
	}

	if address := net.ParseIP(host); address == nil || !address.IsLoopback() {
		t.Errorf("state URL host %q is not loopback", host)
	}
}

func waitForMCPHTTPTestState(t *testing.T, path string) mcp.ServerState {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)

	for {
		data, err := os.ReadFile(path)
		if err == nil {
			var state mcp.ServerState
			if err := json.Unmarshal(data, &state); err == nil && state.URL != "" {
				return state
			}
		}

		if time.Now().After(deadline) {
			t.Fatalf("waiting for MCP state file %s: %v", path, err)
		}

		time.Sleep(10 * time.Millisecond)
	}
}
