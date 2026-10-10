package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/davidherring123/godot-cli/internal/bridge"
	"github.com/davidherring123/godot-cli/internal/core/scene"
)

const verifierPrefix = "GODOT_CLI_BENCHMARK_RESULT:"

type godotEditor struct {
	command *exec.Cmd
	done    chan error
	stdout  bytes.Buffer
	stderr  bytes.Buffer
}

func importProject(
	ctx context.Context,
	godotPath string,
	projectRoot string,
) error {
	command := exec.CommandContext(
		ctx,
		godotPath,
		"--headless",
		"--import",
		"--path",
		projectRoot,
	)

	if output, err := command.CombinedOutput(); err != nil {
		return fmt.Errorf("importing Godot project: %w\n%s", err, output)
	}
	return nil
}

func startGodotEditor(
	ctx context.Context,
	godotPath string,
	projectRoot string,
	initialScene string,
) (*godotEditor, error) {
	editor := &godotEditor{}
	editor.command = exec.Command(
		godotPath,
		"--headless",
		"--editor",
		"--path",
		projectRoot,
	)
	editor.command.Stdout = &editor.stdout
	editor.command.Stderr = &editor.stderr

	if err := editor.command.Start(); err != nil {
		return nil, fmt.Errorf("starting Godot editor: %w", err)
	}
	editor.done = make(chan error, 1)

	go func() {
		editor.done <- editor.command.Wait()
	}()

	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		client, err := bridge.Connect(projectRoot)
		if err == nil {
			_ = client.Close()

			break
		}
		select {
		case processErr := <-editor.done:
			editor.done = nil

			return nil, fmt.Errorf(
				"Godot exited before bridge startup: %v\n%s",
				processErr,
				editor.logs(),
			)
		case <-ctx.Done():
			_ = editor.Close()

			return nil, fmt.Errorf(
				"waiting for Godot bridge: %w\n%s",
				ctx.Err(),
				editor.logs(),
			)
		case <-ticker.C:
		}
	}
	openErr := make(chan error, 1)

	go func() {
		_, err := scene.Open(
			projectRoot,
			scene.OpenParams{Scene: initialScene},
		)
		openErr <- err
	}()

	select {
	case err := <-openErr:
		if err != nil {
			_ = editor.Close()

			return nil, fmt.Errorf("opening initial scene: %w", err)
		}
	case <-ctx.Done():
		_ = editor.Close()

		return nil, fmt.Errorf(
			"opening initial scene: %w\n%s",
			ctx.Err(),
			editor.logs(),
		)
	}
	return editor, nil
}

func (editor *godotEditor) Close() error {
	if editor == nil || editor.command == nil || editor.done == nil {
		return nil
	}
	_ = editor.command.Process.Signal(os.Interrupt)

	select {
	case <-editor.done:
		editor.done = nil

		return nil
	case <-time.After(5 * time.Second):
		if err := editor.command.Process.Kill(); err != nil {
			return err
		}
		<-editor.done
		editor.done = nil

		return nil
	}
}

func (editor *godotEditor) logs() string {
	return fmt.Sprintf(
		"stdout:\n%s\nstderr:\n%s",
		editor.stdout.String(),
		editor.stderr.String(),
	)
}

type verifySpec struct {
	Node       string   `json:"node"`
	Absent     bool     `json:"absent"`
	Properties []string `json:"properties"`
}

func verifySavedScene(
	ctx context.Context,
	godotPath string,
	projectRoot string,
	verifierPath string,
	verification verification,
) (probeResult, string, error) {
	checks := verification.checks()
	spec := make([]verifySpec, 0, len(checks))

	for _, check := range checks {
		propertyNames := make([]string, 0, len(check.Properties))
		for name := range check.Properties {
			propertyNames = append(propertyNames, name)
		}
		sort.Strings(propertyNames)
		spec = append(spec, verifySpec{
			Node:       check.Node,
			Absent:     check.Absent,
			Properties: propertyNames,
		})
	}

	encodedSpec, err := json.Marshal(spec)

	if err != nil {
		return probeResult{}, "", err
	}

	arguments := []string{
		"--headless",
		"--path",
		projectRoot,
		"--script",
		verifierPath,
		"--",
		verification.Scene,
		string(encodedSpec),
	}

	command := exec.CommandContext(ctx, godotPath, arguments...)

	output, err := command.CombinedOutput()
	text := string(output)

	for _, line := range strings.Split(text, "\n") {
		if !strings.HasPrefix(line, verifierPrefix) {
			continue
		}
		var result probeResult

		if decodeErr := json.Unmarshal(
			[]byte(strings.TrimPrefix(line, verifierPrefix)),
			&result,
		); decodeErr != nil {
			return probeResult{}, text, decodeErr
		}
		if err != nil {
			return result, text, fmt.Errorf("Godot verifier failed: %w", err)
		}
		return result, text, nil
	}
	if err != nil {
		return probeResult{}, text, fmt.Errorf(
			"Godot verifier failed: %w",
			err,
		)
	}
	return probeResult{}, text, errors.New(
		"Godot verifier did not produce a result",
	)
}
