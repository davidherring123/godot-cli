class_name NodeService
extends CommandService


func get_commands() -> Dictionary:
    return {
        "node.inspect": inspect
    }


func inspect(request: Request) -> Response:
    var scene_path = request.params.get("scene")
    var node_path = request.params.get("node")

    if not scene_path is String or scene_path.is_empty():
        return Response.failure(
            "invalid_scene_path",
            "A scene path is required"
        )

    if not node_path is String or node_path.is_empty():
        return Response.failure(
            "invalid_node_path",
            "A node path is required"
        )

    var packed_scene := ResourceLoader.load(
        scene_path,
        "PackedScene"
    ) as PackedScene

    if packed_scene == null:
        return Response.failure(
            "scene_load_failed",
            "Could not load scene: %s" % scene_path
        )

    var root := packed_scene.instantiate()

    if root == null:
        return Response.failure(
            "scene_load_failed",
            "Could not instantiate scene: %s" % scene_path
        )

    var node: Node = root

    if node_path != ".":
        node = root.get_node_or_null(NodePath(node_path))

    if node == null:
        root.free()

        return Response.failure(
            "node_not_found",
            "Could not find node: %s" % node_path
        )

    var result := {
        "scene": scene_path,
        "path": node_path,
        "name": str(node.name),
        "type": node.get_class(),
        "properties": _get_properties(node)
    }

    root.free()

    return Response.success(result)

func _get_properties(node: Node) -> Array[Dictionary]:
    var properties: Array[Dictionary] = []

    for property in node.get_property_list():
        var usage: int = property.get("usage", 0)
        var property_type: int = property.get("type", TYPE_NIL)

        if (usage & PROPERTY_USAGE_EDITOR) == 0:
            continue

        if property_type == TYPE_NIL:
            continue

        var property_name: String = property.get("name", "")
        var value = node.get(property_name)

        var property_hint: int = property.get("hint", PROPERTY_HINT_NONE)
        var hint_string: String = property.get("hint_string", "")
        
        var serialized_hint := PropertyHintSerializer.serialize(
            property_hint,
            hint_string,
            value
        )
        
        var serialized_property := {
            "name": property_name,
            "type": type_string(property_type),
            "value": VariantSerializer.serialize(value)
        }

        if serialized_hint != null:
            serialized_property["hint"] = serialized_hint

        properties.append(serialized_property)

    return properties