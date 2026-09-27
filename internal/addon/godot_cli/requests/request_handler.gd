class_name RequestHandler
extends RefCounted

var handlers: Dictionary = {}

var scene_service: SceneService
var node_service: NodeService

func _init(editor_interface: EditorInterface) -> void:
    scene_service = SceneService.new()
    node_service = NodeService.new()

    _register_service(scene_service)
    _register_service(node_service)

func _register_service(service: CommandService) -> void:
    for command in service.get_commands():
        if handlers.has(command):
            push_error("Duplicate command registered: %s" % command)
            continue
        
        handlers[command] = service.get_commands()[command]

func handle(request: Request) -> Response:
    var command: String = request.command

    if not handlers.has(command):
        return Response.failure("unknown_command", "Unknown command: %s" % command)
    
    var handler: Callable = handlers[command]
    var handler_response: Response = handler.call(request)

    return handler_response
