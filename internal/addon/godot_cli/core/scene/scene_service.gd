class_name SceneService
extends CommandService

var editor_interface: EditorInterface


func _init(editor_interface: EditorInterface) -> void:
    self.editor_interface = editor_interface

func get_commands() -> Dictionary:
    return {
        "scene.tree": get_tree
    }

func get_tree(request: Request) -> Response:
    var scene_path = request.params.get("scene")

    if not scene_path is String or scene_path.is_empty():
        return Response.failure("invalid_scene_path", "A scene path is required")

    var packed_scene := ResourceLoader.load(scene_path, "PackedScene") as PackedScene
    
    if packed_scene == null:
        return Response.failure("scene_load_failed", "Could not load scene: %s" % scene_path)
    
    var root := packed_scene.instantiate()

    if root == null:
        return Response.failure("scene_load_failed", "Could not instantiate scene: %s" % scene_path)
    
    var result := {
        "scene": scene_path,
        "root": _node_to_dict(root, root),
    }

    root.free()

    return Response.success(result)

func _node_to_dict(node: Node, root: Node) -> Dictionary:
    var children: Array[Dictionary] = []

    for child in node.get_children():
        children.append(_node_to_dict(child, root))
    
    return {
        "name": str(node.name),
        "type": node.get_class(),
        "path": str(root.get_path_to(node)),
        "children": children
    }