package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"sort"
	"strings"
)

const verifierPrefix = "GODOT_CLI_BENCHMARK_RESULT:"

type verifySpec struct {
	Node       string   `json:"node"`
	Absent     bool     `json:"absent"`
	Properties []string `json:"properties"`
}

type probeResult struct {
	Loaded bool         `json:"loaded"`
	Error  string       `json:"error,omitempty"`
	Checks []probeCheck `json:"checks"`
}

type probeCheck struct {
	Node       string         `json:"node"`
	Found      bool           `json:"found"`
	Properties map[string]any `json:"properties,omitempty"`
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
