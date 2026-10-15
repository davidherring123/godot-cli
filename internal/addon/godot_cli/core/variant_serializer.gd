class_name VariantSerializer
extends RefCounted

static func serialize(value: Variant) -> Variant:
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
                "position": serialize(value.position),
                "size": serialize(value.size)
            }

        TYPE_ARRAY:
            return _serialize_array(value)

        TYPE_DICTIONARY:
            return _serialize_dictionary(value)

        TYPE_PACKED_BYTE_ARRAY, TYPE_PACKED_INT32_ARRAY, TYPE_PACKED_INT64_ARRAY, TYPE_PACKED_FLOAT32_ARRAY, TYPE_PACKED_FLOAT64_ARRAY, TYPE_PACKED_STRING_ARRAY, TYPE_PACKED_VECTOR2_ARRAY, TYPE_PACKED_VECTOR3_ARRAY, TYPE_PACKED_VECTOR4_ARRAY, TYPE_PACKED_COLOR_ARRAY:
            return _serialize_array(value)

        TYPE_OBJECT:
            return _serialize_object(value)

        TYPE_NIL, TYPE_BOOL, TYPE_INT, TYPE_FLOAT, TYPE_STRING:
            return value

        _:
            return str(value)

static func _serialize_array(value: Variant) -> Array:
    var result: Array = []

    for item in value:
        result.append(serialize(item))

    return result

static func _serialize_dictionary(value: Dictionary) -> Dictionary:
    var result := {}

    for key in value:
        result[str(key)] = serialize(value[key])

    return result

static func _serialize_object(value: Object) -> Variant:
    if value is Resource:
        return {
            "resource": value.resource_path,
            "type": value.get_class()
        }

    return str(value)
