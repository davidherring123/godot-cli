package addon_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/davidherring123/godot-cli/internal/addon"
)

const mcpPanelTestScript = `extends SceneTree

const SupervisorScript = preload("res://addons/godot_mcp/mcp/backend_supervisor.gd")
const PanelScript = preload("res://addons/godot_mcp/ui/mcp_panel.gd")

var failures: Array[String] = []

func _initialize() -> void:
    var supervisor = SupervisorScript.new()

    _check(not supervisor.is_bundled(), "empty project unexpectedly found a backend")
    _check(
        supervisor.status == SupervisorScript.Status.NOT_BUNDLED,
        "missing backend status was %d" % supervisor.status
    )
    _check(
        SupervisorScript.bundle_directory().begins_with("res://addons/godot_mcp/bin/"),
        "bundle directory is not under the addon"
    )
    _check(
        SupervisorScript.executable_relative_path().get_file().begins_with("godot-mcp-host"),
        "backend executable name is wrong"
    )

    var panel = PanelScript.new(supervisor)
    panel.set_bridge_status(true, 49152)
    root.add_child(panel)
    await process_frame

    var tabs := panel.find_child("PanelTabs", true, false) as TabContainer
    _check(tabs != null, "panel tabs are missing")

    if tabs != null:
        _check(tabs.get_tab_count() == 2, "panel should have two tabs")
        _check(
            tabs.get_tab_title(0) == "MCP Status",
            "status tab title is wrong: %s" % tabs.get_tab_title(0)
        )
        _check(
            tabs.get_tab_title(1) == "Integrations",
            "integrations tab title is wrong: %s" % tabs.get_tab_title(1)
        )
        _check(tabs.current_tab == 0, "panel should open on MCP Status")

    var status_tab := panel.find_child("MCPStatusTab", true, false) as ScrollContainer
    var integrations_tab := panel.find_child("IntegrationsTab", true, false) as ScrollContainer
    _check(status_tab != null, "MCP Status tab content is missing")
    _check(integrations_tab != null, "Integrations tab content is missing")

    var heading := panel.find_child("PanelHeading", true, false) as Label
    _check(heading != null, "panel heading is missing")

    if heading != null:
        _check(heading.text == "Godot MCP", "panel heading is wrong: %s" % heading.text)
        _check(
            String(heading.theme_type_variation) == "HeaderLarge",
            "panel heading does not use the editor heading style"
        )
        _check(
            not heading.has_theme_font_size_override("font_size"),
            "panel heading uses a fixed font size"
        )

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

    var warning := panel.find_child("IntegrationWarning", true, false) as Control
    var warning_label := panel.find_child(
        "IntegrationWarningLabel", true, false
    ) as Label
    var warning_button := panel.find_child(
        "IntegrationWarningButton", true, false
    ) as Button
    var error_banner := panel.find_child(
        "IntegrationErrorBanner", true, false
    ) as Control
    var error_label := panel.find_child(
        "IntegrationErrorLabel", true, false
    ) as Label

    _check(
        warning != null and not warning.visible,
        "integration warning should stay hidden until status loads"
    )
    _check(warning_label != null, "integration warning label is missing")
    _check(warning_button != null, "integration warning button is missing")
    _check(
        error_banner != null and not error_banner.visible,
        "integration error banner should stay hidden"
    )
    _check(
        error_label != null and error_label.text.is_empty(),
        "integration error label should start empty"
    )
    _check(
        panel.find_child("LastErrorValue", true, false) == null,
        "Last error row should be removed"
    )
    _check(
        panel.find_child("IntegrationResultRow", true, false) == null,
        "Last integration result row should be removed"
    )
    _check(
        panel.find_child("IntegrationResultValue", true, false) == null,
        "Last integration result value should be removed"
    )

    var integrations_heading := panel.find_child("IntegrationsHeading", true, false) as Label
    _check(integrations_heading != null, "integrations heading is missing")

    if integrations_heading != null:
        _check(
            integrations_heading.text == "Integrations",
            "integrations heading is wrong: %s" % integrations_heading.text
        )

    var refresh_integrations := panel.find_child(
        "RefreshIntegrationsButton", true, false
    ) as Button
    var install_all := panel.find_child("InstallAllButton", true, false) as Button
    var opencode_mcp := panel.find_child("OpenCodeMcpStatus", true, false) as Label
    var opencode_skill := panel.find_child("OpenCodeSkillStatus", true, false) as Label
    var claude_mcp := panel.find_child("ClaudeCodeMcpStatus", true, false) as Label
    var claude_skill := panel.find_child("ClaudeCodeSkillStatus", true, false) as Label
    var codex_mcp := panel.find_child("CodexMcpStatus", true, false) as Label
    var codex_skill := panel.find_child("CodexSkillStatus", true, false) as Label
    var opencode_button := panel.find_child("OpenCodeInstallButton", true, false) as Button
    var claude_button := panel.find_child("ClaudeCodeInstallButton", true, false) as Button
    var codex_button := panel.find_child("CodexInstallButton", true, false) as Button

    _check(
        refresh_integrations != null and refresh_integrations.disabled,
        "refresh status should be disabled without a backend"
    )
    _check(
        install_all != null and install_all.disabled,
        "install all should be disabled without a backend"
    )
    _check(
        opencode_button != null and opencode_button.disabled,
        "opencode install should be disabled without a backend"
    )
    _check(
        claude_button != null and claude_button.disabled,
        "claude install should be disabled without a backend"
    )
    _check(
        codex_button != null and codex_button.disabled,
        "codex install should be disabled without a backend"
    )

    for label in [
        opencode_mcp,
        opencode_skill,
        claude_mcp,
        claude_skill,
        codex_mcp,
        codex_skill,
    ]:
        _check(label != null, "integration status label is missing")

        if label != null:
            _check(
                label.text == "Unavailable",
                "integration status is not unavailable without a backend: %s" % label.text
            )

    _check(
        status_tab != null and status_tab.is_ancestor_of(version),
        "addon version is not on the MCP Status tab"
    )
    _check(
        status_tab != null and status_tab.is_ancestor_of(start),
        "lifecycle controls are not on the MCP Status tab"
    )
    _check(
        status_tab != null and status_tab.is_ancestor_of(warning),
        "integration warning is not on the MCP Status tab"
    )
    _check(
        integrations_tab != null and integrations_tab.is_ancestor_of(opencode_mcp),
        "client status is not on the Integrations tab"
    )
    _check(
        integrations_tab != null and integrations_tab.is_ancestor_of(refresh_integrations),
        "refresh control is not on the Integrations tab"
    )
    _check(
        integrations_tab != null and integrations_tab.is_ancestor_of(error_banner),
        "integration error banner is not on the Integrations tab"
    )

    if warning_button != null and tabs != null:
        warning_button.pressed.emit()
        _check(
            tabs.current_tab == 1,
            "warning button did not open the Integrations tab"
        )
        tabs.current_tab = 0

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

    panel._refresh()
    _check(
        status != null and status.text.contains("bind failed"),
        "backend error is not shown through backend status: %s" % status.text
    )
    _check(
        status != null and status.tooltip_text.contains("bind failed"),
        "backend error tooltip is missing"
    )

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

const SupervisorScript = preload("res://addons/godot_mcp/mcp/backend_supervisor.gd")

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

const mcpIntegrationsTestScript = `extends SceneTree

const SupervisorScript = preload("res://addons/godot_mcp/mcp/backend_supervisor.gd")
const PanelScript = preload("res://addons/godot_mcp/ui/mcp_panel.gd")

var failures: Array[String] = []

func _initialize() -> void:
    var supervisor = SupervisorScript.new()
    _check(supervisor.is_bundled(), "bundled backend was not found")

    var panel = PanelScript.new(supervisor)
    panel.set_bridge_status(true, 49152)
    root.add_child(panel)
    await process_frame

    var tabs := panel.find_child("PanelTabs", true, false) as TabContainer
    var status_tab := panel.find_child("MCPStatusTab", true, false) as ScrollContainer
    var integrations_tab := panel.find_child("IntegrationsTab", true, false) as ScrollContainer

    _check(tabs != null, "panel tabs are missing")

    if tabs != null:
        _check(tabs.get_tab_count() == 2, "panel should have two tabs")
        _check(
            tabs.get_tab_title(0) == "MCP Status",
            "status tab title is wrong: %s" % tabs.get_tab_title(0)
        )
        _check(
            tabs.get_tab_title(1) == "Integrations",
            "integrations tab title is wrong: %s" % tabs.get_tab_title(1)
        )
        _check(tabs.current_tab == 0, "panel should open on MCP Status")

    _check(status_tab != null, "MCP Status tab content is missing")
    _check(integrations_tab != null, "Integrations tab content is missing")

    var heading := panel.find_child("PanelHeading", true, false) as Label
    _check(heading != null and heading.text == "Godot MCP", "panel heading is wrong")
    _check(
        heading != null and String(heading.theme_type_variation) == "HeaderLarge",
        "panel heading does not use the editor heading style"
    )

    var warning := panel.find_child("IntegrationWarning", true, false) as Control
    var warning_label := panel.find_child(
        "IntegrationWarningLabel", true, false
    ) as Label
    var warning_button := panel.find_child(
        "IntegrationWarningButton", true, false
    ) as Button
    var error_banner := panel.find_child(
        "IntegrationErrorBanner", true, false
    ) as Control
    var error_label := panel.find_child(
        "IntegrationErrorLabel", true, false
    ) as Label

    _check(
        warning != null and not warning.visible,
        "warning should be hidden when a client has MCP configured"
    )
    _check(warning_label != null, "integration warning label is missing")
    _check(warning_button != null, "integration warning button is missing")
    _check(
        error_banner != null and not error_banner.visible,
        "integration error banner should be hidden"
    )
    _check(
        error_label != null and error_label.text.is_empty(),
        "integration error label should start empty"
    )
    _check(
        panel.find_child("LastErrorValue", true, false) == null,
        "Last error row should be removed"
    )
    _check(
        panel.find_child("IntegrationResultRow", true, false) == null,
        "Last integration result row should be removed"
    )
    _check(
        panel.find_child("IntegrationResultValue", true, false) == null,
        "Last integration result value should be removed"
    )

    var refresh := panel.find_child("RefreshIntegrationsButton", true, false) as Button
    var install_all := panel.find_child("InstallAllButton", true, false) as Button
    var opencode_mcp := panel.find_child("OpenCodeMcpStatus", true, false) as Label
    var opencode_skill := panel.find_child("OpenCodeSkillStatus", true, false) as Label
    var claude_mcp := panel.find_child("ClaudeCodeMcpStatus", true, false) as Label
    var claude_skill := panel.find_child("ClaudeCodeSkillStatus", true, false) as Label
    var codex_mcp := panel.find_child("CodexMcpStatus", true, false) as Label
    var codex_skill := panel.find_child("CodexSkillStatus", true, false) as Label
    var opencode_button := panel.find_child("OpenCodeInstallButton", true, false) as Button
    var claude_button := panel.find_child("ClaudeCodeInstallButton", true, false) as Button
    var codex_button := panel.find_child("CodexInstallButton", true, false) as Button
    var start := panel.find_child("StartButton", true, false) as Button
    var restart := panel.find_child("RestartButton", true, false) as Button
    var stop := panel.find_child("StopButton", true, false) as Button

    _check(refresh != null and not refresh.disabled, "refresh should be enabled")
    _check(install_all != null and not install_all.disabled, "install all should be enabled")
    _check(
        opencode_button != null and opencode_button.text == "Repair",
        "opencode should offer repair"
    )
    _check(
        claude_button != null and claude_button.text == "Install",
        "claude should offer install"
    )
    _check(
        codex_button != null and codex_button.text == "Repair",
        "codex should offer repair"
    )
    _check(
        start != null and not start.disabled,
        "start should stay enabled when a backend is bundled"
    )
    _check(
        restart != null and not restart.disabled,
        "restart should stay enabled when a backend is bundled"
    )
    _check(stop != null and stop.disabled, "stop should be disabled without a process")

    _check(
        opencode_mcp != null and opencode_mcp.text == "Installed",
        "opencode MCP status is wrong"
    )
    _check(
        opencode_skill != null and opencode_skill.text == "Missing",
        "opencode skill status is wrong"
    )
    _check(
        claude_mcp != null and claude_mcp.text == "Missing",
        "claude MCP status is wrong"
    )
    _check(
        claude_skill != null and claude_skill.text == "Missing",
        "claude skill status is wrong"
    )
    _check(
        codex_mcp != null and codex_mcp.text == "Repair needed",
        "codex MCP status is wrong"
    )
    _check(
        codex_skill != null and codex_skill.text == "Installed",
        "codex skill status is wrong"
    )

    _check(
        status_tab != null and status_tab.is_ancestor_of(start),
        "lifecycle controls are not on the MCP Status tab"
    )
    _check(
        status_tab != null and status_tab.is_ancestor_of(warning),
        "integration warning is not on the MCP Status tab"
    )
    _check(
        integrations_tab != null and integrations_tab.is_ancestor_of(opencode_mcp),
        "client status is not on the Integrations tab"
    )
    _check(
        integrations_tab != null and integrations_tab.is_ancestor_of(refresh),
        "refresh control is not on the Integrations tab"
    )
    _check(
        integrations_tab != null and integrations_tab.is_ancestor_of(error_banner),
        "integration error banner is not on the Integrations tab"
    )

    opencode_button.pressed.emit()
    _check(
        opencode_skill.text == "Installed",
        "opencode install did not refresh status: %s" % opencode_skill.text
    )
    _check(not error_banner.visible, "opencode install left an integration error")

    install_all.pressed.emit()
    _check(
        claude_mcp.text == "Installed",
        "install all did not refresh status: %s" % claude_mcp.text
    )
    _check(
        codex_mcp.text == "Installed",
        "install all did not repair codex MCP: %s" % codex_mcp.text
    )
    _check(not error_banner.visible, "install all left an integration error")

    refresh.pressed.emit()
    _check(not error_banner.visible, "refresh left an integration error")
    _check(
        not warning.visible,
        "warning should be hidden when clients have MCP configured"
    )

    var fail_marker := FileAccess.open("res://fail-integration", FileAccess.WRITE)
    _check(fail_marker != null, "could not create integration failure marker")

    if fail_marker != null:
        fail_marker.close()

    refresh.pressed.emit()
    _check(error_banner.visible, "integration failure did not show the error banner")
    _check(
        error_label.text.contains("integration failed"),
        "integration error is missing: %s" % error_label.text
    )
    _check(not warning.visible, "integration failure incorrectly showed the warning")
    _check(not refresh.disabled, "refresh did not recover after an integration error")

    opencode_button.pressed.emit()
    _check(
        error_banner.visible,
        "install failure did not show the error banner"
    )
    _check(
        error_label.text.contains("integration failed"),
        "install error is missing: %s" % error_label.text
    )
    _check(
        not warning.visible,
        "install failure incorrectly showed the warning"
    )

    DirAccess.remove_absolute(
        ProjectSettings.globalize_path("res://fail-integration")
    )

    refresh.pressed.emit()
    _check(not error_banner.visible, "integration error banner did not clear")
    _check(error_label.text.is_empty(), "integration error text did not clear")

    var no_config_marker := FileAccess.open("res://no-mcp-config", FileAccess.WRITE)
    _check(no_config_marker != null, "could not create missing configuration marker")

    if no_config_marker != null:
        no_config_marker.close()

    refresh.pressed.emit()
    _check(
        warning.visible,
        "warning should show when no client has MCP configured"
    )
    _check(
        warning_label.text.contains("No AI client"),
        "warning text is not clear: %s" % warning_label.text
    )
    _check(tabs != null and tabs.current_tab == 0, "warning changed tabs on its own")

    if warning_button != null and tabs != null:
        warning_button.pressed.emit()
        _check(
            tabs.current_tab == 1,
            "warning button did not open the Integrations tab"
        )
        tabs.current_tab = 0

    DirAccess.remove_absolute(
        ProjectSettings.globalize_path("res://no-mcp-config")
    )
    refresh.pressed.emit()
    _check(
        not warning.visible,
        "warning should hide after a client is configured"
    )

    var initial_fail_marker := FileAccess.open(
        "res://fail-integration", FileAccess.WRITE
    )
    _check(
        initial_fail_marker != null,
        "could not create the initial status failure marker"
    )

    if initial_fail_marker != null:
        initial_fail_marker.close()

    var failed_panel = PanelScript.new(supervisor)
    var failed_warning := failed_panel.find_child(
        "IntegrationWarning", true, false
    ) as Control
    var failed_error := failed_panel.find_child(
        "IntegrationErrorBanner", true, false
    ) as Control
    _check(
        failed_warning != null and not failed_warning.visible,
        "warning appeared before integration status loaded"
    )
    _check(
        failed_error != null and failed_error.visible,
        "initial integration status failure did not show a contextual error"
    )
    failed_panel.free()
    DirAccess.remove_absolute(
        ProjectSettings.globalize_path("res://fail-integration")
    )

    panel.free()

    if failures.is_empty():
        print("MCP_INTEGRATIONS_OK")
        quit(0)
        return

    for failure in failures:
        print("MCP_INTEGRATIONS_FAILURE: %s" % failure)

    quit(1)

func _check(condition: bool, message: String) -> void:
    if not condition:
        failures.append(message)
`

const fakeIntegrationHost = `#!/bin/sh

log="__LOG__"
project_root="__PROJECT_ROOT__"
opencode_marker="__OPENCODE_MARKER__"
claude_marker="__CLAUDE_MARKER__"
codex_marker="__CODEX_MARKER__"
host_path="$0"

{
    printf '%s\n' '---'
    for argument in "$@"; do
        printf '%s\n' "$argument"
    done
} >> "$log"

if [ "$1" != "integrations" ]; then
    printf '%s\n' '{"ok":false,"error":"unexpected command"}'
    exit 2
fi

if [ -f "$project_root/fail-integration" ]; then
    printf '%s\n' '{"error":"integration failed"}'
    exit 1
fi

opencode_installed=false
if [ -f "$opencode_marker" ]; then
    opencode_installed=true
fi

claude_installed=false
if [ -f "$claude_marker" ]; then
    claude_installed=true
fi

codex_installed=false
if [ -f "$codex_marker" ]; then
    codex_installed=true
fi

no_mcp_config=false
if [ -f "$project_root/no-mcp-config" ]; then
    no_mcp_config=true
fi

opencode_configured=true
claude_configured=$claude_installed
codex_configured=true

if [ "$no_mcp_config" = true ]; then
    opencode_configured=false
    claude_configured=false
    codex_configured=false
fi

if [ "$2" = "install" ]; then
    opencode_changed=false
    claude_changed=false
    codex_changed=false

    case "$3" in
        opencode)
            if [ "$opencode_installed" = false ]; then
                opencode_changed=true
            fi
            : > "$opencode_marker"
            printf '{"project_root":"%s","host_path":"%s","clients":[{"client":"opencode","config_path":"%s/opencode.json","config_changed":false,"skill_path":"%s/.agents/skills/godot-mcp/SKILL.md","skill_changed":true}]}\n' "$project_root" "$host_path" "$project_root" "$project_root"
            ;;
        claude)
            : > "$claude_marker"
            printf '{"project_root":"%s","host_path":"%s","clients":[{"client":"claude","config_path":"%s/.mcp.json","config_changed":true,"skill_path":"%s/.claude/skills/godot-mcp/SKILL.md","skill_changed":true}]}\n' "$project_root" "$host_path" "$project_root" "$project_root"
            ;;
        codex)
            : > "$codex_marker"
            printf '{"project_root":"%s","host_path":"%s","clients":[{"client":"codex","config_path":"%s/.codex/config.toml","config_changed":true,"skill_path":"%s/.agents/skills/godot-mcp/SKILL.md","skill_changed":true}]}\n' "$project_root" "$host_path" "$project_root" "$project_root"
            ;;
        all)
            if [ "$opencode_installed" = false ]; then
                opencode_changed=true
            fi
            if [ "$claude_installed" = false ]; then
                claude_changed=true
            fi
            if [ "$codex_installed" = false ]; then
                codex_changed=true
            fi
            : > "$opencode_marker"
            : > "$claude_marker"
            : > "$codex_marker"
            printf '{"project_root":"%s","host_path":"%s","clients":[{"client":"opencode","config_path":"%s/opencode.json","config_changed":false,"skill_path":"%s/.agents/skills/godot-mcp/SKILL.md","skill_changed":%s},{"client":"claude","config_path":"%s/.mcp.json","config_changed":%s,"skill_path":"%s/.claude/skills/godot-mcp/SKILL.md","skill_changed":%s},{"client":"codex","config_path":"%s/.codex/config.toml","config_changed":%s,"skill_path":"%s/.agents/skills/godot-mcp/SKILL.md","skill_changed":%s}]}\n' "$project_root" "$host_path" "$project_root" "$project_root" "$opencode_changed" "$project_root" "$claude_changed" "$project_root" "$claude_changed" "$project_root" "$codex_changed" "$project_root" "$codex_changed"
            ;;
        *)
            printf '%s\n' '{"ok":false,"error":"unknown integration target"}'
            exit 2
            ;;
    esac

    exit 0
fi

opencode_up_to_date=false
opencode_skill_installed=false
opencode_skill_current=false
if [ "$opencode_installed" = true ]; then
    opencode_up_to_date=true
    opencode_skill_installed=true
    opencode_skill_current=true
fi

claude_up_to_date=false
claude_skill_installed=false
claude_skill_current=false
if [ "$claude_installed" = true ]; then
    claude_up_to_date=true
    claude_skill_installed=true
    claude_skill_current=true
fi

codex_up_to_date=false
codex_command="[\"/stale/godot-mcp-host\",\"connect\",\"--project\",\"$project_root\"]"
if [ "$codex_installed" = true ]; then
    codex_up_to_date=true
    codex_command="[\"$host_path\",\"connect\",\"--project\",\"$project_root\"]"
fi

if [ "$2" = "status" ]; then
    cat <<EOF
{"project_root":"$project_root","host_path":"$host_path","clients":[{"client":"opencode","config_path":"$project_root/opencode.json","config_exists":true,"configured":$opencode_configured,"up_to_date":$opencode_up_to_date,"command":["$host_path","connect","--project","$project_root"],"skill_path":"$project_root/.agents/skills/godot-mcp/SKILL.md","skill_installed":$opencode_skill_installed,"skill_current":$opencode_skill_current},{"client":"claude","config_path":"$project_root/.mcp.json","config_exists":$claude_installed,"configured":$claude_configured,"up_to_date":$claude_up_to_date,"skill_path":"$project_root/.claude/skills/godot-mcp/SKILL.md","skill_installed":$claude_skill_installed,"skill_current":$claude_skill_current},{"client":"codex","config_path":"$project_root/.codex/config.toml","config_exists":true,"configured":$codex_configured,"up_to_date":$codex_up_to_date,"command":$codex_command,"skill_path":"$project_root/.agents/skills/godot-mcp/SKILL.md","skill_installed":true,"skill_current":true}]}
EOF
    exit 0
fi

printf '%s\n' '{"ok":false,"error":"unknown integrations action"}'
exit 2
`

func TestMCPIntegrationsPanelUsesBundledHost(t *testing.T) {
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

	logPath := filepath.Join(root, "integration_args.log")
	backend := strings.NewReplacer(
		"__LOG__", logPath,
		"__PROJECT_ROOT__", root,
		"__OPENCODE_MARKER__", filepath.Join(root, "opencode.installed"),
		"__CLAUDE_MARKER__", filepath.Join(root, "claude.installed"),
		"__CODEX_MARKER__", filepath.Join(root, "codex.installed"),
	).Replace(fakeIntegrationHost)

	backendPath := filepath.Join(bundleDirectory, "godot-mcp-host")
	if err := os.WriteFile(backendPath, []byte(backend), 0755); err != nil {
		t.Fatal(err)
	}

	writeScript(t, root, "mcp_integrations_test.gd", mcpIntegrationsTestScript)
	runMCPTestScript(
		t,
		godot,
		root,
		"res://mcp_integrations_test.gd",
		"MCP_INTEGRATIONS_OK",
	)

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("reading integration arguments: %v", err)
	}

	calls := readIntegrationCalls(string(data))

	for _, expected := range [][]string{
		{"integrations", "status", "--project"},
		{"integrations", "install", "opencode", "--project"},
		{"integrations", "install", "all", "--project"},
	} {
		if !hasIntegrationCall(calls, root, expected) {
			t.Fatalf("missing host call %q in:\n%s", expected, data)
		}
	}
}

func readIntegrationCalls(output string) [][]string {
	var calls [][]string

	for _, block := range strings.Split(output, "---\n") {
		block = strings.TrimSpace(block)

		if block == "" {
			continue
		}

		calls = append(calls, strings.Split(block, "\n"))
	}

	return calls
}

func hasIntegrationCall(calls [][]string, projectRoot string, prefix []string) bool {
	for _, call := range calls {
		if len(call) != len(prefix)+1 {
			continue
		}

		matches := true

		for index, argument := range prefix {
			if call[index] != argument {
				matches = false
				break
			}
		}

		if matches && integrationPathsEqual(call[len(call)-1], projectRoot) {
			return true
		}
	}

	return false
}

func integrationPathsEqual(left, right string) bool {
	resolve := func(path string) string {
		absolute, err := filepath.Abs(path)
		if err == nil {
			path = absolute
		}

		resolved, err := filepath.EvalSymlinks(path)
		if err == nil {
			path = resolved
		}

		return filepath.Clean(path)
	}

	return resolve(left) == resolve(right)
}

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

	logPath := filepath.Join(root, "backend_args.log")
	fakeBackend := `#!/bin/sh
log="__LOG__"
printf '%s\n' "$@" > "$log"
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
	fakeBackend = strings.ReplaceAll(fakeBackend, "__LOG__", logPath)

	backendPath := filepath.Join(bundleDirectory, "godot-mcp-host")
	if err := os.WriteFile(backendPath, []byte(fakeBackend), 0755); err != nil {
		t.Fatal(err)
	}

	writeScript(t, root, "mcp_backend_test.gd", mcpBackendTestScript)
	runMCPTestScript(t, godot, root, "res://mcp_backend_test.gd", "MCP_BACKEND_OK")

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("reading backend arguments: %v", err)
	}

	arguments := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(arguments) == 0 || arguments[0] != "serve" {
		t.Fatalf("backend arguments = %v, want serve", arguments)
	}

	for _, unwanted := range []string{"mcp", "--transport"} {
		if slices.Contains(arguments, unwanted) {
			t.Fatalf("backend arguments still contain %q: %v", unwanted, arguments)
		}
	}
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

	if !strings.Contains(text, "godot-mcp: bridge listening on") {
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
