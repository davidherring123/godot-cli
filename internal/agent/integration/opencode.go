package integration

import (
	"fmt"
	"path/filepath"
	"slices"
)

func openCodeCandidates(projectRoot string) []string {
	return []string{
		filepath.Join(projectRoot, "opencode.json"),
		filepath.Join(projectRoot, "opencode.jsonc"),
		filepath.Join(projectRoot, ".opencode", "opencode.json"),
		filepath.Join(projectRoot, ".opencode", "opencode.jsonc"),
	}
}

func preferredOpenCodeConfig(projectRoot string) string {
	candidates := openCodeCandidates(projectRoot)
	preferred := candidates[0]

	for _, candidate := range candidates {
		if fileExists(candidate) {
			preferred = candidate
		}
	}

	return preferred
}

func openCodeServer(object map[string]any) ([]string, bool) {
	mcp, ok := object["mcp"].(map[string]any)
	if !ok {
		return nil, false
	}

	servers, ok := mcp["servers"].(map[string]any)
	if !ok {
		return nil, false
	}

	server, ok := servers[ServerName].(map[string]any)
	if !ok {
		return nil, false
	}

	return jsonCommand(server["command"]), true
}

func inspectOpenCode(projectRoot, hostPath string) (ClientStatus, error) {
	expected := expectedCommand(projectRoot, hostPath)
	candidates := openCodeCandidates(projectRoot)

	configPath := preferredOpenCodeConfig(projectRoot)
	configExists := fileExists(configPath)
	configured := false
	command := []string(nil)
	upToDate := false

	for index := len(candidates) - 1; index >= 0; index-- {
		path := candidates[index]

		if !fileExists(path) {
			continue
		}

		object, _, err := loadJSONObject(path)
		if err != nil {
			return ClientStatus{}, err
		}

		configuredCommand, found := openCodeServer(object)
		if !found {
			continue
		}

		mcp := object["mcp"].(map[string]any)
		servers := mcp["servers"].(map[string]any)
		server := servers[ServerName].(map[string]any)

		configPath = path
		configExists = true
		configured = true
		command = configuredCommand
		upToDate = jsonString(server["type"]) == "local" && slices.Equal(command, expected)
		break
	}

	skillPath, skillInstalled, skillCurrent, err := skillStatus(projectRoot, ClientOpenCode)
	if err != nil {
		return ClientStatus{}, err
	}

	return ClientStatus{
		Client:         ClientOpenCode,
		ConfigPath:     configPath,
		ConfigExists:   configExists,
		Configured:     configured,
		UpToDate:       configured && upToDate && skillCurrent,
		Command:        command,
		SkillPath:      skillPath,
		SkillInstalled: skillInstalled,
		SkillCurrent:   skillCurrent,
	}, nil
}

func installOpenCode(projectRoot, hostPath string) (string, bool, error) {
	expected := expectedCommand(projectRoot, hostPath)
	candidates := openCodeCandidates(projectRoot)

	targetPath := preferredOpenCodeConfig(projectRoot)
	targetObject := map[string]any{}
	targetExists := false

	for _, path := range candidates {
		if !fileExists(path) {
			continue
		}

		object, _, err := loadJSONObject(path)
		if err != nil {
			return "", false, err
		}

		if _, found := openCodeServer(object); found {
			targetPath = path
			targetObject = object
			targetExists = true
		}
	}

	if !targetExists && fileExists(targetPath) {
		object, _, err := loadJSONObject(targetPath)
		if err != nil {
			return "", false, err
		}

		targetObject = object
	}

	mcp, err := ensureObject(targetObject, "mcp", targetPath)
	if err != nil {
		return "", false, err
	}

	servers, err := ensureObject(mcp, "servers", targetPath)
	if err != nil {
		return "", false, err
	}

	server := map[string]any{}

	if existing, ok := servers[ServerName].(map[string]any); ok {
		for key, value := range existing {
			server[key] = value
		}
	} else if existing, exists := servers[ServerName]; exists && existing != nil {
		return "", false, fmt.Errorf("%q server %q must be a JSON object", targetPath, ServerName)
	}

	server["type"] = "local"
	server["command"] = expected

	delete(server, "url")
	delete(server, "headers")
	delete(server, "oauth")

	servers[ServerName] = server

	changed, err := saveJSONObject(targetPath, targetObject)
	if err != nil {
		return "", false, err
	}

	return targetPath, changed, nil
}
