class_name VariantSerializer
extends RefCounted

static func serialize(value: Variant, path_overrides: Dictionary = {}) -> Variant:
    if value == null:
        return null

    match typeof(value):
        TYPE_STRING_NAME:
            return str(value)

        TYPE_NODE_PATH:
            return str(value)

        TYPE_VECTOR2, TYPE_VECTOR2I:
            return {
                "x": value.x,
                "y": value.y
            }

        TYPE_VECTOR3, TYPE_VECTOR3I:
            return {
                "x": value.x,
                "y": value.y,
                "z": value.z
            }

        TYPE_VECTOR4, TYPE_VECTOR4I:
            return {
                "x": value.x,
                "y": value.y,
                "z": value.z,
                "w": value.w
            }

        TYPE_COLOR:
            return {
                "r": value.r,
                "g": value.g,
                "b": value.b,
                "a": value.a
            }

        TYPE_RECT2, TYPE_RECT2I:
            return {
                "position": serialize(value.position, path_overrides),
                "size": serialize(value.size, path_overrides)
            }

        TYPE_ARRAY:
            return _serialize_array(value, path_overrides)

        TYPE_DICTIONARY:
            return _serialize_dictionary(value, path_overrides)

        TYPE_PACKED_BYTE_ARRAY, TYPE_PACKED_INT32_ARRAY, TYPE_PACKED_INT64_ARRAY, TYPE_PACKED_FLOAT32_ARRAY, TYPE_PACKED_FLOAT64_ARRAY, TYPE_PACKED_STRING_ARRAY, TYPE_PACKED_VECTOR2_ARRAY, TYPE_PACKED_VECTOR3_ARRAY, TYPE_PACKED_VECTOR4_ARRAY, TYPE_PACKED_COLOR_ARRAY:
            return _serialize_array(value, path_overrides)

        TYPE_OBJECT:
            return _serialize_object(value, path_overrides)

        TYPE_NIL, TYPE_BOOL, TYPE_INT, TYPE_FLOAT, TYPE_STRING:
            return value

        _:
            return str(value)

static func _serialize_array(value: Variant, path_overrides: Dictionary) -> Array:
    var result: Array = []

    for item in value:
        result.append(serialize(item, path_overrides))

    return result

static func _serialize_dictionary(value: Dictionary, path_overrides: Dictionary) -> Dictionary:
    var result := {}

    for key in value:
        result[str(key)] = serialize(value[key], path_overrides)

    return result

static func _serialize_object(value: Object, path_overrides: Dictionary) -> Variant:
    if value is Resource:
        var path := str(value.resource_path)

        if path.is_empty():
            path = str(path_overrides.get(value.get_instance_id(), ""))

        return {
            "resource": path,
            "type": value.get_class()
        }

    return str(value)
