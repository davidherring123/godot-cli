class_name Bridge
extends RefCounted

const HOST := "127.0.0.1"
const START_PORT := 49152
const END_PORT := 49252

var server := TCPServer.new()
var clients: Array[Dictionary] = []


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
        var peer: StreamPeerTCP = client["peer"]
        peer.disconnect_from_host()
    
    clients.clear()
    server.stop()


func poll() -> void:
    while server.is_connection_available():
        var peer := server.take_connection()

        if peer != null:
            clients.append({
                "peer": peer,
                "buffer": "",
            })
        
    for i in range(clients.size() - 1, -1, -1):
        _poll_client(i)
    
func _poll_client(index: int) -> void:
    var client := clients[index]
    var peer: StreamPeerTCP = client["peer"]

    peer.poll()

    if peer.get_status() == StreamPeerSocket.STATUS_NONE \
    or peer.get_status() == StreamPeerSocket.STATUS_ERROR:
        clients.remove_at(index)
        return
    
    var available := peer.get_available_bytes()

    if available <= 0:
        return

    client["buffer"] += peer.get_utf8_string(available)

    var buffer: String = client["buffer"]
    
    while true:
        var newline := buffer.find("\n")

        if newline == -1:
            break
        
        var line := buffer.substr(0, newline).strip_edges()
        buffer = buffer.substr(newline + 1)

        if not line.is_empty():
            _handle_request(peer, line)
    
    client["buffer"] = buffer
    clients[index] = client

func _handle_request(peer: StreamPeerTCP, line: String) -> void:
    var request = JSON.parse_string(line)

    if not request is Dictionary:
        _send(peer, {
            "ok": false,
            "error": "invalid request"
        })
        return
    
    match request.get("command", ""):
        "ping":
            _send(peer, {
                "ok": true,
                "result": "pong"
            })
        
        _:
            _send(peer, {
                "ok": false,
                "error": "unknown command"
            })
        
    
func _send(peer: StreamPeerTCP, response: Dictionary) -> void:
    var data := (JSON.stringify(response) + "\n").to_utf8_buffer()

    var error := peer.put_data(data)

    if error != OK:
        push_error("godot-cli: failed to send response")
