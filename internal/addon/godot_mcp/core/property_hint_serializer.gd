class_name PropertyHintSerializer
extends RefCounted

static func serialize(
    hint: int,
    hint_string: String,
    value: Variant
) -> Variant:
    match hint:
        PROPERTY_HINT_NONE:
            return null

        PROPERTY_HINT_ENUM:
            return {
                "type": "enum",
                "options": hint_string.split(",")
            }

        PROPERTY_HINT_RANGE:
            return {
                "type": "range",
                "value": hint_string
            }

        PROPERTY_HINT_RESOURCE_TYPE:
            return {
                "type": "resource_type",
                "types": hint_string.split(",")
            }

        PROPERTY_HINT_MULTILINE_TEXT:
            return {
                "type": "multiline_text"
            }

        PROPERTY_HINT_LAYERS_2D_PHYSICS:
            return _serialize_layers(
                "layers_2d_physics",
                "layer_names/2d_physics/layer_",
                int(value)
            )

        PROPERTY_HINT_LAYERS_2D_RENDER:
            return _serialize_layers(
                "layers_2d_render",
                "layer_names/2d_render/layer_",
                int(value)
            )

        PROPERTY_HINT_LAYERS_2D_NAVIGATION:
            return _serialize_layers(
                "layers_2d_navigation",
                "layer_names/2d_navigation/layer_",
                int(value)
            )

        _:
            return {
                "type": "unknown",
                "value": hint_string
            }

static func _serialize_layers(
    type: String,
    setting_prefix: String,
    value: int
) -> Dictionary:
    var selected: Array[Dictionary] = []

    for index in range(1, 33):
        var bit := 1 << (index - 1)

        if (value & bit) == 0:
            continue

        var setting := setting_prefix + str(index)

        var layer_name := str(
            ProjectSettings.get_setting(
                setting,
                "Layer %d" % index
            )
        )

        if layer_name.is_empty():
            layer_name = "Layer %d" % index

        selected.append({
            "index": index,
            "name": layer_name
        })

    return {
        "type": type,
        "selected": selected
    }