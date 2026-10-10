package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unicode"
)

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
	return task, nil
}

func enableAddon(projectRoot string) error {
	path := filepath.Join(projectRoot, "project.godot")
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.WriteString(
		"\n[editor_plugins]\n\n" +
			"enabled=PackedStringArray(\"res://addons/godot_cli/plugin.cfg\")\n",
	)

	return err
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

func findExecutable(candidates ...string) (string, error) {
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		path, err := exec.LookPath(candidate)
		if err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("could not find executable from %v", candidates)
}

func buildBinary(repositoryRoot, output, packagePath string) error {
	command := exec.Command("go", "build", "-o", output, packagePath)
	command.Dir = repositoryRoot

	if output, err := command.CombinedOutput(); err != nil {
		return fmt.Errorf("building %s: %w\n%s", packagePath, err, output)
	}
	return nil
}

func commandVersion(path string) string {
	command := exec.Command(path, "--version")

	output, err := command.CombinedOutput()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

func writeJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0644)
}

func sanitizePath(value string) string {
	value = strings.Map(func(character rune) rune {
		if unicode.IsLetter(character) || unicode.IsDigit(character) {
			return character
		}
		return '-'
	}, value)

	return strings.Trim(value, "-")
}

func withEnvironment(base []string, values map[string]string) []string {
	environment := make([]string, 0, len(base)+len(values))

	for _, entry := range base {
		name, _, _ := strings.Cut(entry, "=")

		if _, replaced := values[name]; !replaced {
			environment = append(environment, entry)
		}
	}
	for name, value := range values {
		environment = append(environment, name+"="+value)
	}
	return environment
}

func isolatedOpenCodeEnvironment(
	base []string,
	tempDir string,
	values map[string]string,
) ([]string, error) {
	configHome := filepath.Join(tempDir, "xdg-config")
	configDir := filepath.Join(configHome, "opencode")

	if err := os.MkdirAll(configDir, 0755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(
		filepath.Join(configDir, "opencode.json"),
		[]byte("{}\n"),
		0644,
	); err != nil {
		return nil, err
	}
	values["XDG_CONFIG_HOME"] = configHome

	return withEnvironment(base, values), nil
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
				return fmt.Errorf("fixture contains symbolic link %s", relative)
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
