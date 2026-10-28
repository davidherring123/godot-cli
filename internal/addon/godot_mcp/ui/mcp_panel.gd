@tool
extends VBoxContainer

const BackendSupervisor = preload("res://addons/godot_mcp/mcp/backend_supervisor.gd")
const IntegrationManager = preload("res://addons/godot_mcp/mcp/integration_manager.gd")
const PLUGIN_CONFIG_PATH := "res://addons/godot_mcp/plugin.cfg"

const COLOR_ERROR := Color(0.9, 0.35, 0.35)
const COLOR_RUNNING := Color(0.42, 0.78, 0.45)
const COLOR_STARTING := Color(0.45, 0.65, 0.95)
const COLOR_WARNING := Color(0.9, 0.65, 0.25)
const COLOR_MUTED := Color(0.65, 0.65, 0.65)

const TAB_STATUS := 0
const TAB_INTEGRATIONS := 1

var _supervisor: BackendSupervisor
var _integrations: IntegrationManager
var _addon_version := "unknown"
var _bridge_active := false
var _bridge_port := 0

var _tabs: TabContainer
var _version_label: Label
var _bridge_label: Label
var _backend_status_label: Label
var _endpoint_edit: LineEdit
var _pid_label: Label
var _start_button: Button
var _restart_button: Button
var _stop_button: Button
var _copy_button: Button
var _copy_reset_timer: Timer
var _integration_warning: Control
var _integration_warning_label: Label
var _integration_error_banner: Control
var _integration_error_label: Label
var _refresh_integrations_button: Button
var _install_all_button: Button
var _integration_buttons := {}
var _integration_mcp_labels := {}
var _integration_skill_labels := {}
var _key_labels: Array[Label] = []
var _content_labels: Array[Label] = []
var _subtitle_label: Label

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

func _ready() -> void:
    _apply_editor_theme()

func _exit_tree() -> void:
    if _supervisor != null \
    and _supervisor.changed.is_connected(_refresh):
        _supervisor.changed.disconnect(_refresh)

    if _integrations != null \
    and _integrations.changed.is_connected(_refresh):
        _integrations.changed.disconnect(_refresh)

func _build_ui() -> void:
    var heading := Label.new()
    heading.name = "PanelHeading"
    heading.text = "Godot MCP"
    heading.theme_type_variation = "HeaderLarge"
    add_child(heading)

    _subtitle_label = Label.new()
    _subtitle_label.name = "PanelSubtitle"
    _subtitle_label.text = "Manage the bundled MCP backend and AI client integrations for this project."
    _subtitle_label.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
    add_child(_subtitle_label)

    _tabs = TabContainer.new()
    _tabs.name = "PanelTabs"
    _tabs.size_flags_horizontal = Control.SIZE_EXPAND_FILL
    _tabs.size_flags_vertical = Control.SIZE_EXPAND_FILL
    add_child(_tabs)

    var status_tab := _new_tab_scroll("MCPStatusTab")
    _tabs.add_child(status_tab)
    _build_status_tab(
        _new_tab_layout(status_tab, "StatusMargin", "StatusLayout")
    )

    var integrations_tab := _new_tab_scroll("IntegrationsTab")
    _tabs.add_child(integrations_tab)
    _build_integrations_tab(
        _new_tab_layout(integrations_tab, "IntegrationsMargin", "IntegrationsLayout")
    )

    _tabs.set_tab_title(TAB_STATUS, "MCP Status")
    _tabs.set_tab_title(TAB_INTEGRATIONS, "Integrations")
    _tabs.current_tab = TAB_STATUS

    _copy_reset_timer = Timer.new()
    _copy_reset_timer.one_shot = true
    _copy_reset_timer.wait_time = 1.2
    _copy_reset_timer.timeout.connect(_reset_copy_button)
    add_child(_copy_reset_timer)

func _new_tab_scroll(node_name: String) -> ScrollContainer:
    var scroll := ScrollContainer.new()
    scroll.name = node_name
    scroll.size_flags_horizontal = Control.SIZE_EXPAND_FILL
    scroll.size_flags_vertical = Control.SIZE_EXPAND_FILL
    scroll.horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED
    return scroll

func _apply_editor_theme() -> void:
    if _tabs == null:
        return

    var base := _editor_color("base_color", Color(0.16, 0.16, 0.16))
    var surface := _editor_color("dark_color_1", base.darkened(0.12))
    var inactive := _editor_color("dark_color_2", surface.darkened(0.12))
    var hover := _editor_color("dark_color_3", inactive.lightened(0.08))
    var accent := _editor_color("accent_color", COLOR_STARTING)
    var text := _editor_color("font_color", Color(0.82, 0.82, 0.82))
    var muted := _editor_color("font_disabled_color", COLOR_MUTED)

    var panel_style := StyleBoxFlat.new()
    panel_style.bg_color = surface
    panel_style.border_color = inactive
    panel_style.set_border_width_all(1)
    panel_style.set_corner_radius_all(4)
    _tabs.add_theme_stylebox_override("panel", panel_style)

    var tab_bar := _tabs.get_tab_bar()
    tab_bar.add_theme_stylebox_override(
        "tab_selected",
        _tab_style(surface, accent, true)
    )
    tab_bar.add_theme_stylebox_override(
        "tab_unselected",
        _tab_style(inactive, accent, false)
    )
    tab_bar.add_theme_stylebox_override(
        "tab_hovered",
        _tab_style(hover, accent, false)
    )
    tab_bar.add_theme_color_override("font_selected_color", text)
    tab_bar.add_theme_color_override("font_unselected_color", muted)
    tab_bar.add_theme_color_override("font_hovered_color", text)

    var tab_font_size := tab_bar.get_theme_font_size("font_size")

    if tab_font_size > 0:
        tab_bar.add_theme_font_size_override("font_size", tab_font_size + 1)

    if _subtitle_label != null:
        _subtitle_label.add_theme_color_override("font_color", muted)

    for label in _key_labels:
        label.add_theme_color_override("font_color", muted)

    var body_font_size := get_theme_font_size("font_size", "Label")

    if body_font_size > 0:
        for label in _content_labels:
            label.add_theme_font_size_override("font_size", body_font_size + 1)

func _editor_color(color_name: String, fallback: Color) -> Color:
    if has_theme_color(color_name, "Editor"):
        return get_theme_color(color_name, "Editor")

    return fallback

func _tab_style(
    background: Color,
    accent: Color,
    selected: bool
) -> StyleBoxFlat:
    var style := StyleBoxFlat.new()
    style.bg_color = background
    style.set_corner_radius_all(4)
    style.set_content_margin_all(12)
    style.set_content_margin(SIDE_LEFT, 16)
    style.set_content_margin(SIDE_TOP, 18)
    style.set_content_margin(SIDE_RIGHT, 16)

    if selected:
        style.border_color = accent
        style.border_width_top = 2

    return style

func _new_tab_layout(
    scroll: ScrollContainer,
    margin_name: String,
    layout_name: String
) -> VBoxContainer:
    var margin := MarginContainer.new()
    margin.name = margin_name
    margin.size_flags_horizontal = Control.SIZE_EXPAND_FILL
    margin.size_flags_vertical = Control.SIZE_EXPAND_FILL
    margin.add_theme_constant_override("margin_left", 16)
    margin.add_theme_constant_override("margin_top", 12)
    margin.add_theme_constant_override("margin_right", 16)
    margin.add_theme_constant_override("margin_bottom", 12)
    scroll.add_child(margin)

    var layout := VBoxContainer.new()
    layout.name = layout_name
    layout.size_flags_horizontal = Control.SIZE_EXPAND_FILL
    layout.add_theme_constant_override("separation", 10)
    margin.add_child(layout)
    return layout

func _build_status_tab(layout: VBoxContainer) -> void:
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
    _backend_status_label.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
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

    _integration_warning = _build_integration_warning()
    layout.add_child(_integration_warning)

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

func _build_integrations_tab(layout: VBoxContainer) -> void:
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
    _content_labels.append(integration_hint)
    layout.add_child(integration_hint)

    _integration_error_banner = _build_integration_error_banner()
    layout.add_child(_integration_error_banner)

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

func _build_integration_warning() -> PanelContainer:
    var panel := PanelContainer.new()
    panel.name = "IntegrationWarning"
    panel.visible = false
    panel.size_flags_horizontal = Control.SIZE_EXPAND_FILL

    var style := StyleBoxFlat.new()
    style.bg_color = Color(0.35, 0.26, 0.08, 0.65)
    style.set_corner_radius_all(4)
    style.set_content_margin_all(8)
    panel.add_theme_stylebox_override("panel", style)

    var row := HBoxContainer.new()
    row.name = "IntegrationWarningRow"
    row.add_theme_constant_override("separation", 8)
    panel.add_child(row)

    _integration_warning_label = Label.new()
    _integration_warning_label.name = "IntegrationWarningLabel"
    _integration_warning_label.text = \
        "No AI client has MCP configured for this project. Set one up in Integrations."
    _integration_warning_label.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
    _integration_warning_label.size_flags_horizontal = Control.SIZE_EXPAND_FILL
    _integration_warning_label.add_theme_color_override(
        "font_color",
        COLOR_WARNING
    )
    row.add_child(_integration_warning_label)

    var button := Button.new()
    button.name = "IntegrationWarningButton"
    button.text = "Open Integrations"
    button.tooltip_text = \
        "Open the Integrations tab to install MCP and workflow-skill support."
    button.size_flags_vertical = Control.SIZE_SHRINK_CENTER
    button.pressed.connect(_on_integration_warning_pressed)
    row.add_child(button)
    return panel

func _build_integration_error_banner() -> PanelContainer:
    var panel := PanelContainer.new()
    panel.name = "IntegrationErrorBanner"
    panel.visible = false
    panel.size_flags_horizontal = Control.SIZE_EXPAND_FILL

    var style := StyleBoxFlat.new()
    style.bg_color = Color(0.35, 0.1, 0.1, 0.65)
    style.set_corner_radius_all(4)
    style.set_content_margin_all(8)
    panel.add_theme_stylebox_override("panel", style)

    _integration_error_label = _new_value_label("IntegrationErrorLabel")
    _integration_error_label.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
    _integration_error_label.add_theme_color_override(
        "font_color",
        COLOR_ERROR
    )
    panel.add_child(_integration_error_label)
    return panel

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
    client_label.theme_type_variation = "HeaderSmall"
    _content_labels.append(client_label)
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
    _content_labels.append(label)
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
    _key_labels.append(key)
    _content_labels.append(key)
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
    _backend_status_label.tooltip_text = _backend_status_tooltip()
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
        mcp_label.tooltip_text = detail
        skill_label.tooltip_text = detail

        button.text = _integrations.button_text(client_id)
        button.disabled = controls_disabled

    _refresh_integration_error()
    _refresh_integration_warning()

func _refresh_integration_error() -> void:
    var error_text := _integrations.last_error

    _integration_error_banner.visible = not error_text.is_empty()

    if _integration_error_banner.visible:
        _integration_error_label.text = "Integration error: %s" % error_text
    else:
        _integration_error_label.text = ""

func _refresh_integration_warning() -> void:
    _integration_warning.visible = \
        _integrations.is_status_loaded() \
        and _integrations.last_error.is_empty() \
        and not _integrations.has_any_mcp_configured() \
        and _integrations.all_mcp_missing()

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
            var suffix := ""

            if not _supervisor.last_error.is_empty():
                suffix = ": %s" % _supervisor.last_error

            if _supervisor.has_process():
                return "Error (backend still running)%s" % suffix

            return "Error (backend stopped)%s" % suffix
        _:
            return "Unknown"

func _backend_status_tooltip() -> String:
    var parts: PackedStringArray = []

    if _supervisor.is_bundled():
        parts.append("Executable: %s" % _supervisor.get_executable_path())

    if not _supervisor.last_error.is_empty():
        parts.append("Error: %s" % _supervisor.last_error)

    return "\n".join(parts)

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

func _on_integration_warning_pressed() -> void:
    _tabs.current_tab = TAB_INTEGRATIONS

func _read_addon_version() -> String:
    var config := ConfigFile.new()

    if config.load(PLUGIN_CONFIG_PATH) != OK:
        return "unknown"

    return str(config.get_value("plugin", "version", "unknown"))
