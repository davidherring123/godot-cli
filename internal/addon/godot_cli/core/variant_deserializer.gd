class_name VariantDeserializer
extends RefCounted

static func deserialize(value: Variant, type: int) -> Dictionary:
    match type:
        TYPE_BOOL:
            if value is bool:
                return {"value": value}

        TYPE_INT:
            if _integer(value):
                return {"value": int(value)}

        TYPE_FLOAT:
            if _number(value):
                return {"value": float(value)}

        TYPE_STRING:
            if value is String:
                return {"value": value}

        TYPE_STRING_NAME:
            if value is String:
                return {"value": StringName(value)}

        TYPE_NODE_PATH:
            if value is String:
                return {"value": NodePath(value)}

        TYPE_VECTOR2, TYPE_VECTOR2I:
            if _components(value, ["x", "y"], type == TYPE_VECTOR2I):
                if type == TYPE_VECTOR2I:
                    return {"value": Vector2i(int(value.x), int(value.y))}

                return {"value": Vector2(value.x, value.y)}

        TYPE_VECTOR3, TYPE_VECTOR3I:
            if _components(value, ["x", "y", "z"], type == TYPE_VECTOR3I):
                if type == TYPE_VECTOR3I:
                    return {"value": Vector3i(int(value.x), int(value.y), int(value.z))}

                return {"value": Vector3(value.x, value.y, value.z)}

        TYPE_VECTOR4, TYPE_VECTOR4I:
            if _components(value, ["x", "y", "z", "w"], type == TYPE_VECTOR4I):
                if type == TYPE_VECTOR4I:
                    return {
                        "value": Vector4i(
                            int(value.x),
                            int(value.y),
                            int(value.z),
                            int(value.w)
                        )
                    }

                return {"value": Vector4(value.x, value.y, value.z, value.w)}

        TYPE_COLOR:
            if _components(value, ["r", "g", "b", "a"]):
                return {"value": Color(value.r, value.g, value.b, value.a)}

        TYPE_RECT2, TYPE_RECT2I:
            if (
                not value is Dictionary
                or value.size() != 2
                or not value.has_all(["position", "size"])
            ):
                return {"error": "Expected position and size objects"}

            var integers := type == TYPE_RECT2I

            if (
                not _components(value.position, ["x", "y"], integers)
                or not _components(value.size, ["x", "y"], integers)
            ):
                return {
                    "error": (
                        "Expected position and size vectors matching %s"
                        % type_string(type)
                    )
                }

            if integers:
                return {
                    "value": Rect2i(
                        Vector2i(int(value.position.x), int(value.position.y)),
                        Vector2i(int(value.size.x), int(value.size.y))
                    )
                }

            return {
                "value": Rect2(
                    Vector2(value.position.x, value.position.y),
                    Vector2(value.size.x, value.size.y)
                )
            }

        _:
            return {
                "code": ErrorCode.UNSUPPORTED,
                "error": "Setting %s properties is not supported yet" % type_string(type),
            }

    return {"error": "Expected a JSON value matching %s" % type_string(type)}

static func _number(value: Variant) -> bool:
    return (value is int or value is float) and is_finite(float(value))

static func _integer(value: Variant) -> bool:
    if value is int:
        return true

    return (
        value is float
        and is_finite(value)
        and floor(value) == value
        and abs(value) <= 9007199254740991.0
    )

static func _components(value: Variant, names: Array, integers: bool = false) -> bool:
    if not value is Dictionary or value.size() != names.size() or not value.has_all(names):
        return false

    for name in names:
        if not _number(value[name]):
            return false

        if abs(value[name]) > 3.402823466e38:
            return false

        if integers and (
            not _integer(value[name])
            or value[name] < -2147483648
            or value[name] > 2147483647
        ):
            return false

    return true
