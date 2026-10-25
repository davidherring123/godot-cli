class_name VariantDeserializer
extends RefCounted

static func deserialize(
    value: Variant,
    type: int,
    hint: int = PROPERTY_HINT_NONE,
    hint_string: String = "",
    resource_overrides: Dictionary = {}
) -> Response:
    match type:
        TYPE_BOOL:
            if value is bool:
                return Response.success(value)

        TYPE_INT:
            if _integer(value):
                return Response.success(int(value))

        TYPE_FLOAT:
            if _number(value):
                return Response.success(float(value))

        TYPE_STRING:
            if value is String:
                return Response.success(value)

        TYPE_STRING_NAME:
            if value is String:
                return Response.success(StringName(value))

        TYPE_NODE_PATH:
            if value is String:
                return Response.success(NodePath(value))

        TYPE_VECTOR2, TYPE_VECTOR2I:
            if _components(value, ["x", "y"], type == TYPE_VECTOR2I):
                if type == TYPE_VECTOR2I:
                    return Response.success(Vector2i(int(value.x), int(value.y)))

                return Response.success(Vector2(value.x, value.y))

        TYPE_VECTOR3, TYPE_VECTOR3I:
            if _components(value, ["x", "y", "z"], type == TYPE_VECTOR3I):
                if type == TYPE_VECTOR3I:
                    return Response.success(
                        Vector3i(int(value.x), int(value.y), int(value.z))
                    )

                return Response.success(Vector3(value.x, value.y, value.z))

        TYPE_VECTOR4, TYPE_VECTOR4I:
            if _components(value, ["x", "y", "z", "w"], type == TYPE_VECTOR4I):
                if type == TYPE_VECTOR4I:
                    return Response.success(
                        Vector4i(
                            int(value.x),
                            int(value.y),
                            int(value.z),
                            int(value.w)
                        )
                    )

                return Response.success(Vector4(value.x, value.y, value.z, value.w))

        TYPE_COLOR:
            if _components(value, ["r", "g", "b", "a"]):
                return Response.success(Color(value.r, value.g, value.b, value.a))

        TYPE_RECT2, TYPE_RECT2I:
            if (
                not value is Dictionary
                or value.size() != 2
                or not value.has_all(["position", "size"])
            ):
                return Response.failure(
                    ErrorCode.INVALID_ARGUMENT,
                    "Expected position and size objects"
                )

            var integers := type == TYPE_RECT2I

            if (
                not _components(value.position, ["x", "y"], integers)
                or not _components(value.size, ["x", "y"], integers)
            ):
                return Response.failure(
                    ErrorCode.INVALID_ARGUMENT,
                    "Expected position and size vectors matching %s" % type_string(type)
                )

            if integers:
                return Response.success(
                    Rect2i(
                        Vector2i(int(value.position.x), int(value.position.y)),
                        Vector2i(int(value.size.x), int(value.size.y))
                    )
                )

            return Response.success(
                Rect2(
                    Vector2(value.position.x, value.position.y),
                    Vector2(value.size.x, value.size.y)
                )
            )

        TYPE_ARRAY:
            if value is Array:
                return Response.success(value)

            return _expected(type)

        TYPE_DICTIONARY:
            if value is Dictionary:
                return Response.success(value)

            return _expected(type)

        TYPE_PACKED_BYTE_ARRAY:
            return _packed(value, type)

        TYPE_PACKED_INT32_ARRAY:
            return _packed(value, type)

        TYPE_PACKED_INT64_ARRAY:
            return _packed(value, type)

        TYPE_PACKED_FLOAT32_ARRAY:
            return _packed(value, type)

        TYPE_PACKED_FLOAT64_ARRAY:
            return _packed(value, type)

        TYPE_PACKED_STRING_ARRAY:
            return _packed(value, type)

        TYPE_PACKED_VECTOR2_ARRAY:
            return _packed(value, type)

        TYPE_PACKED_VECTOR3_ARRAY:
            return _packed(value, type)

        TYPE_PACKED_VECTOR4_ARRAY:
            return _packed(value, type)

        TYPE_PACKED_COLOR_ARRAY:
            return _packed(value, type)

        TYPE_OBJECT:
            return _object(value, hint, hint_string, resource_overrides)

        _:
            return Response.failure(
                ErrorCode.UNSUPPORTED,
                "Setting %s properties is not supported yet" % type_string(type)
            )

    return Response.failure(
        ErrorCode.INVALID_ARGUMENT,
        "Expected a JSON value matching %s" % type_string(type)
    )

static func _object(
    value: Variant,
    hint: int,
    hint_string: String,
    resource_overrides: Dictionary = {}
) -> Response:
    if value == null:
        return Response.success(null)

    if not value is Dictionary or not value.has("resource"):
        return Response.failure(
            ErrorCode.INVALID_ARGUMENT,
            "Expected null or a resource reference like {\"resource\": \"res://...\"}"
        )

    var path = value["resource"]

    if not path is String or not path.begins_with("res://"):
        return Response.failure(
            ErrorCode.INVALID_ARGUMENT,
            "A resource reference requires a res:// path"
        )

    path = path.simplify_path()

    var resource: Resource = null

    if resource_overrides.has(path):
        var pending = resource_overrides[path]

        if pending == null:
            return Response.failure(
                ErrorCode.NOT_FOUND,
                "Resource not found: %s" % path
            )

        if not pending is Resource:
            return Response.failure(
                ErrorCode.ACTION_FAILED,
                "Could not load resource: %s" % path
            )

        resource = pending
    else:
        if not ResourceLoader.exists(path):
            return Response.failure(
                ErrorCode.NOT_FOUND,
                "Resource not found: %s" % path
            )

        resource = ResourceLoader.load(path)

        if resource == null:
            return Response.failure(
                ErrorCode.ACTION_FAILED,
                "Could not load resource: %s" % path
            )

    if not resource is Resource:
        return Response.failure(
            ErrorCode.INVALID_ARGUMENT,
            "Not a resource: %s" % path
        )

    var type_error := _resource_type_error(resource, hint, hint_string)

    if not type_error.is_empty():
        return Response.failure(ErrorCode.INVALID_ARGUMENT, type_error)

    return Response.success(resource)

static func _resource_type_error(
    resource: Resource,
    hint: int,
    hint_string: String
) -> String:
    if hint != PROPERTY_HINT_RESOURCE_TYPE or hint_string.is_empty():
        return ""

    var expected: PackedStringArray = []

    for raw_type in hint_string.split(",", false):
        var type_name := raw_type.strip_edges()

        if not type_name.is_empty():
            expected.append(type_name)

    if expected.is_empty():
        return ""

    for type_name in expected:
        if _resource_matches(resource, type_name):
            return ""

    return "Expected %s, got %s" % [" or ".join(expected), resource.get_class()]

static func _resource_matches(resource: Resource, type_name: String) -> bool:
    if resource.is_class(type_name):
        return true

    var script: Script = resource.get_script()

    while script != null:
        if script.get_global_name() == type_name:
            return true

        script = script.get_base_script()

    return false

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

static func _expected(type: int) -> Response:
    return Response.failure(
        ErrorCode.INVALID_ARGUMENT,
        "Expected a JSON value matching %s" % type_string(type)
    )

static func _packed(value: Variant, type: int) -> Response:
    if not value is Array:
        return _expected(type)

    var items: Array = []

    match type:
        TYPE_PACKED_BYTE_ARRAY:
            for item in value:
                if not _integer(item) or item < 0 or item > 255:
                    return _expected(type)

                items.append(int(item))

            return Response.success(PackedByteArray(items))

        TYPE_PACKED_INT32_ARRAY, TYPE_PACKED_INT64_ARRAY:
            for item in value:
                if not _integer(item):
                    return _expected(type)

                if type == TYPE_PACKED_INT32_ARRAY and (
                    item < -2147483648 or item > 2147483647
                ):
                    return _expected(type)

                items.append(int(item))

            if type == TYPE_PACKED_INT32_ARRAY:
                return Response.success(PackedInt32Array(items))

            return Response.success(PackedInt64Array(items))

        TYPE_PACKED_FLOAT32_ARRAY, TYPE_PACKED_FLOAT64_ARRAY:
            for item in value:
                if not _number(item):
                    return _expected(type)

                items.append(float(item))

            if type == TYPE_PACKED_FLOAT32_ARRAY:
                return Response.success(PackedFloat32Array(items))

            return Response.success(PackedFloat64Array(items))

        TYPE_PACKED_STRING_ARRAY:
            for item in value:
                if not item is String:
                    return _expected(type)

                items.append(item)

            return Response.success(PackedStringArray(items))

        TYPE_PACKED_VECTOR2_ARRAY:
            for item in value:
                if not _components(item, ["x", "y"]):
                    return _expected(type)

                items.append(Vector2(item.x, item.y))

            return Response.success(PackedVector2Array(items))

        TYPE_PACKED_VECTOR3_ARRAY:
            for item in value:
                if not _components(item, ["x", "y", "z"]):
                    return _expected(type)

                items.append(Vector3(item.x, item.y, item.z))

            return Response.success(PackedVector3Array(items))

        TYPE_PACKED_VECTOR4_ARRAY:
            for item in value:
                if not _components(item, ["x", "y", "z", "w"]):
                    return _expected(type)

                items.append(Vector4(item.x, item.y, item.z, item.w))

            return Response.success(PackedVector4Array(items))

        TYPE_PACKED_COLOR_ARRAY:
            for item in value:
                if not _components(item, ["r", "g", "b", "a"]):
                    return _expected(type)

                items.append(Color(item.r, item.g, item.b, item.a))

            return Response.success(PackedColorArray(items))

    return _expected(type)
