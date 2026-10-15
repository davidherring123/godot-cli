class_name InputService
extends ActionService

const PREFIX := "input/"

func get_actions() -> Dictionary:
    return {
        "input.list": ActionHandler.new(list_actions, false, false, false),
        "input.inspect": ActionHandler.new(inspect_action, false, false, false),
        "input.add": ActionHandler.new(add_action, false, false, false),
        "input.set": ActionHandler.new(set_action, false, false, false),
        "input.rename": ActionHandler.new(rename_action, false, false, false),
        "input.delete": ActionHandler.new(delete_action, false, false, false),
        "input.event_add": ActionHandler.new(add_event, false, false, false),
        "input.event_delete": ActionHandler.new(delete_event, false, false, false),
    }

func list_actions(_request: Request) -> Response:
    var actions: Array[Dictionary] = []

    for definition in ProjectSettings.get_property_list():
        var name: String = definition.get("name", "")

        if not name.begins_with(PREFIX):
            continue

        var usage: int = definition.get("usage", 0)

        if (usage & PROPERTY_USAGE_STORAGE) == 0:
            continue

        var value = ProjectSettings.get_setting(name)

        if not value is Dictionary:
            continue

        actions.append(_payload(name.substr(PREFIX.length()), value))

    actions.sort_custom(_sort_actions)

    return Response.success({"actions": actions})

func inspect_action(request: Request) -> Response:
    var response := _action_name(request.params.get("action"))

    if not response.code.is_empty():
        return response

    var action: String = response.result
    var value := _read_action(action)

    if value.is_empty():
        return _not_found(action)

    return Response.success(_payload(action, value))

func add_action(request: Request) -> Response:
    var response := _action_name(request.params.get("action"))

    if not response.code.is_empty():
        return response

    var action: String = response.result
    var key := PREFIX + action

    if ProjectSettings.has_setting(key):
        return Response.failure(
            ErrorCode.CONFLICT,
            "Action already exists: %s" % action
        )

    var deadzone := 0.5

    if request.params.has("deadzone"):
        var deadzone_response := _deadzone(request.params["deadzone"])

        if not deadzone_response.code.is_empty():
            return deadzone_response

        deadzone = deadzone_response.result

    ProjectSettings.set_setting(key, {"deadzone": deadzone, "events": []})

    var save_error := _save()

    if save_error != null:
        ProjectSettings.set_setting(key, null)

        return save_error

    _reload_input_map()

    return Response.success({
        "action": action,
        "deadzone": deadzone,
        "events": [],
        "changed": true,
    })

func set_action(request: Request) -> Response:
    var response := _action_name(request.params.get("action"))

    if not response.code.is_empty():
        return response

    var action: String = response.result
    var key := PREFIX + action
    var value := _read_action(action)

    if value.is_empty():
        return _not_found(action)

    var deadzone_response := _deadzone(request.params.get("deadzone"))

    if not deadzone_response.code.is_empty():
        return deadzone_response

    var deadzone: float = deadzone_response.result

    if float(value.get("deadzone", 0.5)) == deadzone:
        return Response.success({
            "action": action,
            "deadzone": deadzone,
            "events": _serialize_events(value),
            "changed": false,
        })

    var updated := value.duplicate()

    updated["deadzone"] = deadzone

    ProjectSettings.set_setting(key, updated)

    var save_error := _save()

    if save_error != null:
        ProjectSettings.set_setting(key, value)

        return save_error

    _reload_input_map()

    return Response.success({
        "action": action,
        "deadzone": deadzone,
        "events": _serialize_events(updated),
        "changed": true,
    })

func rename_action(request: Request) -> Response:
    var response := _action_name(request.params.get("action"))

    if not response.code.is_empty():
        return response

    var action: String = response.result
    var new_name: String = request.params.get("new_name", "")
    var name_response := _action_name(new_name)

    if not name_response.code.is_empty():
        return name_response

    new_name = name_response.result

    var value := _read_action(action)

    if value.is_empty():
        return _not_found(action)

    if _is_builtin_action(action):
        return Response.failure(
            ErrorCode.FAILED_PRECONDITION,
            "Built-in action cannot be renamed: %s" % action
        )

    if new_name == action:
        return Response.success({
            "action": action,
            "new_name": new_name,
            "deadzone": float(value.get("deadzone", 0.5)),
            "events": _serialize_events(value),
            "changed": false,
        })

    var new_key := PREFIX + new_name

    if ProjectSettings.has_setting(new_key):
        return Response.failure(
            ErrorCode.CONFLICT,
            "Action already exists: %s" % new_name
        )

    var old_key := PREFIX + action

    ProjectSettings.set_setting(new_key, value.duplicate())
    ProjectSettings.set_setting(old_key, null)

    var save_error := _save()

    if save_error != null:
        ProjectSettings.set_setting(old_key, value)
        ProjectSettings.set_setting(new_key, null)

        return save_error

    _reload_input_map()

    return Response.success({
        "action": action,
        "new_name": new_name,
        "deadzone": float(value.get("deadzone", 0.5)),
        "events": _serialize_events(value),
        "changed": true,
    })

func delete_action(request: Request) -> Response:
    var response := _action_name(request.params.get("action"))

    if not response.code.is_empty():
        return response

    var action: String = response.result
    var key := PREFIX + action
    var value := _read_action(action)

    if value.is_empty():
        return _not_found(action)

    if _is_builtin_action(action):
        return Response.failure(
            ErrorCode.FAILED_PRECONDITION,
            "Built-in action cannot be deleted: %s" % action
        )

    ProjectSettings.set_setting(key, null)

    var save_error := _save()

    if save_error != null:
        ProjectSettings.set_setting(key, value)

        return save_error

    _reload_input_map()

    return Response.success({"action": action, "deleted": true})

func add_event(request: Request) -> Response:
    var response := _action_name(request.params.get("action"))

    if not response.code.is_empty():
        return response

    var action: String = response.result
    var key := PREFIX + action
    var value := _read_action(action)

    if value.is_empty():
        return _not_found(action)

    var decoded := InputEventCodec.deserialize(request.params.get("event"))

    if not decoded.code.is_empty():
        return decoded

    var event: InputEvent = decoded.result
    var events: Array = _events(value).duplicate()

    events.append(event)

    var updated := value.duplicate()

    updated["events"] = events

    ProjectSettings.set_setting(key, updated)

    var save_error := _save()

    if save_error != null:
        ProjectSettings.set_setting(key, value)

        return save_error

    _reload_input_map()

    return Response.success({
        "action": action,
        "index": events.size() - 1,
        "event": InputEventCodec.serialize(event),
        "changed": true,
    })

func delete_event(request: Request) -> Response:
    var response := _action_name(request.params.get("action"))

    if not response.code.is_empty():
        return response

    var action: String = response.result
    var key := PREFIX + action
    var value := _read_action(action)

    if value.is_empty():
        return _not_found(action)

    var index_response := _event_index(request.params.get("index"))

    if not index_response.code.is_empty():
        return index_response

    var index: int = index_response.result
    var events: Array = _events(value)

    if index >= events.size():
        return Response.failure(
            ErrorCode.NOT_FOUND,
            "Binding index out of range: %d" % index
        )

    var remaining: Array = events.duplicate()

    remaining.remove_at(index)

    var updated := value.duplicate()

    updated["events"] = remaining

    ProjectSettings.set_setting(key, updated)

    var save_error := _save()

    if save_error != null:
        ProjectSettings.set_setting(key, value)

        return save_error

    _reload_input_map()

    return Response.success({
        "action": action,
        "index": index,
        "events": _serialize_events(updated),
        "changed": true,
    })

func _action_name(value: Variant) -> Response:
    if not value is String or value.is_empty():
        return Response.failure(
            ErrorCode.INVALID_ARGUMENT,
            "An action name is required"
        )

    var action: String = value

    if action.contains("/") or action != action.strip_edges() or action.begins_with("."):
        return Response.failure(
            ErrorCode.INVALID_ARGUMENT,
            "Invalid action name: %s" % action
        )

    return Response.success(action)

func _deadzone(value: Variant) -> Response:
    if not value is float and not value is int:
        return Response.failure(
            ErrorCode.INVALID_ARGUMENT,
            "deadzone must be a number"
        )

    var deadzone := float(value)

    if deadzone < 0.0 or deadzone > 1.0:
        return Response.failure(
            ErrorCode.INVALID_ARGUMENT,
            "deadzone must be between 0 and 1"
        )

    return Response.success(deadzone)

func _event_index(value: Variant) -> Response:
    if not _integer(value) or int(value) < 0:
        return Response.failure(
            ErrorCode.INVALID_ARGUMENT,
            "index must be a non-negative integer"
        )

    return Response.success(int(value))

func _read_action(action: String) -> Dictionary:
    var key := PREFIX + action

    if not ProjectSettings.has_setting(key):
        return {}

    var value = ProjectSettings.get_setting(key)

    if value is Dictionary:
        return value

    return {}

func _is_builtin_action(action: String) -> bool:
    return ProjectSettings.property_get_revert(PREFIX + action) is Dictionary

func _events(value: Dictionary) -> Array:
    var raw = value.get("events", [])

    if raw is Array:
        return raw

    return []

func _payload(action: String, value: Dictionary) -> Dictionary:
    return {
        "name": action,
        "deadzone": float(value.get("deadzone", 0.5)),
        "events": _serialize_events(value),
    }

func _serialize_events(value: Dictionary) -> Array:
    var events: Array = []

    for event in _events(value):
        if event is InputEvent:
            events.append(InputEventCodec.serialize(event))

    return events

func _sort_actions(a: Dictionary, b: Dictionary) -> bool:
    return a["name"] < b["name"]

func _not_found(action: String) -> Response:
    return Response.failure(ErrorCode.NOT_FOUND, "Action not found: %s" % action)

func _save() -> Response:
    var error := ProjectSettings.save()

    if error != OK:
        return Response.failure(
            ErrorCode.ACTION_FAILED,
            "Could not save project.godot: %s" % error_string(error)
        )

    return null

func _reload_input_map() -> void:
    InputMap.load_from_project_settings()

func _integer(value: Variant) -> bool:
    if value is int:
        return true

    return value is float and is_finite(value) and floor(value) == value
