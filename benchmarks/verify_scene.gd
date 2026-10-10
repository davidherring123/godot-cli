extends SceneTree

const RESULT_PREFIX := "GODOT_CLI_BENCHMARK_RESULT:"

func _initialize() -> void:
    var args := OS.get_cmdline_user_args()

    if args.size() < 2:
        _finish({"loaded": false, "error": "Expected scene and checks arguments"}, 1)
        return

    var resource = ResourceLoader.load(args[0], "PackedScene")

    if not resource is PackedScene:
        _finish({"loaded": false, "error": "Could not load scene"}, 1)
        return

    var checks = JSON.parse_string(args[1])

    if not checks is Array:
        _finish({"loaded": false, "error": "Checks must be a JSON array"}, 1)
        return

    var root: Node = resource.instantiate()
    var results: Array[Dictionary] = []

    for check in checks:
        if not check is Dictionary:
            continue

        var path := str(check.get("node", ""))
        var node: Node = root.get_node_or_null(NodePath(path))
        var entry := {
            "node": path,
            "found": node != null,
        }

        if node != null:
            var properties := {}

            for property_name in check.get("properties", []):
                properties[str(property_name)] = _serialize(node.get(str(property_name)))

            entry["properties"] = properties

        results.append(entry)

    root.queue_free()

    _finish({
        "loaded": true,
        "checks": results,
    }, 0)

func _serialize(value: Variant) -> Variant:
    if value is Vector2:
        return {"x": value.x, "y": value.y}

    if value is Vector2i:
        return {"x": value.x, "y": value.y}

    if value is Vector3:
        return {"x": value.x, "y": value.y, "z": value.z}

    if value is Vector4:
        return {"x": value.x, "y": value.y, "z": value.z, "w": value.w}

    if value is Color:
        return {"r": value.r, "g": value.g, "b": value.b, "a": value.a}

    if value is Rect2:
        return {
            "position": _serialize(value.position),
            "size": _serialize(value.size),
        }

    if value is Resource:
        return value.resource_path

    if value is Object:
        return str(value)

    return value

func _finish(result: Dictionary, code: int) -> void:
    print(RESULT_PREFIX + JSON.stringify(result))
    quit(code)
