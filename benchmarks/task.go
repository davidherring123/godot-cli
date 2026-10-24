package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

type taskDefinition struct {
	ID           string       `json:"id"`
	Goal         string       `json:"goal"`
	InitialScene string       `json:"initialScene"`
	Semantic     bool         `json:"semantic"`
	Verify       verification `json:"verify"`
}

// UnmarshalJSON requires every task to state whether direct serialization
// edits are forbidden. Keeping this explicit avoids silently disabling the
// MCP policy when a task is added or copied.
func (task *taskDefinition) UnmarshalJSON(data []byte) error {
	type plainTask taskDefinition

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	if _, ok := fields["semantic"]; !ok {
		return errors.New("benchmark task must declare whether it is semantic")
	}

	var decoded plainTask
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*task = taskDefinition(decoded)
	return nil
}

type verification struct {
	Scene  string        `json:"scene"`
	Checks []verifyCheck `json:"checks"`
}

// verifyCheck is a declarative structural check. The generic verifier in
// verify_scene.gd observes the saved project and reports whether the check
// passes. Kind selects the check; every requirement and threshold lives in
// this JSON rather than in verifier code.
type verifyCheck struct {
	Kind         string   `json:"kind"`
	Types        []string `json:"types,omitempty"`
	Actions      []string `json:"actions,omitempty"`
	TextContains []string `json:"textContains,omitempty"`
	TextAny      []string `json:"textAny,omitempty"`
	Min          int      `json:"min,omitempty"`
	Collision    bool     `json:"collision,omitempty"`
	Script       bool     `json:"script,omitempty"`
	Visible      bool     `json:"visible,omitempty"`
	Enabled      bool     `json:"enabled,omitempty"`
	Events       bool     `json:"events,omitempty"`
	Loadable     bool     `json:"loadable,omitempty"`
	Container    string   `json:"container,omitempty"`
	Scene        string   `json:"scene,omitempty"`
}

// UnmarshalJSON rejects unknown check fields so a misspelled requirement
// cannot silently weaken a benchmark.
func (check *verifyCheck) UnmarshalJSON(data []byte) error {
	type plainCheck verifyCheck

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()

	var decoded plainCheck
	if err := decoder.Decode(&decoded); err != nil {
		return err
	}
	*check = verifyCheck(decoded)
	return nil
}

func loadTask(repositoryRoot, name string) (taskDefinition, error) {
	path := filepath.Join(
		repositoryRoot,
		"benchmarks",
		"tasks",
		name+".json",
	)

	data, err := os.ReadFile(path)
	if err != nil {
		return taskDefinition{}, err
	}
	var task taskDefinition

	if err := json.Unmarshal(data, &task); err != nil {
		return taskDefinition{}, err
	}
	if task.ID == "" || task.Goal == "" || task.InitialScene == "" {
		return taskDefinition{}, errors.New("benchmark task is incomplete")
	}
	if task.Verify.Scene == "" {
		return taskDefinition{}, errors.New("benchmark task is missing a verification scene")
	}
	if len(task.Verify.Checks) == 0 {
		return taskDefinition{}, errors.New("benchmark task has no verification checks")
	}
	for _, check := range task.Verify.Checks {
		if check.Kind == "" {
			return taskDefinition{}, errors.New(
				"benchmark task has a verification check without a kind",
			)
		}
	}
	return task, nil
}
