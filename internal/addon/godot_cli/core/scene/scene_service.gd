class_name SceneService
extends ActionService

var scene_context: EditorSceneContext

func _init(context: EditorSceneContext) -> void:
    scene_context = context

func get_actions() -> Dictionary:
    return {
        "scene.current": ActionHandler.new(current, true, false),
        "scene.open": ActionHandler.new(open_scene, false, false),
        "scene.tree": ActionHandler.new(get_tree, true, false),
        "scene.save": ActionHandler.new(save, true, false),
    }

func current(_request: Request) -> Response:
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
