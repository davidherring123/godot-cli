@tool
extends RefCounted

signal changed

enum Status {
    NOT_BUNDLED,
    STOPPED,
    STARTING,
    RUNNING,
    ERROR,
}

const HOST := "127.0.0.1"
const STATE_RESOURCE_PATH := "res://.godot/godot-cli/mcp/backend.json"
const EXECUTABLE_BASENAME := "godot-mcp-host"
const POLL_INTERVAL := 0.4

# Bundles use Godot's operating-system and architecture names, for example
# bin/macos-arm64, bin/linux-x86_64, or bin/windows-x86_64.

var status: Status = Status.STOPPED
var endpoint := ""
var last_error := ""
var state_pid := 0
var state_port := 0

var _project_root := ""
var _state_path := ""
var _executable_path := ""
var _managed_pid := 0
var _state_owned := false
var _poll_elapsed := 0.0
var _state_read_error := ""

func _init() -> void:
    _project_root = ProjectSettings.globalize_path("res://").trim_suffix("/")
    _state_path = ProjectSettings.globalize_path(STATE_RESOURCE_PATH)
    _executable_path = _find_bundled_executable()

    if _executable_path.is_empty():
        status = Status.NOT_BUNDLED
    else:
        status = Status.STOPPED

static func platform_name() -> String:
    match OS.get_name():
        "macOS":
            return "macos"
        "Windows":
            return "windows"
        _:
            return OS.get_name().to_lower()

static func architecture_name() -> String:
    return Engine.get_architecture_name().to_lower()

static func bundle_directory() -> String:
    return "res://addons/godot_cli/bin/%s-%s" % [
        platform_name(),
        architecture_name(),
    ]

static func executable_relative_path() -> String:
    var file_name := EXECUTABLE_BASENAME

    if OS.get_name() == "Windows":
        file_name += ".exe"

    return bundle_directory().path_join(file_name)

func is_bundled() -> bool:
    return not _executable_path.is_empty()

func get_executable_path() -> String:
    return _executable_path

func get_state_path() -> String:
    return _state_path

func get_managed_pid() -> int:
    return _managed_pid

func has_process() -> bool:
    return _managed_pid > 0

func start() -> bool:
    if _managed_pid > 0:
        return true

    if _executable_path.is_empty():
        _executable_path = _find_bundled_executable()

    if _executable_path.is_empty():
        status = Status.NOT_BUNDLED
        endpoint = ""
        state_pid = 0
        state_port = 0
        last_error = ""
        changed.emit()
        return false

    var directory_error := DirAccess.make_dir_recursive_absolute(
        _state_path.get_base_dir()
    )

    if directory_error != OK:
        status = Status.ERROR
        last_error = "Could not create the MCP backend state directory: %s" % _state_path.get_base_dir()
        changed.emit()
        return false

    if FileAccess.file_exists(_state_path):
        var remove_error := DirAccess.remove_absolute(_state_path)

        if remove_error != OK:
            status = Status.ERROR
            last_error = "Could not clear the previous MCP backend state file: %s" % _state_path
            changed.emit()
            return false

    status = Status.STARTING
    endpoint = ""
    state_pid = 0
    state_port = 0
    last_error = ""
    _state_read_error = ""
    _state_owned = false
    _poll_elapsed = 0.0

    var arguments := PackedStringArray([
        "mcp",
        "--transport",
        "http",
        "--host",
        HOST,
        "--port",
        "0",
        "--project",
        _project_root,
        "--state-file",
        _state_path,
    ])

    var pid := OS.create_process(
        ProjectSettings.globalize_path(_executable_path),
        arguments
    )

    if pid <= 0:
        status = Status.ERROR
        last_error = "Could not start the bundled MCP backend: %s" % _executable_path
        changed.emit()
        return false

    _managed_pid = pid
    _state_owned = true
    changed.emit()
    return true

func stop() -> void:
    var preserve_error := status == Status.ERROR

    if _managed_pid > 0:
        if OS.is_process_running(_managed_pid):
            var kill_error := OS.kill(_managed_pid)

            if kill_error != OK:
                status = Status.ERROR
                last_error = "Could not stop MCP backend process %d (error %d)." % [
                    _managed_pid,
                    kill_error,
                ]
                changed.emit()
                return

        _managed_pid = 0

    if _state_owned:
        _remove_state_file()
        _state_owned = false

    if status != Status.NOT_BUNDLED:
        status = Status.STOPPED

    endpoint = ""
    state_pid = 0
    state_port = 0
    _state_read_error = ""

    if not preserve_error:
        last_error = ""

    changed.emit()

func restart() -> bool:
    stop()
    return start()

func shutdown() -> void:
    if _managed_pid > 0:
        if OS.is_process_running(_managed_pid):
            OS.kill(_managed_pid)

        _managed_pid = 0

    if _state_owned:
        _remove_state_file()
        _state_owned = false

func poll(delta: float) -> void:
    if _managed_pid <= 0:
        return

    _poll_elapsed += delta

    if _poll_elapsed < POLL_INTERVAL:
        return

    _poll_elapsed = 0.0
    var before := _snapshot()
    _poll_managed_backend()

    if _snapshot() != before:
        changed.emit()

func _poll_managed_backend() -> void:
    var state := _read_state_file()

    if not state.is_empty():
        _apply_state(state)
    elif not _state_read_error.is_empty():
        last_error = _state_read_error

    if not OS.is_process_running(_managed_pid):
        _managed_pid = 0
        endpoint = ""
        state_pid = 0
        state_port = 0

        if status != Status.STOPPED:
            status = Status.ERROR

            if last_error.is_empty():
                last_error = "The MCP backend process exited unexpectedly."

func _read_state_file() -> Dictionary:
    _state_read_error = ""

    if not FileAccess.file_exists(_state_path):
        return {}

    var file := FileAccess.open(_state_path, FileAccess.READ)

    if file == null:
        _state_read_error = "Could not read the MCP backend state file: %s" % _state_path
        return {}

    var content := file.get_as_text()
    file.close()

    if content.strip_edges().is_empty():
        return {}

    var json := JSON.new()
    var parse_error := json.parse(content)

    if parse_error != OK:
        _state_read_error = "Could not parse the MCP backend state file (line %d): %s" % [
            json.get_error_line(),
            json.get_error_message(),
        ]
        return {}

    if not json.data is Dictionary:
        _state_read_error = "The MCP backend state file must contain a JSON object."
        return {}

    return json.data

func _apply_state(state: Dictionary) -> void:
    var state_status := _string_value(
        state.get("status", state.get("state", ""))
    ).to_lower().strip_edges()
    var current_error := _string_value(state.get("error", ""))
    var last_state_error := _string_value(
        state.get(
            "last_error",
            state.get("lastError", state.get("message", ""))
        )
    )
    var state_host := _string_value(state.get("host", HOST))

    if state_host.is_empty():
        state_host = HOST

    state_pid = int(state.get(
        "pid",
        state.get("process_id", state.get("processId", 0))
    ))
    state_port = int(state.get(
        "port",
        state.get("http_port", state.get("httpPort", 0))
    ))

    var state_endpoint := _string_value(state.get(
        "endpoint",
        state.get(
            "url",
            state.get("mcp_url", state.get("server_url", ""))
        )
    ))

    if not state_endpoint.is_empty():
        endpoint = state_endpoint
    elif state_port > 0:
        endpoint = "http://%s:%d" % [state_host, state_port]

    if not current_error.is_empty():
        status = Status.ERROR
        last_error = current_error
        return

    if state_status in ["error", "failed", "failure"]:
        status = Status.ERROR
        last_error = last_state_error

        if last_error.is_empty():
            last_error = "The MCP backend reported an error."

        return

    if state_status in ["starting", "initializing", "binding", "pending"]:
        status = Status.STARTING
    elif state_status in ["running", "ready", "listening", "started", "ok"]:
        status = Status.RUNNING
    elif state_status in ["stopped", "stopping", "exited"]:
        status = Status.STOPPED
        endpoint = ""
        state_port = 0
    elif state.has("running") and state.get("running") == true:
        status = Status.RUNNING
    elif state.has("ready") and state.get("ready") == true:
        status = Status.RUNNING
    elif not endpoint.is_empty() or state_pid > 0:
        status = Status.RUNNING
    else:
        status = Status.STARTING

    if not last_state_error.is_empty():
        last_error = last_state_error
    elif status == Status.RUNNING:
        last_error = ""

func _snapshot() -> Array:
    return [
        status,
        endpoint,
        last_error,
        state_pid,
        state_port,
        _managed_pid,
    ]

func _find_bundled_executable() -> String:
    var path := executable_relative_path()

    if FileAccess.file_exists(path):
        return path

    return ""

func _remove_state_file() -> void:
    if FileAccess.file_exists(_state_path):
        DirAccess.remove_absolute(_state_path)

func _string_value(value) -> String:
    if value is String or value is StringName:
        return str(value)

    return ""
