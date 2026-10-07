class_name Request
extends RefCounted

var action: String
var params: Dictionary

func _init(
    action_name: String,
    params: Dictionary = {}
) -> void:
    action = action_name
    self.params = params

static func from_dict(data: Dictionary) -> Request:
    return Request.new(
        data.get("action", ""),
        data.get("params", {})
    )
