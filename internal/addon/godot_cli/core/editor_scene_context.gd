class_name EditorSceneContext
extends RefCounted

var editor: EditorInterface

# Batch prevalidation runs the same node mutations against a temporary copy of
# the scene. The override keeps those edits away from the live editor scene.
var root_override: Node = null

# Preview contexts must never save: editor.save_scene() acts on the live tab,
# not on the temporary copy.
var preview := false

func _init(editor_interface: EditorInterface) -> void:
    editor = editor_interface

# for_root returns a context that resolves the scene root to root instead of
# the editor's active scene. Used by scene.apply's dry-run/prevalidation pass.
func for_root(root: Node) -> EditorSceneContext:
    var context := EditorSceneContext.new(editor)
    context.root_override = root
    context.preview = true
    return context

func edited_root() -> Node:
    if root_override != null and is_instance_valid(root_override):
        return root_override

    return editor.get_edited_scene_root()

func validate(params: Dictionary) -> Response:
    var root := edited_root()

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

    return expected_scene_error(root, expected)

func expected_scene_error(root: Node, expected: String) -> Response:
    if expected.is_empty():
        return null

    if root == null:
        return Response.failure(
            ErrorCode.FAILED_PRECONDITION,
            "Expected %s, but no scene is being edited" % expected
        )

    if root.scene_file_path != expected:
        return Response.failure(
            ErrorCode.CONFLICT,
            "Expected %s, but the active scene is %s" % [expected, root.scene_file_path]
        )

    return null

func describe() -> Dictionary:
    var root := edited_root()

    if root == null:
        return {
            "scene": "",
            "name": "",
            "type": "",
            "unsaved": false,
            "active": false,
        }

    var unsaved := (
        root.scene_file_path.is_empty()
        or editor.get_unsaved_scenes().has(root.scene_file_path)
    )

    return {
        "scene": root.scene_file_path,
        "name": str(root.name),
        "type": root.get_class(),
        "unsaved": unsaved,
        "active": true,
    }

func save() -> Response:
    if preview:
        return Response.failure(
            ErrorCode.INTERNAL_ERROR,
            "Cannot save while validating a scene batch on a temporary copy"
        )

    var root := edited_root()
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

    var root := edited_root()
    var node := root.get_node_or_null(node_path)

    if node == root or (node != null and root.is_ancestor_of(node)):
        return node

    return null
