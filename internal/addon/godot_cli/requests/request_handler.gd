class_name RequestHandler
extends RefCounted

var actions: Dictionary = {}

var scene_context: EditorSceneContext
var scene_service: SceneService
var node_service: NodeService
var busy := false

func _init(editor_interface: EditorInterface, undo_redo: EditorUndoRedoManager) -> void:
    scene_context = EditorSceneContext.new(editor_interface)

    scene_service = SceneService.new(scene_context)
    node_service = NodeService.new(scene_context, undo_redo)

    _register_service(scene_service)
    _register_service(node_service)

func _register_service(service: ActionService) -> void:
    var service_actions := service.get_actions()

    for action_name in service_actions:
        if actions.has(action_name):
            push_error("Duplicate action registered: %s" % action_name)
            continue

        actions[action_name] = service_actions[action_name]

func handle(request: Request) -> Response:
    if busy:
        return Response.failure(
            ErrorCode.BUSY,
            "Another editor action is in progress; retry after it finishes"
        )

    var action_name: String = request.action

    if not actions.has(action_name):
        return Response.failure(
            ErrorCode.UNSUPPORTED,
            "Unknown action: %s" % action_name
        )

    var action: ActionHandler = actions[action_name]
    var error := _validate_options(request, action)

    if error != null:
        return error

    error = _run_preconditions(request, action)

    if error != null:
        return error

    busy = true

    var response = await action.callable.call(request)

    if response is Response and response.code.is_empty():
        response = _run_post_effects(request, action, response)

    busy = false

    return response

func _validate_options(request: Request, action: ActionHandler) -> Response:
    if request.params.has("expected_scene") and not action.requires_scene:
        return Response.failure(
            ErrorCode.INVALID_ARGUMENT,
            "This action does not support expected_scene"
        )

    var save_value = request.params.get("save", false)

    if not save_value is bool:
        return Response.failure(ErrorCode.INVALID_ARGUMENT, "save must be a boolean")

    if save_value and not action.supports_save:
        return Response.failure(
            ErrorCode.INVALID_ARGUMENT,
            "This action does not support save"
        )

    return null

func _run_preconditions(request: Request, action: ActionHandler) -> Response:
    if action.requires_scene:
        return scene_context.validate(request.params)

    return null

func _run_post_effects(
    request: Request,
    action: ActionHandler,
    response: Response
) -> Response:
    if not action.supports_save or not request.params.get("save", false):
        return response

    if not response.result is Dictionary:
        return Response.failure(
            ErrorCode.INTERNAL_ERROR,
            "Action returned a result that cannot include post-effects"
        )

    var save_response := scene_context.save()

    if not save_response.code.is_empty():
        return Response.failure(
            save_response.code,
            "Action completed, but the scene could not be saved: %s" % save_response.message
        )

    response.result["saved"] = true

    return response
