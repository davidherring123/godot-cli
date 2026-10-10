package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"time"
)

func smokeOpenCodeAPI(requestedPath string) (string, error) {
	opencodePath, err := findExecutable(requestedPath)
	if err != nil {
		return "", err
	}
	tempDir, err := os.MkdirTemp("", "godot-cli-opencode-smoke-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tempDir)

	environment, err := isolatedOpenCodeEnvironment(
		os.Environ(),
		tempDir,
		map[string]string{},
	)
	if err != nil {
		return "", fmt.Errorf("isolating OpenCode configuration: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	server, err := startOpenCodeServer(ctx, opencodePath, environment)
	if err != nil {
		return "", err
	}
	defer server.Close()

	sessionID, err := server.createSession(
		ctx,
		tempDir,
		"build",
		approachDirect,
		"godot-cli benchmark API smoke test",
	)
	if err != nil {
		return "", fmt.Errorf("creating empty session: %w", err)
	}
	deleted := false
	defer func() {
		if deleted {
			return
		}
		cleanupContext, cancelCleanup := context.WithTimeout(
			context.Background(),
			5*time.Second,
		)
		defer cancelCleanup()
		_ = server.deleteSession(cleanupContext, sessionID)
	}()

	session, err := server.getSession(ctx, sessionID)
	if err != nil {
		return "", fmt.Errorf("reading empty session: %w", err)
	}
	if session.ID != sessionID {
		return "", fmt.Errorf("session envelope returned ID %q, expected %q", session.ID, sessionID)
	}
	expectedPermissions := benchmarkPermissions(approachDirect)
	if !reflect.DeepEqual(session.Permissions, expectedPermissions) {
		return "", fmt.Errorf(
			"session permissions were not preserved: got %#v, expected %#v",
			session.Permissions,
			expectedPermissions,
		)
	}
	collectedSession, messages, _, err := server.collectSession(
		ctx,
		sessionID,
	)
	if err != nil {
		return "", fmt.Errorf("collecting empty session: %w", err)
	}
	if collectedSession.ID != sessionID || len(messages) != 0 {
		return "", fmt.Errorf(
			"unexpected empty-session artifacts: session %q, %d messages",
			collectedSession.ID,
			len(messages),
		)
	}
	configResponse, err := server.request(
		ctx,
		"GET",
		locationAPIPath("/api/config", tempDir),
		nil,
		nil,
	)
	if err != nil {
		return "", fmt.Errorf("reading effective configuration: %w", err)
	}
	if len(configResponse) == 0 || !json.Valid(configResponse) {
		return "", fmt.Errorf("configuration endpoint did not return JSON")
	}
	if err := server.deleteSession(ctx, sessionID); err != nil {
		return "", fmt.Errorf("deleting empty session: %w", err)
	}
	deleted = true
	if _, err := server.getSession(ctx, sessionID); err == nil {
		return "", fmt.Errorf("deleted session %s is still readable", sessionID)
	}
	return fmt.Sprintf(
		"Validated OpenCode %s session %s",
		commandVersion(opencodePath),
		sessionID,
	), nil
}
