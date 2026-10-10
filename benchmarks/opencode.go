package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"
)

type openCodeServer struct {
	baseURL  string
	password string
	command  *exec.Cmd
	done     chan error
	stdout   bytes.Buffer
	stderr   bytes.Buffer
}

type sessionInfo struct {
	ID          string           `json:"id"`
	Outcome     string           `json:"outcome"`
	Cost        *float64         `json:"cost"`
	Tokens      *tokenUsage      `json:"tokens"`
	Permissions []permissionRule `json:"permissions"`
}

type permissionRule struct {
	Action   string `json:"action"`
	Resource string `json:"resource"`
	Effect   string `json:"effect"`
}

const mcpWildcardAction = "*_*"

type sessionMessage struct {
	Type    string            `json:"type"`
	Content []json.RawMessage `json:"content"`
}

type toolPart struct {
	Type  string `json:"type"`
	Name  string `json:"name"`
	State struct {
		Input   json.RawMessage `json:"input"`
		Content json.RawMessage `json:"content"`
	} `json:"state"`
}

func startOpenCodeServer(
	ctx context.Context,
	opencodePath string,
	environment []string,
) (*openCodeServer, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()
	passwordBytes := make([]byte, 32)
	if _, err := rand.Read(passwordBytes); err != nil {
		return nil, fmt.Errorf("generating OpenCode server password: %w", err)
	}
	server := &openCodeServer{
		baseURL:  fmt.Sprintf("http://127.0.0.1:%d", port),
		password: base64.RawURLEncoding.EncodeToString(passwordBytes),
	}
	server.command = exec.Command(
		opencodePath,
		"serve",
		"--hostname",
		"127.0.0.1",
		"--port",
		fmt.Sprint(port),
	)
	server.command.Env = withEnvironment(environment, map[string]string{
		"OPENCODE_PASSWORD": server.password,
	})
	server.command.Stdout = &server.stdout
	server.command.Stderr = &server.stderr

	if err := server.command.Start(); err != nil {
		return nil, fmt.Errorf("starting OpenCode server: %w", err)
	}
	server.done = make(chan error, 1)

	go func() {
		server.done <- server.command.Wait()
	}()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		request, _ := http.NewRequestWithContext(
			ctx,
			http.MethodGet,
			server.baseURL+"/api/info",
			nil,
		)
		request.SetBasicAuth("opencode", server.password)

		response, requestErr := http.DefaultClient.Do(request)
		if requestErr == nil {
			_ = response.Body.Close()

			if response.StatusCode == http.StatusOK {
				return server, nil
			}
		}
		select {
		case processErr := <-server.done:
			server.done = nil

			return nil, fmt.Errorf(
				"OpenCode server exited during startup: %v\n%s",
				processErr,
				server.logs(),
			)
		case <-ctx.Done():
			_ = server.Close()

			return nil, fmt.Errorf(
				"waiting for OpenCode server: %w\n%s",
				ctx.Err(),
				server.logs(),
			)
		case <-ticker.C:
		}
	}
}

func (server *openCodeServer) createSession(
	ctx context.Context,
	projectRoot string,
	agent string,
	approach string,
	title string,
) (string, error) {
	permissions := benchmarkPermissions(approach)

	payload := map[string]any{
		"title":       title,
		"agent":       agent,
		"permissions": permissions,
		"location": map[string]string{
			"directory": projectRoot,
		},
	}
	var response struct {
		Data sessionInfo `json:"data"`
	}
	if _, err := server.request(
		ctx,
		http.MethodPost,
		"/api/session",
		payload,
		&response,
	); err != nil {
		return "", err
	}
	if response.Data.ID == "" {
		return "", fmt.Errorf("OpenCode returned an empty session ID")
	}
	return response.Data.ID, nil
}

func benchmarkPermissions(approach string) []permissionRule {
	permissions := []permissionRule{
		{
			Action:   mcpWildcardAction,
			Resource: "*",
			Effect:   "deny",
		},
		{
			Action:   "skill",
			Resource: "*",
			Effect:   "deny",
		},
	}
	if approach == approachCLI {
		permissions = append(permissions, permissionRule{
			Action:   "skill",
			Resource: "godot-cli",
			Effect:   "allow",
		})
	}
	return permissions
}

func (server *openCodeServer) deleteSession(
	ctx context.Context,
	sessionID string,
) error {
	_, err := server.request(
		ctx,
		http.MethodDelete,
		"/api/session/"+url.PathEscape(sessionID),
		nil,
		nil,
	)

	return err
}

func (server *openCodeServer) getSession(
	ctx context.Context,
	sessionID string,
) (sessionInfo, error) {
	var response struct {
		Data sessionInfo `json:"data"`
	}
	_, err := server.request(
		ctx,
		http.MethodGet,
		"/api/session/"+url.PathEscape(sessionID),
		nil,
		&response,
	)

	return response.Data, err
}

func locationAPIPath(path, directory string) string {
	query := url.Values{}
	query.Set("location[directory]", directory)

	return path + "?" + query.Encode()
}

func (server *openCodeServer) run(
	ctx context.Context,
	opencodePath string,
	projectRoot string,
	sessionID string,
	model string,
	agent string,
	prompt string,
	environment []string,
) error {
	command := exec.CommandContext(
		ctx,
		opencodePath,
		"run",
		"--server",
		server.baseURL,
		"--session",
		sessionID,
		"--model",
		model,
		"--agent",
		agent,
		"--format",
		"json",
		"--auto",
		prompt,
	)
	command.Dir = projectRoot
	command.Env = withEnvironment(environment, map[string]string{
		"OPENCODE_PASSWORD": server.password,
	})
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr

	if err := command.Run(); err != nil {
		return fmt.Errorf("OpenCode run failed: %w", err)
	}
	return nil
}

func (server *openCodeServer) collectSession(
	ctx context.Context,
	sessionID string,
) (sessionInfo, []sessionMessage, contextMetrics, error) {
	var sessionResponse struct {
		Data sessionInfo `json:"data"`
	}
	if _, err := server.request(
		ctx,
		http.MethodGet,
		"/api/session/"+url.PathEscape(sessionID),
		nil,
		&sessionResponse,
	); err != nil {
		return sessionInfo{}, nil, contextMetrics{}, err
	}
	messages, err := server.collectMessages(ctx, sessionID)
	if err != nil {
		return sessionInfo{}, nil, contextMetrics{}, err
	}
	var contextResponse struct {
		Data []json.RawMessage `json:"data"`
	}
	_, err = server.request(
		ctx,
		http.MethodGet,
		"/api/session/"+url.PathEscape(sessionID)+"/context",
		nil,
		&contextResponse,
	)
	if err != nil {
		return sessionInfo{}, nil, contextMetrics{}, err
	}
	activeContext, _ := json.Marshal(contextResponse.Data)
	metrics := contextMetrics{
		Messages: len(contextResponse.Data),
		Bytes:    len(activeContext),
	}
	for _, message := range contextResponse.Data {
		metrics.ToolOutputBytes += toolOutputBytes(message)
	}
	return sessionResponse.Data, messages, metrics, nil
}

func (server *openCodeServer) collectMessages(
	ctx context.Context,
	sessionID string,
) ([]sessionMessage, error) {
	var messages []sessionMessage
	var cursor string

	for {
		query := url.Values{}
		query.Set("limit", "200")

		if cursor == "" {
			query.Set("order", "asc")
		} else {
			query.Set("cursor", cursor)
		}
		path := "/api/session/" + url.PathEscape(sessionID) +
			"/message?" + query.Encode()
		var response struct {
			Data   []sessionMessage `json:"data"`
			Cursor struct {
				Next *string `json:"next"`
			} `json:"cursor"`
		}
		if _, err := server.request(
			ctx,
			http.MethodGet,
			path,
			nil,
			&response,
		); err != nil {
			return nil, err
		}
		messages = append(messages, response.Data...)
		if response.Cursor.Next == nil || *response.Cursor.Next == "" {
			return messages, nil
		}
		cursor = *response.Cursor.Next
	}
}

func (server *openCodeServer) request(
	ctx context.Context,
	method string,
	path string,
	payload any,
	result any,
) ([]byte, error) {
	var body io.Reader

	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(data)
	}
	request, err := http.NewRequestWithContext(
		ctx,
		method,
		server.baseURL+path,
		body,
	)
	if err != nil {
		return nil, err
	}
	request.SetBasicAuth("opencode", server.password)

	if payload != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	data, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return data, fmt.Errorf(
			"OpenCode API %s %s returned %s: %s",
			method,
			path,
			response.Status,
			strings.TrimSpace(string(data)),
		)
	}
	if result != nil {
		if err := json.Unmarshal(data, result); err != nil {
			return data, err
		}
	}
	return data, nil
}

func (server *openCodeServer) Close() error {
	if server == nil || server.command == nil || server.done == nil {
		return nil
	}
	_ = server.command.Process.Signal(os.Interrupt)

	select {
	case <-server.done:
		server.done = nil

		return nil
	case <-time.After(5 * time.Second):
		if err := server.command.Process.Kill(); err != nil {
			return err
		}
		<-server.done
		server.done = nil

		return nil
	}
}

func (server *openCodeServer) logs() string {
	return fmt.Sprintf(
		"stdout:\n%s\nstderr:\n%s",
		server.stdout.String(),
		server.stderr.String(),
	)
}

func summarizeMessages(
	messages []sessionMessage,
) []toolCall {
	var tools []toolCall

	for _, message := range messages {
		if message.Type != "assistant" {
			continue
		}
		for _, content := range message.Content {
			var kind struct {
				Type string `json:"type"`
			}
			if json.Unmarshal(content, &kind) != nil {
				continue
			}
			if kind.Type != "tool" {
				continue
			}
			var part toolPart
			if json.Unmarshal(content, &part) == nil {
				tools = append(tools, toolCall{
					Name:  part.Name,
					Input: part.State.Input,
				})
			}
		}
	}
	return tools
}

func summarizeTokens(tokens *tokenUsage) *resultTokens {
	if tokens == nil {
		return nil
	}
	summary := &resultTokens{
		Input:      tokens.Input,
		Output:     tokens.Output,
		Reasoning:  tokens.Reasoning,
		CacheRead:  tokens.Cache.Read,
		CacheWrite: tokens.Cache.Write,
	}
	if tokens.Input != nil && tokens.Cache.Read != nil {
		prompt := *tokens.Input + *tokens.Cache.Read
		summary.PromptTokens = &prompt

		if prompt > 0 {
			rate := *tokens.Cache.Read / prompt
			summary.CacheHitRate = &rate
		}
	}
	return summary
}

func toolOutputBytes(message json.RawMessage) int {
	var parsed sessionMessage

	if json.Unmarshal(message, &parsed) != nil {
		return 0
	}
	bytes := 0

	for _, content := range parsed.Content {
		var part toolPart

		if json.Unmarshal(content, &part) == nil && part.Type == "tool" {
			bytes += len(part.State.Content)
		}
	}
	return bytes
}
