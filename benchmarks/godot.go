package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/davidherring123/godot-cli/internal/bridge"
	"github.com/davidherring123/godot-cli/internal/core/scene"
)

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
