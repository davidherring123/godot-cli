@tool
extends EditorPlugin

const McpBackendSupervisor = preload("res://addons/godot_mcp/mcp/backend_supervisor.gd")
const McpPanel = preload("res://addons/godot_mcp/ui/mcp_panel.gd")

const INSTANCES_DIR := "res://.godot/godot-mcp/instances"

var request_handler: RequestHandler
var bridge: Bridge
var mcp_backend: McpBackendSupervisor
var mcp_panel: McpPanel
var instance_path := ""

func _enter_tree() -> void:
    request_handler = RequestHandler.new(get_editor_interface(), get_undo_redo())
    bridge = Bridge.new(request_handler)

    var error := bridge.start()

    if error != OK:
        push_error("godot-mcp: failed to start bridge")
        bridge = null
        return

    _register_instance(bridge.get_port())

    mcp_backend = McpBackendSupervisor.new()
    mcp_panel = McpPanel.new(mcp_backend)
    add_control_to_bottom_panel(mcp_panel, "Godot MCP")
    mcp_panel.set_bridge_status(true, bridge.get_port())

    # The bundled backend is an implementation detail of the addon. Start it
    # automatically so installation and enabling the plugin complete onboarding.
    if mcp_backend.is_bundled():
        mcp_backend.start()

    set_process(true)

    print(
        "godot-mcp: bridge listening on %s:" % Bridge.HOST,
        bridge.get_port()
    )

func _process(delta: float) -> void:
    if bridge != null:
        bridge.poll()

    if mcp_backend != null:
        mcp_backend.poll(delta)

func _exit_tree() -> void:
    set_process(false)

    if mcp_backend != null:
        mcp_backend.shutdown()
        mcp_backend = null

    if mcp_panel != null:
        remove_control_from_bottom_panel(mcp_panel)
        mcp_panel.queue_free()
        mcp_panel = null

    if bridge != null:
        bridge.stop()
        bridge = null

    if not instance_path.is_empty() \
    and FileAccess.file_exists(instance_path):
        DirAccess.remove_absolute(instance_path)

    print("godot-mcp: plugin disabled")

func _register_instance(port: int) -> void:
    var instances_dir := ProjectSettings.globalize_path(INSTANCES_DIR)

    var error := DirAccess.make_dir_recursive_absolute(instances_dir)

    if error != OK:
        push_error("godot-mcp: failed to create instances directory")
        return

    _vc_ignore_instance(instances_dir)

    var pid := OS.get_process_id()

    instance_path = "%s/%d.json" % [instances_dir, pid]

    var file := FileAccess.open(instance_path, FileAccess.WRITE)

    if file == null:
        push_error("godot-mcp: failed to create instance file")
        return

    var data := {
        "pid": pid,
        "port": port,
        "project": ProjectSettings.globalize_path("res://"),
        "version": "dev",
        "executable": OS.get_executable_path()
    }

    file.store_string(JSON.stringify(data, "\t"))

func _vc_ignore_instance(instances_dir: String) -> void:
    var ignore_path := instances_dir.path_join(".gitignore")

    if FileAccess.file_exists(ignore_path):
        return

    var file := FileAccess.open(ignore_path, FileAccess.WRITE)

    if file != null:
        file.store_string("*.json\n")
