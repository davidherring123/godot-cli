package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	mcpgo "github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	// HTTPPath is the Streamable HTTP endpoint exposed by RunHTTP.
	HTTPPath = "/mcp"

	defaultHTTPHost  = "127.0.0.1"
	httpShutdownWait = 5 * time.Second
)

// ServerState is the discovery document written by RunHTTP when StateFile is
// configured. Its JSON field names are a stable contract for editor addons and
// other local clients.
type ServerState struct {
	PID       int       `json:"pid"`
	Transport string    `json:"transport"`
	URL       string    `json:"url"`
	Version   string    `json:"version"`
	Project   string    `json:"project"`
	StartedAt time.Time `json:"started_at"`
}

// RunHTTP serves the MCP Streamable HTTP transport on a loopback address until
// ctx is cancelled. It binds the listener before writing the optional state
// file so a published URL is always connectable.
func RunHTTP(ctx context.Context, options Options) error {
	if options.ProjectRoot == "" {
		return errors.New("project root is required")
	}

	host, err := loopbackHost(options.Host)
	if err != nil {
		return err
	}

	if options.Port < 0 || options.Port > 65535 {
		return fmt.Errorf("MCP HTTP port %d is outside the range 0-65535", options.Port)
	}

	server, err := NewServer(options)
	if err != nil {
		return err
	}

	listener, err := net.Listen("tcp", net.JoinHostPort(host, strconv.Itoa(options.Port)))
	if err != nil {
		return fmt.Errorf("binding MCP HTTP listener: %w", err)
	}

	address := listener.Addr()
	version := options.Version
	if version == "" {
		version = "dev"
	}

	state := ServerState{
		PID:       os.Getpid(),
		Transport: TransportHTTP,
		URL:       httpURL(address),
		Version:   version,
		Project:   options.ProjectRoot,
		StartedAt: time.Now().UTC(),
	}

	if options.StateFile != "" {
		if err := writeServerState(options.StateFile, state); err != nil {
			_ = listener.Close()

			return fmt.Errorf("writing MCP state file: %w", err)
		}

		defer removeServerStateIfOwned(options.StateFile, state)
	}

	handler := mcpgo.NewStreamableHTTPHandler(
		func(*http.Request) *mcpgo.Server { return server },
		&mcpgo.StreamableHTTPOptions{
			// Tool state is project-scoped and the server keeps no per-session
			// state, so sessionless Streamable HTTP avoids idle session
			// retention and keeps shutdown deterministic.
			Stateless: true,
		},
	)

	mux := http.NewServeMux()
	mux.Handle(HTTPPath, handler)

	httpServer := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	fmt.Fprintf(os.Stderr, "godot-cli mcp: serving Streamable HTTP at %s\n", state.URL)

	serveErr := make(chan error, 1)

	go func() {
		serveErr <- httpServer.Serve(listener)
	}()

	select {
	case err := <-serveErr:
		if err == nil || errors.Is(err, http.ErrServerClosed) {
			return nil
		}

		return fmt.Errorf("serving MCP over HTTP: %w", err)
	case <-ctx.Done():
		shutdownContext, cancel := context.WithTimeout(context.Background(), httpShutdownWait)
		defer cancel()

		shutdownErr := httpServer.Shutdown(shutdownContext)
		if shutdownErr != nil {
			_ = httpServer.Close()
		}

		serveErr := <-serveErr
		if shutdownErr != nil {
			return fmt.Errorf("shutting down MCP HTTP server: %w", shutdownErr)
		}

		if serveErr != nil &&
			!errors.Is(serveErr, http.ErrServerClosed) &&
			!errors.Is(serveErr, net.ErrClosed) {
			return fmt.Errorf("serving MCP over HTTP: %w", serveErr)
		}

		return nil
	}
}

func loopbackHost(host string) (string, error) {
	host = strings.TrimSpace(host)
	if host == "" {
		return defaultHTTPHost, nil
	}

	trimmed := strings.Trim(host, "[]")

	if strings.EqualFold(trimmed, "localhost") {
		addresses, err := net.LookupIP("localhost")
		if err != nil {
			return "", fmt.Errorf("resolving localhost: %w", err)
		}

		var fallback string

		for _, address := range addresses {
			if !address.IsLoopback() {
				continue
			}

			if address.To4() != nil {
				return address.String(), nil
			}

			if fallback == "" {
				fallback = address.String()
			}
		}

		if fallback != "" {
			return fallback, nil
		}

		return "", errors.New("localhost did not resolve to a loopback address")
	}

	address := net.ParseIP(trimmed)
	if address == nil || !address.IsLoopback() {
		return "", fmt.Errorf(
			"MCP HTTP host %q is not a loopback address; only loopback hosts are supported",
			host,
		)
	}

	return address.String(), nil
}

func httpURL(address net.Addr) string {
	if tcp, ok := address.(*net.TCPAddr); ok && tcp.IP != nil {
		return "http://" + net.JoinHostPort(tcp.IP.String(), strconv.Itoa(tcp.Port)) + HTTPPath
	}

	return "http://" + address.String() + HTTPPath
}

func writeServerState(path string, state ServerState) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return err
	}

	temporary, err := os.CreateTemp(directory, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}

	temporaryPath := temporary.Name()

	defer func() {
		_ = os.Remove(temporaryPath)
	}()

	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()

		return err
	}

	if _, err := temporary.Write(append(data, '\n')); err != nil {
		_ = temporary.Close()

		return err
	}

	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()

		return err
	}

	if err := temporary.Close(); err != nil {
		return err
	}

	return os.Rename(temporaryPath, path)
}

func removeServerStateIfOwned(path string, expected ServerState) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}

	var current ServerState
	if err := json.Unmarshal(data, &current); err != nil {
		return
	}

	if current.PID != expected.PID ||
		current.Transport != expected.Transport ||
		current.URL != expected.URL ||
		!current.StartedAt.Equal(expected.StartedAt) {
		return
	}

	_ = os.Remove(path)
}
