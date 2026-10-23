package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/davidherring123/godot-cli/internal/addon"
	"github.com/davidherring123/godot-cli/internal/agent/skill"
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
	approachMCP    = "mcp"
	approachDirect = "direct-files"
)

// resolveApproaches expands the public --approach value. "both" is the
// MCP-first comparison: MCP is listed first so it runs first in an even suite.
func resolveApproaches(value string) ([]string, error) {
	switch value {
	case approachMCP:
		return []string{approachMCP}, nil
	case approachDirect:
		return []string{approachDirect}, nil
	case approachBoth:
		return []string{approachMCP, approachDirect}, nil
	default:
		return nil, errors.New(
			"--approach must be mcp, direct-files, or both",
		)
	}
}

func usesGodotEditor(approach string) bool {
	return approach == approachMCP
}

// benchmarkSetup is the live state shared by model runs and the model-free
// MCP smoke check.
type benchmarkSetup struct {
	prepared    preparedRun
	editor      *godotEditor
	server      *openCodeServer
	sessionID   string
	traceOffset int64
	preflight   mcpPreflightReport
}

func (setup *benchmarkSetup) deleteSession(ctx context.Context) error {
	if setup.server == nil || setup.sessionID == "" {
		return nil
	}
	return setup.server.deleteSession(ctx, setup.sessionID)
}

// cleanup deletes the session and closes the editor and OpenCode server. It is
// best-effort: session deletion failures are ignored because the server is
// about to exit, and shutdown errors are reported only when the run succeeded.
func (setup *benchmarkSetup) cleanup(runErr error) error {
	if setup == nil {
		return runErr
	}
	deleteContext, cancelDelete := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	_ = setup.deleteSession(deleteContext)
	cancelDelete()

	if err := closeRunResources(setup.editor, setup.server, nil); err != nil && runErr == nil {
		runErr = err
	}
	setup.editor = nil
	setup.server = nil
	return runErr
}

// startBenchmarkSetup owns everything that happens before the model timer. On
// error it closes any resources it opened; the caller persists the returned
// error as a setup failure.
func startBenchmarkSetup(
	repositoryRoot string,
	options runOptions,
	task taskDefinition,
	tempDir string,
	logf func(format string, args ...any),
) (benchmarkSetup, error) {
	var setup benchmarkSetup

	prepared, err := prepareRun(repositoryRoot, options, task, tempDir, logf)
	setup.prepared = prepared
	if err != nil {
		return setup, err
	}

	if usesGodotEditor(options.Approach) {
		logf("Starting Godot editor")
		startupContext, cancelStartup := context.WithTimeout(
			context.Background(),
			30*time.Second,
		)

		setup.editor, err = startGodotEditor(
			startupContext,
			prepared.godotPath,
			prepared.projectRoot,
			task.InitialScene,
		)
		cancelStartup()
		if err != nil {
			return setup, err
		}
	}

	logf("Starting dedicated OpenCode server")
	startupContext, cancelStartup := context.WithTimeout(
		context.Background(),
		30*time.Second,
	)

	setup.server, err = startOpenCodeServer(
		startupContext,
		prepared.opencodePath,
		prepared.environment,
	)
	cancelStartup()
	if err != nil {
		_ = setup.cleanup(nil)
		return setup, err
	}

	sessionContext, cancelSession := context.WithTimeout(
		context.Background(),
		30*time.Second,
	)
	setup.sessionID, err = setup.server.createSession(
		sessionContext,
		prepared.projectRoot,
		options.Agent,
		options.Approach,
		"godot-cli benchmark: "+task.ID,
	)
	cancelSession()
	if err != nil {
		_ = setup.cleanup(nil)
		return setup, err
	}

	if options.Approach == approachMCP {
		logf("Preflighting MCP tools")
		preflightContext, cancelPreflight := context.WithTimeout(
			context.Background(),
			60*time.Second,
		)
		setup.preflight, err = setup.server.preflightMCP(
			preflightContext,
			prepared.projectRoot,
			setup.sessionID,
			task.InitialScene,
		)
		cancelPreflight()
		if err != nil {
			_ = setup.cleanup(nil)
			return setup, fmt.Errorf("MCP preflight: %w", err)
		}
	}

	// Capture the trace offset only after preflight and immediately before the
	// model timer starts, so setup traffic never counts as agent usage.
	setup.traceOffset, err = traceFileSize(prepared.tracePath)
	if err != nil {
		_ = setup.cleanup(nil)
		return setup, err
	}
	return setup, nil
}

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

	result := benchmarkResult{
		Task:     task.ID,
		Approach: options.Approach,
		Model:    options.Model,
	}
	setupStartedAt := time.Now()
	var setup benchmarkSetup

	// Setup failures are persisted with their own field and duration. They are
	// not conflated with agent or task failures in the result or suite summary.
	failSetup := func(setupErr error) (string, error) {
		result.SetupMS = time.Since(setupStartedAt).Milliseconds()
		result.SetupFailure = setupErr.Error()
		result.Failure = setupErr.Error()
		result.Success = false

		_ = setup.cleanup(nil)

		path, writeErr := persistBenchmarkResult(
			resultPath,
			options.ResultsDir,
			runName,
			result,
			setup.prepared,
			"",
		)
		if writeErr != nil {
			return "", errors.Join(setupErr, writeErr)
		}
		return path, setupErr
	}

	setup, err = startBenchmarkSetup(
		repositoryRoot,
		options,
		task,
		tempDir,
		logf,
	)
	result.SessionID = setup.sessionID
	if err != nil {
		return failSetup(err)
	}
	result.SetupMS = time.Since(setupStartedAt).Milliseconds()

	prepared := setup.prepared
	prompt := taskPrompt(task, options.Approach)

	logf("Running OpenCode session %s", setup.sessionID)
	agentStartedAt := time.Now()
	runContext, cancelRun := context.WithTimeout(
		context.Background(),
		options.Timeout,
	)

	runErr := setup.server.run(
		runContext,
		prepared.opencodePath,
		prepared.projectRoot,
		setup.sessionID,
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
	session, messages, contextMetrics, collectErr := setup.server.collectSession(
		collectContext,
		setup.sessionID,
	)
	cancelCollect()
	if collectErr != nil && runErr == nil {
		runErr = collectErr
	}
	result.CostUSD = session.Cost
	result.Tokens = summarizeTokens(session.Tokens)
	result.Context = contextMetrics
	result.Tools = summarizeMessages(messages)
	var toolObservationErr error
	if options.Approach == approachMCP {
		observeContext, cancelObserve := context.WithTimeout(context.Background(), 10*time.Second)
		result.ExecutedTools, toolObservationErr = setup.server.mcpObservedToolCalls(
			observeContext, prepared.projectRoot, setup.sessionID, false,
		)
		cancelObserve()
	}

	runErr = setup.cleanup(runErr)

	var traceErr error
	if options.Approach == approachMCP {
		calls, invalidLines, err := readMCPTrace(
			prepared.tracePath,
			setup.traceOffset,
		)
		if err != nil {
			traceErr = err
		} else {
			stats := summarizeMCPTrace(calls, setup.sessionID)
			stats.InvalidLines = invalidLines
			result.MCP = &stats
		}
	}

	evaluation, verifyOutput := verifyAndEvaluate(
		repositoryRoot,
		task,
		prepared,
		result,
		session,
		runErr,
		traceErr,
		toolObservationErr,
		logf,
	)
	result.AgentFailure = strings.Join(evaluation.AgentFailures, "; ")
	result.TaskFailure = strings.Join(evaluation.TaskFailures, "; ")

	var failures []string
	failures = append(failures, evaluation.AgentFailures...)
	failures = append(failures, evaluation.TaskFailures...)
	result.Success = len(failures) == 0
	result.Failure = strings.Join(failures, "; ")

	path, err := persistBenchmarkResult(
		resultPath,
		options.ResultsDir,
		runName,
		result,
		prepared,
		verifyOutput,
	)
	if err != nil {
		return "", err
	}
	if result.Success {
		logf("Benchmark succeeded")
	} else {
		logf("Benchmark failed: %s", result.Failure)
	}
	return path, nil
}

type preparedRun struct {
	projectRoot   string
	godotPath     string
	opencodePath  string
	environment   []string
	tracePath     string
	mcpConfigPath string
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

	tracePath := ""
	mcpConfigPath := ""
	if options.Approach == approachMCP {
		cliPath := filepath.Join(tempDir, "godot-cli")

		logf("Building godot-cli")

		if err := buildBinary(repositoryRoot, cliPath, "./cmd/godot-cli"); err != nil {
			return preparedRun{}, err
		}
		if err := addon.Install(projectRoot); err != nil {
			return preparedRun{}, fmt.Errorf("installing addon: %w", err)
		}
		if _, err := skill.Install(projectRoot); err != nil {
			return preparedRun{}, fmt.Errorf("installing OpenCode skill: %w", err)
		}
		if err := enableAddon(projectRoot); err != nil {
			return preparedRun{}, err
		}

		tracePath = filepath.Join(tempDir, "mcp-trace.jsonl")
		logf("Configuring isolated godot MCP server")

		mcpConfigPath, err = writeMCPConfig(projectRoot, cliPath, tracePath)
		if err != nil {
			return preparedRun{}, fmt.Errorf("configuring godot MCP server: %w", err)
		}
	}

	environment, err := isolatedBenchmarkEnvironment(tempDir, map[string]string{})
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
		tracePath:     tracePath,
		mcpConfigPath: mcpConfigPath,
	}, nil
}

// isolatedBenchmarkEnvironment removes inherited OpenCode config overrides
// before applying the benchmark config home. Provider credentials live in the
// data directory (or the environment) and are preserved, but a developer's
// global, explicit, or inline config cannot add unrelated MCP servers.
func isolatedBenchmarkEnvironment(
	tempDir string,
	values map[string]string,
) ([]string, error) {
	inherited := make([]string, 0, len(os.Environ()))
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		switch name {
		case "OPENCODE_CONFIG",
			"OPENCODE_CONFIG_CONTENT",
			"OPENCODE_CONFIG_DIR",
			"OPENCODE_CONFIG_PROJECT_DISABLE",
			"OPENCODE_DISABLE_PROJECT_CONFIG":
			continue
		default:
			inherited = append(inherited, entry)
		}
	}
	return isolatedOpenCodeEnvironment(inherited, tempDir, values)
}

// writeMCPConfig writes the fixture-local OpenCode V2 configuration. It
// replaces mcp.servers entirely so a fixture or ancestor cannot contribute an
// unrelated server, while leaving non-MCP fields from an existing project
// config intact. Code Mode stays enabled (the V2 default).
func writeMCPConfig(
	projectRoot string,
	binaryPath string,
	tracePath string,
) (string, error) {
	root, err := filepath.Abs(projectRoot)
	if err != nil {
		return "", err
	}
	binary, err := filepath.Abs(binaryPath)
	if err != nil {
		return "", err
	}
	trace, err := filepath.Abs(tracePath)
	if err != nil {
		return "", err
	}
	configPath := filepath.Join(root, "opencode.json")
	config := map[string]any{}

	if data, readErr := os.ReadFile(configPath); readErr == nil {
		var existing map[string]any
		if json.Unmarshal(data, &existing) == nil {
			config = existing
		}
	}
	if _, ok := config["$schema"]; !ok {
		config["$schema"] = "https://opencode.ai/config.json"
	}
	config["mcp"] = map[string]any{
		"servers": map[string]any{
			"godot": map[string]any{
				"type": "local",
				"command": []string{
					binary,
					"mcp",
					"--project",
					root,
					"--trace-file",
					trace,
				},
				"cwd":      root,
				"codemode": true,
			},
		},
	}

	// A project .opencode config has higher precedence than opencode.json and
	// could re-introduce servers. The fixture copy is disposable, so remove
	// these overrides rather than trying to merge them.
	for _, name := range []string{"opencode.json", "opencode.jsonc"} {
		_ = os.Remove(filepath.Join(root, ".opencode", name))
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(configPath, append(data, '\n'), 0644); err != nil {
		return "", err
	}
	return configPath, nil
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

type evaluationInput struct {
	Task               taskDefinition
	Result             benchmarkResult
	ProjectRoot        string
	ResponseOutcome    string
	Probe              probeResult
	RunErr             error
	VerifyErr          error
	MCPTraceErr        error
	ToolObservationErr error
}

type evaluation struct {
	AgentFailures []string
	TaskFailures  []string
}

func evaluateResult(input evaluationInput) evaluation {
	var result evaluation

	if input.RunErr != nil {
		result.AgentFailures = append(result.AgentFailures, input.RunErr.Error())
	}
	if input.ResponseOutcome != "" && input.ResponseOutcome != "succeeded" {
		result.AgentFailures = append(
			result.AgentFailures,
			"OpenCode session outcome: "+input.ResponseOutcome,
		)
	}
	if input.VerifyErr != nil {
		result.TaskFailures = append(result.TaskFailures, input.VerifyErr.Error())
	}
	result.TaskFailures = append(
		result.TaskFailures,
		verificationFailures(input.Task, input.Probe)...,
	)

	if input.Result.Approach == approachMCP {
		if input.ToolObservationErr != nil {
			result.TaskFailures = append(result.TaskFailures,
				"observing executed tools: "+input.ToolObservationErr.Error())
		}
		if input.MCPTraceErr != nil {
			result.TaskFailures = append(
				result.TaskFailures,
				"reading MCP trace: "+input.MCPTraceErr.Error(),
			)
		}
		if input.Result.MCP == nil || input.Result.MCP.Calls == 0 {
			result.TaskFailures = append(
				result.TaskFailures,
				"agent did not invoke godot-cli MCP tools",
			)
		}
		result.TaskFailures = append(
			result.TaskFailures,
			directMutationViolations(
				input.Result.Tools,
				input.Result.ExecutedTools,
				input.ToolObservationErr,
				input.ProjectRoot,
				input.Task.Semantic,
			)...,
		)
	}
	return result
}

func taskPrompt(task taskDefinition, approach string) string {
	if approach == approachDirect {
		return task.Goal + " Directly edit the Godot project files. " +
			"Do not use godot-cli, the godot MCP server, or any MCP tools."
	}
	return task.Goal + " Use the godot MCP tools exposed to this session for all " +
		"scene inspection and changes. Do not edit .tscn files directly. " +
		"Do not invoke the godot-cli command-line executable; MCP is the only " +
		"supported route for this run."
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

type runArtifacts struct {
	TracePath              string `json:"tracePath,omitempty"`
	MCPConfigPath          string `json:"mcpConfigPath,omitempty"`
	VerificationOutputPath string `json:"verificationOutputPath,omitempty"`
}

func persistBenchmarkResult(
	resultPath string,
	resultsDir string,
	runName string,
	result benchmarkResult,
	prepared preparedRun,
	verificationOutput string,
) (string, error) {
	// Setup diagnostics can include tails of client logs. Redact obvious
	// credential shapes before persisting failure text or verifier output.
	result.Failure = redactSecrets(result.Failure)
	result.SetupFailure = redactSecrets(result.SetupFailure)
	result.AgentFailure = redactSecrets(result.AgentFailure)
	result.TaskFailure = redactSecrets(result.TaskFailure)
	verificationOutput = redactSecrets(verificationOutput)

	result.Artifacts = collectArtifacts(
		resultsDir,
		runName,
		prepared,
		verificationOutput,
	)
	if err := writeJSON(resultPath, result); err != nil {
		return "", err
	}
	return resultPath, nil
}

var secretPatterns = []struct {
	pattern     *regexp.Regexp
	replacement string
}{
	{
		pattern:     regexp.MustCompile(`(?i)sk-[a-z0-9_-]{8,}`),
		replacement: "[redacted]",
	},
	{
		pattern:     regexp.MustCompile(`(?i)(bearer\s+)[a-z0-9._~+/=-]+`),
		replacement: "${1}[redacted]",
	},
	{
		pattern: regexp.MustCompile(
			`(?i)((?:api[_-]?key|access[_-]?token|client[_-]?secret|password)\s*[=:]\s*)("[^"]*"|'[^']*'|[^\s,;]+)`,
		),
		replacement: "${1}[redacted]",
	},
}

func redactSecrets(value string) string {
	for _, secret := range secretPatterns {
		value = secret.pattern.ReplaceAllString(value, secret.replacement)
	}
	return value
}

// collectArtifacts copies only benchmark-owned diagnostics into the results
// directory. Environment variables, OpenCode logs, and prompts are never
// persisted because they can contain credentials.
func collectArtifacts(
	resultsDir string,
	runName string,
	prepared preparedRun,
	verificationOutput string,
) *runArtifacts {
	if resultsDir == "" {
		return nil
	}
	artifacts := &runArtifacts{}

	if prepared.tracePath != "" {
		target := filepath.Join(resultsDir, runName+"-trace.jsonl")
		if copyFile(prepared.tracePath, target) == nil {
			artifacts.TracePath = target
		}
	}
	if prepared.mcpConfigPath != "" {
		target := filepath.Join(resultsDir, runName+"-mcp-config.json")
		if writeMCPConfigArtifact(prepared.mcpConfigPath, target) == nil {
			artifacts.MCPConfigPath = target
		}
	}
	if verificationOutput != "" {
		target := filepath.Join(resultsDir, runName+"-verify.txt")
		if writeArtifactText(target, verificationOutput) == nil {
			artifacts.VerificationOutputPath = target
		}
	}
	if artifacts.TracePath == "" &&
		artifacts.MCPConfigPath == "" &&
		artifacts.VerificationOutputPath == "" {
		return nil
	}
	return artifacts
}

func copyFile(source, destination string) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer func() { _ = input.Close() }()

	output, err := os.OpenFile(
		destination,
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
}

// writeMCPConfigArtifact keeps only the benchmark-owned MCP section. Provider
// blocks and any literal credentials a fixture config might contain never
// reach the results directory.
func writeMCPConfigArtifact(source, destination string) error {
	data, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	var config map[string]any
	if err := json.Unmarshal(data, &config); err != nil {
		return err
	}
	artifact := map[string]any{}
	if schema, ok := config["$schema"]; ok {
		artifact["$schema"] = schema
	}
	if mcp, ok := config["mcp"]; ok {
		artifact["mcp"] = mcp
	}
	data, err = json.MarshalIndent(artifact, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(destination, append(data, '\n'), 0644)
}

func writeArtifactText(path, text string) error {
	const maxArtifactBytes = 1 << 20
	if len(text) > maxArtifactBytes {
		text = text[:maxArtifactBytes] + "\n[truncated]\n"
	}
	return os.WriteFile(path, []byte(text), 0644)
}
