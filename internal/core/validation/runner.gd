extends SceneTree

const REPORT_PREFIX := "GODOT_MCP_VALIDATE:"
const RUN_FRAMES := 3

var scene_path := ""
var errors: Array[String] = []
var valid := false

func _initialize() -> void:
    var args := OS.get_cmdline_user_args()

    if args.is_empty():
        _report()
        quit(2)
        return

    scene_path = args[0]

    if not ResourceLoader.exists(scene_path, "PackedScene"):
        errors.append("Scene not found: %s" % scene_path)
        _report()
        quit(1)
        return

    var packed := ResourceLoader.load(scene_path, "PackedScene") as PackedScene

    if packed == null:
        errors.append("Could not load scene: %s" % scene_path)
        _report()
        quit(1)
        return

    var instance := packed.instantiate()

    if instance == null:
        errors.append("Could not instantiate scene: %s" % scene_path)
        _report()
        quit(1)
        return

    root.add_child(instance)

    for _frame in RUN_FRAMES:
        await process_frame

    valid = true
    _report()
    quit(0)

func _report() -> void:
    print(REPORT_PREFIX + JSON.stringify({
        "valid": valid,
        "scene": scene_path,
        "errors": errors,
    }))
