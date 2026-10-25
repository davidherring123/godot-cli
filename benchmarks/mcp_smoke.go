package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"time"
)

// smokeGodotMCP validates the complete benchmark-facing MCP path, including
// session lifecycle, without invoking a model.
func smokeGodotMCP(repositoryRoot string, options runOptions) (string, error) {
	if options.Task == "" {
		options.Task = "playable_platformer"
	}
	if options.Agent == "" {
		options.Agent = "build"
	}
	// The smoke test always exercises the MCP approach, regardless of the
	// caller's default approach flag.
	options.Approach = approachMCP

	task, err := loadTask(repositoryRoot, options.Task)
	if err != nil {
		return "", err
	}
	tempDir, err := os.MkdirTemp("", "godot-cli-mcp-smoke-")
	if err != nil {
		return "", err
	}
	if !options.KeepProject {
		defer func() { _ = os.RemoveAll(tempDir) }()
	}

	startedAt := time.Now()
	logf := func(format string, args ...any) {
		elapsed := time.Since(startedAt).Round(time.Millisecond)
		fmt.Printf("[+%s] %s\n", elapsed, fmt.Sprintf(format, args...))
	}
	if options.KeepProject {
		logf("Keeping MCP smoke workspace at %s", tempDir)
	}

	logf("Preparing isolated MCP benchmark project")
	setup, err := startBenchmarkSetup(repositoryRoot, options, task, tempDir, logf)
	if err != nil {
		return "", fmt.Errorf("preparing MCP smoke project: %w", err)
	}
	defer func() {
		if closeErr := setup.cleanup(nil); closeErr != nil {
			logf("Closing MCP smoke resources: %v", closeErr)
		}
	}()

	sessionID := setup.sessionID
	server := setup.server

	logf("Validating OpenCode session API for session %s", sessionID)
	apiContext, cancelAPI := context.WithTimeout(
		context.Background(),
		30*time.Second,
	)
	defer cancelAPI()

	session, err := server.getSession(apiContext, sessionID)
	if err != nil {
		return "", fmt.Errorf("reading session: %w", err)
	}
	if session.ID != sessionID {
		return "", fmt.Errorf(
			"session envelope returned ID %q, expected %q",
			session.ID,
			sessionID,
		)
	}
	expectedPermissions := benchmarkPermissions(approachMCP)
	if !reflect.DeepEqual(session.Permissions, expectedPermissions) {
		return "", fmt.Errorf(
			"session permissions were not preserved: got %#v, expected %#v",
			session.Permissions,
			expectedPermissions,
		)
	}
	collectedSession, messages, _, err := server.collectSession(
		apiContext,
		sessionID,
	)
	if err != nil {
		return "", fmt.Errorf("collecting model-free session: %w", err)
	}
	if collectedSession.ID != sessionID || len(messages) != 0 {
		return "", fmt.Errorf(
			"unexpected model-free session artifacts: session %q, %d messages",
			collectedSession.ID,
			len(messages),
		)
	}
	configResponse, err := server.request(
		apiContext,
		"GET",
		locationAPIPath("/api/config", setup.prepared.projectRoot),
		nil,
		nil,
	)
	if err != nil {
		return "", fmt.Errorf("reading effective configuration: %w", err)
	}
	if len(configResponse) == 0 || !json.Valid(configResponse) {
		return "", fmt.Errorf("configuration endpoint did not return JSON")
	}
	if err := setup.deleteSession(apiContext); err != nil {
		return "", fmt.Errorf("deleting smoke session: %w", err)
	}
	if _, err := server.getSession(apiContext, sessionID); err == nil {
		return "", fmt.Errorf("deleted session %s is still readable", sessionID)
	}

	report := setup.preflight
	return fmt.Sprintf(
		"Validated godot-cli MCP through OpenCode %s session %s: %d godot tools, permissions allow godot_* + execute and the godot-cli skill (instructions eligible), scene_list listed %s, scene_current reported it active; session collected empty and was deleted",
		commandVersion(setup.prepared.opencodePath),
		sessionID,
		countMCPTools(report.Tools),
		task.InitialScene,
	), nil
}
