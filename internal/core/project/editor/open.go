package editor

import (
	"errors"
	"fmt"
	"os/exec"
	"time"

	"github.com/davidherring123/godot-cli/internal/port/editorinstance"
	"github.com/davidherring123/godot-cli/internal/port/godot"
)

const (
	defaultTimeout = 30 * time.Second
	pollInterval   = 250 * time.Millisecond
)

// Open starts the Godot editor for the project and waits for the godot-cli
// addon bridge to register. It returns the existing instance when an editor is
// already running.
func Open(projectRoot string, params OpenParams) (OpenResult, error) {
	instance, err := editorinstance.Find(projectRoot)
	switch {
	case err == nil:
		return openResult(instance, false, true), nil
	case !errors.Is(err, editorinstance.ErrNoEditor):
		return OpenResult{}, err
	}

	godotPath, err := godot.FindExecutable()
	if err != nil {
		return OpenResult{}, err
	}

	command := exec.Command(godotPath, "--editor", "--path", projectRoot)

	configureDetached(command)

	if err := command.Start(); err != nil {
		return OpenResult{}, fmt.Errorf("starting Godot editor: %w", err)
	}

	waited := make(chan error, 1)

	go func() {
		waited <- command.Wait()
	}()

	timeout := defaultTimeout

	if params.TimeoutSeconds > 0 {
		timeout = time.Duration(params.TimeoutSeconds * float64(time.Second))
	}

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	deadline := time.NewTimer(timeout)
	defer deadline.Stop()

	for {
		select {
		case <-ticker.C:
			instance, err := editorinstance.Find(projectRoot)
			if err == nil {
				return openResult(instance, true, false), nil
			}

			if !errors.Is(err, editorinstance.ErrNoEditor) {
				return OpenResult{}, err
			}
		case err := <-waited:
			if err != nil {
				return OpenResult{}, fmt.Errorf(
					"godot exited before the editor bridge started: %w",
					err,
				)
			}

			return OpenResult{}, errors.New(
				"godot exited before the editor bridge started",
			)
		case <-deadline.C:
			return OpenResult{}, fmt.Errorf(
				"godot did not start the editor bridge within %s; install and enable the addon with `godot-cli init`, then restart Godot",
				timeout,
			)
		}
	}
}

func openResult(instance editorinstance.Instance, opened bool, alreadyRunning bool) OpenResult {
	return OpenResult{
		Opened:         opened,
		AlreadyRunning: alreadyRunning,
		PID:            instance.PID,
		Port:           instance.Port,
		Version:        instance.Version,
	}
}
