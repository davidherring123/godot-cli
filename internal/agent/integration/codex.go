package integration

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

const codexServerPath = "mcp_servers." + ServerName

func codexConfigPath(projectRoot string) string {
	return filepath.Join(projectRoot, ".codex", "config.toml")
}

func codexCommand(data []byte) ([]string, bool) {
	lines, _ := splitTOMLines(data)

	for _, block := range parseTOMLBlocks(lines) {
		if block.path != codexServerPath || block.array {
			continue
		}

		command := ""
		var arguments []string

		for _, assignment := range tomlAssignments(lines, block.start+1, block.end) {
			switch assignment.key {
			case "command":
				command, _ = parseTOMLString(assignment.value)
			case "args":
				arguments, _ = parseTOMLStringArray(assignment.value)
			}
		}

		fullCommand := make([]string, 0, len(arguments)+1)
		fullCommand = append(fullCommand, command)
		fullCommand = append(fullCommand, arguments...)

		return fullCommand, true
	}

	return nil, false
}

func inspectCodex(projectRoot, hostPath string) (ClientStatus, error) {
	configPath := codexConfigPath(projectRoot)
	expected := expectedCommand(projectRoot, hostPath)
	status := ClientStatus{
		Client:     ClientCodex,
		ConfigPath: configPath,
	}

	data, err := os.ReadFile(configPath)
	if err == nil {
		status.ConfigExists = true

		command, found := codexCommand(data)
		if found {
			status.Configured = true
			status.Command = command
			status.UpToDate = slices.Equal(command, expected)
		}
	} else if !os.IsNotExist(err) {
		return ClientStatus{}, fmt.Errorf("reading %q: %w", configPath, err)
	}

	skillPath, skillInstalled, skillCurrent, err := skillStatus(projectRoot, ClientCodex)
	if err != nil {
		return ClientStatus{}, err
	}

	status.SkillPath = skillPath
	status.SkillInstalled = skillInstalled
	status.SkillCurrent = skillCurrent
	status.UpToDate = status.Configured && status.UpToDate && skillCurrent

	return status, nil
}

func installCodex(projectRoot, hostPath string) (string, bool, error) {
	configPath := codexConfigPath(projectRoot)
	expected := expectedCommand(projectRoot, hostPath)

	existing, err := os.ReadFile(configPath)
	if err != nil && !os.IsNotExist(err) {
		return "", false, fmt.Errorf("reading %q: %w", configPath, err)
	}

	updated, err := upsertCodexServer(existing, expected)
	if err != nil {
		return "", false, fmt.Errorf("merging %q: %w", configPath, err)
	}

	changed, err := writeFileIfChanged(configPath, updated)
	if err != nil {
		return "", false, err
	}

	return configPath, changed, nil
}

func upsertCodexServer(existing []byte, expected []string) ([]byte, error) {
	lines, newline := splitTOMLines(existing)
	blocks := parseTOMLBlocks(lines)

	baseIndex := -1

	for index, block := range blocks {
		if block.path != codexServerPath {
			continue
		}

		if block.array {
			return nil, fmt.Errorf("%s must be a table, not an array of tables", codexServerPath)
		}

		if baseIndex >= 0 {
			return nil, fmt.Errorf("%s is defined more than once", codexServerPath)
		}

		baseIndex = index
	}

	if err := rejectInlineCodexServer(blocks, lines); err != nil {
		return nil, err
	}

	commandLine := "command = " + tomlQuote(expected[0])
	argsLine := "args = " + tomlStringArray(expected[1:])

	var result []string

	if baseIndex >= 0 {
		block := blocks[baseIndex]
		assignments := tomlAssignments(lines, block.start+1, block.end)
		remove := make(map[int]bool)

		for _, assignment := range assignments {
			if managedCodexKey(assignment.key) {
				for index := assignment.start; index < assignment.end; index++ {
					remove[index] = true
				}
			}
		}

		result = make([]string, 0, len(lines)+2)
		result = append(result, lines[:block.start+1]...)
		result = append(result, commandLine, argsLine)

		for index := block.start + 1; index < block.end; index++ {
			if !remove[index] {
				result = append(result, lines[index])
			}
		}

		result = append(result, lines[block.end:]...)
	} else {
		result = append(make([]string, 0, len(lines)+5), lines...)

		for len(result) > 0 && strings.TrimSpace(result[len(result)-1]) == "" {
			result = result[:len(result)-1]
		}

		if len(result) > 0 {
			result = append(result, "")
		}

		result = append(result, "["+codexServerPath+"]", commandLine, argsLine)
	}

	for len(result) > 0 && strings.TrimSpace(result[len(result)-1]) == "" {
		result = result[:len(result)-1]
	}

	return []byte(strings.Join(result, newline) + newline), nil
}

func rejectInlineCodexServer(blocks []tomlBlock, lines []string) error {
	for _, block := range blocks {
		if block.path == codexServerPath || strings.HasPrefix(block.path, codexServerPath+".") {
			continue
		}

		for _, assignment := range tomlAssignments(lines, block.start+1, block.end) {
			keyPath := assignment.key
			if block.path != "" {
				keyPath = block.path + "." + assignment.key
			}

			if keyPath == "mcp_servers" && strings.HasPrefix(strings.TrimSpace(assignment.value), "{") {
				return fmt.Errorf(
					"inline mcp_servers table is not supported; use %s table syntax",
					codexServerPath,
				)
			}

			if keyPath == codexServerPath || strings.HasPrefix(keyPath, codexServerPath+".") {
				return fmt.Errorf(
					"%s is defined with inline or dotted keys; use table syntax",
					codexServerPath,
				)
			}
		}
	}

	return nil
}

func managedCodexKey(key string) bool {
	switch key {
	case "command", "args", "url", "auth", "bearer_token_env_var",
		"http_headers", "env_http_headers", "http_headers_helper":
		return true
	default:
		return false
	}
}
