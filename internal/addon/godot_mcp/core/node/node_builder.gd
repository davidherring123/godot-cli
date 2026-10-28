class_name NodeBuilder
extends RefCounted

static func instantiate(node_type: String) -> Node:
    if ClassDB.class_exists(node_type):
        if not ClassDB.can_instantiate(node_type):
            return null

        var instance = ClassDB.instantiate(node_type)

        if instance is Node:
            return instance

        return null

    for entry in ProjectSettings.get_global_class_list():
        if str(entry.get("class", "")) != node_type:
            continue

        var script: Script = load(entry.get("path", ""))

        if script == null:
            return null

        var instance = script.new()

        if instance is Node:
            return instance

        return null

    return null
