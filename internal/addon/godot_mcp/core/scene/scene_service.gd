class_name SceneService
extends ActionService

var scene_context: EditorSceneContext

func _init(context: EditorSceneContext) -> void:
    scene_context = context

func get_actions() -> Dictionary:
    return {
        "scene.current": ActionHandler.new(current, false, false, true),
        "scene.open": ActionHandler.new(open_scene, false, false, false),
        "scene.create": ActionHandler.new(create_scene, false, false, false),
        "scene.tree": ActionHandler.new(get_tree, true, false, true),
        "scene.save": ActionHandler.new(save, true, false, true),
        "validate.scene": ActionHandler.new(validate_scene, true, false, true),
    }

func current(request: Request) -> Response:
    var expected = request.params.get("expected_scene", "")

    if not expected is String:
        return Response.failure(
            ErrorCode.INVALID_ARGUMENT,
            "expected_scene must be a string"
        )

    var root := scene_context.editor.get_edited_scene_root()
    var error := scene_context.expected_scene_error(root, expected)

    if error != null:
        return error

    return Response.success(scene_context.describe())

func open_scene(request: Request) -> Response:
    var path = request.params.get("scene")

    if not path is String or not path.begins_with("res://"):
        return Response.failure(
            ErrorCode.INVALID_ARGUMENT,
            "A res:// scene path is required"
        )

    path = path.simplify_path()

    if not path.begins_with("res://"):
        return Response.failure(
            ErrorCode.INVALID_ARGUMENT,
            "Scene must be inside the project"
        )

    return await _activate_scene(path)

func create_scene(request: Request) -> Response:
    var path = request.params.get("scene")

    if not path is String or not path.begins_with("res://"):
        return Response.failure(
            ErrorCode.INVALID_ARGUMENT,
            "A res:// scene path is required"
        )

    path = path.simplify_path()

    if not path.begins_with("res://"):
        return Response.failure(
            ErrorCode.INVALID_ARGUMENT,
            "Scene must be inside the project"
        )

    var extension: String = path.get_extension().to_lower()

    if extension != "tscn" and extension != "scn":
        return Response.failure(
            ErrorCode.INVALID_ARGUMENT,
            "Scene path must end with .tscn or .scn: %s" % path
        )

    var root_type = request.params.get("root_type")

    if not root_type is String or root_type.is_empty():
        return Response.failure(
            ErrorCode.INVALID_ARGUMENT,
            "A root node type is required"
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

    if FileAccess.file_exists(path):
        return Response.failure(
            ErrorCode.CONFLICT,
            "A file already exists at: %s" % path
        )

    var root := NodeBuilder.instantiate(root_type)

    if root == null:
        return Response.failure(
            ErrorCode.INVALID_ARGUMENT,
            "Unknown or non-instantiable node type: %s" % root_type
        )

    if requested_name.is_empty():
        requested_name = path.get_file().get_basename().validate_node_name()

    if requested_name.is_empty() or requested_name != requested_name.validate_node_name():
        root.free()

        return Response.failure(
            ErrorCode.INVALID_ARGUMENT,
            "Invalid root node name: %s" % requested_name
        )

    root.name = requested_name

    var property_error := PropertyCodec.apply_properties(root, values)

    if property_error != null:
        root.free()

        return property_error

    var packed := PackedScene.new()
    var pack_error := packed.pack(root)

    root.free()

    if pack_error != OK:
        return Response.failure(
            ErrorCode.ACTION_FAILED,
            "Could not build scene: %s" % error_string(pack_error)
        )

    var directory: String = path.get_base_dir()

    if not DirAccess.dir_exists_absolute(directory):
        var directory_error := DirAccess.make_dir_recursive_absolute(directory)

        if directory_error != OK:
            return Response.failure(
                ErrorCode.ACTION_FAILED,
                "Could not create %s: %s" % [directory, error_string(directory_error)]
            )

    var save_error := ResourceSaver.save(packed, path)

    if save_error != OK:
        return Response.failure(
            ErrorCode.ACTION_FAILED,
            "Could not save scene %s: %s" % [path, error_string(save_error)]
        )

    var open_response = await _activate_scene(path)

    if not open_response.code.is_empty():
        return open_response

    var active := scene_context.editor.get_edited_scene_root()
    var serialized := {}

    for property_name in values:
        serialized[property_name] = VariantSerializer.serialize(active.get(property_name))

    var result: Dictionary = open_response.result
    result["properties"] = serialized

    return Response.success(result)

func _activate_scene(path: String) -> Response:
    var editor := scene_context.editor
    var root := editor.get_edited_scene_root()

    if root != null and root.scene_file_path == path:
        return Response.success(scene_context.describe())

    if not editor.get_open_scenes().has(path):
        if not ResourceLoader.exists(path, "PackedScene"):
            return Response.failure(
                ErrorCode.NOT_FOUND,
                "Scene not found: %s" % path
            )

        if not ResourceLoader.load(path, "PackedScene") is PackedScene:
            return Response.failure(
                ErrorCode.ACTION_FAILED,
                "Could not load scene: %s" % path
            )

    editor.open_scene_from_path(path)

    var deadline := Time.get_ticks_msec() + 10000

    while Time.get_ticks_msec() < deadline:
        await editor.get_base_control().get_tree().process_frame

        root = editor.get_edited_scene_root()

        if root != null and root.scene_file_path == path:
            return Response.success(scene_context.describe())

    return Response.failure(
        ErrorCode.ACTION_FAILED,
        "Editor did not activate scene: %s" % path
    )

func get_tree(_request: Request) -> Response:
    var root := scene_context.editor.get_edited_scene_root()

    return Response.success({
        "scene": root.scene_file_path,
        "root": _node_to_dict(root, root),
    })

func save(_request: Request) -> Response:
    return scene_context.save()

func validate_scene(_request: Request) -> Response:
    var root := scene_context.editor.get_edited_scene_root()
    var errors: Array[String] = []

    _collect_script_errors(root, errors, {})
    _collect_serialization_errors(root, errors)

    var description := scene_context.describe()

    return Response.success({
        "valid": errors.is_empty(),
        "scene": description["scene"],
        "unsaved": description["unsaved"],
        "errors": errors,
    })

func _collect_script_errors(
    node: Node,
    errors: Array[String],
    seen: Dictionary
) -> void:
    var script := node.get_script()

    if script != null:
        var path := str(script.resource_path)

        if not path.is_empty() and not seen.has(path):
            seen[path] = true

            var parse_error := _parse_script(path)

            if parse_error != OK:
                errors.append(
                    "Script does not parse (%s): %s"
                    % [error_string(parse_error), path]
                )

    for child in node.get_children():
        _collect_script_errors(child, errors, seen)

func _parse_script(path: String) -> int:
    var fresh := ResourceLoader.load(
        path, "Script", ResourceLoader.CACHE_MODE_IGNORE
    ) as GDScript

    if fresh == null or fresh.can_instantiate():
        return OK

    return fresh.reload()

func _collect_serialization_errors(root: Node, errors: Array[String]) -> void:
    var packed := PackedScene.new()
    var pack_error := packed.pack(root)

    if pack_error != OK:
        errors.append("Could not pack the scene: %s" % error_string(pack_error))
        return

    var copy := packed.instantiate()

    if copy == null:
        errors.append("Could not instantiate the packed scene")
        return

    copy.free()

func _node_to_dict(node: Node, root: Node) -> Dictionary:
    var children: Array[Dictionary] = []

    for child in node.get_children():
        children.append(_node_to_dict(child, root))

    return {
        "name": str(node.name),
        "type": node.get_class(),
        "path": str(root.get_path_to(node)),
        "children": children,
    }
