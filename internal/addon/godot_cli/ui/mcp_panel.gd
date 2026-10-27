@tool
extends VBoxContainer

const BackendSupervisor = preload("res://addons/godot_cli/mcp/backend_supervisor.gd")
const PLUGIN_CONFIG_PATH := "res://addons/godot_cli/plugin.cfg"

const COLOR_ERROR := Color(0.9, 0.35, 0.35)
const COLOR_RUNNING := Color(0.42, 0.78, 0.45)
const COLOR_STARTING := Color(0.45, 0.65, 0.95)
const COLOR_WARNING := Color(0.9, 0.65, 0.25)

var _supervisor: BackendSupervisor
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

func _init(backend_supervisor: BackendSupervisor) -> void:
    _supervisor = backend_supervisor
    _addon_version = _read_addon_version()

    name = "MCP"
    custom_minimum_size = Vector2(0, 190)
    size_flags_horizontal = Control.SIZE_EXPAND_FILL
    size_flags_vertical = Control.SIZE_EXPAND_FILL

    _build_ui()
    _supervisor.changed.connect(_refresh)
    _refresh()

func set_bridge_status(active: bool, port: int) -> void:
    _bridge_active = active
    _bridge_port = port
    _refresh()

func _exit_tree() -> void:
    if _supervisor != null \
    and _supervisor.changed.is_connected(_refresh):
        _supervisor.changed.disconnect(_refresh)

func _build_ui() -> void:
    var margin := MarginContainer.new()
    margin.size_flags_horizontal = Control.SIZE_EXPAND_FILL
    margin.size_flags_vertical = Control.SIZE_EXPAND_FILL
    margin.add_theme_constant_override("margin_left", 12)
    margin.add_theme_constant_override("margin_top", 8)
    margin.add_theme_constant_override("margin_right", 12)
    margin.add_theme_constant_override("margin_bottom", 8)
    add_child(margin)

    var layout := VBoxContainer.new()
    layout.add_theme_constant_override("separation", 8)
    margin.add_child(layout)

    var heading := Label.new()
    heading.text = "Godot CLI MCP"
    heading.add_theme_font_size_override("font_size", 16)
    layout.add_child(heading)

    var grid := GridContainer.new()
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

    _copy_reset_timer = Timer.new()
    _copy_reset_timer.one_shot = true
    _copy_reset_timer.wait_time = 1.2
    _copy_reset_timer.timeout.connect(_reset_copy_button)
    add_child(_copy_reset_timer)

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
    key.custom_minimum_size.x = 120
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

func _read_addon_version() -> String:
    var config := ConfigFile.new()

    if config.load(PLUGIN_CONFIG_PATH) != OK:
        return "unknown"

    return str(config.get_value("plugin", "version", "unknown"))
