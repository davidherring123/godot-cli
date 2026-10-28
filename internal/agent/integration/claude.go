package integration

import (
	"fmt"
	"path/filepath"
	"slices"
)

func claudeConfigPath(projectRoot string) string {
	return filepath.Join(projectRoot, ".mcp.json")
}

func claudeServer(object map[string]any) ([]string, bool) {
	servers, ok := object["mcpServers"].(map[string]any)
	if !ok {
		return nil, false
	}

	server, ok := servers[ServerName].(map[string]any)
	if !ok {
		return nil, false
	}

	return jsonCommand(server["args"]), true
}

func inspectClaude(projectRoot, hostPath string) (ClientStatus, error) {
	configPath := claudeConfigPath(projectRoot)
	expected := expectedCommand(projectRoot, hostPath)
	status := ClientStatus{
		Client:     ClientClaude,
		ConfigPath: configPath,
	}

	object, exists, err := loadJSONObject(configPath)
	if err != nil {
		return ClientStatus{}, err
	}

	status.ConfigExists = exists

	if exists {
		args, found := claudeServer(object)
		if found {
			servers := object["mcpServers"].(map[string]any)
			server := servers[ServerName].(map[string]any)
			transport := jsonString(server["type"])
			command := append([]string{jsonString(server["command"])}, args...)

			status.Configured = true
			status.Command = command
			status.UpToDate = (transport == "" || transport == "stdio") &&
				slices.Equal(command, expected)
		}
	}

	skillPath, skillInstalled, skillCurrent, err := skillStatus(projectRoot, ClientClaude)
	if err != nil {
		return ClientStatus{}, err
	}

	status.SkillPath = skillPath
	status.SkillInstalled = skillInstalled
	status.SkillCurrent = skillCurrent
	status.UpToDate = status.Configured && status.UpToDate && skillCurrent

	return status, nil
}

func installClaude(projectRoot, hostPath string) (string, bool, error) {
	configPath := claudeConfigPath(projectRoot)
	expected := expectedCommand(projectRoot, hostPath)

	object, _, err := loadJSONObject(configPath)
	if err != nil {
		return "", false, err
	}

	servers, err := ensureObject(object, "mcpServers", configPath)
	if err != nil {
		return "", false, err
	}

	server := map[string]any{}

	if existing, ok := servers[ServerName].(map[string]any); ok {
		for key, value := range existing {
			server[key] = value
		}
	} else if existing, exists := servers[ServerName]; exists && existing != nil {
		return "", false, fmt.Errorf("%q server %q must be a JSON object", configPath, ServerName)
	}

	server["type"] = "stdio"
	server["command"] = hostPath
	server["args"] = expected[1:]

	delete(server, "url")
	delete(server, "headers")
	delete(server, "oauth")
	delete(server, "headersHelper")

	servers[ServerName] = server

	changed, err := saveJSONObject(configPath, object)
	if err != nil {
		return "", false, err
	}

	return configPath, changed, nil
}
