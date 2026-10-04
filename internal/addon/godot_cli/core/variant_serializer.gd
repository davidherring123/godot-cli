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

        TYPE_VECTOR2:
            return {
                "x": value.x,
                "y": value.y
            }

        TYPE_VECTOR3:
            return {
                "x": value.x,
                "y": value.y,
                "z": value.z
            }

        TYPE_COLOR:
            return {
                "r": value.r,
                "g": value.g,
                "b": value.b,
                "a": value.a
            }

        TYPE_RECT2:
            return {
                "position": serialize(value.position),
                "size": serialize(value.size)
            }

        TYPE_NIL, TYPE_BOOL, TYPE_INT, TYPE_FLOAT, TYPE_STRING:
            return value

        _:
            return str(value)