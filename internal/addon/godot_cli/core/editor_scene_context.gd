class_name EditorSceneContext
extends RefCounted

var editor: EditorInterface

func _init(editor_interface: EditorInterface) -> void:
    editor = editor_interface

func validate(params: Dictionary) -> Response:
    var root := editor.get_edited_scene_root()

    if root == null:
        return Response.failure(
            ErrorCode.FAILED_PRECONDITION,
            "No scene is currently being edited"
        )

    var expected = params.get("expected_scene", "")

    if not expected is String:
        return Response.failure(
            ErrorCode.INVALID_ARGUMENT,
            "expected_scene must be a string"
        )

    if not expected.is_empty() and root.scene_file_path != expected:
        return Response.failure(
            ErrorCode.CONFLICT,
            "Expected %s, but the active scene is %s" % [expected, root.scene_file_path]
        )

    return null

func describe() -> Dictionary:
    var root := editor.get_edited_scene_root()
    var unsaved := (
        root.scene_file_path.is_empty()
        or editor.get_unsaved_scenes().has(root.scene_file_path)
    )

    return {
        "scene": root.scene_file_path,
        "name": str(root.name),
        "type": root.get_class(),
        "unsaved": unsaved,
    }

func save() -> Response:
    var root := editor.get_edited_scene_root()
    var path := root.scene_file_path

    if path.is_empty():
        return Response.failure(
            ErrorCode.FAILED_PRECONDITION,
            "Use the editor's Save As before saving this scene through godot-cli"
        )

    var result := editor.save_scene()

    if result != OK:
        return Response.failure(
            ErrorCode.ACTION_FAILED,
            "Could not save %s: %s" % [path, error_string(result)]
        )

    return Response.success({
        "scene": path,
        "saved": true,
    })

func find_node(path: String) -> Node:
    var node_path := NodePath(path)

    if node_path.is_absolute() or node_path.get_subname_count() > 0:
        return null

    var root := editor.get_edited_scene_root()
    var node := root.get_node_or_null(node_path)

    if node == root or (node != null and root.is_ancestor_of(node)):
        return node

    return null
