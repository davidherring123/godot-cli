class_name InputService
extends ActionService

const PREFIX := "input/"

var defer_save := false
var defer_reload := false

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
        "input.map_apply": ActionHandler.new(apply, false, false, false),
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

    _finish_mutation()

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

    _finish_mutation()

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

    _finish_mutation()

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

    _finish_mutation()

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

    _finish_mutation()

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

    _finish_mutation()

    return Response.success({
        "action": action,
        "index": index,
        "events": _serialize_events(updated),
        "changed": true,
    })

func apply(request: Request) -> Response:
    var operations = request.params.get("operations")
    var size_error := validate_batch_operations(operations)

    if size_error != null:
        return size_error

    var dry_run_requested = request.params.get("dry_run", false)

    if not dry_run_requested is bool:
        return Response.failure(
            ErrorCode.INVALID_ARGUMENT,
            "dry_run must be a boolean"
        )

    var snapshot := _snapshot_actions(operations)
    var project_existed := FileAccess.file_exists("res://project.godot")
    var project_snapshot := (
        FileAccess.get_file_as_bytes("res://project.godot")
        if project_existed
        else PackedByteArray()
    )
    var summaries: Array = []

    defer_save = true
    defer_reload = true

    for index in operations.size():
        var operation = operations[index]

        if not operation is Dictionary:
            var invalid := Response.failure(
                ErrorCode.INVALID_ARGUMENT,
                "Each operation must be an object"
            )

            defer_save = false
            defer_reload = false
            _restore_actions(snapshot)
            _reload_input_map()
            return batch_operation_failure(
                index,
                operation,
                "",
                invalid,
                "no changes were saved"
            )

        var kind := str(operation.get("operation", ""))
        var target := str(operation.get("action", ""))
        var response: Response

        match kind:
            "add":
                response = add_action(Request.new("input.add", operation))
            "set":
                response = set_action(Request.new("input.set", operation))
            "rename":
                response = rename_action(Request.new("input.rename", operation))
            "delete":
                response = delete_action(Request.new("input.delete", operation))
            "event_add":
                response = add_event(Request.new("input.event_add", operation))
            "event_delete":
                response = delete_event(Request.new("input.event_delete", operation))
            _:
                response = Response.failure(
                    ErrorCode.INVALID_ARGUMENT,
                    "Unsupported operation: %s" % kind
                )

        if not response.code.is_empty():
            defer_save = false
            defer_reload = false
            _restore_actions(snapshot)
            _reload_input_map()
            return batch_operation_failure(
                index,
                operation,
                target,
                response,
                "no changes were saved"
            )

        summaries.append(_apply_operation_summary(index, operation, response.result))

    defer_save = false
    defer_reload = false

    if dry_run_requested:
        _restore_actions(snapshot)
        _reload_input_map()
        return Response.success(_apply_summary(summaries, true, false))

    var save_error := ProjectSettings.save()

    if save_error != OK:
        _restore_actions(snapshot)
        var restore_error := _restore_project_file(project_existed, project_snapshot)
        var suffix := "restored project.godot and in-memory state"

        if not restore_error.is_empty():
            suffix = "rollback failed: %s" % restore_error

        _reload_input_map()

        return Response.failure(
            ErrorCode.ACTION_FAILED,
            "Could not save project.godot after %d operation(s): %s; %s" % [
                summaries.size(),
                error_string(save_error),
                suffix,
            ]
        )

    _reload_input_map()

    return Response.success(_apply_summary(summaries, false, true))

func _apply_operation_summary(
    index: int,
    operation: Dictionary,
    result: Dictionary
) -> Dictionary:
    var kind := str(operation.get("operation", ""))
    var action := str(result.get("action", operation.get("action", "")))
    var changed := bool(result.get("changed", true))
    var summary := {
        "index": index,
        "operation": kind,
        "action": action,
        "status": kind,
        "changed": changed,
    }

    match kind:
        "add":
            summary["status"] = "added"
        "set":
            summary["status"] = "updated" if changed else "unchanged"
        "rename":
            summary["status"] = "renamed" if changed else "unchanged"
        "delete":
            summary["status"] = "deleted"
        "event_add":
            summary["status"] = "binding_added"
        "event_delete":
            summary["status"] = "binding_deleted"

    if result.has("new_name"):
        summary["new_name"] = result["new_name"]

    if kind == "event_add" or kind == "event_delete":
        summary["binding_index"] = int(result.get("index", 0))

    return summary

func _apply_summary(
    results: Array,
    preview: bool,
    applied: bool
) -> Dictionary:
    var changed := 0

    for result in results:
        if bool(result.get("changed", false)):
            changed += 1

    return {
        "dry_run": preview,
        "applied": applied,
        "operation_count": results.size(),
        "changed": changed,
        "unchanged": results.size() - changed,
        "operations": results,
    }

func _snapshot_actions(operations: Array) -> Dictionary:
    var snapshot := {}

    for operation in operations:
        if not operation is Dictionary:
            continue

        var value = operation.get("action")

        if value is String and not value.is_empty():
            var name: String = value
            _snapshot_action(snapshot, PREFIX + name)

        if str(operation.get("operation", "")) == "rename":
            var new_value = operation.get("new_name")

            if new_value is String and not new_value.is_empty():
                var new_name: String = new_value
                _snapshot_action(snapshot, PREFIX + new_name)

    return snapshot

func _snapshot_action(snapshot: Dictionary, key: String) -> void:
    if snapshot.has(key):
        return

    snapshot[key] = {
        "exists": ProjectSettings.has_setting(key),
        "value": ProjectSettings.get_setting(key),
    }

func _restore_actions(snapshot: Dictionary) -> void:
    for key in snapshot:
        var state: Dictionary = snapshot[key]

        if bool(state.get("exists", false)):
            ProjectSettings.set_setting(key, state["value"])
        else:
            ProjectSettings.set_setting(key, null)

func _restore_project_file(
    existed: bool,
    data: PackedByteArray
) -> String:
    if existed:
        var file := FileAccess.open("res://project.godot", FileAccess.WRITE)

        if file == null:
            return error_string(FileAccess.get_open_error())

        file.store_buffer(data)
        file.close()
        return ""

    if FileAccess.file_exists("res://project.godot"):
        var error := DirAccess.remove_absolute("res://project.godot")

        if error != OK:
            return error_string(error)

    return ""

func _finish_mutation() -> void:
    if not defer_reload:
        _reload_input_map()

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
    if defer_save:
        return null

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
