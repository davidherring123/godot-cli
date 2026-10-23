class_name SceneEditSession
extends RefCounted

# NodeService records mutations through the same UndoRedo-shaped interface for
# standalone actions and for scene.apply batches. This session executes do
# methods immediately so later operations can validate against their result,
# but it defers EditorUndoRedoManager registration until the whole batch has
# succeeded. A failed batch therefore never leaves phantom history behind:
# rollback directly runs the accumulated undo methods in reverse order.

const NO_ARG := &"__godot_cli_no_arg__"

var target: Object
var batch_name: String

var _pending_do: Array[Dictionary] = []
var _pending_undo: Array[Dictionary] = []
var _do_items: Array[Dictionary] = []
var _undo_items: Array[Dictionary] = []
var _context: Node = null
var _op_backward := false
var _applied_count := 0
var _execution_error := ""
var _rollback_notes: Array[String] = []

func _init(
    undo_target: Object = null,
    title := "godot-cli: Apply scene batch",
    custom_context: Node = null
) -> void:
    target = undo_target
    batch_name = title
    _context = custom_context

func create_action(
    _name: String,
    _merge_mode: int,
    custom_context: Variant = null,
    backward_undo_ops: bool = false
) -> void:
    _op_backward = backward_undo_ops

    if _context == null and custom_context is Node:
        _context = custom_context

    _pending_do.clear()
    _pending_undo.clear()

func add_do_method(
    object: Object,
    method: StringName,
    arg1 = NO_ARG,
    arg2 = NO_ARG,
    arg3 = NO_ARG,
    arg4 = NO_ARG
) -> void:
    _pending_do.append({
        "kind": "method",
        "object": object,
        "method": method,
        "args": _arguments(arg1, arg2, arg3, arg4),
    })

func add_do_property(object: Object, property: StringName, value: Variant) -> void:
    _pending_do.append({
        "kind": "property",
        "object": object,
        "property": property,
        "value": value,
    })

func add_do_reference(object: Object) -> void:
    _pending_do.append({"kind": "reference", "object": object})

func add_undo_method(
    object: Object,
    method: StringName,
    arg1 = NO_ARG,
    arg2 = NO_ARG,
    arg3 = NO_ARG,
    arg4 = NO_ARG
) -> void:
    _pending_undo.append({
        "kind": "method",
        "object": object,
        "method": method,
        "args": _arguments(arg1, arg2, arg3, arg4),
    })

func add_undo_property(object: Object, property: StringName, value: Variant) -> void:
    _pending_undo.append({
        "kind": "property",
        "object": object,
        "property": property,
        "value": value,
    })

func add_undo_reference(object: Object) -> void:
    _pending_undo.append({"kind": "reference", "object": object})

# commit_action executes this operation's do methods immediately and records
# both directions in the batch. It never touches editor history: the whole
# batch is registered only by finish() after every operation succeeds.
func commit_action() -> void:
    if _pending_do.is_empty() and _pending_undo.is_empty():
        return

    _execution_error = ""

    for item in _pending_do:
        var failure := _execute_do(item)

        if not failure.is_empty():
            _execution_error = failure
            break

    var registered: Array[Dictionary] = _pending_undo.duplicate()

    # The eventual editor action uses backward undo ordering so operations roll
    # back in reverse batch order. Operations that originally used forward undo
    # ordering are inserted reversed to keep their internal order.
    if not _op_backward:
        registered.reverse()

    if not _execution_error.is_empty():
        _rollback_notes.append(
            "operation %d failed while applying: %s"
            % [_applied_count, _execution_error]
        )
    elif not _pending_do.is_empty() and registered.is_empty():
        _execution_error = "the operation applied changes without recording undo methods"
        _rollback_notes.append(
            "operation %d applied changes without recording undo methods"
            % _applied_count
        )

    _do_items.append_array(_pending_do)
    _undo_items.append_array(registered)
    _applied_count += 1

    _pending_do.clear()
    _pending_undo.clear()

# discard_pending drops an operation that failed before it committed. Earlier
# operations stay applied until rollback or finish.
func discard_pending() -> void:
    _pending_do.clear()
    _pending_undo.clear()

func has_applied() -> bool:
    return _applied_count > 0

# take_execution_error reports a do method that could not run. The caller
# treats that operation as failed and rolls back through rollback().
func take_execution_error() -> String:
    var error := _execution_error
    _execution_error = ""
    return error

# finish registers the entire batch as one editor undo action without running
# the do methods again. Returns true when an action was committed.
func finish() -> bool:
    if target == null or (_do_items.is_empty() and _undo_items.is_empty()):
        _clear()
        return false

    target.create_action(batch_name, UndoRedo.MERGE_DISABLE, _context, true)

    for item in _do_items:
        _register_do(item)

    for item in _undo_items:
        _register_undo(item)

    target.commit_action(false)
    _clear()

    return true

# rollback directly executes every accumulated undo item in reverse order.
# It never commits the failed batch to editor history, so a clean scene stays
# clean. The returned dictionary reports incomplete rollback distinctly.
func rollback() -> Dictionary:
    discard_pending()

    var total := _undo_items.size()
    var notes := _rollback_notes.duplicate()
    var failures: Array[String] = []

    for index in range(total - 1, -1, -1):
        var failure := _execute_undo(_undo_items[index])

        if not failure.is_empty():
            failures.append(failure)

    var restored := total - failures.size()
    var all_failures: Array[String] = []
    all_failures.append_array(notes)
    all_failures.append_array(failures)

    _clear()

    return {
        "ok": all_failures.is_empty(),
        "restored": restored,
        "total": total,
        "failures": all_failures,
    }

func _clear() -> void:
    _pending_do.clear()
    _pending_undo.clear()
    _do_items.clear()
    _undo_items.clear()
    _rollback_notes.clear()
    _applied_count = 0
    _execution_error = ""

func _arguments(arg1, arg2, arg3, arg4) -> Array:
    var result: Array = []

    for argument in [arg1, arg2, arg3, arg4]:
        if typeof(argument) == TYPE_STRING_NAME and argument == NO_ARG:
            break

        result.append(argument)

    return result

func _execute_do(item: Dictionary) -> String:
    var object = item.get("object")

    if object == null or not is_instance_valid(object):
        return "could not run %s because its object is no longer valid" % _item_label(item)

    if _has_invalid_argument(item):
        return "could not run %s because an argument is no longer valid" % _item_label(item)

    match item["kind"]:
        "method":
            object.callv(item["method"], item["args"])
        "property":
            object.set(item["property"], item["value"])

    return ""

func _execute_undo(item: Dictionary) -> String:
    if item["kind"] == "reference":
        return ""

    var object = item.get("object")

    if object == null or not is_instance_valid(object):
        return "could not undo %s because its object is no longer valid" % _item_label(item)

    if _has_invalid_argument(item):
        return "could not undo %s because an argument is no longer valid" % _item_label(item)

    match item["kind"]:
        "method":
            object.callv(item["method"], item["args"])
        "property":
            object.set(item["property"], item["value"])

    return ""

func _has_invalid_argument(item: Dictionary) -> bool:
    for argument in item.get("args", []):
        if argument is Object and not is_instance_valid(argument):
            return true

    return false

func _item_label(item: Dictionary) -> String:
    if item["kind"] == "property":
        return str(item["property"])

    return str(item.get("method", item["kind"]))

func _register_do(item: Dictionary) -> void:
    match item["kind"]:
        "method":
            var args: Array = [item["object"], item["method"]]
            args.append_array(item["args"])
            Callable(target, "add_do_method").callv(args)
        "property":
            target.add_do_property(item["object"], item["property"], item["value"])
        "reference":
            target.add_do_reference(item["object"])

func _register_undo(item: Dictionary) -> void:
    match item["kind"]:
        "method":
            var args: Array = [item["object"], item["method"]]
            args.append_array(item["args"])
            Callable(target, "add_undo_method").callv(args)
        "property":
            target.add_undo_property(item["object"], item["property"], item["value"])
        "reference":
            target.add_undo_reference(item["object"])
