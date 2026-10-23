package runtime

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/davidherring123/godot-cli/internal/core/bridge"
	"github.com/davidherring123/godot-cli/internal/core/godot"
	"github.com/davidherring123/godot-cli/internal/core/scenepath"
)

//go:embed runner.gd
var runnerSource string

const readinessPoll = 50 * time.Millisecond

// Start launches the project's game process and waits until its scene has
// rendered enough frames to accept screenshots and report errors.
func Start(projectRoot string, params StartParams) (StatusResult, error) {
	pruneSessions(projectRoot)

	release, err := acquireStartLock(projectRoot)
	if err != nil {
		return StatusResult{}, err
	}
	defer release()

	active, err := activeSession(projectRoot)
	if err != nil {
		return StatusResult{}, err
	}

	if active != nil {
		return StatusResult{}, newError(
			string(bridge.ErrorCodeConflict),
			"a runtime session is already active for this project (%s); call runtime.status, or stop it with force=true if it is unresponsive",
			active.metadata.Session,
		)
	}

	_ = ensureRuntimeIgnore(projectRoot)

	scene, err := scenepath.Resolve(projectRoot, params.Scene)
	if err != nil {
		return StatusResult{}, newError(
			string(bridge.ErrorCodeInvalidArgument),
			"%v",
			err,
		)
	}

	godotPath, err := godot.FindExecutable()
	if err != nil {
		return StatusResult{}, err
	}

	session := newSessionID()
	directory := filepath.Join(sessionsRoot(projectRoot), session)

	if err := prepareSession(directory); err != nil {
		return StatusResult{}, newError(
			string(bridge.ErrorCodeInternalError),
			"creating runtime session directory: %v",
			err,
		)
	}

	runnerPath := filepath.Join(directory, "runner.gd")

	if err := os.WriteFile(runnerPath, []byte(runnerSource), 0644); err != nil {
		_ = os.RemoveAll(directory)

		return StatusResult{}, newError(
			string(bridge.ErrorCodeInternalError),
			"writing runtime runner: %v",
			err,
		)
	}

	stdout, stderr, err := openLogs(directory)
	if err != nil {
		_ = os.RemoveAll(directory)

		return StatusResult{}, err
	}

	args := []string{}

	if params.Headless {
		args = append(args, "--headless")
	}

	args = append(
		args,
		"--path", projectRoot,
		"--script", runnerPath,
		"--",
		"--session-dir", directory,
		"--scene", scene,
	)

	command := exec.Command(godotPath, args...)

	configureDetached(command)

	command.Stdout = stdout
	command.Stderr = stderr

	if err := command.Start(); err != nil {
		closeFiles(stdout, stderr)
		_ = os.RemoveAll(directory)

		return StatusResult{}, newError(
			string(bridge.ErrorCodeActionFailed),
			"starting Godot runtime: %v",
			err,
		)
	}

	closeFiles(stdout, stderr)

	// Reap the detached child while this process lives. Without Wait, a
	// stopped game stays a zombie and liveness checks keep reporting it as
	// alive. When the CLI exits instead, the child is reparented and reaped by
	// the OS.
	go func() { _ = command.Wait() }()

	pid := command.Process.Pid

	metadata := sessionMetadata{
		Session:   session,
		PID:       pid,
		Scene:     scene,
		Headless:  params.Headless,
		StartedAt: time.Now().UTC(),
	}

	if err := writeJSONAtomic(filepath.Join(directory, "session.json"), metadata); err != nil {
		_ = killProcess(pid)
		_ = os.RemoveAll(directory)

		return StatusResult{}, newError(
			string(bridge.ErrorCodeInternalError),
			"writing runtime session metadata: %v",
			err,
		)
	}

	timeout := defaultStartWait

	if params.TimeoutSeconds > 0 {
		timeout = time.Duration(params.TimeoutSeconds * float64(time.Second))
	}

	return waitForReady(projectRoot, directory, pid, timeout)
}

func waitForReady(
	projectRoot string,
	directory string,
	pid int,
	timeout time.Duration,
) (StatusResult, error) {
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()

	ticker := time.NewTicker(readinessPoll)
	defer ticker.Stop()

	session := filepath.Base(directory)

	for {
		if ready, found := readReady(directory); found {
			if !ready.OK {
				_ = terminate(pid, directory, time.Second)

				return StatusResult{}, startError(projectRoot, directory, ready.Message)
			}

			info, err := inspectSession(projectRoot, session)
			if err != nil {
				return StatusResult{}, err
			}

			return buildStatus(projectRoot, info), nil
		}

		if !processAlive(pid) {
			return StatusResult{}, startError(
				projectRoot,
				directory,
				"Godot exited before the runtime became ready",
			)
		}

		select {
		case <-deadline.C:
			_ = terminate(pid, directory, time.Second)

			return StatusResult{}, startError(
				projectRoot,
				directory,
				fmt.Sprintf(
					"Godot did not become ready within %s; the project may still be importing or no display is available (try headless=true)",
					timeout,
				),
			)
		case <-ticker.C:
		}
	}
}

func startError(projectRoot string, directory string, fallback string) error {
	code := string(bridge.ErrorCodeActionFailed)
	message := fallback

	if report, found := readExit(directory); found {
		switch report.Reason {
		case "scene_not_found", "scene_load_failed", "scene_instantiate_failed":
			code = string(bridge.ErrorCodeInvalidArgument)
		}

		if message == "" {
			message = fmt.Sprintf("Godot exited with %s (code %d)", report.Reason, report.Code)
		}
	}

	if entries, _ := readErrors(directory); len(entries) > 0 {
		messages := make([]string, 0, 3)

		for _, entry := range entries {
			if len(messages) >= 3 {
				break
			}

			messages = append(messages, entry.Message)
		}

		message += "; " + strings.Join(messages, " | ")
	}

	return newError(
		code,
		"%s; runtime logs: %s",
		message,
		logHint(projectRoot, directory),
	)
}

func logHint(projectRoot string, directory string) string {
	if path, err := resPath(projectRoot, directory); err == nil {
		return path + "/stdout.log and " + path + "/stderr.log"
	}

	return directory
}

func ensureRuntimeIgnore(projectRoot string) error {
	path := filepath.Join(runtimeRoot(projectRoot), ".gitignore")

	if _, err := os.Stat(path); err == nil {
		return nil
	}

	return os.WriteFile(path, []byte("sessions/\n.start.lock\n"), 0644)
}

func prepareSession(directory string) error {
	for _, name := range []string{"", "requests", "responses", "captures"} {
		path := directory

		if name != "" {
			path = filepath.Join(directory, name)
		}

		if err := os.MkdirAll(path, 0755); err != nil {
			return err
		}
	}

	return nil
}

func openLogs(directory string) (*os.File, *os.File, error) {
	stdout, err := os.OpenFile(
		filepath.Join(directory, "stdout.log"),
		os.O_CREATE|os.O_WRONLY|os.O_TRUNC,
		0644,
	)
	if err != nil {
		return nil, nil, newError(
			string(bridge.ErrorCodeInternalError),
			"opening runtime stdout log: %v",
			err,
		)
	}

	stderr, err := os.OpenFile(
		filepath.Join(directory, "stderr.log"),
		os.O_CREATE|os.O_WRONLY|os.O_TRUNC,
		0644,
	)
	if err != nil {
		_ = stdout.Close()

		return nil, nil, newError(
			string(bridge.ErrorCodeInternalError),
			"opening runtime stderr log: %v",
			err,
		)
	}

	return stdout, stderr, nil
}

func closeFiles(files ...*os.File) {
	for _, file := range files {
		if file != nil {
			_ = file.Close()
		}
	}
}

func acquireStartLock(projectRoot string) (func(), error) {
	if err := os.MkdirAll(sessionsRoot(projectRoot), 0755); err != nil {
		return nil, newError(
			string(bridge.ErrorCodeInternalError),
			"creating runtime directory: %v",
			err,
		)
	}

	lock := filepath.Join(runtimeRoot(projectRoot), ".start.lock")

	for attempt := 0; attempt < 2; attempt++ {
		err := os.Mkdir(lock, 0755)
		if err == nil {
			return func() { _ = os.Remove(lock) }, nil
		}

		if !os.IsExist(err) {
			return nil, newError(
				string(bridge.ErrorCodeInternalError),
				"acquiring runtime start lock: %v",
				err,
			)
		}

		stat, statErr := os.Stat(lock)

		if statErr == nil && time.Since(stat.ModTime()) > startLockStaleFor {
			_ = os.RemoveAll(lock)

			continue
		}

		return nil, newError(
			string(bridge.ErrorCodeBusy),
			"another runtime start is in progress; retry after it finishes",
		)
	}

	return nil, newError(
		string(bridge.ErrorCodeBusy),
		"another runtime start is in progress; retry after it finishes",
	)
}

func terminate(pid int, directory string, graceful time.Duration) bool {
	_ = os.WriteFile(
		filepath.Join(directory, "stop.request"),
		[]byte("stop\n"),
		0644,
	)

	if waitForExit(pid, graceful) {
		return true
	}

	if err := killProcess(pid); err != nil && processAlive(pid) {
		return false
	}

	return waitForExit(pid, killWait)
}

func waitForExit(pid int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		if !processAlive(pid) {
			return true
		}

		time.Sleep(25 * time.Millisecond)
	}

	return !processAlive(pid)
}

func killProcess(pid int) error {
	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}

	return process.Kill()
}
