@tool
extends VBoxContainer

const BackendSupervisor = preload("res://addons/godot_cli/mcp/backend_supervisor.gd")
const IntegrationManager = preload("res://addons/godot_cli/mcp/integration_manager.gd")
const PLUGIN_CONFIG_PATH := "res://addons/godot_cli/plugin.cfg"

const COLOR_ERROR := Color(0.9, 0.35, 0.35)
const COLOR_RUNNING := Color(0.42, 0.78, 0.45)
const COLOR_STARTING := Color(0.45, 0.65, 0.95)
const COLOR_WARNING := Color(0.9, 0.65, 0.25)
const COLOR_MUTED := Color(0.65, 0.65, 0.65)

var _supervisor: BackendSupervisor
var _integrations: IntegrationManager
var _addon_version := "unknown"
var _bridge_active := false
var _bridge_port := 0

var _version_label: Label
var _bridge_label: Label
var _backend_status_label: Label
var _endpoint_edit: LineEdit
var _pid_label: Label
var _last_error_label: Label
var _start_button: Button
var _restart_button: Button
var _stop_button: Button
var _copy_button: Button
var _copy_reset_timer: Timer
var _refresh_integrations_button: Button
var _install_all_button: Button
var _integration_result_label: Label
var _integration_buttons := {}
var _integration_mcp_labels := {}
var _integration_skill_labels := {}

func _init(backend_supervisor: BackendSupervisor) -> void:
    _supervisor = backend_supervisor
    _integrations = IntegrationManager.new()
    _addon_version = _read_addon_version()

    name = "MCP"
    custom_minimum_size = Vector2(0, 190)
    size_flags_horizontal = Control.SIZE_EXPAND_FILL
    size_flags_vertical = Control.SIZE_EXPAND_FILL

    _build_ui()
    _supervisor.changed.connect(_refresh)
    _integrations.changed.connect(_refresh)
    _refresh()

    if _integrations.is_available():
        _integrations.refresh()

func set_bridge_status(active: bool, port: int) -> void:
    _bridge_active = active
    _bridge_port = port
    _refresh()

func _exit_tree() -> void:
    if _supervisor != null \
    and _supervisor.changed.is_connected(_refresh):
        _supervisor.changed.disconnect(_refresh)

    if _integrations != null \
    and _integrations.changed.is_connected(_refresh):
        _integrations.changed.disconnect(_refresh)

func _build_ui() -> void:
    var scroll := ScrollContainer.new()
    scroll.name = "PanelScroll"
    scroll.size_flags_horizontal = Control.SIZE_EXPAND_FILL
    scroll.size_flags_vertical = Control.SIZE_EXPAND_FILL
    scroll.horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED
    add_child(scroll)

    var margin := MarginContainer.new()
    margin.name = "PanelMargin"
    margin.size_flags_horizontal = Control.SIZE_EXPAND_FILL
    margin.size_flags_vertical = Control.SIZE_EXPAND_FILL
    margin.add_theme_constant_override("margin_left", 16)
    margin.add_theme_constant_override("margin_top", 12)
    margin.add_theme_constant_override("margin_right", 16)
    margin.add_theme_constant_override("margin_bottom", 12)
    scroll.add_child(margin)

    var layout := VBoxContainer.new()
    layout.name = "PanelLayout"
    layout.size_flags_horizontal = Control.SIZE_EXPAND_FILL
    layout.add_theme_constant_override("separation", 10)
    margin.add_child(layout)

    var heading := Label.new()
    heading.name = "PanelHeading"
    heading.text = "Godot MCP"
    heading.theme_type_variation = "HeaderLarge"
    layout.add_child(heading)

    var subtitle := Label.new()
    subtitle.name = "PanelSubtitle"
    subtitle.text = "Manage the bundled MCP backend and AI client integrations for this project."
    subtitle.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
    layout.add_child(subtitle)

    var backend_heading := Label.new()
    backend_heading.name = "BackendHeading"
    backend_heading.text = "Editor backend"
    backend_heading.theme_type_variation = "HeaderMedium"
    layout.add_child(backend_heading)

    var grid := GridContainer.new()
    grid.name = "BackendGrid"
    grid.columns = 2
    grid.size_flags_horizontal = Control.SIZE_EXPAND_FILL
    grid.add_theme_constant_override("h_separation", 16)
    grid.add_theme_constant_override("v_separation", 6)
    layout.add_child(grid)

    _version_label = _new_value_label("AddonVersionValue")
    _add_row(grid, "Addon version", _version_label, "AddonVersionValue")

    _bridge_label = _new_value_label("BridgeStatusValue")
    _add_row(grid, "Editor bridge", _bridge_label, "BridgeStatusValue")

    _backend_status_label = _new_value_label("BackendStatusValue")
    _add_row(grid, "MCP backend", _backend_status_label, "BackendStatusValue")

    var endpoint_row := HBoxContainer.new()
    endpoint_row.add_theme_constant_override("separation", 8)
    endpoint_row.size_flags_horizontal = Control.SIZE_EXPAND_FILL

    _endpoint_edit = LineEdit.new()
    _endpoint_edit.name = "EndpointValue"
    _endpoint_edit.editable = false
    _endpoint_edit.size_flags_horizontal = Control.SIZE_EXPAND_FILL
    _endpoint_edit.placeholder_text = "Not running"
    _endpoint_edit.tooltip_text = "The MCP HTTP endpoint published by the backend state file."
    endpoint_row.add_child(_endpoint_edit)

    _copy_button = Button.new()
    _copy_button.name = "CopyEndpointButton"
    _copy_button.text = "Copy Endpoint"
    _copy_button.tooltip_text = "Copy the MCP HTTP endpoint to the clipboard."
    _copy_button.pressed.connect(_on_copy_endpoint_pressed)
    endpoint_row.add_child(_copy_button)

    _add_row(grid, "Endpoint", endpoint_row, "EndpointRow")

    _pid_label = _new_value_label("PIDValue")
    _add_row(grid, "PID", _pid_label, "PIDValue")

    _last_error_label = _new_value_label("LastErrorValue")
    _last_error_label.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
    _add_row(grid, "Last error", _last_error_label, "LastErrorValue")

    var actions := HBoxContainer.new()
    actions.name = "BackendActions"
    actions.add_theme_constant_override("separation", 8)
    layout.add_child(actions)

    _start_button = Button.new()
    _start_button.name = "StartButton"
    _start_button.text = "Start"
    _start_button.tooltip_text = "Start the bundled MCP backend for this project."
    _start_button.pressed.connect(_on_start_pressed)
    actions.add_child(_start_button)

    _restart_button = Button.new()
    _restart_button.name = "RestartButton"
    _restart_button.text = "Restart"
    _restart_button.tooltip_text = "Restart the MCP backend managed by this editor."
    _restart_button.pressed.connect(_on_restart_pressed)
    actions.add_child(_restart_button)

    _stop_button = Button.new()
    _stop_button.name = "StopButton"
    _stop_button.text = "Stop"
    _stop_button.tooltip_text = "Stop only the MCP backend process started by this editor."
    _stop_button.pressed.connect(_on_stop_pressed)
    actions.add_child(_stop_button)

    var separator := HSeparator.new()
    separator.name = "IntegrationsSeparator"
    layout.add_child(separator)

    var integrations_header := HBoxContainer.new()
    integrations_header.name = "IntegrationsHeader"
    integrations_header.add_theme_constant_override("separation", 8)
    layout.add_child(integrations_header)

    var integrations_heading := Label.new()
    integrations_heading.name = "IntegrationsHeading"
    integrations_heading.text = "Integrations"
    integrations_heading.theme_type_variation = "HeaderMedium"
    integrations_header.add_child(integrations_heading)

    var header_spacer := Control.new()
    header_spacer.size_flags_horizontal = Control.SIZE_EXPAND_FILL
    integrations_header.add_child(header_spacer)

    _refresh_integrations_button = Button.new()
    _refresh_integrations_button.name = "RefreshIntegrationsButton"
    _refresh_integrations_button.text = "Refresh Status"
    _refresh_integrations_button.tooltip_text = \
        "Read MCP and workflow-skill status from the bundled host."
    _refresh_integrations_button.pressed.connect(
        _on_refresh_integrations_pressed
    )
    integrations_header.add_child(_refresh_integrations_button)

    var integration_hint := Label.new()
    integration_hint.name = "IntegrationHint"
    integration_hint.text = \
        "Install the MCP server registration and workflow skill for each client."
    integration_hint.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
    layout.add_child(integration_hint)

    var integrations_grid := GridContainer.new()
    integrations_grid.name = "IntegrationsGrid"
    integrations_grid.columns = 4
    integrations_grid.size_flags_horizontal = Control.SIZE_EXPAND_FILL
    integrations_grid.add_theme_constant_override("h_separation", 16)
    integrations_grid.add_theme_constant_override("v_separation", 6)
    layout.add_child(integrations_grid)

    _add_integration_headers(integrations_grid)
    _add_integration_row(integrations_grid, "opencode", "OpenCode")
    _add_integration_row(integrations_grid, "claude", "Claude Code")
    _add_integration_row(integrations_grid, "codex", "Codex")

    var integration_actions := HBoxContainer.new()
    integration_actions.name = "IntegrationActions"
    integration_actions.add_theme_constant_override("separation", 8)
    layout.add_child(integration_actions)

    _install_all_button = Button.new()
    _install_all_button.name = "InstallAllButton"
    _install_all_button.text = "Install All"
    _install_all_button.tooltip_text = \
        "Install or repair MCP and workflow-skill setup for every client."
    _install_all_button.pressed.connect(_on_install_all_pressed)
    integration_actions.add_child(_install_all_button)

    var result_row := HBoxContainer.new()
    result_row.name = "IntegrationResultRow"
    result_row.add_theme_constant_override("separation", 12)
    layout.add_child(result_row)

    var result_key := Label.new()
    result_key.name = "IntegrationResultKey"
    result_key.text = "Last integration result"
    result_key.custom_minimum_size.x = 132
    result_key.size_flags_vertical = Control.SIZE_SHRINK_BEGIN
    result_row.add_child(result_key)

    _integration_result_label = _new_value_label("IntegrationResultValue")
    _integration_result_label.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
    result_row.add_child(_integration_result_label)

    _copy_reset_timer = Timer.new()
    _copy_reset_timer.one_shot = true
    _copy_reset_timer.wait_time = 1.2
    _copy_reset_timer.timeout.connect(_reset_copy_button)
    add_child(_copy_reset_timer)

func _add_integration_headers(grid: GridContainer) -> void:
    for header_text in ["Client", "MCP", "Skill"]:
        var header := Label.new()
        header.name = "%sHeader" % header_text
        header.text = header_text
        header.theme_type_variation = "HeaderSmall"
        grid.add_child(header)

    grid.add_child(Control.new())

func _add_integration_row(
    grid: GridContainer,
    client_id: String,
    label_text: String
) -> void:
    var client_label := Label.new()
    client_label.name = "%sLabel" % _client_node_prefix(client_id)
    client_label.text = label_text
    client_label.tooltip_text = "Install MCP and workflow-skill support for %s." % label_text
    grid.add_child(client_label)

    var mcp_label := _new_value_label(
        "%sMcpStatus" % _client_node_prefix(client_id)
    )
    mcp_label.text = "Unknown"
    grid.add_child(mcp_label)
    _integration_mcp_labels[client_id] = mcp_label

    var skill_label := _new_value_label(
        "%sSkillStatus" % _client_node_prefix(client_id)
    )
    skill_label.text = "Unknown"
    grid.add_child(skill_label)
    _integration_skill_labels[client_id] = skill_label

    var button := Button.new()
    button.name = "%sInstallButton" % _client_node_prefix(client_id)
    button.text = "Install"
    button.tooltip_text = "Install or repair %s MCP and skill setup." % label_text
    button.pressed.connect(_on_integration_install_pressed.bind(client_id))
    grid.add_child(button)
    _integration_buttons[client_id] = button

func _client_node_prefix(client_id: String) -> String:
    match client_id:
        "opencode":
            return "OpenCode"
        "claude":
            return "ClaudeCode"
        "codex":
            return "Codex"
        _:
            return client_id

func _new_value_label(node_name: String) -> Label:
    var label := Label.new()
    label.name = node_name
    label.size_flags_horizontal = Control.SIZE_EXPAND_FILL
    return label

func _add_row(
    grid: GridContainer,
    label_text: String,
    value: Control,
    value_name: String
) -> void:
    var key := Label.new()
    key.text = label_text
    key.custom_minimum_size.x = 132
    key.size_flags_vertical = Control.SIZE_SHRINK_BEGIN
    grid.add_child(key)

    value.name = value_name
    value.size_flags_horizontal = Control.SIZE_EXPAND_FILL
    grid.add_child(value)

func _refresh() -> void:
    _version_label.text = _addon_version

    if _bridge_active:
        _bridge_label.text = "Listening (port %d)" % _bridge_port
    else:
        _bridge_label.text = "Unavailable"

    _backend_status_label.text = _backend_status_text()
    _backend_status_label.tooltip_text = ""

    if _supervisor.is_bundled():
        _backend_status_label.tooltip_text = "Executable: %s" % _supervisor.get_executable_path()

    _apply_status_color(_backend_status_label, _backend_status_color())

    if _endpoint_edit.text != _supervisor.endpoint:
        _endpoint_edit.text = _supervisor.endpoint

    _endpoint_edit.tooltip_text = _supervisor.endpoint

    var pid := _supervisor.get_managed_pid()

    if pid <= 0:
        pid = _supervisor.state_pid

    if pid > 0:
        _pid_label.text = str(pid)
    else:
        _pid_label.text = "—"

    if _supervisor.last_error.is_empty():
        _last_error_label.text = "None"
        _last_error_label.remove_theme_color_override("font_color")
    else:
        _last_error_label.text = _supervisor.last_error
        _last_error_label.add_theme_color_override("font_color", COLOR_ERROR)

    _start_button.disabled = \
        not _supervisor.is_bundled() or _supervisor.has_process()
    _restart_button.disabled = not _supervisor.is_bundled()
    _stop_button.disabled = not _supervisor.has_process()
    _copy_button.disabled = _supervisor.endpoint.is_empty()

    _refresh_integrations()

func _refresh_integrations() -> void:
    _integrations.configure(
        _supervisor.get_executable_path(),
        ProjectSettings.globalize_path("res://")
    )

    var available := _integrations.is_available()
    var busy := _integrations.is_busy()
    var controls_disabled := not available or busy

    _refresh_integrations_button.disabled = controls_disabled
    _install_all_button.disabled = controls_disabled

    for client_id in IntegrationManager.CLIENT_ORDER:
        var status := _integrations.get_status(client_id)
        var mcp_label: Label = _integration_mcp_labels[client_id]
        var skill_label: Label = _integration_skill_labels[client_id]
        var button: Button = _integration_buttons[client_id]

        _apply_integration_status(
            mcp_label,
            str(status.get("mcp", IntegrationManager.STATE_UNKNOWN)),
            available
        )
        _apply_integration_status(
            skill_label,
            str(status.get("skill", IntegrationManager.STATE_UNKNOWN)),
            available
        )

        var detail := str(status.get("detail", ""))

        if not detail.is_empty():
            mcp_label.tooltip_text = detail
            skill_label.tooltip_text = detail

        button.text = _integrations.button_text(client_id)
        button.disabled = controls_disabled

    if not _integrations.last_error.is_empty():
        _integration_result_label.text = _integrations.last_error
        _integration_result_label.add_theme_color_override(
            "font_color",
            COLOR_ERROR
        )
    elif not _integrations.last_result.is_empty():
        _integration_result_label.text = _integrations.last_result
        _integration_result_label.remove_theme_color_override("font_color")
    else:
        _integration_result_label.text = "Not checked"
        _integration_result_label.add_theme_color_override(
            "font_color",
            COLOR_MUTED
        )

func _apply_integration_status(
    label: Label,
    state: String,
    available: bool
) -> void:
    if not available:
        label.text = "Unavailable"
        label.add_theme_color_override("font_color", COLOR_WARNING)
        return

    label.text = _integrations.status_text(state)
    _apply_status_color(label, _integration_status_color(state))

func _integration_status_color(state: String) -> Color:
    match state:
        IntegrationManager.STATE_INSTALLED:
            return COLOR_RUNNING
        IntegrationManager.STATE_MISSING:
            return COLOR_WARNING
        IntegrationManager.STATE_REPAIR:
            return COLOR_WARNING
        IntegrationManager.STATE_UNSUPPORTED:
            return COLOR_MUTED
        IntegrationManager.STATE_ERROR:
            return COLOR_ERROR
        _:
            return COLOR_MUTED

func _backend_status_text() -> String:
    match _supervisor.status:
        BackendSupervisor.Status.NOT_BUNDLED:
            return "Backend not included in this development build"
        BackendSupervisor.Status.STOPPED:
            return "Stopped"
        BackendSupervisor.Status.STARTING:
            if _supervisor.endpoint.is_empty():
                return "Starting (waiting for state file)"
            return "Starting"
        BackendSupervisor.Status.RUNNING:
            return "Running"
        BackendSupervisor.Status.ERROR:
            if _supervisor.has_process():
                return "Error (backend still running)"
            return "Error (backend stopped)"
        _:
            return "Unknown"

func _backend_status_color() -> Color:
    match _supervisor.status:
        BackendSupervisor.Status.NOT_BUNDLED:
            return COLOR_WARNING
        BackendSupervisor.Status.STARTING:
            return COLOR_STARTING
        BackendSupervisor.Status.RUNNING:
            return COLOR_RUNNING
        BackendSupervisor.Status.ERROR:
            return COLOR_ERROR
        _:
            return Color(0, 0, 0, 0)

func _apply_status_color(label: Label, color: Color) -> void:
    if color.a == 0.0:
        label.remove_theme_color_override("font_color")
    else:
        label.add_theme_color_override("font_color", color)

func _on_start_pressed() -> void:
    _supervisor.start()
    _refresh()

func _on_restart_pressed() -> void:
    _supervisor.restart()
    _refresh()

func _on_stop_pressed() -> void:
    _supervisor.stop()
    _refresh()

func _on_copy_endpoint_pressed() -> void:
    if _supervisor.endpoint.is_empty():
        return

    DisplayServer.clipboard_set(_supervisor.endpoint)
    _copy_button.text = "Copied"
    _copy_reset_timer.start()

func _reset_copy_button() -> void:
    _copy_button.text = "Copy Endpoint"

func _on_refresh_integrations_pressed() -> void:
    _integrations.refresh()
    _refresh()

func _on_install_all_pressed() -> void:
    _integrations.install("all")
    _refresh()

func _on_integration_install_pressed(client_id: String) -> void:
    _integrations.install(client_id)
    _refresh()

func _read_addon_version() -> String:
    var config := ConfigFile.new()

    if config.load(PLUGIN_CONFIG_PATH) != OK:
        return "unknown"

    return str(config.get_value("plugin", "version", "unknown"))
