class_name PropertyCodec
extends RefCounted

class DecodedProperties:
    var converted: Dictionary = {}
    var previous: Dictionary = {}
    var error: Response = null

static func apply_properties(
    object: Object,
    values: Dictionary,
    resource_overrides: Dictionary = {}
) -> Response:
    if values.has("script"):
        var decoded_script := decode_properties(
            object,
            {"script": values["script"]},
            resource_overrides
        )

        if decoded_script.error != null:
            return decoded_script.error

        object.set_script(decoded_script.converted["script"])

    var remaining: Dictionary = values.duplicate()

    remaining.erase("script")

    var decoded := decode_properties(object, remaining, resource_overrides)

    if decoded.error != null:
        return decoded.error

    for property_name in decoded.converted:
        object.set(property_name, decoded.converted[property_name])

    return null

static func decode_properties(
    object: Object,
    values: Dictionary,
    resource_overrides: Dictionary = {}
) -> DecodedProperties:
    var definitions := {}

    for property in object.get_property_list():
        definitions[property.name] = property

    var names: Array = values.keys()

    names.sort()

    var result := DecodedProperties.new()

    for property_name in names:
        if not definitions.has(property_name):
            return decode_failure(
                ErrorCode.NOT_FOUND,
                "Unknown property: %s" % property_name
            )

        var definition: Dictionary = definitions[property_name]
        var usage: int = definition.get("usage", 0)

        if (usage & PROPERTY_USAGE_EDITOR) == 0 or (usage & PROPERTY_USAGE_READ_ONLY) != 0:
            return decode_failure(
                ErrorCode.FAILED_PRECONDITION,
                "Property is not editor-writable: %s" % property_name
            )

        var decoded := VariantDeserializer.deserialize(
            values[property_name],
            definition.type,
            definition.get("hint", PROPERTY_HINT_NONE),
            str(definition.get("hint_string", "")),
            resource_overrides
        )

        if not decoded.code.is_empty():
            return decode_failure(
                decoded.code,
                "%s: %s" % [property_name, decoded.message]
            )

        result.converted[property_name] = decoded.result
        result.previous[property_name] = object.get(property_name)

    return result

static func decode_failure(code: String, message: String) -> DecodedProperties:
    var result := DecodedProperties.new()

    result.error = Response.failure(code, message)

    return result

static func collect_properties(object: Object, filters: Dictionary = {}) -> Array[Dictionary]:
    var properties: Array[Dictionary] = []

    for property in object.get_property_list():
        var usage: int = property.get("usage", 0)
        var property_type: int = property.get("type", TYPE_NIL)

        if (usage & PROPERTY_USAGE_EDITOR) == 0:
            continue

        if property_type == TYPE_NIL:
            continue

        var property_name: String = property.get("name", "")

        if not filters.is_empty() and not filters.has(property_name):
            continue

        if filters.has(property_name):
            filters[property_name] = true

        var value = object.get(property_name)

        var property_hint: int = property.get("hint", PROPERTY_HINT_NONE)
        var hint_string: String = property.get("hint_string", "")

        var serialized_hint := PropertyHintSerializer.serialize(
            property_hint,
            hint_string,
            value
        )

        var serialized_property := {
            "name": property_name,
            "type": type_string(property_type),
            "value": VariantSerializer.serialize(value)
        }

        if serialized_hint != null:
            serialized_property["hint"] = serialized_hint

        properties.append(serialized_property)

    return properties
