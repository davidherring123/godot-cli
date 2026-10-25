package validate

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/davidherring123/godot-cli/internal/core/project/scenepath"
	"github.com/davidherring123/godot-cli/internal/port/godot"
)

const (
	reportPrefix   = "GODOT_CLI_VALIDATE:"
	defaultTimeout = 15 * time.Second
	interruptGrace = 3 * time.Second
)

// ErrGodotNotFound reports that no Godot executable could be located.
var ErrGodotNotFound = godot.ErrNotFound

//go:embed runner.gd
var runnerSource string

var errorLinePattern = regexp.MustCompile(`^(SCRIPT ERROR|USER ERROR|ERROR|FATAL):`)

type runnerReport struct {
	Valid  bool     `json:"valid"`
	Scene  string   `json:"scene"`
	Errors []string `json:"errors"`
}

type processOutput struct {
	stdout   string
	stderr   string
	exitErr  error
	timedOut bool
}

// Run loads one scene in a fresh headless Godot process, runs it briefly, and
// reports load, script, resource, and early runtime errors. It never opens the
// editor and never writes to the project.
func Run(projectRoot string, params RunParams) (RunResult, error) {
	godotPath, err := godot.FindExecutable()
	if err != nil {
		return RunResult{}, err
	}

	scene, err := resolveScene(projectRoot, params.Scene)
	if err != nil {
		return RunResult{}, err
	}

	runnerPath, err := writeRunner()
	if err != nil {
		return RunResult{}, err
	}

	defer func() { _ = os.Remove(runnerPath) }()

	limit := params.Timeout
	if limit <= 0 {
		limit = defaultTimeout
	}

	output, err := execute(godotPath, projectRoot, runnerPath, scene, limit)
	if err != nil {
		return RunResult{}, err
	}

	return evaluate(scene, limit, output), nil
}

func execute(
	godotPath string,
	projectRoot string,
	runnerPath string,
	scene string,
	limit time.Duration,
) (processOutput, error) {
	command := exec.Command(
		godotPath,
		"--headless",
		"--path", projectRoot,
		"--script", runnerPath,
		"--", scene,
	)

	var stdout, stderr bytes.Buffer

	command.Stdout = &stdout
	command.Stderr = &stderr

	if err := command.Start(); err != nil {
		return processOutput{}, fmt.Errorf("starting Godot: %w", err)
	}

	done := make(chan error, 1)

	go func() { done <- command.Wait() }()

	run := processOutput{}

	select {
	case run.exitErr = <-done:
	case <-time.After(limit):
		run.timedOut = true

		_ = command.Process.Signal(os.Interrupt)

		select {
		case <-done:
		case <-time.After(interruptGrace):
			_ = command.Process.Kill()
			<-done
		}
	}

	run.stdout = stdout.String()
	run.stderr = stderr.String()

	return run, nil
}

func evaluate(scene string, limit time.Duration, run processOutput) RunResult {
	runner, reported := parseReport(run.stdout)

	errors := []string{}
	seen := map[string]bool{}

	add := func(messages ...string) {
		for _, message := range messages {
			if message == "" || seen[message] {
				continue
			}

			seen[message] = true
			errors = append(errors, message)
		}
	}

	if reported {
		add(runner.Errors...)

		if runner.Valid && run.exitErr != nil && !run.timedOut {
			add(fmt.Sprintf("Godot exited with an error: %v", run.exitErr))
		}
	} else {
		add("Godot did not report a validation result")

		if run.exitErr != nil && !run.timedOut {
			add(fmt.Sprintf("Godot exited unexpectedly: %v", run.exitErr))
		}
	}

	add(scanErrors(run.stderr)...)
	add(scanErrors(run.stdout)...)

	if run.timedOut {
		add(fmt.Sprintf("Godot did not exit within %s", limit))
	}

	return RunResult{
		Valid:  reported && runner.Valid && len(errors) == 0,
		Scene:  scene,
		Errors: errors,
	}
}

func parseReport(stdout string) (runnerReport, bool) {
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(line)

		if !strings.HasPrefix(line, reportPrefix) {
			continue
		}

		var report runnerReport

		if err := json.Unmarshal([]byte(strings.TrimPrefix(line, reportPrefix)), &report); err != nil {
			return runnerReport{}, false
		}

		return report, true
	}

	return runnerReport{}, false
}

func scanErrors(text string) []string {
	found := []string{}

	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)

		if errorLinePattern.MatchString(line) {
			found = append(found, line)
		}
	}

	return found
}

func resolveScene(projectRoot string, scene string) (string, error) {
	return scenepath.Resolve(projectRoot, scene)
}

func writeRunner() (string, error) {
	file, err := os.CreateTemp("", "godot-cli-validate-*.gd")
	if err != nil {
		return "", fmt.Errorf("creating validation runner: %w", err)
	}

	_, writeErr := file.WriteString(runnerSource)
	closeErr := file.Close()

	if writeErr != nil || closeErr != nil {
		_ = os.Remove(file.Name())

		return "", fmt.Errorf(
			"writing validation runner: %w",
			errors.Join(writeErr, closeErr),
		)
	}

	return file.Name(), nil
}
