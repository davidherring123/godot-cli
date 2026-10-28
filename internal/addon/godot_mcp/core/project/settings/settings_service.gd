class_name ProjectSettingsService
extends ActionService

var defer_save := false
var batch_overrides: Dictionary = {}
var track_overrides := false

func get_actions() -> Dictionary:
    return {
        "project.settings.list": ActionHandler.new(list_settings, false, false, false),
        "project.settings.get": ActionHandler.new(get_setting_action, false, false, false),
        "project.settings.set": ActionHandler.new(set_setting_action, false, false, false),
        "project.settings.unset": ActionHandler.new(unset_setting_action, false, false, false),
        "project.settings.apply": ActionHandler.new(apply, false, false, false),
    }

func list_settings(request: Request) -> Response:
    var prefix = request.params.get("prefix", "")

    if not prefix is String:
        return Response.failure(ErrorCode.INVALID_ARGUMENT, "prefix must be a string")

    var overrides := _overrides()
    var settings: Array[Dictionary] = []

    for definition in ProjectSettings.get_property_list():
        if not _is_editor_setting(definition):
            continue

        var name: String = definition.get("name", "")

        if not prefix.is_empty() and not name.begins_with(prefix):
            continue

        settings.append(_describe(name, definition, overrides))

    settings.sort_custom(_sort_settings)

    return Response.success({"settings": settings})

func get_setting_action(request: Request) -> Response:
    var response := _setting_name(request.params.get("setting"))

    if not response.code.is_empty():
        return response

    var name: String = response.result
    var definition := _find_definition(name)

    if definition.is_empty():
        return _not_found(name)

    return Response.success(_describe(name, definition, _overrides()))

func set_setting_action(request: Request) -> Response:
    var response := _setting_name(request.params.get("setting"))

    if not response.code.is_empty():
        return response

    var name: String = response.result
    var definition := _find_definition(name)

    if definition.is_empty():
        return _not_found(name)

    if not request.params.has("value"):
        return Response.failure(ErrorCode.INVALID_ARGUMENT, "A value is required")

    var decoded := VariantDeserializer.deserialize(
        request.params["value"],
        int(definition.get("type", TYPE_NIL)),
        int(definition.get("hint", PROPERTY_HINT_NONE)),
        str(definition.get("hint_string", ""))
    )

    if not decoded.code.is_empty():
        return decoded

    var converted = decoded.result
    var previous = ProjectSettings.get_setting(name)
    var overridden := _overrides().has(name)
    var default = ProjectSettings.property_get_revert(name)

    if previous == converted:
        return Response.success(_change_result(
            name,
            definition,
            converted,
            default,
            overridden,
            false
        ))

    ProjectSettings.set_setting(name, converted)

    if track_overrides:
        # project.godot omits values that match the engine default, so a set
        # back to the default clears the override in the simulated batch state.
        if converted == default:
            batch_overrides.erase(name)
        else:
            batch_overrides[name] = true

    var save_error := _save()

    if save_error != null:
        ProjectSettings.set_setting(name, previous)

        if track_overrides:
            if overridden:
                batch_overrides[name] = true
            else:
                batch_overrides.erase(name)

        return save_error

    return Response.success(_change_result(
        name,
        definition,
        converted,
        default,
        _overrides().has(name),
        true
    ))

func unset_setting_action(request: Request) -> Response:
    var response := _setting_name(request.params.get("setting"))

    if not response.code.is_empty():
        return response

    var name: String = response.result
    var definition := _find_definition(name)

    if definition.is_empty():
        return _not_found(name)

    var default = ProjectSettings.property_get_revert(name)

    if not _overrides().has(name):
        return Response.success(_change_result(
            name,
            definition,
            default,
            default,
            false,
            false
        ))

    var previous = ProjectSettings.get_setting(name)

    ProjectSettings.set_setting(name, default)

    if track_overrides:
        batch_overrides.erase(name)

    var save_error := _save()

    if save_error != null:
        ProjectSettings.set_setting(name, previous)

        if track_overrides:
            batch_overrides[name] = true

        return save_error

    return Response.success(_change_result(
        name,
        definition,
        default,
        default,
        false,
        true
    ))

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

    var snapshot := _snapshot_settings(operations)
    var project_existed := FileAccess.file_exists("res://project.godot")
    var project_snapshot := (
        FileAccess.get_file_as_bytes("res://project.godot")
        if project_existed
        else PackedByteArray()
    )
    var summaries: Array = []
    var allowed_fields := {
        "set": ["operation", "setting", "value"],
        "unset": ["operation", "setting"],
    }

    # A batch has to reason about overrides as it goes: project.godot is not
    # saved until the end, so reading the file after every operation would
    # ignore earlier set/unset operations in this same batch.
    track_overrides = false
    batch_overrides = _overrides()
    track_overrides = true
    defer_save = true

    for index in operations.size():
        var operation = operations[index]

        if not operation is Dictionary:
            var invalid := Response.failure(
                ErrorCode.INVALID_ARGUMENT,
                "Each operation must be an object"
            )

            _end_batch()
            _restore_settings(snapshot)
            return batch_operation_failure(
                index,
                operation,
                "",
                invalid,
                "no changes were saved"
            )

        var kind := str(operation.get("operation", ""))
        var target := str(operation.get("setting", ""))
        var field_error := validate_operation_fields(
            index,
            operation,
            allowed_fields
        )

        if field_error != null:
            _end_batch()
            _restore_settings(snapshot)
            return batch_operation_failure(
                index,
                operation,
                target,
                field_error,
                "no changes were saved"
            )

        var response: Response

        match kind:
            "set":
                response = set_setting_action(
                    Request.new("project.settings.set", operation)
                )
            "unset":
                response = unset_setting_action(
                    Request.new("project.settings.unset", operation)
                )
            _:
                response = Response.failure(
                    ErrorCode.INVALID_ARGUMENT,
                    "Unsupported operation: %s" % kind
                )

        if not response.code.is_empty():
            _end_batch()
            _restore_settings(snapshot)
            return batch_operation_failure(
                index,
                operation,
                target,
                response,
                "no changes were saved"
            )

        summaries.append(_apply_operation_summary(index, operation, response.result))

    if dry_run_requested:
        _end_batch()
        _restore_settings(snapshot)
        return Response.success(_apply_summary(summaries, true, false))

    if _apply_changed_count(summaries) == 0:
        _end_batch()
        return Response.success(_apply_summary(summaries, false, true))

    var save_error := ProjectSettings.save()

    if save_error != OK:
        _restore_settings(snapshot)
        var restore_error := _restore_project_file(project_existed, project_snapshot)

        _end_batch()

        if not restore_error.is_empty():
            return Response.failure(
                ErrorCode.INTERNAL_ERROR,
                "Could not save project.godot after %d operation(s): %s; rollback failed: %s" % [
                    summaries.size(),
                    error_string(save_error),
                    restore_error,
                ]
            )

        return Response.failure(
            ErrorCode.ACTION_FAILED,
            "Could not save project.godot after %d operation(s): %s; restored project.godot and in-memory state" % [
                summaries.size(),
                error_string(save_error),
            ]
        )

    _end_batch()

    return Response.success(_apply_summary(summaries, false, true))

func _end_batch() -> void:
    defer_save = false
    track_overrides = false
    batch_overrides.clear()

func _apply_changed_count(results: Array) -> int:
    var changed := 0

    for result in results:
        if bool(result.get("changed", false)):
            changed += 1

    return changed

func _apply_operation_summary(
    index: int,
    operation: Dictionary,
    result: Dictionary
) -> Dictionary:
    var kind := str(operation.get("operation", ""))
    var changed := bool(result.get("changed", true))
    var summary := {
        "index": index,
        "operation": kind,
        "setting": str(result.get("setting", operation.get("setting", ""))),
        "status": kind,
        "changed": changed,
    }

    match kind:
        "set":
            summary["status"] = "updated" if changed else "unchanged"
        "unset":
            summary["status"] = "unset" if changed else "unchanged"

    if result.has("type"):
        summary["type"] = result["type"]

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

func _snapshot_settings(operations: Array) -> Dictionary:
    var snapshot := {}

    for operation in operations:
        if not operation is Dictionary:
            continue

        var value = operation.get("setting")

        if not value is String or value.is_empty():
            continue

        var name: String = value

        if snapshot.has(name):
            continue

        if _find_definition(name).is_empty():
            continue

        snapshot[name] = ProjectSettings.get_setting(name)

    return snapshot

func _restore_settings(snapshot: Dictionary) -> void:
    for name in snapshot:
        ProjectSettings.set_setting(name, snapshot[name])

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

func _describe(name: String, definition: Dictionary, overrides: Dictionary) -> Dictionary:
    var value = ProjectSettings.get_setting(name)
    var default = ProjectSettings.property_get_revert(name)
    var hint: int = definition.get("hint", PROPERTY_HINT_NONE)
    var hint_string := str(definition.get("hint_string", ""))
    var described := {
        "name": name,
        "type": type_string(int(definition.get("type", TYPE_NIL))),
        "value": VariantSerializer.serialize(value),
        "default": VariantSerializer.serialize(default),
        "overridden": overrides.has(name),
    }
    var serialized_hint := PropertyHintSerializer.serialize(hint, hint_string, value)

    if serialized_hint != null:
        described["hint"] = serialized_hint

    return described

func _change_result(
    name: String,
    definition: Dictionary,
    value: Variant,
    default: Variant,
    overridden: bool,
    changed: bool
) -> Dictionary:
    return {
        "setting": name,
        "type": type_string(int(definition.get("type", TYPE_NIL))),
        "value": VariantSerializer.serialize(value),
        "default": VariantSerializer.serialize(default),
        "overridden": overridden,
        "changed": changed,
    }

func _is_editor_setting(definition: Dictionary) -> bool:
    var name: String = definition.get("name", "")

    if name.is_empty() or not name.contains("/"):
        return false

    var usage: int = definition.get("usage", 0)

    if (usage & PROPERTY_USAGE_CATEGORY) != 0 or (usage & PROPERTY_USAGE_GROUP) != 0:
        return false

    if (usage & PROPERTY_USAGE_INTERNAL) != 0:
        return false

    return (usage & PROPERTY_USAGE_EDITOR) != 0

func _find_definition(name: String) -> Dictionary:
    for definition in ProjectSettings.get_property_list():
        if str(definition.get("name", "")) == name:
            return definition

    return {}

func _overrides() -> Dictionary:
    if track_overrides:
        return batch_overrides

    var overrides := {}
    var config := ConfigFile.new()

    if config.load("res://project.godot") != OK:
        return overrides

    for section in config.get_sections():
        for key in config.get_section_keys(section):
            overrides["%s/%s" % [section, key]] = true

    return overrides

func _setting_name(value: Variant) -> Response:
    if not value is String or value.is_empty():
        return Response.failure(
            ErrorCode.INVALID_ARGUMENT,
            "A project setting name is required"
        )

    var name: String = value

    if name != name.strip_edges():
        return Response.failure(
            ErrorCode.INVALID_ARGUMENT,
            "Invalid project setting name: %s" % name
        )

    return Response.success(name)

func _not_found(name: String) -> Response:
    return Response.failure(
        ErrorCode.NOT_FOUND,
        "Project setting not found: %s" % name
    )

func _sort_settings(a: Dictionary, b: Dictionary) -> bool:
    return a["name"] < b["name"]

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
