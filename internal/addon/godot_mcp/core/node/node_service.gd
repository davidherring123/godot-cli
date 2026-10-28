class_name NodeService
extends ActionService

# Node.DUPLICATE_FROM_EDITOR is only available in editor builds and is not
# exposed to scripting; it makes scene instances duplicate with edit state.
const DUPLICATE_FROM_EDITOR := 32

var scene_context: EditorSceneContext
var undo_redo: Object

func _init(context: EditorSceneContext, history: Object) -> void:
    scene_context = context
    undo_redo = history

func get_actions() -> Dictionary:
    return {
        "node.inspect": ActionHandler.new(inspect, true, false, true),
        "node.set": ActionHandler.new(set_properties, true, true, true),
        "node.add": ActionHandler.new(add, true, true, true),
        "node.instantiate": ActionHandler.new(instantiate_scene, true, true, true),
        "node.rename": ActionHandler.new(rename, true, true, true),
        "node.duplicate": ActionHandler.new(duplicate_node, true, true, true),
        "node.move": ActionHandler.new(move_node, true, true, true),
        "node.delete": ActionHandler.new(delete, true, true, true),
    }

func inspect(request: Request) -> Response:
    var node_path = request.params.get("node")

    if not node_path is String or node_path.is_empty():
        return Response.failure(
            ErrorCode.INVALID_ARGUMENT,
            "A node path is required"
        )

    var root := scene_context.edited_root()
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
        "properties": PropertyCodec.collect_properties(node, filters)
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

    var root := scene_context.edited_root()
    var node := scene_context.find_node(node_path)

    if node == null:
        return Response.failure(
            ErrorCode.NOT_FOUND,
            "Node not found in the active scene: %s" % node_path
        )

    var editable_error := _editable_error(node, root, "editing")

    if editable_error != null:
        return editable_error

    var decoded := PropertyCodec.decode_properties(node, values)

    if decoded.error != null:
        return decoded.error

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
            "godot-mcp: Set %s properties" % node.name,
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

    var root := scene_context.edited_root()
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
        var name_error := _name_error(parent, requested_name)

        if name_error != null:
            return name_error

    var node := NodeBuilder.instantiate(node_type)

    if node == null:
        return Response.failure(
            ErrorCode.INVALID_ARGUMENT,
            "Unknown or non-instantiable node type: %s" % node_type
        )

    node.name = requested_name if not requested_name.is_empty() else node_type

    if values.has("script"):
        var decoded_script := PropertyCodec.decode_properties(
            node,
            {"script": values["script"]}
        )

        if decoded_script.error != null:
            node.free()

            return decoded_script.error

        node.set_script(decoded_script.converted["script"])

    var remaining: Dictionary = values.duplicate()

    remaining.erase("script")

    var decoded := PropertyCodec.decode_properties(node, remaining)

    if decoded.error != null:
        node.free()

        return decoded.error

    undo_redo.create_action(
        "godot-mcp: Add %s" % node.name,
        UndoRedo.MERGE_DISABLE,
        root,
        true
    )

    undo_redo.add_do_method(parent, "add_child", node, requested_name.is_empty())
    undo_redo.add_do_method(node, "set_owner", root)

    for property_name in decoded.converted:
        undo_redo.add_do_property(node, property_name, decoded.converted[property_name])

    undo_redo.add_do_reference(node)
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

func instantiate_scene(request: Request) -> Response:
    var scene_path = request.params.get("scene")

    if not scene_path is String or not scene_path.begins_with("res://"):
        return Response.failure(
            ErrorCode.INVALID_ARGUMENT,
            "A res:// scene path is required"
        )

    scene_path = scene_path.simplify_path()

    if not scene_path.begins_with("res://"):
        return Response.failure(
            ErrorCode.INVALID_ARGUMENT,
            "Scene must be inside the project"
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

    if values.has("script"):
        return Response.failure(
            ErrorCode.INVALID_ARGUMENT,
            "Set the script in the source scene instead of overriding it on an instance"
        )

    var root := scene_context.edited_root()
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
        var name_error := _name_error(parent, requested_name)

        if name_error != null:
            return name_error

    if not root.scene_file_path.is_empty() and scene_path == root.scene_file_path:
        return Response.failure(
            ErrorCode.FAILED_PRECONDITION,
            "The active scene cannot be instantiated into itself"
        )

    if not ResourceLoader.exists(scene_path, "PackedScene"):
        return Response.failure(
            ErrorCode.NOT_FOUND,
            "Scene not found: %s" % scene_path
        )

    var packed: PackedScene = ResourceLoader.load(scene_path, "PackedScene")

    if packed == null:
        return Response.failure(
            ErrorCode.ACTION_FAILED,
            "Could not load scene: %s" % scene_path
        )

    var instance := packed.instantiate(PackedScene.GEN_EDIT_STATE_INSTANCE)

    if instance == null:
        return Response.failure(
            ErrorCode.ACTION_FAILED,
            "Could not instantiate scene: %s" % scene_path
        )

    var dependency_error := _instance_dependency_error(instance, root.scene_file_path)

    if dependency_error != null:
        instance.free()

        return dependency_error

    if not requested_name.is_empty():
        instance.name = requested_name

    var decoded := PropertyCodec.decode_properties(instance, values)

    if decoded.error != null:
        instance.free()

        return decoded.error

    undo_redo.create_action(
        "godot-mcp: Instantiate %s" % scene_path,
        UndoRedo.MERGE_DISABLE,
        root,
        true
    )

    undo_redo.add_do_method(parent, "add_child", instance, requested_name.is_empty())
    undo_redo.add_do_method(instance, "set_owner", root)

    for property_name in decoded.converted:
        undo_redo.add_do_property(instance, property_name, decoded.converted[property_name])

    undo_redo.add_do_reference(instance)
    undo_redo.add_undo_method(parent, "remove_child", instance)
    undo_redo.commit_action()

    var result := {}

    for property_name in values:
        result[property_name] = VariantSerializer.serialize(instance.get(property_name))

    return Response.success({
        "scene": root.scene_file_path,
        "source": scene_path,
        "path": str(root.get_path_to(instance)),
        "name": str(instance.name),
        "type": instance.get_class(),
        "properties": result,
    })

func rename(request: Request) -> Response:
    var node_path = request.params.get("node")

    if not node_path is String or node_path.is_empty():
        return Response.failure(ErrorCode.INVALID_ARGUMENT, "A node path is required")

    var requested_name = request.params.get("name")

    if not requested_name is String or requested_name.is_empty():
        return Response.failure(ErrorCode.INVALID_ARGUMENT, "A new name is required")

    var root := scene_context.edited_root()
    var node := scene_context.find_node(node_path)

    if node == null:
        return Response.failure(
            ErrorCode.NOT_FOUND,
            "Node not found in the active scene: %s" % node_path
        )

    var editable_error := _editable_error(node, root, "renaming")

    if editable_error != null:
        return editable_error

    var old_name := str(node.name)
    var old_path := str(root.get_path_to(node))

    if requested_name == old_name:
        return Response.success({
            "scene": root.scene_file_path,
            "path": old_path,
            "name": old_name,
            "new_path": old_path,
            "new_name": old_name,
            "changed": false,
        })

    if node != root:
        var name_error := _name_error(node.get_parent(), requested_name)

        if name_error != null:
            return name_error

    var renames := _build_rename_map(node, requested_name)
    var updates := _writable_rename_updates(_collect_rename_updates(root, renames))
    var track_updates := _collect_track_updates(root, renames)

    undo_redo.create_action(
        "godot-mcp: Rename %s to %s" % [old_name, requested_name],
        UndoRedo.MERGE_DISABLE,
        root
    )

    undo_redo.add_do_method(node, "set_name", requested_name)

    for update in updates:
        undo_redo.add_do_property(update.object, update.property, update.value)
        undo_redo.add_undo_property(update.object, update.property, update.previous)

    for update in track_updates:
        undo_redo.add_do_method(update.animation, "track_set_path", update.index, update.path)
        undo_redo.add_undo_method(update.animation, "track_set_path", update.index, update.previous)

    undo_redo.add_undo_method(node, "set_name", old_name)
    undo_redo.commit_action()

    return Response.success({
        "scene": root.scene_file_path,
        "path": old_path,
        "name": old_name,
        "new_path": str(root.get_path_to(node)),
        "new_name": str(node.name),
        "changed": true,
    })

func duplicate_node(request: Request) -> Response:
    var node_path = request.params.get("node")

    if not node_path is String or node_path.is_empty():
        return Response.failure(ErrorCode.INVALID_ARGUMENT, "A node path is required")

    var requested_name = request.params.get("name", "")

    if not requested_name is String:
        return Response.failure(ErrorCode.INVALID_ARGUMENT, "name must be a string")

    var root := scene_context.edited_root()
    var node := scene_context.find_node(node_path)

    if node == null:
        return Response.failure(
            ErrorCode.NOT_FOUND,
            "Node not found in the active scene: %s" % node_path
        )

    if node == root:
        return Response.failure(
            ErrorCode.FAILED_PRECONDITION,
            "The scene root cannot be duplicated"
        )

    var editable_error := _editable_error(node, root, "duplicating")

    if editable_error != null:
        return editable_error

    var parent := node.get_parent()

    if not requested_name.is_empty():
        if requested_name != requested_name.validate_node_name():
            return Response.failure(
                ErrorCode.INVALID_ARGUMENT,
                "Invalid node name: %s" % requested_name
            )

        var name_error := _name_error(parent, requested_name)

        if name_error != null:
            return name_error

    var flags := (
        Node.DUPLICATE_SIGNALS
        | Node.DUPLICATE_GROUPS
        | Node.DUPLICATE_SCRIPTS
        | Node.DUPLICATE_USE_INSTANTIATION
        | DUPLICATE_FROM_EDITOR
    )

    var copy := node.duplicate(flags)

    if copy == null:
        return Response.failure(
            ErrorCode.ACTION_FAILED,
            "Could not duplicate node: %s" % node_path
        )

    if not requested_name.is_empty():
        copy.name = requested_name

    var owned := _duplicate_owned_nodes(copy)

    undo_redo.create_action(
        "godot-mcp: Duplicate %s" % node.name,
        UndoRedo.MERGE_DISABLE,
        root,
        true
    )

    undo_redo.add_do_method(parent, "add_child", copy, requested_name.is_empty())
    undo_redo.add_do_method(parent, "move_child", copy, node.get_index() + 1)

    for duplicate in owned:
        undo_redo.add_do_method(duplicate, "set_owner", root)

    undo_redo.add_do_reference(copy)
    undo_redo.add_undo_method(parent, "remove_child", copy)
    undo_redo.commit_action()

    return Response.success({
        "scene": root.scene_file_path,
        "path": str(root.get_path_to(node)),
        "name": str(node.name),
        "new_path": str(root.get_path_to(copy)),
        "new_name": str(copy.name),
        "type": copy.get_class(),
    })

func move_node(request: Request) -> Response:
    var node_path = request.params.get("node")

    if not node_path is String or node_path.is_empty():
        return Response.failure(ErrorCode.INVALID_ARGUMENT, "A node path is required")

    var parent_path = request.params.get("parent")

    if not parent_path is String or parent_path.is_empty():
        return Response.failure(ErrorCode.INVALID_ARGUMENT, "A parent node path is required")

    var requested_index = request.params.get("index", -1)

    if not _is_integer(requested_index):
        return Response.failure(ErrorCode.INVALID_ARGUMENT, "index must be an integer")

    if requested_index < 0 and requested_index != -1:
        return Response.failure(
            ErrorCode.INVALID_ARGUMENT,
            "index must be zero or greater; omit it to move the node to the end"
        )

    var keep_transform = request.params.get("keep_global_transform", true)

    if not keep_transform is bool:
        return Response.failure(
            ErrorCode.INVALID_ARGUMENT,
            "keep_global_transform must be a boolean"
        )

    var root := scene_context.edited_root()
    var node := scene_context.find_node(node_path)

    if node == null:
        return Response.failure(
            ErrorCode.NOT_FOUND,
            "Node not found in the active scene: %s" % node_path
        )

    var new_parent := scene_context.find_node(parent_path)

    if new_parent == null:
        return Response.failure(
            ErrorCode.NOT_FOUND,
            "Parent node not found in the active scene: %s" % parent_path
        )

    if node == root:
        return Response.failure(
            ErrorCode.FAILED_PRECONDITION,
            "The scene root cannot be moved"
        )

    if new_parent == node or node.is_ancestor_of(new_parent):
        return Response.failure(
            ErrorCode.FAILED_PRECONDITION,
            "A node cannot be moved under itself"
        )

    var editable_error := _editable_error(node, root, "moving")

    if editable_error != null:
        return editable_error

    editable_error = _editable_error(new_parent, root, "adding")

    if editable_error != null:
        return editable_error

    var old_parent := node.get_parent()
    var old_index := node.get_index()
    var old_name := str(node.name)
    var old_path := str(root.get_path_to(node))
    var same_parent := old_parent == new_parent

    var final_name := old_name

    if not same_parent:
        final_name = _unique_child_name(new_parent, old_name)

    var count_after := new_parent.get_child_count() + (0 if same_parent else 1)
    var final_index := int(requested_index)

    if final_index < 0:
        final_index = count_after - 1
    elif final_index >= count_after:
        return Response.failure(
            ErrorCode.INVALID_ARGUMENT,
            "index must be less than %d for this parent" % count_after
        )

    if same_parent and final_index == old_index and final_name == old_name:
        return Response.success({
            "scene": root.scene_file_path,
            "path": old_path,
            "name": old_name,
            "new_path": old_path,
            "new_name": old_name,
            "changed": false,
        })

    var renames := {}

    _fill_rename_map(node, NodePath(str(new_parent.get_path()) + "/" + final_name), renames)

    var updates := _writable_rename_updates(_collect_rename_updates(root, renames))
    var track_updates := _collect_track_updates(root, renames)
    var owner_changes := _move_owner_changes(node, new_parent, root)

    undo_redo.create_action(
        "godot-mcp: Move %s" % old_name,
        UndoRedo.MERGE_DISABLE,
        root
    )

    if not same_parent:
        undo_redo.add_do_method(node, "reparent", new_parent, keep_transform)

    if final_name != old_name:
        undo_redo.add_do_method(node, "set_name", final_name)

    undo_redo.add_do_method(new_parent, "move_child", node, final_index)

    for change in owner_changes:
        undo_redo.add_do_method(change.node, "set_owner", root)

    for update in updates:
        undo_redo.add_do_property(update.object, update.property, update.value)

    for update in track_updates:
        undo_redo.add_do_method(update.animation, "track_set_path", update.index, update.path)

    # Undo methods execute in the order they are added.
    if not same_parent:
        undo_redo.add_undo_method(node, "reparent", old_parent, keep_transform)

    undo_redo.add_undo_method(old_parent, "move_child", node, old_index)

    if final_name != old_name:
        undo_redo.add_undo_method(node, "set_name", old_name)

    for change in owner_changes:
        undo_redo.add_undo_method(change.node, "set_owner", change.owner)

    for update in track_updates:
        undo_redo.add_undo_method(update.animation, "track_set_path", update.index, update.previous)

    for update in updates:
        undo_redo.add_undo_property(update.object, update.property, update.previous)

    undo_redo.commit_action()

    return Response.success({
        "scene": root.scene_file_path,
        "path": old_path,
        "name": old_name,
        "new_path": str(root.get_path_to(node)),
        "new_name": str(node.name),
        "changed": true,
    })

func delete(request: Request) -> Response:
    var node_path = request.params.get("node")

    if not node_path is String or node_path.is_empty():
        return Response.failure(ErrorCode.INVALID_ARGUMENT, "A node path is required")

    var root := scene_context.edited_root()
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
        "godot-mcp: Delete %s" % node.name,
        UndoRedo.MERGE_DISABLE,
        root
    )

    undo_redo.add_do_method(parent, "remove_child", node)
    undo_redo.add_undo_method(parent, "add_child", node, true)
    undo_redo.add_undo_method(node, "set_owner", root)
    undo_redo.add_undo_method(parent, "move_child", node, index)

    undo_redo.add_undo_reference(node)
    undo_redo.commit_action()

    return Response.success({
        "scene": root.scene_file_path,
        "path": path,
        "deleted": true,
    })

func _has_child_named(parent: Node, child_name: String) -> bool:
    for child in parent.get_children():
        if str(child.name) == child_name:
            return true

    return false

func _name_error(parent: Node, requested_name: String) -> Response:
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

    return null

func _instance_dependency_error(instance: Node, destination_path: String) -> Response:
    var unsaved_scenes := scene_context.editor.get_unsaved_scenes()

    return _find_instance_dependency_error(instance, destination_path, unsaved_scenes)

func _find_instance_dependency_error(
    node: Node,
    destination_path: String,
    unsaved_scenes: PackedStringArray
) -> Response:
    var scene_path := node.scene_file_path

    if not scene_path.is_empty():
        if not destination_path.is_empty() and scene_path == destination_path:
            return Response.failure(
                ErrorCode.FAILED_PRECONDITION,
                "Cannot instantiate %s because the active scene is inside it" % scene_path
            )

        if unsaved_scenes.has(scene_path):
            return Response.failure(
                ErrorCode.FAILED_PRECONDITION,
                "Cannot instantiate %s because it has unsaved changes; save it first" % scene_path
            )

    for child in node.get_children():
        var error := _find_instance_dependency_error(child, destination_path, unsaved_scenes)

        if error != null:
            return error

    return null

func _build_rename_map(node: Node, new_name: String) -> Dictionary:
    var renames := {}

    _fill_rename_map(node, NodePathUtil.with_last_name(node.get_path(), new_name), renames)

    return renames

func _fill_rename_map(node: Node, new_path: NodePath, renames: Dictionary) -> void:
    renames[node] = new_path

    for child in node.get_children():
        _fill_rename_map(
            child,
            NodePath(str(new_path) + "/" + str(node.get_path_to(child))),
            renames
        )

func _collect_rename_updates(root: Node, renames: Dictionary) -> Array[Dictionary]:
    var updates: Array[Dictionary] = []

    _collect_node_rename_updates(root, renames, updates, {})

    return updates

# Prevalidation runs against a packed-scene copy whose built-in subresources
# are shared with the live scene. Node property updates on the copy are safe,
# but writing remapped NodePaths into those shared resources (Animation and
# custom NodePath-bearing Resource instances) would mutate live state. Dry-run
# still collects the updates so rename/move topology is validated, then drops
# only the resource-targeted writes.
func _writable_rename_updates(updates: Array[Dictionary]) -> Array[Dictionary]:
    if not scene_context.preview:
        return updates

    var writable: Array[Dictionary] = []

    for update in updates:
        if update.get("object") is Node:
            writable.append(update)

    return writable

func _collect_node_rename_updates(
    node: Node,
    renames: Dictionary,
    updates: Array[Dictionary],
    visited_resources: Dictionary
) -> void:
    for property in node.get_property_list():
        if not _recurse_into_property(property):
            continue

        var property_name: StringName = property.name
        var previous = node.get(property_name)
        var value := _remap_rename_value(
            node,
            previous,
            renames,
            false,
            updates,
            visited_resources
        )

        if value != previous:
            updates.append({
                "object": node,
                "property": property_name,
                "previous": previous,
                "value": value,
            })

    for child in node.get_children():
        _collect_node_rename_updates(child, renames, updates, visited_resources)

func _collect_resource_rename_updates(
    owner_node: Node,
    resource: Resource,
    renames: Dictionary,
    updates: Array[Dictionary],
    visited_resources: Dictionary
) -> void:
    for property in resource.get_property_list():
        if not _recurse_into_property(property):
            continue

        var property_name: StringName = property.name
        var previous = resource.get(property_name)
        var value := _remap_rename_value(
            owner_node,
            previous,
            renames,
            true,
            updates,
            visited_resources
        )

        if value != previous:
            updates.append({
                "object": resource,
                "property": property_name,
                "previous": previous,
                "value": value,
            })

func _remap_rename_value(
    owner_node: Node,
    value: Variant,
    renames: Dictionary,
    inside_resource: bool,
    updates: Array[Dictionary],
    visited_resources: Dictionary
) -> Variant:
    match typeof(value):
        TYPE_NODE_PATH:
            var path: NodePath = value

            if path.is_empty():
                return value

            if inside_resource and not owner_node.has_node(path):
                return value

            var updated = _update_renamed_path(owner_node, path, renames)

            return updated if updated != null else value
        TYPE_ARRAY:
            var array: Array = value
            var result: Array = array

            for index in array.size():
                var item := _remap_rename_value(
                    owner_node,
                    array[index],
                    renames,
                    inside_resource,
                    updates,
                    visited_resources
                )

                if item != array[index]:
                    if result == array:
                        result = array.duplicate()

                    result[index] = item

            return result
        TYPE_DICTIONARY:
            var dictionary: Dictionary = value
            var result: Dictionary = dictionary

            for key in dictionary:
                var item := _remap_rename_value(
                    owner_node,
                    dictionary[key],
                    renames,
                    inside_resource,
                    updates,
                    visited_resources
                )

                if item != dictionary[key]:
                    if result == dictionary:
                        result = dictionary.duplicate()

                    result[key] = item

            return result
        TYPE_OBJECT:
            var resource := value as Resource

            if resource == null:
                return value

            if resource is Animation or resource is Material:
                return value

            if not resource.is_built_in():
                return value

            if visited_resources.has(resource):
                return value

            visited_resources[resource] = true

            _collect_resource_rename_updates(
                owner_node,
                resource,
                renames,
                updates,
                visited_resources
            )

            return value

    return value

func _update_renamed_path(
    owner_node: Node,
    path: NodePath,
    renames: Dictionary
) -> Variant:
    var target := owner_node.get_node_or_null(path)

    if target == null:
        return null

    var subnames := ""

    if path.get_subname_count() > 0:
        subnames = ":" + str(path.get_concatenated_subnames())

    if renames.has(target):
        var new_target: NodePath = renames[target]
        var new_owner: NodePath = renames.get(owner_node, owner_node.get_path())

        return NodePath(str(NodePathUtil.relative(new_owner, new_target)) + subnames)

    if renames.has(owner_node):
        var new_owner: NodePath = renames[owner_node]
        var old_absolute := NodePathUtil.resolve(owner_node.get_path(), path)

        return NodePath(str(NodePathUtil.relative(new_owner, old_absolute)) + subnames)

    return null

func _recurse_into_property(property: Dictionary) -> bool:
    var usage: int = property.get("usage", 0)

    if (usage & (PROPERTY_USAGE_STORAGE | PROPERTY_USAGE_EDITOR)) == 0:
        return false

    # PROPERTY_HINT_NO_NODEPATH is not exposed to scripting. It only marks
    # string, AABB, and internal array properties in the engine, which are not
    # types remapped here, so the check is omitted.

    var property_type: int = property.get("type", TYPE_NIL)

    return (
        property_type == TYPE_OBJECT
        or property_type == TYPE_NODE_PATH
        or property_type == TYPE_ARRAY
        or property_type == TYPE_DICTIONARY
    )

func _collect_track_updates(root: Node, renames: Dictionary) -> Array[Dictionary]:
    var updates: Array[Dictionary] = []

    # A packed-scene preview shares built-in Animation resources with the live
    # scene. Never apply or record track remaps for a preview pass: doing so
    # would edit live animation tracks even during dry_run.
    if scene_context.preview:
        return updates

    var settings := scene_context.editor.get_editor_settings()

    if not bool(settings.get_setting("editors/animation/autorename_animation_tracks")):
        return updates

    _collect_mixer_track_updates(root, renames, updates)

    return updates

func _collect_mixer_track_updates(
    node: Node,
    renames: Dictionary,
    updates: Array[Dictionary]
) -> void:
    var mixer := node as AnimationMixer

    if mixer != null and not _mixer_ignores_renames(mixer):
        _collect_animation_updates(mixer, renames, updates)

    for child in node.get_children():
        _collect_mixer_track_updates(child, renames, updates)

func _mixer_ignores_renames(mixer: AnimationMixer) -> bool:
    var tree := mixer as AnimationTree

    if tree == null:
        return false

    return tree.get_node_or_null(tree.get_animation_player()) is AnimationPlayer

func _collect_animation_updates(
    mixer: AnimationMixer,
    renames: Dictionary,
    updates: Array[Dictionary]
) -> void:
    var mixer_root := mixer.get_node_or_null(mixer.get_root_node())

    if mixer_root == null:
        return

    var new_root: NodePath = renames.get(mixer_root, mixer_root.get_path())

    for animation_name in mixer.get_animation_list():
        var animation: Animation = mixer.get_animation(animation_name)

        if animation == null:
            continue

        for index in animation.get_track_count():
            if animation.track_is_imported(index):
                continue

            var track_path := animation.track_get_path(index)
            var target := mixer_root.get_node_or_null(track_path)

            if target == null or not renames.has(target):
                continue

            var subnames := ""

            if track_path.get_subname_count() > 0:
                subnames = ":" + str(track_path.get_concatenated_subnames())

            var new_path := NodePath(
                str(NodePathUtil.relative(new_root, renames[target])) + subnames
            )

            if new_path == track_path:
                continue

            updates.append({
                "animation": animation,
                "index": index,
                "previous": track_path,
                "path": new_path,
            })

func _duplicate_owned_nodes(node: Node) -> Array[Node]:
    var result: Array[Node] = []

    if node.owner == null:
        result.append(node)

    for child in node.get_children():
        result.append_array(_duplicate_owned_nodes(child))

    return result

func _move_owner_changes(node: Node, new_parent: Node, root: Node) -> Array[Dictionary]:
    var valid_ancestors := {}
    var current: Node = new_parent

    while current != null:
        valid_ancestors[current] = true
        current = current.get_parent()

    var changes: Array[Dictionary] = []

    _collect_move_owner_changes(node, valid_ancestors, root, changes)

    return changes

func _collect_move_owner_changes(
    node: Node,
    valid_ancestors: Dictionary,
    root: Node,
    changes: Array[Dictionary]
) -> void:
    if node.owner == null or (node.owner != root and not valid_ancestors.has(node.owner)):
        changes.append({"node": node, "owner": node.owner})

    valid_ancestors[node] = true

    for child in node.get_children():
        _collect_move_owner_changes(child, valid_ancestors, root, changes)

func _unique_child_name(parent: Node, base_name: String) -> String:
    if not _has_child_named(parent, base_name):
        return base_name

    var index := 2

    while _has_child_named(parent, base_name + str(index)):
        index += 1

    return base_name + str(index)

func _is_integer(value: Variant) -> bool:
    if value is int:
        return true

    return (
        value is float
        and is_finite(value)
        and floor(value) == value
        and abs(value) <= 9007199254740991.0
    )

func _editable_error(node: Node, root: Node, operation: String) -> Response:
    if node == root or node.owner == root:
        return null

    if node.owner == null or not root.is_editable_instance(node.owner):
        return Response.failure(
            ErrorCode.FAILED_PRECONDITION,
            "Enable Editable Children for this instance before %s its child nodes" % operation
        )

    return null
