class_name InputEventCodec
extends RefCounted

static func serialize(event: InputEvent) -> Dictionary:
    var data := {"device": int(event.device)}

    if event is InputEventKey:
        var key: InputEventKey = event
        data["type"] = "key"

        if key.keycode != 0:
            data["keycode"] = int(key.keycode)

        if key.physical_keycode != 0:
            data["physical_keycode"] = int(key.physical_keycode)

        if key.shift_pressed:
            data["shift"] = true

        if key.ctrl_pressed:
            data["ctrl"] = true

        if key.alt_pressed:
            data["alt"] = true

        if key.meta_pressed:
            data["meta"] = true

        if key.echo:
            data["echo"] = true

        return data

    if event is InputEventMouseButton:
        var mouse: InputEventMouseButton = event
        data["type"] = "mouse_button"
        data["button_index"] = int(mouse.button_index)

        return data

    if event is InputEventJoypadButton:
        var button: InputEventJoypadButton = event
        data["type"] = "joypad_button"
        data["button_index"] = int(button.button_index)

        return data

    if event is InputEventJoypadMotion:
        var motion: InputEventJoypadMotion = event
        data["type"] = "joypad_motion"
        data["axis"] = int(motion.axis)
        data["axis_value"] = motion.axis_value

        return data

    data["type"] = event.get_class()

    return data

static func deserialize(value: Variant) -> Response:
    if not value is Dictionary:
        return Response.failure(
            ErrorCode.INVALID_ARGUMENT,
            "event must be a JSON object"
        )

    var event_type = value.get("type", "")

    if not event_type is String or event_type.is_empty():
        return Response.failure(
            ErrorCode.INVALID_ARGUMENT,
            "event.type is required"
        )

    var device = null

    if value.has("device"):
        if not _integer(value["device"]):
            return Response.failure(
                ErrorCode.INVALID_ARGUMENT,
                "event.device must be an integer"
            )

        device = int(value["device"])

    match event_type:
        "key":
            return _key(value, device)
        "mouse_button":
            return _mouse_button(value, device)
        "joypad_button":
            return _joypad_button(value, device)
        "joypad_motion":
            return _joypad_motion(value, device)

    return Response.failure(
        ErrorCode.INVALID_ARGUMENT,
        "Unsupported event type: %s" % event_type
    )

static func _key(value: Dictionary, device: Variant) -> Response:
    for field in ["keycode", "physical_keycode"]:
        if value.has(field) and not _integer(value[field]):
            return Response.failure(
                ErrorCode.INVALID_ARGUMENT,
                "event.%s must be an integer" % field
            )

    for field in ["shift", "ctrl", "alt", "meta", "echo"]:
        if value.has(field) and not value[field] is bool:
            return Response.failure(
                ErrorCode.INVALID_ARGUMENT,
                "event.%s must be a boolean" % field
            )

    var keycode := int(value.get("keycode", 0))
    var physical := int(value.get("physical_keycode", 0))

    if keycode == 0 and physical == 0:
        return Response.failure(
            ErrorCode.INVALID_ARGUMENT,
            "A key binding requires a keycode or physical_keycode"
        )

    var event := InputEventKey.new()

    if device != null:
        event.device = device

    if physical != 0:
        event.physical_keycode = physical
    else:
        event.keycode = keycode

    event.shift_pressed = bool(value.get("shift", false))
    event.ctrl_pressed = bool(value.get("ctrl", false))
    event.alt_pressed = bool(value.get("alt", false))
    event.meta_pressed = bool(value.get("meta", false))
    event.echo = bool(value.get("echo", false))

    return Response.success(event)

static func _mouse_button(value: Dictionary, device: Variant) -> Response:
    if not value.has("button_index") or not _integer(value["button_index"]):
        return Response.failure(
            ErrorCode.INVALID_ARGUMENT,
            "A mouse button binding requires an integer button_index"
        )

    var event := InputEventMouseButton.new()

    if device != null:
        event.device = device

    event.button_index = int(value["button_index"])

    return Response.success(event)

static func _joypad_button(value: Dictionary, device: Variant) -> Response:
    if not value.has("button_index") or not _integer(value["button_index"]):
        return Response.failure(
            ErrorCode.INVALID_ARGUMENT,
            "A joypad button binding requires an integer button_index"
        )

    var event := InputEventJoypadButton.new()

    if device != null:
        event.device = device

    event.button_index = int(value["button_index"])

    return Response.success(event)

static func _joypad_motion(value: Dictionary, device: Variant) -> Response:
    if (
        not value.has("axis")
        or not _integer(value["axis"])
        or not value.has("axis_value")
        or not _number(value["axis_value"])
    ):
        return Response.failure(
            ErrorCode.INVALID_ARGUMENT,
            "A joypad motion binding requires an integer axis and numeric axis_value"
        )

    var event := InputEventJoypadMotion.new()

    if device != null:
        event.device = device

    event.axis = int(value["axis"])
    event.axis_value = float(value["axis_value"])

    return Response.success(event)

static func _integer(value: Variant) -> bool:
    if value is int:
        return true

    return value is float and is_finite(value) and floor(value) == value

static func _number(value: Variant) -> bool:
    return (value is int or value is float) and is_finite(float(value))
