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
    var nodes := _all_nodes(root)
    var results: Array[Dictionary] = []

    for check in checks:
        if not check is Dictionary:
            results.append({
                "kind": "invalid",
                "passed": false,
                "detail": "verification check must be an object",
            })
            continue
        results.append(_run_check(check, nodes))

    root.queue_free()
    _finish({"loaded": true, "checks": results}, 0)

func _run_check(check: Dictionary, nodes: Array[Node]) -> Dictionary:
    var kind := str(check.get("kind", ""))

    match kind:
        "physicsBodies":
            return _check_physics_bodies(check, nodes)
        "drawableContent":
            return _check_drawable_content(check, nodes)
        "camera":
            return _check_camera(check, nodes)
        "hudText":
            return _check_hud_text(check, nodes)
        "mainScene":
            return _check_main_scene(check)
        "inputActions":
            return _check_input_actions(check)
        _:
            return _result(kind, false, "unknown verification check kind: " + kind)

func _check_physics_bodies(check: Dictionary, nodes: Array[Node]) -> Dictionary:
    var types := _types(check)

    if types.is_empty():
        return _result("physicsBodies", false, "types is required")

    var minimum := int(check.get("min", 1))
    var require_collision := bool(check.get("collision", false))
    var require_script := bool(check.get("script", false))
    var require_visible := bool(check.get("visible", false))
    var count := 0

    for node in nodes:
        if not _matches_any_type(node, types):
            continue
        if require_collision and not _has_collision(node):
            continue
        if require_script and node.get_script() == null:
            continue
        if require_visible and not _has_visible_drawable(node):
            continue
        count += 1

    var conditions: Array[String] = []

    if require_collision:
        conditions.append("collision")
    if require_script:
        conditions.append("script")
    if require_visible:
        conditions.append("visible drawable")

    var qualifier := ""

    if not conditions.is_empty():
        qualifier = " with " + ", ".join(conditions)

    var detail := "found %d of %d required %s%s" % [
        count,
        minimum,
        ", ".join(types),
        qualifier,
    ]

    return _result("physicsBodies", count >= minimum, detail)

func _check_drawable_content(check: Dictionary, nodes: Array[Node]) -> Dictionary:
    var minimum := int(check.get("min", 1))
    var count := 0

    for node in nodes:
        if _is_drawable(node):
            count += 1

    return _result(
        "drawableContent",
        count >= minimum,
        "found %d of %d required drawable nodes" % [count, minimum],
    )

func _check_camera(check: Dictionary, nodes: Array[Node]) -> Dictionary:
    var types := _types(check)

    if types.is_empty():
        return _result("camera", false, "types is required")

    var minimum := int(check.get("min", 1))
    var require_enabled := bool(check.get("enabled", false))
    var count := 0

    for node in nodes:
        if not _matches_any_type(node, types):
            continue
        if require_enabled and not bool(node.get("enabled")):
            continue
        count += 1

    var detail := "found %d of %d required %s" % [
        count,
        minimum,
        ", ".join(types),
    ]

    if require_enabled:
        detail += " with enabled set"

    return _result("camera", count >= minimum, detail)

func _check_hud_text(check: Dictionary, nodes: Array[Node]) -> Dictionary:
    var types := _types(check)

    if types.is_empty():
        return _result("hudText", false, "types is required")

    var minimum := int(check.get("min", 1))
    var require_visible := bool(check.get("visible", false))
    var container := str(check.get("container", ""))
    var text_contains := _strings(check, "textContains")
    var text_any := _strings(check, "textAny")
    var count := 0

    for node in nodes:
        if not _matches_any_type(node, types):
            continue
        if require_visible and not (node is CanvasItem and _is_visible(node as CanvasItem)):
            continue
        if not _has_text(node):
            continue
        if container != "" and not _has_ancestor_type(node, container):
            continue
        if not _text_matches(str(node.get("text")), text_contains, text_any):
            continue
        count += 1

    var detail := "found %d of %d required text nodes" % [count, minimum]

    if require_visible:
        detail += " that are visible"
    if container != "":
        detail += " under " + container
    if not text_contains.is_empty():
        detail += " containing " + ", ".join(text_contains)
    if not text_any.is_empty():
        detail += " containing any of " + ", ".join(text_any)

    return _result("hudText", count >= minimum, detail)

func _check_main_scene(check: Dictionary) -> Dictionary:
    var scene_path := str(
        ProjectSettings.get_setting("application/run/main_scene", "")
    )

    if scene_path == "":
        return _result(
            "mainScene",
            false,
            "application/run/main_scene is not configured",
        )

    var expected_scene := str(check.get("scene", ""))

    if expected_scene != "" and scene_path != expected_scene:
        return _result(
            "mainScene",
            false,
            "main scene is %s, expected %s" % [scene_path, expected_scene],
        )

    if bool(check.get("loadable", false)):
        if not ResourceLoader.exists(scene_path):
            return _result(
                "mainScene",
                false,
                "main scene does not exist: " + scene_path,
            )

        var resource = ResourceLoader.load(scene_path, "PackedScene")

        if not resource is PackedScene:
            return _result(
                "mainScene",
                false,
                "main scene is not a loadable packed scene: " + scene_path,
            )
        return _result(
            "mainScene",
            true,
            "main scene is configured and loadable: " + scene_path,
        )

    return _result(
        "mainScene",
        true,
        "main scene is configured: " + scene_path,
    )

func _check_input_actions(check: Dictionary) -> Dictionary:
    var minimum := int(check.get("min", 1))
    var require_events := bool(check.get("events", false))
    var required := _strings(check, "actions")
    var project_actions := _project_input_actions()
    var valid: Array[String] = []
    var missing: Array[String] = []
    var without_events: Array[String] = []
    var unregistered: Array[String] = []

    if required.is_empty():
        for action in project_actions:
            if not InputMap.has_action(action):
                unregistered.append(action)
                continue
            if require_events and InputMap.action_get_events(action).is_empty():
                continue
            valid.append(action)
    else:
        minimum = required.size()

        for action in required:
            if not InputMap.has_action(action):
                missing.append(action)
                continue
            if require_events and InputMap.action_get_events(action).is_empty():
                without_events.append(action)
                continue
            valid.append(action)

    var label := "required project input actions"

    if not required.is_empty():
        label = "required input actions"

    var detail := "found %d of %d %s" % [valid.size(), minimum, label]

    if require_events:
        detail += " with events"
    if not missing.is_empty():
        detail += "; missing actions: " + ", ".join(missing)
    if not without_events.is_empty():
        detail += "; actions without events: " + ", ".join(without_events)
    if not unregistered.is_empty():
        detail += "; unregistered actions: " + ", ".join(unregistered)

    return _result(
        "inputActions",
        valid.size() >= minimum and missing.is_empty() and without_events.is_empty(),
        detail,
    )

func _result(kind: String, passed: bool, detail: String) -> Dictionary:
    return {
        "kind": kind,
        "passed": passed,
        "detail": detail,
    }

func _types(check: Dictionary) -> Array[String]:
    var result: Array[String] = []

    for value in check.get("types", []):
        var type_name := str(value)

        if type_name != "":
            result.append(type_name)

    return result

func _strings(check: Dictionary, key: String) -> Array[String]:
    var result: Array[String] = []

    for value in check.get(key, []):
        var text := str(value)

        if text != "":
            result.append(text)

    return result

func _text_matches(text: String, required: Array[String], any_of: Array[String]) -> bool:
    var lowered := text.to_lower()

    for value in required:
        if not lowered.contains(value.to_lower()):
            return false
    if any_of.is_empty():
        return true

    for value in any_of:
        if lowered.contains(value.to_lower()):
            return true
    return false

func _matches_any_type(node: Node, types: Array[String]) -> bool:
    for type_name in types:
        if node.is_class(type_name):
            return true
    return false

func _all_nodes(root: Node) -> Array[Node]:
    var result: Array[Node] = [root]

    for child in root.get_children():
        result.append_array(_all_nodes(child))

    return result

func _has_collision(node: Node) -> bool:
    for child in _all_nodes(node):
        if child is CollisionShape2D:
            if (child as CollisionShape2D).shape != null:
                return true
        elif child is CollisionPolygon2D:
            if (child as CollisionPolygon2D).polygon.size() >= 3:
                return true
    return false

func _has_visible_drawable(node: Node) -> bool:
    for child in _all_nodes(node):
        if _is_drawable(child):
            return true
    return false

func _is_drawable(node: Node) -> bool:
    if not node is CanvasItem:
        return false

    var item := node as CanvasItem

    if not _is_visible(item):
        return false

    return _has_drawable_content(item)

func _has_drawable_content(item: CanvasItem) -> bool:
    if item is Label:
        return not (item as Label).text.strip_edges().is_empty()
    if item is RichTextLabel:
        return not (item as RichTextLabel).text.strip_edges().is_empty()
    if item is ColorRect:
        var rect := item as ColorRect

        return (
            rect.color.a > 0.0
            and rect.size.x > 0.0
            and rect.size.y > 0.0
        )
    if item is Polygon2D:
        var polygon := item as Polygon2D

        return polygon.polygon.size() >= 3 and polygon.color.a > 0.0
    if item is Sprite2D:
        return (item as Sprite2D).texture != null
    if item is AnimatedSprite2D:
        return (item as AnimatedSprite2D).sprite_frames != null
    if item is TextureRect:
        return (item as TextureRect).texture != null
    if item is NinePatchRect:
        return (item as NinePatchRect).texture != null
    if item is Line2D:
        var line := item as Line2D

        return line.points.size() >= 2 and line.width > 0.0
    if item is MeshInstance2D:
        return (item as MeshInstance2D).mesh != null

    return _script_draws(item)

func _has_text(node: Node) -> bool:
    var value = node.get("text")

    return value != null and not str(value).strip_edges().is_empty()

func _script_draws(node: Node) -> bool:
    var script = node.get_script()

    if script == null:
        return false

    var source = script.get("source_code")

    return source != null and "func _draw" in str(source)

func _is_visible(item: CanvasItem) -> bool:
    var current: Node = item

    while current != null:
        if current is CanvasItem and not (current as CanvasItem).visible:
            return false
        current = current.get_parent()

    return true

func _has_ancestor_type(node: Node, type_name: String) -> bool:
    var parent := node.get_parent()

    while parent != null:
        if parent.is_class(type_name):
            return true
        parent = parent.get_parent()

    return false

func _project_input_actions() -> Array[String]:
    var actions: Array[String] = []
    var config := ConfigFile.new()

    if config.load("res://project.godot") != OK:
        return actions
    if not config.has_section("input"):
        return actions

    for action in config.get_section_keys("input"):
        var name := str(action)

        if name != "":
            actions.append(name)

    return actions

func _finish(result: Dictionary, code: int) -> void:
    print(RESULT_PREFIX + JSON.stringify(result))
    quit(code)
