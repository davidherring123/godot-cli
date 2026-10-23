@tool
extends Node2D

# Creates editor-only, unowned state. PackedScene.pack omits unowned children, so
# scene.apply prevalidation sees a different tree than the live editor scene.
func _ready() -> void:
    if not Engine.is_editor_hint():
        return

    var world := get_node_or_null("World")

    if world == null or world.has_node("RuntimeOnly"):
        return

    var node := Node2D.new()
    node.name = "RuntimeOnly"
    world.add_child(node)
