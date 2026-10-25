extends SceneTree

const SESSION_DIR_ARG := "--session-dir"
const SCENE_ARG := "--scene"
const READY_FRAMES := 2
const HEARTBEAT_INTERVAL := 0.5

var session_dir := ""
var scene_path := ""
var start_ticks := 0
var frame_count := 0
var heartbeat_elapsed := 0.0
var ready_written := false
var quitting := false

func _initialize() -> void:
    start_ticks = Time.get_ticks_msec()

    var args := OS.get_cmdline_user_args()
    session_dir = _argument(args, SESSION_DIR_ARG)
    scene_path = _argument(args, SCENE_ARG)

    if session_dir.is_empty() or scene_path.is_empty():
        printerr("godot-cli runtime: missing --session-dir or --scene")
        quit(2)
        return

    if not _make_directories():
        printerr("godot-cli runtime: could not create the session directories")
        quit(2)
        return

    if not ResourceLoader.exists(scene_path, "PackedScene"):
        _write_ready(false, "Scene not found: %s" % scene_path)
        _finish(1, "scene_not_found")
        return

    var packed := ResourceLoader.load(scene_path, "PackedScene") as PackedScene

    if packed == null:
        _write_ready(false, "Could not load scene: %s" % scene_path)
        _finish(1, "scene_load_failed")
        return

    var instance := packed.instantiate()

    if instance == null:
        _write_ready(false, "Could not instantiate scene: %s" % scene_path)
        _finish(1, "scene_instantiate_failed")
        return

    get_root().add_child(instance)
    current_scene = instance

func _process(delta: float) -> bool:
    frame_count += 1

    if not ready_written and frame_count >= READY_FRAMES:
        _write_ready(true, "")
        _write_heartbeat()

    heartbeat_elapsed += delta

    if heartbeat_elapsed >= HEARTBEAT_INTERVAL:
        heartbeat_elapsed = 0.0
        _write_heartbeat()

    if FileAccess.file_exists(_path("stop.request")):
        _finish(0, "stop")
        return false

    _process_capture_requests()

    return false

func _finalize() -> void:
    if not quitting:
        _finish(0, "closed")

func _argument(args: PackedStringArray, name: String) -> String:
    if args.is_empty():
        return ""

    for index in range(args.size() - 1):
        if args[index] == name:
            return args[index + 1]

    return ""

func _make_directories() -> bool:
    for name in ["requests", "responses", "captures"]:
        var error := DirAccess.make_dir_recursive_absolute(
            session_dir.path_join(name)
        )

        if error != OK and error != ERR_ALREADY_EXISTS:
            return false

    return true

func _path(name: String) -> String:
    return session_dir.path_join(name)

func _write_ready(ok: bool, message: String) -> void:
    if ready_written and ok:
        return

    ready_written = true

    var window_size := get_root().size

    _write_json(_path("ready.json"), {
        "ok": ok,
        "message": message,
        "scene": scene_path,
        "display_server": DisplayServer.get_name(),
        "headless": DisplayServer.get_name() == "headless",
        "window_size": {
            "width": window_size.x,
            "height": window_size.y,
        },
        "frames": frame_count,
        "pid": OS.get_process_id(),
    })

func _write_heartbeat() -> void:
    _write_json(_path("heartbeat.json"), {
        "frames": frame_count,
        "uptime_ms": Time.get_ticks_msec() - start_ticks,
    })

func _finish(code: int, reason: String) -> void:
    if quitting:
        return

    quitting = true

    _write_json(_path("exit.json"), {
        "reason": reason,
        "code": code,
        "frames": frame_count,
        "uptime_ms": Time.get_ticks_msec() - start_ticks,
    })

    quit(code)

func _process_capture_requests() -> void:
    var requests_dir := _path("requests")
    var names := DirAccess.get_files_at(requests_dir)

    if names.is_empty():
        return

    names.sort()

    for name in names:
        if not name.ends_with(".json"):
            continue

        var request_path := requests_dir.path_join(name)
        var identifier := name.get_basename()
        var response_path := _path("responses").path_join(identifier + ".json")
        var request = _read_json(request_path)

        DirAccess.remove_absolute(request_path)

        if not request is Dictionary:
            _write_json(response_path, {
                "ok": false,
                "code": "invalid_argument",
                "message": "capture request was not a JSON object",
            })
            continue

        var output := str(request.get("path", ""))

        _write_json(response_path, _capture(output))

func _capture(output: String) -> Dictionary:
    if output.is_empty():
        return {
            "ok": false,
            "code": "invalid_argument",
            "message": "capture request is missing an output path",
        }

    var absolute := output

    if output.begins_with("res://"):
        absolute = ProjectSettings.globalize_path(output)

    absolute = absolute.simplify_path()

    var allowed_root := ProjectSettings.globalize_path(
        "res://.godot/godot-cli/runtime"
    ).simplify_path()

    if not absolute.begins_with(allowed_root + "/"):
        return {
            "ok": false,
            "code": "invalid_argument",
            "message": "capture path must be inside res://.godot/godot-cli/runtime",
        }

    if DisplayServer.get_name() == "headless":
        return {
            "ok": false,
            "code": "unsupported",
            "message": "Screenshots require a rendering display; start the runtime with headless=false.",
        }

    var texture := get_root().get_texture()

    if texture == null:
        return {
            "ok": false,
            "code": "unsupported",
            "message": "The runtime root window has no rendered texture yet.",
        }

    var image := texture.get_image()

    if image == null or image.get_width() == 0 or image.get_height() == 0:
        return {
            "ok": false,
            "code": "unsupported",
            "message": "The runtime root window did not produce an image.",
        }

    var error := image.save_png(absolute)

    if error != OK:
        return {
            "ok": false,
            "code": "action_failed",
            "message": "Godot could not save the screenshot (error %d)." % error,
        }

    var bytes := 0
    var file := FileAccess.open(absolute, FileAccess.READ)

    if file != null:
        bytes = file.get_length()

    return {
        "ok": true,
        "width": image.get_width(),
        "height": image.get_height(),
        "bytes": bytes,
        "frames": frame_count,
    }

func _write_json(path: String, value: Dictionary) -> void:
    var temporary := path + ".tmp"
    var file := FileAccess.open(temporary, FileAccess.WRITE)

    if file == null:
        return

    file.store_string(JSON.stringify(value))
    file.close()

    DirAccess.rename_absolute(temporary, path)

func _read_json(path: String):
    var text := FileAccess.get_file_as_string(path)

    if text.is_empty():
        return null

    return JSON.parse_string(text)
