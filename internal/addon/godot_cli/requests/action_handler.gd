class_name ActionHandler
extends RefCounted

var callable: Callable
var requires_scene: bool
var supports_save: bool
var supports_expected_scene: bool

func _init(
    action_callable: Callable,
    action_requires_scene: bool,
    action_supports_save: bool,
    action_supports_expected_scene: bool
) -> void:
    callable = action_callable
    requires_scene = action_requires_scene
    supports_save = action_supports_save
    supports_expected_scene = action_supports_expected_scene
