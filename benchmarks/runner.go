package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/davidherring123/godot-cli/internal/addon"
	"github.com/davidherring123/godot-cli/internal/agent/skill"
	"github.com/davidherring123/godot-cli/internal/cli"
)

type runOptions struct {
	Model        string
	Agent        string
	Approach     string
	Task         string
	GodotPath    string
	OpenCodePath string
	ResultsDir   string
	KeepProject  bool
	Timeout      time.Duration
	Label        string
}

const (
	approachCLI    = "godot-cli"
	approachDirect = "direct-files"
)

func runBenchmark(
	repositoryRoot string,
	options runOptions,
) (string, error) {
	startedAt := time.Now()
	logf := func(format string, args ...any) {
		elapsed := time.Since(startedAt).Round(time.Millisecond)
		label := ""

		if options.Label != "" {
			label = "[" + options.Label + "]"
		}
		fmt.Printf(
			"%s[+%s] "+format+"\n",
			append([]any{label, elapsed}, args...)...,
		)
	}
	task, err := loadTask(repositoryRoot, options.Task)
	if err != nil {
		return "", err
	}
	if options.Timeout <= 0 {
		options.Timeout = 5 * time.Minute
	}
	stamp := startedAt.UTC().Format("20060102T150405.000000000Z")
	runName := strings.Join([]string{
		stamp,
		task.ID,
		options.Approach,
		sanitizePath(options.Model),
	}, "-")
	resultPath := filepath.Join(options.ResultsDir, runName+".json")

	if err := os.MkdirAll(options.ResultsDir, 0755); err != nil {
		return "", err
	}
	tempDir, err := os.MkdirTemp("", "godot-cli-benchmark-")
	if err != nil {
		return "", err
	}
	if !options.KeepProject {
		defer func() { _ = os.RemoveAll(tempDir) }()
	} else {
		logf("Keeping benchmark workspace at %s", tempDir)
	}
	prepared, err := prepareRun(repositoryRoot, options, task, tempDir, logf)
	if err != nil {
		return "", err
	}

	result := benchmarkResult{
		Task:     task.ID,
		Approach: options.Approach,
		Model:    options.Model,
	}
	var editor *godotEditor

	if options.Approach == approachCLI {
		logf("Starting Godot editor")
		startupContext, cancelStartup := context.WithTimeout(
			context.Background(),
			30*time.Second,
		)

		editor, err = startGodotEditor(
			startupContext,
			prepared.godotPath,
			prepared.projectRoot,
			task.InitialScene,
		)
		cancelStartup()
		if err != nil {
			return "", err
		}
		defer func() { _ = editor.Close() }()
	}
	logf("Starting dedicated OpenCode server")
	startupContext, cancelStartup := context.WithTimeout(
		context.Background(),
		30*time.Second,
	)

	server, err := startOpenCodeServer(
		startupContext,
		prepared.opencodePath,
		prepared.environment,
	)
	cancelStartup()
	if err != nil {
		closeEditor(editor)
		return "", err
	}
	defer func() { _ = server.Close() }()

	sessionContext, cancelSession := context.WithTimeout(
		context.Background(),
		30*time.Second,
	)
	sessionID, err := server.createSession(
		sessionContext,
		prepared.projectRoot,
		options.Agent,
		options.Approach,
		"godot-cli benchmark: "+task.ID,
	)
	if err != nil {
		cancelSession()
		closeEditor(editor)
		return "", err
	}
	cancelSession()

	result.SessionID = sessionID

	prompt := taskPrompt(task, options.Approach)

	logf("Running OpenCode session %s", sessionID)
	agentStartedAt := time.Now()
	runContext, cancelRun := context.WithTimeout(
		context.Background(),
		options.Timeout,
	)

	runErr := server.run(
		runContext,
		prepared.opencodePath,
		prepared.projectRoot,
		sessionID,
		options.Model,
		options.Agent,
		prompt,
		prepared.environment,
	)
	cancelRun()
	result.AgentMS = time.Since(agentStartedAt).Milliseconds()

	collectContext, cancelCollect := context.WithTimeout(
		context.Background(),
		30*time.Second,
	)
	session, messages, contextMetrics, collectErr := server.collectSession(
		collectContext,
		sessionID,
	)
	cancelCollect()
	if collectErr != nil && runErr == nil {
		runErr = collectErr
	}
	result.CostUSD = session.Cost
	result.Tokens = summarizeTokens(session.Tokens)
	result.Context = contextMetrics
	result.Tools = summarizeMessages(messages)

	runErr = closeRunResources(editor, server, runErr)
	logf("Verifying persisted project state")

	verifyContext, cancelVerify := context.WithTimeout(
		context.Background(),
		30*time.Second,
	)
	defer cancelVerify()

	probe, verifyOutput, verifyErr := verifySavedScene(
		verifyContext,
		prepared.godotPath,
		prepared.projectRoot,
		filepath.Join(repositoryRoot, "benchmarks", "verify_scene.gd"),
		task.Verify,
	)

	if verifyErr != nil || !probe.Loaded {
		logf("Godot verifier output:\n%s", verifyOutput)
	}

	usedCLI, err := invocationRecorded(prepared.invocationLog)
	if err != nil {
		return "", err
	}
	result.Success, result.Failure = evaluateResult(
		task,
		result,
		prepared.projectRoot,
		session.Outcome,
		probe,
		usedCLI,
		runErr,
		verifyErr,
	)

	if err := writeJSON(resultPath, result); err != nil {
		return "", err
	}
	if result.Success {
		logf("Benchmark succeeded")
	} else {
		logf("Benchmark failed: %s", result.Failure)
	}
	return resultPath, nil
}

type preparedRun struct {
	projectRoot   string
	godotPath     string
	opencodePath  string
	environment   []string
	invocationLog string
}

func prepareRun(
	repositoryRoot string,
	options runOptions,
	task taskDefinition,
	tempDir string,
	logf func(format string, args ...any),
) (preparedRun, error) {
	projectRoot := filepath.Join(tempDir, "project")
	fixture := filepath.Join(
		repositoryRoot,
		"benchmarks",
		"fixtures",
		task.ID,
	)

	logf("Copying task fixture")

	if err := copyDirectory(fixture, projectRoot); err != nil {
		return preparedRun{}, err
	}
	toolsDir := filepath.Join(tempDir, "bin")
	if err := os.MkdirAll(toolsDir, 0755); err != nil {
		return preparedRun{}, err
	}
	proxyCLI := filepath.Join(toolsDir, "godot-cli")

	logf("Building benchmark proxy")

	if err := buildBinary(
		repositoryRoot,
		proxyCLI,
		"./benchmarks/cmd/godot-cli-proxy",
	); err != nil {
		return preparedRun{}, err
	}
	realCLI := ""

	if options.Approach == approachCLI {
		realCLI = filepath.Join(toolsDir, "godot-cli-real")

		logf("Building godot-cli")

		if err := buildBinary(repositoryRoot, realCLI, "./cmd/godot-cli"); err != nil {
			return preparedRun{}, err
		}
		if err := addon.Install(projectRoot); err != nil {
			return preparedRun{}, fmt.Errorf("installing addon: %w", err)
		}
		if _, err := skill.Install(
			projectRoot,
			cli.NewRootCommand("benchmark"),
		); err != nil {
			return preparedRun{}, fmt.Errorf("installing OpenCode skill: %w", err)
		}
		if err := enableAddon(projectRoot); err != nil {
			return preparedRun{}, err
		}
	}
	godotPath, err := findExecutable(
		options.GodotPath,
		os.Getenv("GODOT_BIN"),
		"godot",
		"godot4",
	)
	if err != nil {
		return preparedRun{}, err
	}
	opencodePath, err := findExecutable(options.OpenCodePath)
	if err != nil {
		return preparedRun{}, err
	}
	invocationLog := filepath.Join(tempDir, "godot-cli-invocations")
	environmentValues := map[string]string{
		"PATH":             toolsDir + string(os.PathListSeparator) + os.Getenv("PATH"),
		"GODOT_CLI_MARKER": invocationLog,
	}
	if options.Approach == approachCLI {
		environmentValues["GODOT_CLI_REAL"] = realCLI
	} else {
		environmentValues["GODOT_CLI_BLOCK"] = "1"
	}
	environment, err := isolatedOpenCodeEnvironment(
		os.Environ(),
		tempDir,
		environmentValues,
	)
	if err != nil {
		return preparedRun{}, fmt.Errorf("isolating OpenCode configuration: %w", err)
	}
	logf("Importing project resources")
	importContext, cancelImport := context.WithTimeout(
		context.Background(),
		60*time.Second,
	)
	err = importProject(importContext, godotPath, projectRoot)
	cancelImport()
	if err != nil {
		return preparedRun{}, err
	}
	return preparedRun{
		projectRoot:   projectRoot,
		godotPath:     godotPath,
		opencodePath:  opencodePath,
		environment:   environment,
		invocationLog: invocationLog,
	}, nil
}

func closeEditor(editor *godotEditor) {
	if editor == nil {
		return
	}

	_ = editor.Close()
}

func closeRunResources(
	editor *godotEditor,
	server *openCodeServer,
	runErr error,
) error {
	if editor != nil {
		if err := editor.Close(); err != nil && runErr == nil {
			runErr = err
		}
	}

	if err := server.Close(); err != nil && runErr == nil {
		runErr = err
	}

	return runErr
}

func evaluateResult(
	task taskDefinition,
	result benchmarkResult,
	projectRoot string,
	responseOutcome string,
	probe probeResult,
	usedCLI bool,
	runErr error,
	verifyErr error,
) (bool, string) {
	var failures []string

	if runErr != nil {
		failures = append(failures, runErr.Error())
	}
	if responseOutcome != "" && responseOutcome != "succeeded" {
		failures = append(failures, "OpenCode session outcome: "+responseOutcome)
	}
	if verifyErr != nil {
		failures = append(failures, verifyErr.Error())
	}
	if !probe.Loaded {
		failures = append(failures, "scene did not load")
	} else {
		for _, check := range task.Verify.Nodes {
			probeCheck, ok := findProbeCheck(probe, check.Node)

			if !ok {
				failures = append(
					failures,
					"verification did not report node: "+check.Node,
				)
				continue
			}

			if check.Absent {
				if probeCheck.Found {
					failures = append(failures, "node still exists: "+check.Node)
				}
				continue
			}

			if !probeCheck.Found {
				failures = append(failures, "node was not found: "+check.Node)
				continue
			}

			failures = append(
				failures,
				compareProperties(check.Node, check.Properties, probeCheck.Properties)...,
			)
		}
	}
	if result.Approach == approachCLI {
		if !usedCLI {
			failures = append(failures, "agent did not invoke godot-cli")
		}
		failures = append(
			failures,
			directEditViolations(result.Tools, projectRoot)...,
		)
	}
	if result.Approach == approachDirect && usedCLI {
		failures = append(failures, "agent attempted to invoke godot-cli")
	}
	return len(failures) == 0, strings.Join(failures, "; ")
}

func directEditViolations(tools []toolCall, projectRoot string) []string {
	var violations []string

	for _, tool := range tools {
		switch tool.Name {
		case "edit", "write":
			var input struct {
				Path string `json:"path"`
			}
			if json.Unmarshal(tool.Input, &input) != nil {
				continue
			}
			relative, inside := projectRelativePath(projectRoot, input.Path)
			if inside {
				violations = append(
					violations,
					"edited project file directly: "+relative,
				)
			}
		case "patch":
			violations = append(
				violations,
				"applied a patch instead of using godot-cli",
			)
		}
	}
	return violations
}

func projectRelativePath(projectRoot, path string) (string, bool) {
	if path == "" {
		return "", false
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(projectRoot, path)
	}
	relative, err := filepath.Rel(projectRoot, filepath.Clean(path))
	if err != nil {
		return "", false
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", false
	}
	return filepath.ToSlash(relative), true
}

func findProbeCheck(probe probeResult, node string) (probeCheck, bool) {
	for _, check := range probe.Checks {
		if check.Node == node {
			return check, true
		}
	}
	return probeCheck{}, false
}

func compareProperties(
	node string,
	expected, actual map[string]any,
) []string {
	var failures []string

	for name, want := range expected {
		got, ok := actual[name]

		if !ok {
			failures = append(
				failures,
				node+" property was not reported: "+name,
			)
			continue
		}
		if !reflect.DeepEqual(want, got) {
			wantJSON, _ := json.Marshal(want)
			gotJSON, _ := json.Marshal(got)

			failures = append(failures, fmt.Sprintf(
				"expected %s property %s to be %s, got %s",
				node,
				name,
				wantJSON,
				gotJSON,
			))
		}
	}
	return failures
}

func invocationRecorded(path string) (bool, error) {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return info.Size() > 0, nil
}

func taskPrompt(task taskDefinition, approach string) string {
	if approach == approachDirect {
		return task.Goal + " Directly edit the Godot project files. Do not use godot-cli or any MCP tools."
	}
	return task.Goal + " Use godot-cli for all scene inspection and changes. Do not edit .tscn files directly."
}
