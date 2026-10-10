class_name NodeService
extends ActionService

const STRUCTURAL_PROPERTIES := [
    "name",
    "owner",
    "script",
    "scene_file_path",
    "unique_name_in_owner",
]

var scene_context: EditorSceneContext
var undo_redo: EditorUndoRedoManager

func _init(context: EditorSceneContext, history: EditorUndoRedoManager) -> void:
    scene_context = context
    undo_redo = history

func get_actions() -> Dictionary:
    return {
        "node.inspect": ActionHandler.new(inspect, true, false),
        "node.set": ActionHandler.new(set_properties, true, true),
        "node.add": ActionHandler.new(add, true, true),
        "node.delete": ActionHandler.new(delete, true, true),
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

    var editable_error := _editable_error(node, root, "editing")

    if editable_error != null:
        return editable_error

    var decoded := _decode_properties(node, values)

    if decoded.has("response"):
        return decoded.response

    var converted: Dictionary = decoded.converted
    var previous: Dictionary = decoded.previous
    var names: Array = converted.keys()

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

func add(request: Request) -> Response:
    var node_type = request.params.get("type")

    if not node_type is String or node_type.is_empty():
        return Response.failure(
            ErrorCode.INVALID_ARGUMENT,
            "A node type is required"
        )

    var parent_path = request.params.get("parent")

    if not parent_path is String or parent_path.is_empty():
        return Response.failure(
            ErrorCode.INVALID_ARGUMENT,
            "A parent node path is required"
        )

    var requested_name = request.params.get("name", "")

    if not requested_name is String:
        return Response.failure(ErrorCode.INVALID_ARGUMENT, "name must be a string")

    var values = request.params.get("properties", {})

    if not values is Dictionary:
        return Response.failure(
            ErrorCode.INVALID_ARGUMENT,
            "properties must be an object"
        )

    var root := scene_context.editor.get_edited_scene_root()
    var parent := scene_context.find_node(parent_path)

    if parent == null:
        return Response.failure(
            ErrorCode.NOT_FOUND,
            "Parent node not found in the active scene: %s" % parent_path
        )

    var editable_error := _editable_error(parent, root, "adding")

    if editable_error != null:
        return editable_error

    if not requested_name.is_empty():
        if requested_name != requested_name.validate_node_name():
            return Response.failure(
                ErrorCode.INVALID_ARGUMENT,
                "Invalid node name: %s" % requested_name
            )

        if _has_child_named(parent, requested_name):
            return Response.failure(
                ErrorCode.CONFLICT,
                "A sibling node is already named: %s" % requested_name
            )

    var node := _instantiate(node_type)

    if node == null:
        return Response.failure(
            ErrorCode.INVALID_ARGUMENT,
            "Unknown or non-instantiable node type: %s" % node_type
        )

    node.name = requested_name if not requested_name.is_empty() else node_type

    if values.has("script"):
        var decoded_script := _decode_properties(
            node,
            {"script": values["script"]},
            ["script"]
        )

        if decoded_script.has("response"):
            node.free()

            return decoded_script.response

        node.set_script(decoded_script.converted["script"])

    var remaining: Dictionary = values.duplicate()

    remaining.erase("script")

    var decoded := _decode_properties(node, remaining)

    if decoded.has("response"):
        node.free()

        return decoded.response

    undo_redo.create_action(
        "godot-cli: Add %s" % node.name,
        UndoRedo.MERGE_DISABLE,
        root,
        true
    )

    undo_redo.add_do_method(parent, "add_child", node, requested_name.is_empty())
    undo_redo.add_do_method(node, "set_owner", root)

    for property_name in decoded.converted:
        undo_redo.add_do_property(node, property_name, decoded.converted[property_name])

    undo_redo.add_undo_method(parent, "remove_child", node)
    undo_redo.commit_action()

    var result := {}

    for property_name in values:
        result[property_name] = VariantSerializer.serialize(node.get(property_name))

    return Response.success({
        "scene": root.scene_file_path,
        "path": str(root.get_path_to(node)),
        "name": str(node.name),
        "type": node.get_class(),
        "properties": result,
    })

func delete(request: Request) -> Response:
    var node_path = request.params.get("node")

    if not node_path is String or node_path.is_empty():
        return Response.failure(ErrorCode.INVALID_ARGUMENT, "A node path is required")

    var root := scene_context.editor.get_edited_scene_root()
    var node := scene_context.find_node(node_path)

    if node == null:
        return Response.failure(
            ErrorCode.NOT_FOUND,
            "Node not found in the active scene: %s" % node_path
        )

    if node == root:
        return Response.failure(
            ErrorCode.FAILED_PRECONDITION,
            "The scene root cannot be deleted"
        )

    var editable_error := _editable_error(node, root, "deleting")

    if editable_error != null:
        return editable_error

    var path := str(root.get_path_to(node))
    var parent := node.get_parent()
    var index := node.get_index()

    undo_redo.create_action(
        "godot-cli: Delete %s" % node.name,
        UndoRedo.MERGE_DISABLE,
        root
    )

    undo_redo.add_do_method(parent, "remove_child", node)
    undo_redo.add_undo_method(parent, "add_child", node, true)
    undo_redo.add_undo_method(node, "set_owner", root)
    undo_redo.add_undo_method(parent, "move_child", node, index)
    undo_redo.commit_action()

    return Response.success({
        "scene": root.scene_file_path,
        "path": path,
        "deleted": true,
    })

func _instantiate(node_type: String) -> Node:
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

func _has_child_named(parent: Node, child_name: String) -> bool:
    for child in parent.get_children():
        if str(child.name) == child_name:
            return true

    return false

func _editable_error(node: Node, root: Node, operation: String) -> Response:
    if node == root or node.owner == root:
        return null

    if node.owner == null or not root.is_editable_instance(node.owner):
        return Response.failure(
            ErrorCode.FAILED_PRECONDITION,
            "Enable Editable Children for this instance before %s its child nodes" % operation
        )

    return null

func _decode_properties(
    node: Node,
    values: Dictionary,
    exempt: Array = []
) -> Dictionary:
    var definitions := {}

    for property in node.get_property_list():
        definitions[property.name] = property

    var names: Array = values.keys()

    names.sort()

    var converted := {}
    var previous := {}

    for property_name in names:
        if not definitions.has(property_name):
            return _decode_failure(
                ErrorCode.NOT_FOUND,
                "Unknown property: %s" % property_name
            )

        var definition: Dictionary = definitions[property_name]
        var usage: int = definition.get("usage", 0)

        if (usage & PROPERTY_USAGE_EDITOR) == 0 or (usage & PROPERTY_USAGE_READ_ONLY) != 0:
            return _decode_failure(
                ErrorCode.FAILED_PRECONDITION,
                "Property is not editor-writable: %s" % property_name
            )

        if property_name in STRUCTURAL_PROPERTIES and not exempt.has(property_name):
            return _decode_failure(
                ErrorCode.UNSUPPORTED,
                "Structural property changes are not supported: %s" % property_name
            )

        var decoded := VariantDeserializer.deserialize(
            values[property_name],
            definition.type,
            definition.get("hint", PROPERTY_HINT_NONE),
            str(definition.get("hint_string", ""))
        )

        if decoded.has("error"):
            return _decode_failure(
                decoded.get("code", ErrorCode.INVALID_ARGUMENT),
                "%s: %s" % [property_name, decoded.error]
            )

        converted[property_name] = decoded.value
        previous[property_name] = node.get(property_name)

    return {
        "converted": converted,
        "previous": previous,
    }

func _decode_failure(code: String, message: String) -> Dictionary:
    return {
        "response": Response.failure(code, message)
    }

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
