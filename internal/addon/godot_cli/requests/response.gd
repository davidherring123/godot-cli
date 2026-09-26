class_name Response
extends RefCounted

var result: Variant
var code: String
var message: String

func _init(
    result: Variant,
    code: String = "",
    message: String = ""
) -> void:
    self.result = result
    self.code = code
    self.message = message

func to_dict() -> Dictionary:
    if code.is_empty():
        return {
            "result": result
        }

    return {
        "code": code,
        "message": message
    }

func to_json() -> String:
    return JSON.stringify(to_dict())

static func success(result: Variant) -> Response:
    return Response.new(result, "", "")

static func failure(code: String, message: String) -> Response:
    return Response.new(null, code, message)