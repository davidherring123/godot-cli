class_name ActionService
extends RefCounted

const MAX_BATCH_OPERATIONS := 100

func get_actions() -> Dictionary:
    return {}

func validate_batch_operations(value: Variant) -> Response:
    if not value is Array or value.is_empty():
        return Response.failure(
            ErrorCode.INVALID_ARGUMENT,
            "A nonempty operations array is required"
        )

    if value.size() > MAX_BATCH_OPERATIONS:
        return Response.failure(
            ErrorCode.INVALID_ARGUMENT,
            "At most %d operations are allowed" % MAX_BATCH_OPERATIONS
        )

    return null

func validate_operation_fields(
    index: int,
    operation: Dictionary,
    allowed: Dictionary
) -> Response:
    var kind := str(operation.get("operation", ""))

    if not allowed.has(kind):
        return null

    var permitted: Array = allowed[kind]

    for key in operation.keys():
        if permitted.has(str(key)):
            continue

        return Response.failure(
            ErrorCode.INVALID_ARGUMENT,
            "operation %d (%s): %s does not accept %s" % [
                index,
                kind,
                kind,
                key,
            ]
        )

    return null

func batch_operation_failure(
    index: int,
    operation: Variant,
    target: String,
    response: Response,
    suffix: String = ""
) -> Response:
    var kind := ""

    if operation is Dictionary:
        kind = str(operation.get("operation", ""))

    var label := kind

    if not target.is_empty():
        if label.is_empty():
            label = target
        else:
            label = "%s %s" % [kind, target]

    var message := "operation %d" % index

    if not label.is_empty():
        message += " (%s)" % label

    message += ": %s" % response.message

    if not suffix.is_empty():
        message += "; %s" % suffix

    return Response.failure(response.code, message)
