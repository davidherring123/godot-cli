package addon_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/davidherring123/godot-cli/internal/addon"
)

const mcpPanelTestScript = `extends SceneTree

const SupervisorScript = preload("res://addons/godot_cli/mcp/backend_supervisor.gd")
const PanelScript = preload("res://addons/godot_cli/ui/mcp_panel.gd")

var failures: Array[String] = []

func _initialize() -> void:
    var supervisor = SupervisorScript.new()

    _check(not supervisor.is_bundled(), "empty project unexpectedly found a backend")
    _check(
        supervisor.status == SupervisorScript.Status.NOT_BUNDLED,
        "missing backend status was %d" % supervisor.status
    )
    _check(
        SupervisorScript.bundle_directory().begins_with("res://addons/godot_cli/bin/"),
        "bundle directory is not under the addon"
    )
    _check(
        SupervisorScript.executable_relative_path().get_file().begins_with("godot-mcp-host"),
        "backend executable name is wrong"
    )

    var panel = PanelScript.new(supervisor)
    panel.set_bridge_status(true, 49152)

    var version := panel.find_child("AddonVersionValue", true, false) as Label
    _check(version != null, "addon version label is missing")

    if version != null:
        _check(
            not version.text.is_empty() and version.text != "unknown",
            "addon version is missing: %s" % version.text
        )

    var bridge := panel.find_child("BridgeStatusValue", true, false) as Label
    _check(bridge != null, "bridge status label is missing")

    if bridge != null:
        _check(bridge.text.contains("49152"), "bridge port is missing: %s" % bridge.text)

    var status := panel.find_child("BackendStatusValue", true, false) as Label
    _check(status != null, "backend status label is missing")

    if status != null:
        _check(
            status.text.to_lower().contains("not included"),
            "missing backend status is not clear: %s" % status.text
        )

    var start := panel.find_child("StartButton", true, false) as Button
    var restart := panel.find_child("RestartButton", true, false) as Button
    var stop := panel.find_child("StopButton", true, false) as Button
    var copy := panel.find_child("CopyEndpointButton", true, false) as Button
    var endpoint := panel.find_child("EndpointValue", true, false) as LineEdit

    _check(start != null and start.disabled, "start should be disabled without a backend")
    _check(restart != null and restart.disabled, "restart should be disabled without a backend")
    _check(stop != null and stop.disabled, "stop should be disabled without a process")
    _check(copy != null and copy.disabled, "copy should be disabled without an endpoint")
    _check(endpoint != null, "endpoint field is missing")

    supervisor._apply_state({
        "status": "running",
        "endpoint": "http://127.0.0.1:4567/mcp",
        "pid": 123,
    })

    _check(
        supervisor.status == SupervisorScript.Status.RUNNING,
        "running state was not applied"
    )
    _check(
        supervisor.endpoint == "http://127.0.0.1:4567/mcp",
        "state endpoint is wrong: %s" % supervisor.endpoint
    )

    supervisor._apply_state({"status": "running", "port": 4568})
    _check(
        supervisor.endpoint == "http://127.0.0.1:4568",
        "port fallback endpoint is wrong: %s" % supervisor.endpoint
    )

    supervisor._apply_state({"status": "error", "error": "bind failed"})
    _check(
        supervisor.status == SupervisorScript.Status.ERROR,
        "error state was not applied"
    )
    _check(supervisor.last_error == "bind failed", "backend error was not retained")

    panel.free()

    if failures.is_empty():
        print("MCP_PANEL_OK")
        quit(0)
        return

    for failure in failures:
        print("MCP_PANEL_FAILURE: %s" % failure)

    quit(1)

func _check(condition: bool, message: String) -> void:
    if not condition:
        failures.append(message)
`

const mcpBackendTestScript = `extends SceneTree

const SupervisorScript = preload("res://addons/godot_cli/mcp/backend_supervisor.gd")

var failures: Array[String] = []

func _initialize() -> void:
    var supervisor = SupervisorScript.new()

    _check(supervisor.is_bundled(), "bundled backend was not found")
    _check(
        supervisor.status == SupervisorScript.Status.STOPPED,
        "fresh supervisor is not stopped"
    )
    _check(supervisor.start(), "start returned false")

    var pid := supervisor.get_managed_pid()
    _check(pid > 0, "start did not record a PID")

    for _attempt in range(30):
        OS.delay_msec(100)
        supervisor.poll(0.5)

        if supervisor.status == SupervisorScript.Status.RUNNING:
            break

    _check(
        supervisor.status == SupervisorScript.Status.RUNNING,
        "backend did not reach running: status=%d error=%s" % [
            supervisor.status,
            supervisor.last_error,
        ]
    )
    _check(
        supervisor.endpoint == "http://127.0.0.1:54321/mcp",
        "backend endpoint is wrong: %s" % supervisor.endpoint
    )
    _check(supervisor.get_managed_pid() == pid, "managed PID changed")
    _check(
        supervisor.get_state_path().get_file().begins_with("backend-"),
        "state file is not scoped to the editor process: %s" % supervisor.get_state_path()
    )
    _check(
        FileAccess.file_exists(supervisor.get_state_path()),
        "state file was not created"
    )

    supervisor.stop()
    OS.delay_msec(200)

    _check(not supervisor.has_process(), "stop retained the managed PID")
    _check(not OS.is_process_running(pid), "stop left the backend process running")
    _check(
        supervisor.status == SupervisorScript.Status.STOPPED,
        "status after stop was %d" % supervisor.status
    )
    _check(
        not FileAccess.file_exists(supervisor.get_state_path()),
        "stop left the state file behind"
    )

    if supervisor.get_managed_pid() > 0:
        supervisor.shutdown()

    if failures.is_empty():
        print("MCP_BACKEND_OK")
        quit(0)
        return

    for failure in failures:
        print("MCP_BACKEND_FAILURE: %s" % failure)

    quit(1)

func _check(condition: bool, message: String) -> void:
    if not condition:
        failures.append(message)
`

func TestMCPPanelReportsMissingBackend(t *testing.T) {
	godot := findGodot(t)
	root := writeMCPProject(t)

	writeScript(t, root, "mcp_panel_test.gd", mcpPanelTestScript)
	runMCPTestScript(t, godot, root, "res://mcp_panel_test.gd", "MCP_PANEL_OK")
}

func TestMCPBackendSupervisorManagesBundledHost(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fake backend uses a POSIX shell script")
	}

	godot := findGodot(t)
	root := writeMCPProject(t)

	bundleDirectory := filepath.Join(
		addon.Path(root),
		"bin",
		mcpBundlePlatformDirectory(),
	)

	if err := os.MkdirAll(bundleDirectory, 0755); err != nil {
		t.Fatal(err)
	}

	fakeBackend := `#!/bin/sh
state=""
while [ "$#" -gt 0 ]; do
    case "$1" in
        --state-file)
            state="$2"
            shift 2
            ;;
        *)
            shift
            ;;
    esac
done
mkdir -p "$(dirname "$state")"
printf '{"pid":%s,"status":"running","endpoint":"http://127.0.0.1:54321/mcp","port":54321}\n' "$$" > "$state"
exec sleep 30
`

	backendPath := filepath.Join(bundleDirectory, "godot-mcp-host")
	if err := os.WriteFile(backendPath, []byte(fakeBackend), 0755); err != nil {
		t.Fatal(err)
	}

	writeScript(t, root, "mcp_backend_test.gd", mcpBackendTestScript)
	runMCPTestScript(t, godot, root, "res://mcp_backend_test.gd", "MCP_BACKEND_OK")
}

func TestPluginLoadsMCPPanelWithoutBundledBackend(t *testing.T) {
	godot := findGodot(t)
	root := writeMCPProject(t)

	if _, err := addon.Enable(root); err != nil {
		t.Fatalf("enabling addon: %v", err)
	}

	command := exec.Command(
		godot,
		"--headless",
		"--editor",
		"--path",
		root,
		"--quit-after",
		"120",
	)

	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("headless editor failed: %v\n%s", err, output)
	}

	text := string(output)

	if !strings.Contains(text, "godot-cli: bridge listening on") {
		t.Fatalf("plugin did not start the bridge:\n%s", text)
	}

	if strings.Contains(text, "SCRIPT ERROR") {
		t.Fatalf("plugin reported a script error:\n%s", text)
	}
}

func writeMCPProject(t *testing.T) string {
	t.Helper()

	root := t.TempDir()

	if err := os.WriteFile(
		filepath.Join(root, "project.godot"),
		[]byte("config_version=5\n\n[application]\n\nconfig/name=\"MCPTest\"\n"),
		0644,
	); err != nil {
		t.Fatal(err)
	}

	if err := addon.Install(root); err != nil {
		t.Fatalf("installing addon: %v", err)
	}

	return root
}

func writeScript(t *testing.T, root, name, content string) {
	t.Helper()

	if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func runMCPTestScript(t *testing.T, godot, root, script, marker string) {
	t.Helper()

	command := exec.Command(
		godot,
		"--headless",
		"--path",
		root,
		"--script",
		script,
	)

	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("%s failed: %v\n%s", script, err, output)
	}

	if !strings.Contains(string(output), marker) {
		t.Fatalf("%s did not report success:\n%s", script, output)
	}
}

func mcpBundlePlatformDirectory() string {
	platform := runtime.GOOS

	if platform == "darwin" {
		platform = "macos"
	}

	architecture := runtime.GOARCH

	switch architecture {
	case "amd64":
		architecture = "x86_64"
	case "386":
		architecture = "x86_32"
	case "arm":
		architecture = "arm32"
	}

	return platform + "-" + architecture
}
