package integration_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/davidherring123/godot-cli/internal/agent/integration"
	"github.com/davidherring123/godot-cli/internal/agent/skill"
	"github.com/davidherring123/godot-cli/internal/release"
)

func TestInstallAllMergesExistingConfiguration(t *testing.T) {
	root, hostPath := installAllFixture(t)

	report, err := integration.Install(
		integration.Options{ProjectRoot: root, HostPath: hostPath},
		integration.SupportedClients(),
	)
	if err != nil {
		t.Fatal(err)
	}

	if len(report.Clients) != 3 {
		t.Fatalf("installed %d clients, want 3", len(report.Clients))
	}

	expected := []string{hostPath, "connect", "--project", root}

	opencode := readJSON(t, filepath.Join(root, "opencode.json"))
	if value := opencode["model"]; value != "test/model" {
		t.Errorf("opencode model = %v, want test/model", value)
	}

	assertJSONServerCommand(t, opencode, expected)

	claude := readJSON(t, filepath.Join(root, ".mcp.json"))
	assertClaudeCommand(t, claude, expected)
	if _, ok := claude["mcpServers"].(map[string]any)["other"]; !ok {
		t.Error("claude config lost the unrelated server")
	}

	codexPath := filepath.Join(root, ".codex", "config.toml")
	codexContent := readFile(t, codexPath)
	for _, fragment := range []string{
		"# keep this comment",
		`model = "gpt-5"`,
		"[mcp_servers.other]",
		`command = "other-server"`,
		"[mcp_servers.godot]",
		`command = "` + hostPath + `"`,
	} {
		if !strings.Contains(codexContent, fragment) {
			t.Errorf("codex config is missing %q:\n%s", fragment, codexContent)
		}
	}

	for _, skillPath := range []string{
		skill.PortablePath(root),
		skill.ClaudePath(root),
	} {
		if content := readFile(t, skillPath); content != string(skill.Content()) {
			t.Errorf("skill %s does not match the canonical template", skillPath)
		}
	}

	status, err := integration.Status(integration.Options{ProjectRoot: root, HostPath: hostPath})
	if err != nil {
		t.Fatal(err)
	}

	for _, client := range status.Clients {
		if !client.Configured || !client.UpToDate {
			t.Errorf("%s status = %+v", client.Client, client)
		}

		if !client.SkillInstalled || !client.SkillCurrent {
			t.Errorf("%s skill status = installed:%t current:%t", client.Client, client.SkillInstalled, client.SkillCurrent)
		}
	}
}

func TestInstallAllIsIdempotent(t *testing.T) {
	root, hostPath := installAllFixture(t)
	options := integration.Options{ProjectRoot: root, HostPath: hostPath}

	if _, err := integration.Install(options, integration.SupportedClients()); err != nil {
		t.Fatal(err)
	}

	before := map[string][]byte{
		"opencode": []byte(readFile(t, filepath.Join(root, "opencode.json"))),
		"claude":   []byte(readFile(t, filepath.Join(root, ".mcp.json"))),
		"codex":    []byte(readFile(t, filepath.Join(root, ".codex", "config.toml"))),
	}

	second, err := integration.Install(options, integration.SupportedClients())
	if err != nil {
		t.Fatal(err)
	}

	for _, client := range second.Clients {
		if client.ConfigChanged || client.SkillChanged {
			t.Errorf("%s changed on the second install: %+v", client.Client, client)
		}
	}

	for name, path := range map[string]string{
		"opencode": filepath.Join(root, "opencode.json"),
		"claude":   filepath.Join(root, ".mcp.json"),
		"codex":    filepath.Join(root, ".codex", "config.toml"),
	} {
		if content := []byte(readFile(t, path)); !bytes.Equal(before[name], content) {
			t.Errorf("%s config was rewritten by the idempotent install", name)
		}
	}
}

func installAllFixture(t *testing.T) (string, string) {
	t.Helper()

	root := testProject(t)
	hostPath := filepath.Join(t.TempDir(), "godot-mcp-host")

	writeFile(t, filepath.Join(root, "opencode.json"), `{
  "$schema": "https://opencode.ai/config.json",
  "model": "test/model",
  "mcp": {
    "servers": {
      "other": {
        "type": "local",
        "command": ["other-server"]
      }
    }
  }
}
`)
	writeFile(t, filepath.Join(root, ".mcp.json"), `{
  "mcpServers": {
    "other": {
      "command": "other-server",
      "args": ["serve"]
    }
  }
}
`)
	writeFile(t, filepath.Join(root, ".codex", "config.toml"), `# keep this comment
model = "gpt-5"

[mcp_servers.other]
command = "other-server"
args = ["serve"]
`)

	return root, hostPath
}

func TestInstallOpenCodeJSONCPreservesUnrelatedConfig(t *testing.T) {
	root := testProject(t)
	hostPath := filepath.Join(t.TempDir(), "godot-mcp-host")
	configPath := filepath.Join(root, ".opencode", "opencode.jsonc")

	writeFile(t, configPath, `{
  // Keep unrelated user configuration.
  "$schema": "https://opencode.ai/config.json",
  "model": "test/model",
  "mcp": {
    "servers": {
      "other": {
        "type": "local",
        "command": ["other-server"],
      },
    },
  },
}
`)

	report, err := integration.Install(
		integration.Options{ProjectRoot: root, HostPath: hostPath},
		[]integration.Client{integration.ClientOpenCode},
	)
	if err != nil {
		t.Fatal(err)
	}

	if report.Clients[0].ConfigPath != configPath {
		t.Fatalf("config path = %q, want %q", report.Clients[0].ConfigPath, configPath)
	}

	if _, err := os.Stat(filepath.Join(root, "opencode.json")); !os.IsNotExist(err) {
		t.Errorf("install created a second OpenCode config: %v", err)
	}

	object := readJSON(t, configPath)
	if object["model"] != "test/model" {
		t.Errorf("model = %v, want test/model", object["model"])
	}

	assertJSONServerCommand(t, object, []string{hostPath, "connect", "--project", root})

	status, err := integration.Status(integration.Options{ProjectRoot: root, HostPath: hostPath})
	if err != nil {
		t.Fatal(err)
	}

	if !status.Clients[0].Configured || !status.Clients[0].UpToDate {
		t.Errorf("OpenCode status after JSONC install = %+v", status.Clients[0])
	}
}

func TestInstallReplacesRemoteEntryAndPreservesServerOptions(t *testing.T) {
	root := testProject(t)
	hostPath := filepath.Join(t.TempDir(), "godot-mcp-host")

	writeFile(t, filepath.Join(root, "opencode.json"), `{
  "mcp": {
    "servers": {
      "godot": {
        "type": "remote",
        "url": "https://example.invalid/mcp",
        "environment": {"OPENCODE_TEST": "1"}
      }
    }
  }
}
`)
	writeFile(t, filepath.Join(root, ".mcp.json"), `{
  "mcpServers": {
    "godot": {
      "type": "http",
      "url": "https://example.invalid/mcp",
      "env": {"CLAUDE_TEST": "1"},
      "timeout": 600000
    }
  }
}
`)
	writeFile(t, filepath.Join(root, ".codex", "config.toml"), `[mcp_servers.godot]
url = "https://example.invalid/mcp"

[mcp_servers.godot.env]
CODEX_TEST = "1"
`)

	if _, err := integration.Install(
		integration.Options{ProjectRoot: root, HostPath: hostPath},
		integration.SupportedClients(),
	); err != nil {
		t.Fatal(err)
	}

	opencode := readJSON(t, filepath.Join(root, "opencode.json"))
	opencodeServer := opencode["mcp"].(map[string]any)["servers"].(map[string]any)["godot"].(map[string]any)
	assertNoJSONKey(t, opencodeServer, "url")
	if environment := opencodeServer["environment"].(map[string]any); environment["OPENCODE_TEST"] != "1" {
		t.Errorf("OpenCode server options were not preserved: %v", opencodeServer)
	}

	claude := readJSON(t, filepath.Join(root, ".mcp.json"))
	claudeServer := claude["mcpServers"].(map[string]any)["godot"].(map[string]any)
	assertNoJSONKey(t, claudeServer, "url")
	if environment := claudeServer["env"].(map[string]any); environment["CLAUDE_TEST"] != "1" {
		t.Errorf("Claude server options were not preserved: %v", claudeServer)
	}
	if claudeServer["timeout"] != float64(600000) {
		t.Errorf("Claude timeout = %v, want 600000", claudeServer["timeout"])
	}

	codexContent := readFile(t, filepath.Join(root, ".codex", "config.toml"))
	if strings.Contains(codexContent, "example.invalid") {
		t.Errorf("Codex remote URL was not removed:\n%s", codexContent)
	}
	if !strings.Contains(codexContent, "[mcp_servers.godot.env]") ||
		!strings.Contains(codexContent, `CODEX_TEST = "1"`) {
		t.Errorf("Codex server options were not preserved:\n%s", codexContent)
	}
	if !strings.Contains(codexContent, `command = "`+hostPath+`"`) {
		t.Errorf("Codex command was not installed:\n%s", codexContent)
	}
}

func TestStatusReportsMissingIntegrations(t *testing.T) {
	root := testProject(t)
	hostPath := filepath.Join(t.TempDir(), "godot-mcp-host")

	status, err := integration.Status(integration.Options{ProjectRoot: root, HostPath: hostPath})
	if err != nil {
		t.Fatal(err)
	}

	if status.ProjectRoot != root || status.HostPath != hostPath {
		t.Errorf("status roots = %q, %q", status.ProjectRoot, status.HostPath)
	}

	for _, client := range status.Clients {
		if client.ConfigExists || client.Configured || client.UpToDate {
			t.Errorf("%s unexpectedly reports an integration: %+v", client.Client, client)
		}

		if client.SkillInstalled || client.SkillCurrent {
			t.Errorf("%s unexpectedly reports a skill: %+v", client.Client, client)
		}
	}
}

func TestResolveHostPathPrefersBundledHost(t *testing.T) {
	root := testProject(t)
	target := release.Target{OS: runtime.GOOS, Arch: runtime.GOARCH}
	bundled := filepath.Join(
		root,
		"addons",
		"godot_mcp",
		"bin",
		target.BundleString(),
		target.HostBinaryName(),
	)

	writeFile(t, bundled, "fake host")

	hostPath, err := integration.ResolveHostPath(root)
	if err != nil {
		t.Fatal(err)
	}

	if hostPath != bundled {
		t.Fatalf("host path = %q, want %q", hostPath, bundled)
	}
}

func TestParseClients(t *testing.T) {
	all, err := integration.ParseClients("all")
	if err != nil {
		t.Fatal(err)
	}

	if len(all) != 3 {
		t.Fatalf("all parsed to %d clients, want 3", len(all))
	}

	one, err := integration.ParseClients("Claude")
	if err != nil {
		t.Fatal(err)
	}

	if len(one) != 1 || one[0] != integration.ClientClaude {
		t.Fatalf("Claude parsed to %v", one)
	}

	if _, err := integration.ParseClients("unknown"); err == nil {
		t.Fatal("unknown client was accepted")
	}
}

func testProject(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	writeFile(t, filepath.Join(root, "project.godot"), "[application]\n")

	return root
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	return string(data)
}

func readJSON(t *testing.T, path string) map[string]any {
	t.Helper()

	var object map[string]any
	if err := json.Unmarshal([]byte(readFile(t, path)), &object); err != nil {
		t.Fatalf("parsing %s: %v", path, err)
	}

	return object
}

func assertJSONServerCommand(t *testing.T, object map[string]any, expected []string) {
	t.Helper()

	mcp := object["mcp"].(map[string]any)
	servers := mcp["servers"].(map[string]any)
	server := servers["godot"].(map[string]any)

	if server["type"] != "local" {
		t.Errorf("OpenCode server type = %v, want local", server["type"])
	}

	command := stringSlice(t, server["command"])
	if !slicesEqual(command, expected) {
		t.Errorf("OpenCode command = %v, want %v", command, expected)
	}

	if _, ok := servers["other"]; !ok {
		t.Error("OpenCode config lost the unrelated server")
	}
}

func assertClaudeCommand(t *testing.T, object map[string]any, expected []string) {
	t.Helper()

	servers := object["mcpServers"].(map[string]any)
	server := servers["godot"].(map[string]any)
	command := append([]string{server["command"].(string)}, stringSlice(t, server["args"])...)

	if !slicesEqual(command, expected) {
		t.Errorf("Claude command = %v, want %v", command, expected)
	}
}

func stringSlice(t *testing.T, value any) []string {
	t.Helper()

	items, ok := value.([]any)
	if !ok {
		t.Fatalf("value is not an array: %T", value)
	}

	result := make([]string, 0, len(items))

	for _, item := range items {
		text, ok := item.(string)
		if !ok {
			t.Fatalf("array item is not a string: %T", item)
		}

		result = append(result, text)
	}

	return result
}

func slicesEqual(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}

	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}

	return true
}

func assertNoJSONKey(t *testing.T, object map[string]any, key string) {
	t.Helper()

	if _, ok := object[key]; ok {
		t.Errorf("unexpected %q field: %v", key, object)
	}
}
