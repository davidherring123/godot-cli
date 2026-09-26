class_name Request
extends RefCounted

var command: String
var params: Dictionary


func _init(
    command: String,
    params: Dictionary = {}
) -> void:
    self.command = command
    self.params = params

static func from_dict(data: Dictionary) -> Request:
    return Request.new(
        str(data.get("command", "")),
        data.get("params", {})
    )