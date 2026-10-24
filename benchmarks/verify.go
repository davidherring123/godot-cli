package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const verifierPrefix = "GODOT_CLI_BENCHMARK_RESULT:"

type probeResult struct {
	Loaded bool         `json:"loaded"`
	Error  string       `json:"error,omitempty"`
	Checks []probeCheck `json:"checks"`
}

type probeCheck struct {
	Kind   string `json:"kind"`
	Passed bool   `json:"passed"`
	Detail string `json:"detail,omitempty"`
}

func verifySavedScene(
	ctx context.Context,
	godotPath string,
	projectRoot string,
	verifierPath string,
	verification verification,
) (probeResult, string, error) {
	encodedChecks, err := json.Marshal(verification.Checks)
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
		string(encodedChecks),
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
			return result, text, fmt.Errorf("godot verifier failed: %w", err)
		}
		return result, text, nil
	}
	if err != nil {
		return probeResult{}, text, fmt.Errorf(
			"godot verifier failed: %w",
			err,
		)
	}
	return probeResult{}, text, errors.New(
		"godot verifier did not produce a result",
	)
}

// verifyAndEvaluate runs the saved-scene verifier and turns its report, along
// with the agent and MCP outcomes, into a pass/fail evaluation.
func verifyAndEvaluate(
	repositoryRoot string,
	task taskDefinition,
	prepared preparedRun,
	result benchmarkResult,
	session sessionInfo,
	runErr error,
	traceErr error,
	toolObservationErr error,
	logf func(format string, args ...any),
) (evaluation, probeResult, string) {
	logf("Verifying persisted project state")

	verifyContext, cancelVerify := context.WithTimeout(
		context.Background(),
		30*time.Second,
	)
	defer cancelVerify()

	verifierPath := filepath.Join(repositoryRoot, "benchmarks", "verify_scene.gd")

	probe, verifyOutput, verifyErr := verifySavedScene(
		verifyContext,
		prepared.godotPath,
		prepared.projectRoot,
		verifierPath,
		task.Verify,
	)
	if verifyErr != nil || !probe.Loaded {
		logf("Godot verifier output:\n%s", verifyOutput)
	}

	return evaluateResult(evaluationInput{
		Task:               task,
		Result:             result,
		ProjectRoot:        prepared.projectRoot,
		ResponseOutcome:    session.Outcome,
		Probe:              probe,
		RunErr:             runErr,
		VerifyErr:          verifyErr,
		MCPTraceErr:        traceErr,
		ToolObservationErr: toolObservationErr,
	}), probe, verifyOutput
}

// verificationFailures converts a probe report into task failures, keeping
// checks aligned by position so a missing or reordered result is an error
// rather than a silent pass.
func verificationFailures(
	task taskDefinition,
	probe probeResult,
) []string {
	if !probe.Loaded {
		if probe.Error != "" {
			return []string{"scene did not load: " + probe.Error}
		}
		return []string{"scene did not load"}
	}

	var failures []string
	for index, check := range task.Verify.Checks {
		if index >= len(probe.Checks) {
			failures = append(
				failures,
				"verification did not report check: "+check.Kind,
			)
			continue
		}

		reported := probe.Checks[index]
		if reported.Kind != check.Kind {
			failures = append(failures, fmt.Sprintf(
				"verification reported %s where %s was expected",
				reported.Kind,
				check.Kind,
			))
			continue
		}
		if reported.Passed {
			continue
		}
		if reported.Detail != "" {
			failures = append(failures, reported.Detail)
		} else {
			failures = append(failures, "verification check failed: "+check.Kind)
		}
	}
	return failures
}
