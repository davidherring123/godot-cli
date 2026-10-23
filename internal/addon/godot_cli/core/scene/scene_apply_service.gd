class_name SceneApplyService
extends ActionService

# scene.apply applies an ordered list of node mutations as one editor undo
# step. Every batch is first replayed against a temporary copy of the active
# scene; only a fully valid batch touches the live scene. Granular node actions
# and batch operations run through the same NodeService methods.

const ID_PATTERN := "^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$"
const MAX_OPERATIONS := 1000

const OPERATION_FIELDS := {
    "node.add": ["op", "id", "type", "parent", "name", "properties"],
    "node.set": ["op", "id", "node", "properties"],
    "node.rename": ["op", "id", "node", "name"],
    "node.duplicate": ["op", "id", "node", "name"],
    "node.move": ["op", "id", "node", "parent", "index", "keep_global_transform"],
    "node.delete": ["op", "id", "node"],
    "node.instantiate": ["op", "id", "scene", "parent", "name", "properties"],
}

var scene_context: EditorSceneContext
var undo_redo: EditorUndoRedoManager

func _init(context: EditorSceneContext, history: EditorUndoRedoManager) -> void:
    scene_context = context
    undo_redo = history

func get_actions() -> Dictionary:
    return {
        "scene.apply": ActionHandler.new(apply, true, true, true),
    }

func apply(request: Request) -> Response:
    var operations = request.params.get("operations")

    if not operations is Array or operations.is_empty():
        return Response.failure(
            ErrorCode.INVALID_ARGUMENT,
            "A nonempty operations array is required"
        )

    if operations.size() > MAX_OPERATIONS:
        return Response.failure(
            ErrorCode.INVALID_ARGUMENT,
            "A batch may contain at most %d operations" % MAX_OPERATIONS
        )

    var dry_run = request.params.get("dry_run", false)

    if not dry_run is bool:
        return Response.failure(ErrorCode.INVALID_ARGUMENT, "dry_run must be a boolean")

    if dry_run and request.params.get("save", false):
        return Response.failure(
            ErrorCode.INVALID_ARGUMENT,
            "save cannot be combined with dry_run"
        )

    var validation_error := _validate_operations(operations)

    if validation_error != null:
        return validation_error

    var root := scene_context.edited_root()

    if root == null:
        return Response.failure(
            ErrorCode.FAILED_PRECONDITION,
            "No scene is currently being edited"
        )

    var scene := root.scene_file_path
    var preview := _preview(root, operations)

    if preview["error"] != null:
        return _failure_with_context(
            preview["error"],
            preview,
            "Prevalidation failed"
        )

    if dry_run:
        return Response.success(
            _result(scene, preview["results"], false, true)
        )

    var session := SceneEditSession.new(undo_redo)
    var node_service := NodeService.new(scene_context, session)
    var outcome := _run(
        operations,
        root,
        scene_context,
        node_service.get_actions(),
        session
    )

    if outcome["error"] != null:
        var applied_before_rollback := session.has_applied()

        session.rollback()

        return _failure_with_context(
            outcome["error"],
            outcome,
            (
                "Batch failed and was rolled back"
                if applied_before_rollback
                else "Batch failed before any operation was applied"
            )
        )

    session.finish()

    return Response.success(
        _result(scene, outcome["results"], true, false)
    )

func _preview(root: Node, operations: Array) -> Dictionary:
    var packed := PackedScene.new()
    var pack_error := packed.pack(root)

    if pack_error != OK:
        return {
            "error": Response.failure(
                ErrorCode.ACTION_FAILED,
                "Could not build a temporary scene copy for prevalidation: %s"
                % error_string(pack_error)
            ),
            "failed_index": -1,
            "failed_op": "",
            "failed_id": "",
            "results": [],
        }

    var copy := packed.instantiate(PackedScene.GEN_EDIT_STATE_INSTANCE)

    if copy == null:
        return {
            "error": Response.failure(
                ErrorCode.ACTION_FAILED,
                "Could not instantiate a temporary scene copy for prevalidation"
            ),
            "failed_index": -1,
            "failed_op": "",
            "failed_id": "",
            "results": [],
        }

    copy.scene_file_path = root.scene_file_path

    var context := scene_context.for_root(copy)
    var session := SceneEditSession.new(null)
    var node_service := NodeService.new(context, session)
    var outcome := _run(
        operations,
        copy,
        context,
        node_service.get_actions(),
        session
    )

    copy.free()

    return outcome

func _run(
    operations: Array,
    root: Node,
    context: EditorSceneContext,
    actions: Dictionary,
    session: SceneEditSession
) -> Dictionary:
    var results: Array[Dictionary] = []
    var ids: Dictionary = {}

    for index in operations.size():
        var operation: Dictionary = operations[index]
        var op_name: String = operation.get("op", "")
        var id: String = operation.get("id", "")
        var resolved := _operation_params(operation, ids, root, context)
        var error: Response = resolved["error"]

        if error != null:
            session.discard_pending()

            return _outcome_error(index, op_name, id, error, results)

        var handler: ActionHandler = actions[op_name]
        var params: Dictionary = resolved["params"]
        var response = handler.callable.call(Request.new(op_name, params))

        if not response is Response:
            session.discard_pending()

            return _outcome_error(
                index,
                op_name,
                id,
                Response.failure(
                    ErrorCode.INTERNAL_ERROR,
                    "Operation returned an invalid response"
                ),
                results
            )

        if not response.code.is_empty():
            session.discard_pending()

            return _outcome_error(index, op_name, id, response, results)

        var result: Dictionary = response.result
        var node := _result_node(op_name, result, resolved, context)
        var entry := _operation_result(index, operation, result, node, resolved)
        results.append(entry)

        if not id.is_empty():
            ids[id] = {
                "node": node,
                "deleted": op_name == "node.delete",
            }

    return {
        "error": null,
        "failed_index": -1,
        "failed_op": "",
        "failed_id": "",
        "results": results,
    }

func _operation_params(
    operation: Dictionary,
    ids: Dictionary,
    root: Node,
    context: EditorSceneContext
) -> Dictionary:
    var op_name: String = operation.get("op", "")
    var params := {}
    var resolved_node: Node = null
    var resolved_parent: Node = null

    match op_name:
        "node.add":
            var parent := _resolve_ref(operation.get("parent"), ids, root, "parent")

            if parent["error"] != null:
                return {"error": parent["error"], "params": {}}

            resolved_parent = parent["node"]

            params = {
                "type": operation.get("type", ""),
                "parent": parent["path"],
                "name": operation.get("name", ""),
                "properties": operation.get("properties", {}),
            }
        "node.set":
            var node := _resolve_ref(operation.get("node"), ids, root, "node")

            if node["error"] != null:
                return {"error": node["error"], "params": {}}

            resolved_node = node["node"]

            params = {
                "node": node["path"],
                "properties": operation.get("properties", {}),
            }
        "node.rename":
            var node := _resolve_ref(operation.get("node"), ids, root, "node")

            if node["error"] != null:
                return {"error": node["error"], "params": {}}

            resolved_node = node["node"]

            params = {
                "node": node["path"],
                "name": operation.get("name", ""),
            }
        "node.duplicate":
            var node := _resolve_ref(operation.get("node"), ids, root, "node")

            if node["error"] != null:
                return {"error": node["error"], "params": {}}

            resolved_node = node["node"]

            params = {
                "node": node["path"],
                "name": operation.get("name", ""),
            }
        "node.move":
            var node := _resolve_ref(operation.get("node"), ids, root, "node")

            if node["error"] != null:
                return {"error": node["error"], "params": {}}

            var parent := _resolve_ref(operation.get("parent"), ids, root, "parent")

            if parent["error"] != null:
                return {"error": parent["error"], "params": {}}

            resolved_node = node["node"]
            resolved_parent = parent["node"]

            params = {
                "node": node["path"],
                "parent": parent["path"],
                "index": operation.get("index", -1),
                "keep_global_transform": operation.get("keep_global_transform", true),
            }
        "node.delete":
            var node := _resolve_ref(operation.get("node"), ids, root, "node")

            if node["error"] != null:
                return {"error": node["error"], "params": {}}

            resolved_node = node["node"]

            params = {"node": node["path"]}
        "node.instantiate":
            var parent := _resolve_ref(operation.get("parent"), ids, root, "parent")

            if parent["error"] != null:
                return {"error": parent["error"], "params": {}}

            resolved_parent = parent["node"]

            params = {
                "scene": operation.get("scene", ""),
                "parent": parent["path"],
                "name": operation.get("name", ""),
                "properties": operation.get("properties", {}),
            }
        _:
            return {
                "error": Response.failure(
                    ErrorCode.INVALID_ARGUMENT,
                    "Unknown operation: %s" % op_name
                ),
                "params": {},
            }

    return {
        "error": null,
        "params": params,
        "node": resolved_node,
        "parent": resolved_parent,
    }

func _resolve_ref(
    value: Variant,
    ids: Dictionary,
    root: Node,
    label: String
) -> Dictionary:
    if not value is String or value.is_empty():
        return {
            "error": Response.failure(
                ErrorCode.INVALID_ARGUMENT,
                "A %s node path is required" % label
            ),
            "node": null,
            "path": "",
        }

    if not value.begins_with("@"):
        return {
            "error": null,
            "node": _find_node(root, value),
            "path": value,
        }

    var id: String = value.substr(1)

    if id.is_empty():
        return {
            "error": Response.failure(
                ErrorCode.INVALID_ARGUMENT,
                "Operation reference %s has an empty id" % value
            ),
            "node": null,
            "path": "",
        }

    if not ids.has(id):
        return {
            "error": Response.failure(
                ErrorCode.NOT_FOUND,
                "Unknown operation reference %s" % value
            ),
            "node": null,
            "path": "",
        }

    var entry: Dictionary = ids[id]

    if entry["deleted"]:
        return {
            "error": Response.failure(
                ErrorCode.NOT_FOUND,
                "Operation reference %s points to a node deleted earlier in the batch"
                % value
            ),
            "node": null,
            "path": "",
        }

    var node: Node = entry["node"]

    if (
        node == null
        or not is_instance_valid(node)
        or (node != root and not root.is_ancestor_of(node))
    ):
        return {
            "error": Response.failure(
                ErrorCode.NOT_FOUND,
                "Operation reference %s no longer points to a node in the scene" % value
            ),
            "node": null,
            "path": "",
        }

    return {
        "error": null,
        "node": node,
        "path": str(root.get_path_to(node)),
    }

func _find_node(root: Node, path: String) -> Node:
    if root == null or path.is_empty():
        return null

    var node_path := NodePath(path)

    if node_path.is_absolute() or node_path.get_subname_count() > 0:
        return null

    var node := root.get_node_or_null(node_path)

    if node == root or (node != null and root.is_ancestor_of(node)):
        return node

    return null

func _result_node(
    op_name: String,
    result: Dictionary,
    resolved: Dictionary,
    context: EditorSceneContext
) -> Node:
    match op_name:
        "node.add", "node.instantiate":
            return context.find_node(str(result.get("path", "")))
        "node.duplicate":
            return context.find_node(str(result.get("new_path", "")))
        "node.rename", "node.move":
            return context.find_node(str(result.get("new_path", "")))
        "node.set":
            return resolved.get("node")
        "node.delete":
            return resolved.get("node")
        _:
            return null

func _operation_result(
    index: int,
    operation: Dictionary,
    result: Dictionary,
    node: Node,
    resolved: Dictionary
) -> Dictionary:
    var op_name: String = operation.get("op", "")
    var entry := {
        "index": index,
        "op": op_name,
        "path": "",
        "name": "",
        "type": "",
        "changed": false,
        "summary": "",
    }

    if operation.has("id"):
        entry["id"] = operation["id"]

    match op_name:
        "node.add":
            entry["path"] = str(result.get("path", ""))
            entry["name"] = str(result.get("name", ""))
            entry["type"] = str(result.get("type", ""))
            entry["changed"] = true
            entry["summary"] = "Added %s at %s" % [entry["type"], entry["path"]]
        "node.instantiate":
            entry["path"] = str(result.get("path", ""))
            entry["name"] = str(result.get("name", ""))
            entry["type"] = str(result.get("type", ""))
            entry["changed"] = true
            entry["summary"] = "Instantiated %s at %s" % [
                str(result.get("source", "")),
                entry["path"],
            ]
        "node.set":
            entry["path"] = str(result.get("path", ""))
            entry["name"] = str(node.name) if node != null else ""
            entry["type"] = node.get_class() if node != null else ""
            entry["changed"] = bool(result.get("changed", false))
            entry["summary"] = _set_summary(operation, entry)
        "node.rename":
            entry["path"] = str(result.get("new_path", ""))
            entry["name"] = str(result.get("new_name", ""))
            entry["type"] = node.get_class() if node != null else ""
            entry["changed"] = bool(result.get("changed", false))
            entry["summary"] = (
                "Renamed %s to %s" % [str(result.get("name", "")), entry["name"]]
                if entry["changed"]
                else "No change"
            )
        "node.duplicate":
            entry["path"] = str(result.get("new_path", ""))
            entry["name"] = str(result.get("new_name", ""))
            entry["type"] = str(result.get("type", ""))
            entry["changed"] = true
            entry["summary"] = "Duplicated %s to %s" % [
                str(result.get("path", "")),
                entry["path"],
            ]
        "node.move":
            entry["path"] = str(result.get("new_path", ""))
            entry["name"] = str(result.get("new_name", ""))
            entry["type"] = node.get_class() if node != null else ""
            entry["changed"] = bool(result.get("changed", false))
            entry["summary"] = (
                "Moved %s to %s" % [str(result.get("path", "")), entry["path"]]
                if entry["changed"]
                else "No change"
            )
        "node.delete":
            entry["path"] = str(result.get("path", ""))
            entry["name"] = str(node.name) if node != null else ""
            entry["type"] = node.get_class() if node != null else ""
            entry["changed"] = bool(result.get("deleted", false))
            entry["summary"] = "Deleted %s" % entry["path"]

    return entry

func _set_summary(operation: Dictionary, entry: Dictionary) -> String:
    var properties = operation.get("properties", {})
    var count: int = properties.size() if properties is Dictionary else 0

    if not entry["changed"]:
        return "No change"

    return "Set %d properties on %s" % [count, entry["path"]]

func _result(
    scene: String,
    results: Array,
    applied: bool,
    dry_run: bool
) -> Dictionary:
    var changed := 0
    var added := 0
    var deleted := 0

    for entry in results:
        if entry["changed"]:
            changed += 1

        match entry["op"]:
            "node.add", "node.instantiate", "node.duplicate":
                added += 1
            "node.delete":
                deleted += 1

    return {
        "scene": scene,
        "applied": applied,
        "dry_run": dry_run,
        "operations": results,
        "summary": {
            "total": results.size(),
            "changed": changed,
            "added": added,
            "deleted": deleted,
        },
        "saved": false,
    }

func _outcome_error(
    index: int,
    op_name: String,
    id: String,
    error: Response,
    results: Array
) -> Dictionary:
    return {
        "error": error,
        "failed_index": index,
        "failed_op": op_name,
        "failed_id": id,
        "results": results,
    }

func _failure_with_context(
    error: Response,
    outcome: Dictionary,
    prefix: String
) -> Response:
    var label := "operation[%d]" % outcome.get("failed_index", -1)
    var op_name: String = outcome.get("failed_op", "")
    var id: String = outcome.get("failed_id", "")

    if not op_name.is_empty():
        label += " %s" % op_name

    if not id.is_empty():
        label += " (id=%s)" % id

    return Response.failure(
        error.code,
        "%s: %s failed: %s" % [prefix, label, error.message]
    )

func _validate_operations(operations: Array) -> Response:
    var pattern := RegEx.new()

    if pattern.compile(ID_PATTERN) != OK:
        return Response.failure(
            ErrorCode.INTERNAL_ERROR,
            "Could not compile the operation id pattern"
        )

    var ids := {}

    for index in operations.size():
        var operation = operations[index]

        if not operation is Dictionary:
            return Response.failure(
                ErrorCode.INVALID_ARGUMENT,
                "operation[%d] must be an object" % index
            )

        var op_name = operation.get("op")

        if not op_name is String or not OPERATION_FIELDS.has(op_name):
            return Response.failure(
                ErrorCode.INVALID_ARGUMENT,
                "operation[%d] has an unknown op" % index
            )

        for key in operation:
            if not OPERATION_FIELDS[op_name].has(key):
                return Response.failure(
                    ErrorCode.INVALID_ARGUMENT,
                    "operation[%d] %s does not accept field %s"
                    % [index, op_name, key]
                )

        for field in ["node", "parent"]:
            if not operation.has(field):
                continue

            var reference = operation[field]

            if not reference is String or not reference.begins_with("@"):
                continue

            var reference_id: String = reference.substr(1)

            if not ids.has(reference_id):
                return Response.failure(
                    ErrorCode.INVALID_ARGUMENT,
                    "operation[%d] %s refers to unknown or not-yet-defined operation id %s"
                    % [index, op_name, reference]
                )

        if operation.has("id"):
            var id = operation["id"]

            if not id is String or pattern.search(id) == null:
                return Response.failure(
                    ErrorCode.INVALID_ARGUMENT,
                    "operation[%d] has an invalid id" % index
                )

            if ids.has(id):
                return Response.failure(
                    ErrorCode.INVALID_ARGUMENT,
                    "Duplicate operation id: %s" % id
                )

            ids[id] = true

    return null
