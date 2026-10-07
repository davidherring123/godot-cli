// Package harness manages the shared Godot editor used by core tests.
package harness

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/davidherring123/godot-cli/internal/addon"
	"github.com/davidherring123/godot-cli/internal/bridge"
)

const (
	startupTimeout  = 30 * time.Second
	shutdownTimeout = 5 * time.Second
)

var ErrGodotNotFound = errors.New("Godot executable not found")

type Session struct {
	Client *bridge.Client

	tempDir string
	root    string
	logf    func(format string, args ...any)

	command *exec.Cmd
	done    chan error
	stdout  lockedBuffer
	stderr  lockedBuffer
}

func (session *Session) Root() string {
	return session.root
}

func Start(outputLogf func(format string, args ...any)) (*Session, error) {
	startedAt := time.Now()

	if outputLogf == nil {
		outputLogf = func(string, ...any) {}
	}

	logf := func(format string, args ...any) {
		elapsed := time.Since(startedAt).Round(time.Millisecond)
		outputArgs := append([]any{elapsed}, args...)

		outputLogf("[+%s] "+format, outputArgs...)
	}

	repositoryRoot, err := findRepositoryRoot()
	if err != nil {
		return nil, err
	}

	fixture := filepath.Join(repositoryRoot, "tests", "fixtures", "core")

	logf("Preparing core project fixture from %s", fixture)

	if info, err := os.Stat(fixture); err != nil {
		return nil, fmt.Errorf("finding core project fixture: %w", err)
	} else if !info.IsDir() {
		return nil, errors.New("core project fixture is not a directory")
	}

	godotPath, err := findGodot()
	if err != nil {
		return nil, err
	}

	logf("Using Godot executable %s", godotPath)

	mode, err := editorMode()
	if err != nil {
		return nil, err
	}

	tempDir, err := os.MkdirTemp("", "godot-cli-tests-")
	if err != nil {
		return nil, fmt.Errorf("creating temporary directory: %w", err)
	}

	session := &Session{
		tempDir: tempDir,
		root:    filepath.Join(tempDir, "project"),
		logf:    logf,
	}

	logf("Copying core project to %s", session.root)

	if err := copyDirectory(fixture, session.root); err != nil {
		_ = session.Close()

		return nil, fmt.Errorf("copying core project fixture: %w", err)
	}

	if err := addon.Install(session.root); err != nil {
		_ = session.Close()

		return nil, fmt.Errorf("installing addon: %w", err)
	}

	logf("Installed current godot-cli addon")

	if err := session.startEditor(godotPath, mode); err != nil {
		_ = session.Close()

		return nil, err
	}

	return session, nil
}

func (session *Session) startEditor(godotPath, mode string) error {
	args := []string{
		"--editor",
		"--path",
		session.root,
	}

	if mode == "headless" {
		args = append([]string{"--headless"}, args...)
	}

	session.logf("Starting Godot editor in %s mode", mode)

	command := exec.Command(godotPath, args...)

	command.Stdout = &session.stdout
	command.Stderr = &session.stderr

	if err := command.Start(); err != nil {
		return fmt.Errorf("starting Godot editor: %w", err)
	}

	session.command = command
	session.done = make(chan error, 1)

	session.logf("Godot editor started with PID %d", command.Process.Pid)

	go func() {
		session.done <- command.Wait()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), startupTimeout)
	defer cancel()

	session.logf("Waiting up to %s for the addon bridge", startupTimeout)

	client, err := session.connect(ctx)
	if err != nil {
		return err
	}

	session.Client = client

	session.logf("Godot editor and addon bridge are ready")

	return nil
}

func (session *Session) connect(ctx context.Context) (*bridge.Client, error) {
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		client, err := bridge.Connect(session.root)
		if err == nil {
			return client, nil
		}

		select {
		case processErr := <-session.done:
			session.done = nil

			return nil, fmt.Errorf(
				"Godot editor exited before its bridge became ready: %v\n%s",
				processErr,
				session.logs(),
			)
		case <-ctx.Done():
			return nil, fmt.Errorf(
				"waiting for Godot bridge: %w\n%s",
				ctx.Err(),
				session.logs(),
			)
		case <-ticker.C:
		}
	}
}

func (session *Session) LogJSON(label string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding %s as JSON: %w", label, err)
	}

	session.logf("%s returned:\n%s", label, data)

	return nil
}

func (session *Session) Close() error {
	if session.Client != nil {
		_ = session.Client.Close()
	}

	if session.command != nil &&
		session.command.Process != nil &&
		session.done != nil {
		session.logf(
			"Stopping Godot editor PID %d",
			session.command.Process.Pid,
		)

		_ = session.command.Process.Signal(os.Interrupt)

		select {
		case <-session.done:
		case <-time.After(shutdownTimeout):
			if err := session.command.Process.Kill(); err != nil {
				return fmt.Errorf("killing Godot editor: %w", err)
			}

			<-session.done
		}
	}

	session.logf("Removing temporary project %s", session.tempDir)

	if err := os.RemoveAll(session.tempDir); err != nil {
		return fmt.Errorf("removing temporary project: %w", err)
	}

	session.logf("Core test session cleanup complete")

	return nil
}

func (session *Session) logs() string {
	return fmt.Sprintf(
		"stdout:\n%s\nstderr:\n%s",
		session.stdout.String(),
		session.stderr.String(),
	)
}

func editorMode() (string, error) {
	mode := strings.ToLower(strings.TrimSpace(os.Getenv("GODOT_MODE")))

	if mode == "" {
		return "headless", nil
	}

	if mode != "headless" && mode != "graphical" {
		return "", errors.New(
			"GODOT_MODE must be \"headless\" or \"graphical\"",
		)
	}

	return mode, nil
}

func findGodot() (string, error) {
	candidates := []string{
		os.Getenv("GODOT_BIN"),
		"godot",
		"godot4",
	}

	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}

		path, err := exec.LookPath(candidate)
		if err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf(
		"%w; set GODOT_BIN to its path",
		ErrGodotNotFound,
	)
}

func findRepositoryRoot() (string, error) {
	current, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		if _, err := os.Stat(filepath.Join(current, "go.mod")); err == nil {
			return current, nil
		}

		parent := filepath.Dir(current)

		if parent == current {
			return "", errors.New("could not find repository root")
		}

		current = parent
	}
}

func copyDirectory(source, destination string) error {
	return filepath.WalkDir(
		source,
		func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}

			relative, err := filepath.Rel(source, path)
			if err != nil {
				return err
			}

			target := filepath.Join(destination, relative)

			if entry.IsDir() {
				return os.MkdirAll(target, 0755)
			}

			if entry.Type()&os.ModeSymlink != 0 {
				return fmt.Errorf(
					"fixture contains unsupported symbolic link %s",
					relative,
				)
			}

			input, err := os.Open(path)
			if err != nil {
				return err
			}
			defer input.Close()

			output, err := os.OpenFile(
				target,
				os.O_CREATE|os.O_WRONLY|os.O_TRUNC,
				0644,
			)
			if err != nil {
				return err
			}

			_, copyErr := io.Copy(output, input)
			closeErr := output.Close()

			if copyErr != nil {
				return copyErr
			}

			return closeErr
		},
	)
}

type lockedBuffer struct {
	mutex sync.Mutex
	data  bytes.Buffer
}

func (buffer *lockedBuffer) Write(data []byte) (int, error) {
	buffer.mutex.Lock()
	defer buffer.mutex.Unlock()

	return buffer.data.Write(data)
}

func (buffer *lockedBuffer) String() string {
	buffer.mutex.Lock()
	defer buffer.mutex.Unlock()

	return buffer.data.String()
}
