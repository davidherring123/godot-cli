@tool
extends EditorPlugin

const INSTANCES_DIR := "res://.godot/godot-cli/instances"

var bridge: Bridge
var instance_path := ""

func _enter_tree() -> void:
    bridge = Bridge.new()

    var error := bridge.start()

    if error != OK:
        push_error("godot-cli: failed to start bridge")
        bridge = null
        return

    _register_instance(bridge.get_port())

    set_process(true)

    print(
        "godot-cli: bridge listening on %s:" % Bridge.HOST,
        bridge.get_port()
    )

func _process(_delta: float) -> void:
    if bridge != null:
        bridge.poll()

func _exit_tree() -> void:
    set_process(false)

    if bridge != null:
        bridge.stop()
        bridge = null
    
    if not instance_path.is_empty() \
    and FileAccess.file_exists(instance_path):
        DirAccess.remove_absolute(instance_path)
    
    print("godot-cli: plugin disabled")

func _register_instance(port: int) -> void:
    var instances_dir := ProjectSettings.globalize_path(INSTANCES_DIR)

    var error := DirAccess.make_dir_recursive_absolute(instances_dir)

    if error != OK:
        push_error("godot-cli: failed to create instances directory")
        return
    
    var pid := OS.get_process_id()

    instance_path = "%s/%d.json" % [instances_dir, pid]

    var file := FileAccess.open(instance_path, FileAccess.WRITE)

    if file == null:
        push_error("godot-cli: failed to create instance file")
        return

    var data := {
        "pid": pid,
        "port": port,
        "project": ProjectSettings.globalize_path("res://"),
        "version": "dev"
    }

    file.store_string(JSON.stringify(data, "\t"))
