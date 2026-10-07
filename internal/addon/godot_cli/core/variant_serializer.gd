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

        TYPE_NIL, TYPE_BOOL, TYPE_INT, TYPE_FLOAT, TYPE_STRING:
            return value

        _:
            return str(value)
