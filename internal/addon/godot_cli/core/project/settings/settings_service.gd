class_name ProjectSettingsService
extends ActionService

func get_actions() -> Dictionary:
    return {
        "project.settings.list": ActionHandler.new(list_settings, false, false, false),
        "project.settings.get": ActionHandler.new(get_setting_action, false, false, false),
        "project.settings.set": ActionHandler.new(set_setting_action, false, false, false),
        "project.settings.unset": ActionHandler.new(unset_setting_action, false, false, false),
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

    var save_error := _save()

    if save_error != null:
        ProjectSettings.set_setting(name, previous)

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

    var save_error := _save()

    if save_error != null:
        ProjectSettings.set_setting(name, previous)

        return save_error

    return Response.success(_change_result(
        name,
        definition,
        default,
        default,
        false,
        true
    ))

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
    var error := ProjectSettings.save()

    if error != OK:
        return Response.failure(
            ErrorCode.ACTION_FAILED,
            "Could not save project.godot: %s" % error_string(error)
        )

    return null
