class_name NodeBuilder
extends RefCounted

class DecodedProperties:
    var converted: Dictionary = {}
    var previous: Dictionary = {}
    var error: Response = null

static func instantiate(node_type: String) -> Node:
    if ClassDB.class_exists(node_type):
        if not ClassDB.can_instantiate(node_type):
            return null

        var instance = ClassDB.instantiate(node_type)

        if instance is Node:
            return instance

        return null

    for entry in ProjectSettings.get_global_class_list():
        if str(entry.get("class", "")) != node_type:
            continue

        var script: Script = load(entry.get("path", ""))

        if script == null:
            return null

        var instance = script.new()

        if instance is Node:
            return instance

        return null

    return null

static func apply_properties(node: Node, values: Dictionary) -> Response:
    if values.has("script"):
        var decoded_script := decode_properties(node, {"script": values["script"]})

        if decoded_script.error != null:
            return decoded_script.error

        node.set_script(decoded_script.converted["script"])

    var remaining: Dictionary = values.duplicate()

    remaining.erase("script")

    var decoded := decode_properties(node, remaining)

    if decoded.error != null:
        return decoded.error

    for property_name in decoded.converted:
        node.set(property_name, decoded.converted[property_name])

    return null

static func decode_properties(node: Node, values: Dictionary) -> DecodedProperties:
    var definitions := {}

    for property in node.get_property_list():
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
            str(definition.get("hint_string", ""))
        )

        if not decoded.code.is_empty():
            return decode_failure(
                decoded.code,
                "%s: %s" % [property_name, decoded.message]
            )

        result.converted[property_name] = decoded.result
        result.previous[property_name] = node.get(property_name)

    return result

static func decode_failure(code: String, message: String) -> DecodedProperties:
    var result := DecodedProperties.new()

    result.error = Response.failure(code, message)

    return result
