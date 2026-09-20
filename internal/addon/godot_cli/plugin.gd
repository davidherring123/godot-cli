@tool
extends EditorPlugin

const INSTANCES_DIR := "res://.godot/godot-cli/instances"

var instance_path: String

func _enter_tree() -> void:
    var pid := OS.get_process_id()

    var error := DirAccess.make_dir_recursive_absolute(INSTANCES_DIR)

    if error != OK:
        push_error("godot-cli: failed to create instances directory")
        return
    
    instance_path = "%s/%d.json" % [INSTANCES_DIR, pid]

    var file := FileAccess.open(instance_path, FileAccess.WRITE)
    if file == null:
        push_error("godot-cli: failed to create instance file")
        return
    
    var data := {
        "pid": pid,
        "project": ProjectSettings.globalize_path("res://"),
        "version": "dev"
    }

    file.store_string(JSON.stringify(data, "\tn"))

    print("godot-cli: registered editor instance ", pid)

func _exit_tree() -> void:
    if instance_path.is_empty():
        return
    
    if FileAccess.file_exists(instance_path):
        DirAccess.remove_absolute(instance_path)
    
    print("Godot CLI disabled")