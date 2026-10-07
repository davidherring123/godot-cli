class_name Bridge
extends RefCounted

const HOST := "127.0.0.1"
const START_PORT := 49152
const END_PORT := 49252

const CLIENT_PEER := "peer"
const CLIENT_BUFFER := "buffer"
const CLIENT_BUSY := "busy"

var server := TCPServer.new()
var clients: Array[Dictionary] = []
var request_handler: RequestHandler

func _init(request_handler: RequestHandler) -> void:
    self.request_handler = request_handler

func start() -> Error:
    for port in range(START_PORT, END_PORT + 1):
        var error := server.listen(port, HOST)

        if error == OK:
            return OK

        server.stop()

    return ERR_CANT_CREATE

func get_port() -> int:
    return server.get_local_port()

func stop() -> void:
    for client in clients:
        var peer: StreamPeerTCP = client[CLIENT_PEER]
        peer.disconnect_from_host()

    clients.clear()
    server.stop()

func poll() -> void:
    while server.is_connection_available():
        var peer := server.take_connection()

        if peer != null:
            clients.append({
                CLIENT_PEER: peer,
                CLIENT_BUFFER: "",
                CLIENT_BUSY: false,
            })

    for i in range(clients.size() - 1, -1, -1):
        _poll_client(i)

func _poll_client(index: int) -> void:
    var client := clients[index]
    var peer: StreamPeerTCP = client[CLIENT_PEER]

    peer.poll()

    if peer.get_status() == StreamPeerSocket.STATUS_NONE \
    or peer.get_status() == StreamPeerSocket.STATUS_ERROR:
        clients.remove_at(index)
        return

    var available := peer.get_available_bytes()

    if available > 0:
        client[CLIENT_BUFFER] += peer.get_utf8_string(available)

    if client[CLIENT_BUSY]:
        return

    var buffer: String = client[CLIENT_BUFFER]
    var newline := buffer.find("\n")

    if newline == -1:
        return

    var line := buffer.substr(0, newline).strip_edges()

    client[CLIENT_BUFFER] = buffer.substr(newline + 1)

    if line.is_empty():
        return

    client[CLIENT_BUSY] = true
    _dispatch(client, line)

func _dispatch(client: Dictionary, line: String) -> void:
    await _handle_request(client[CLIENT_PEER], line)

    client[CLIENT_BUSY] = false

func _handle_request(peer: StreamPeerTCP, line: String) -> void:
    var json_request = JSON.parse_string(line)

    if not json_request is Dictionary:
        _send(peer, Response.failure(ErrorCode.INVALID_REQUEST, "Request must be a JSON object"))
        return

    var action_name = json_request.get("action")

    if not action_name is String or action_name.is_empty():
        _send(
            peer,
            Response.failure(
                ErrorCode.INVALID_REQUEST,
                "action must be a nonempty string"
            )
        )
        return

    if not json_request.get("params", {}) is Dictionary:
        _send(peer, Response.failure(ErrorCode.INVALID_REQUEST, "params must be a JSON object"))
        return

    var request := Request.from_dict(json_request)

    var response = await request_handler.handle(request)

    if not response is Response:
        _send(
            peer,
            Response.failure(
                ErrorCode.INTERNAL_ERROR,
                "Action returned an invalid response"
            )
        )
        return

    _send(peer, response)

func _send(peer: StreamPeerTCP, response: Response) -> void:
    if peer.get_status() != StreamPeerTCP.STATUS_CONNECTED:
        return

    var data := (response.to_json() + "\n").to_utf8_buffer()

    var error := peer.put_data(data)

    if error != OK:
        push_error("godot-cli: failed to send response")
