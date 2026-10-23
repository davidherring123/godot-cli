class_name SceneEditSession
extends RefCounted

# NodeService records mutations through the same UndoRedo-shaped interface for
# standalone actions and for scene.apply batches. This session adapts that
# interface to one editor undo action: methods are executed immediately (so
# later operations can validate against the result) while their inverse
# operations accumulate in the action for atomic rollback.

const NO_ARG := &"__godot_cli_no_arg__"

var target: Object
var batch_name: String

var _pending_do: Array[Dictionary] = []
var _pending_undo: Array[Dictionary] = []
var _context: Node = null
var _op_backward := false
var _action_open := false
var _flushed := false

func _init(undo_target: Object = null, title := "godot-cli: Apply scene batch") -> void:
    target = undo_target
    batch_name = title

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
# both directions in the shared editor action. NodeService calls it once per
# operation, exactly as it does for standalone commands.
func commit_action() -> void:
    if _pending_do.is_empty() and _pending_undo.is_empty():
        return

    for item in _pending_do:
        _execute(item)

    if target != null:
        _ensure_action()

        for item in _pending_do:
            _register_do(item)

        var undo_items: Array[Dictionary] = _pending_undo.duplicate()

        # The shared action uses backward undo ordering so operations roll back
        # in reverse batch order. Operations that originally used forward undo
        # ordering must be inserted reversed to keep their internal order.
        if not _op_backward:
            undo_items.reverse()

        for item in undo_items:
            _register_undo(item)

        _flushed = true

    _pending_do.clear()
    _pending_undo.clear()

# discard_pending drops an operation that failed before it committed. Earlier
# operations stay applied until rollback or finish.
func discard_pending() -> void:
    _pending_do.clear()
    _pending_undo.clear()

func has_applied() -> bool:
    return _flushed or _action_open

func finish() -> void:
    if target == null or not _action_open:
        return

    target.commit_action(false)

    _action_open = false
    _flushed = false

# rollback reverts every committed operation in this session. The failed batch
# is committed without re-running do methods, undone through the editor's undo
# stack, and replaced by an empty action so the partial batch cannot be redone.
func rollback() -> void:
    discard_pending()

    if target == null or not _action_open:
        return

    target.commit_action(false)
    _action_open = false
    _flushed = false

    var history: UndoRedo = target.get_history_undo_redo(
        target.get_object_history_id(_context)
    )

    if history != null:
        history.undo()

    # Commit an empty action so the failed batch cannot be redone.
    target.create_action(
        batch_name + " (rolled back)",
        UndoRedo.MERGE_DISABLE,
        _context,
        true
    )
    target.commit_action()

func _ensure_action() -> void:
    if _action_open:
        return

    target.create_action(batch_name, UndoRedo.MERGE_DISABLE, _context, true)
    _action_open = true

func _arguments(arg1, arg2, arg3, arg4) -> Array:
    var result: Array = []

    for argument in [arg1, arg2, arg3, arg4]:
        if typeof(argument) == TYPE_STRING_NAME and argument == NO_ARG:
            break

        result.append(argument)

    return result

func _execute(item: Dictionary) -> void:
    match item["kind"]:
        "method":
            item["object"].callv(item["method"], item["args"])
        "property":
            item["object"].set(item["property"], item["value"])

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
