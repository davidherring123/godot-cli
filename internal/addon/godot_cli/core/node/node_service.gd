class_name NodeService
extends ActionService

var scene_context: EditorSceneContext
var undo_redo: EditorUndoRedoManager

func _init(context: EditorSceneContext, history: EditorUndoRedoManager) -> void:
    scene_context = context
    undo_redo = history

func get_actions() -> Dictionary:
    return {
        "node.inspect": ActionHandler.new(inspect, true, false),
        "node.set": ActionHandler.new(set_properties, true, true),
    }

func inspect(request: Request) -> Response:
    var node_path = request.params.get("node")

    if not node_path is String or node_path.is_empty():
        return Response.failure(
            ErrorCode.INVALID_ARGUMENT,
            "A node path is required"
        )

    var root := scene_context.editor.get_edited_scene_root()
    var node := scene_context.find_node(node_path)

    if node == null:
        return Response.failure(
            ErrorCode.NOT_FOUND,
            "Could not find node: %s" % node_path
        )

    var requested = request.params.get("properties", [])

    if not requested is Array:
        return Response.failure(
            ErrorCode.INVALID_ARGUMENT,
            "properties must be an array of property names"
        )

    var filters := {}

    for property_name in requested:
        if not property_name is String or property_name.is_empty():
            return Response.failure(
                ErrorCode.INVALID_ARGUMENT,
                "Property filters must be nonempty strings"
            )

        filters[property_name] = false

    var result := {
        "scene": root.scene_file_path,
        "path": str(root.get_path_to(node)),
        "name": str(node.name),
        "type": node.get_class(),
        "properties": _get_properties(node, filters)
    }

    for property_name in filters:
        if not filters[property_name]:
            return Response.failure(
                ErrorCode.NOT_FOUND,
                "Unknown or non-editor-visible property: %s" % property_name
            )

    return Response.success(result)

func set_properties(request: Request) -> Response:
    var node_path = request.params.get("node")

    if not node_path is String or node_path.is_empty():
        return Response.failure(ErrorCode.INVALID_ARGUMENT, "A node path is required")

    var values = request.params.get("properties")

    if not values is Dictionary or values.is_empty():
        return Response.failure(
            ErrorCode.INVALID_ARGUMENT,
            "A nonempty properties object is required"
        )

    var root := scene_context.editor.get_edited_scene_root()
    var node := scene_context.find_node(node_path)

    if node == null:
        return Response.failure(
            ErrorCode.NOT_FOUND,
            "Node not found in the active scene: %s" % node_path
        )

    if node != root and node.owner != root:
        if node.owner == null or not root.is_editable_instance(node.owner):
            return Response.failure(
                ErrorCode.FAILED_PRECONDITION,
                "Enable Editable Children for this instance before editing its child nodes"
            )

    var definitions := {}

    for property in node.get_property_list():
        definitions[property.name] = property

    var names: Array = values.keys()

    names.sort()

    var converted := {}
    var previous := {}

    for property_name in names:
        if not definitions.has(property_name):
            return Response.failure(
                ErrorCode.NOT_FOUND,
                "Unknown property: %s" % property_name
            )

        var definition: Dictionary = definitions[property_name]
        var usage: int = definition.get("usage", 0)

        if (usage & PROPERTY_USAGE_EDITOR) == 0 or (usage & PROPERTY_USAGE_READ_ONLY) != 0:
            return Response.failure(
                ErrorCode.FAILED_PRECONDITION,
                "Property is not editor-writable: %s" % property_name
            )

        if property_name in ["name", "owner", "script", "scene_file_path", "unique_name_in_owner"]:
            return Response.failure(
                ErrorCode.UNSUPPORTED,
                "Structural property changes are not supported by node.set: %s" % property_name
            )

        var decoded := VariantDeserializer.deserialize(values[property_name], definition.type)

        if decoded.has("error"):
            return Response.failure(
                decoded.get("code", ErrorCode.INVALID_ARGUMENT),
                "%s: %s" % [property_name, decoded.error]
            )

        converted[property_name] = decoded.value
        previous[property_name] = node.get(property_name)

    var changed := false

    for property_name in names:
        if converted[property_name] != previous[property_name]:
            changed = true
            break

    if changed:
        undo_redo.create_action(
            "godot-cli: Set %s properties" % node.name,
            UndoRedo.MERGE_DISABLE,
            root,
            true
        )

        undo_redo.add_undo_method(node, "notify_property_list_changed")

        for property_name in names:
            undo_redo.add_do_property(node, property_name, converted[property_name])
            undo_redo.add_undo_property(node, property_name, previous[property_name])

        undo_redo.add_do_method(node, "notify_property_list_changed")
        undo_redo.commit_action()

    var result := {}

    for property_name in names:
        result[property_name] = VariantSerializer.serialize(node.get(property_name))

    return Response.success({
        "scene": root.scene_file_path,
        "path": str(root.get_path_to(node)),
        "properties": result,
        "changed": changed,
    })

func _get_properties(node: Node, filters: Dictionary = {}) -> Array[Dictionary]:
    var properties: Array[Dictionary] = []

    for property in node.get_property_list():
        var usage: int = property.get("usage", 0)
        var property_type: int = property.get("type", TYPE_NIL)

        if (usage & PROPERTY_USAGE_EDITOR) == 0:
            continue

        if property_type == TYPE_NIL:
            continue

        var property_name: String = property.get("name", "")

        if not filters.is_empty() and not filters.has(property_name):
            continue

        if filters.has(property_name):
            filters[property_name] = true

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
