class_name SceneApplyService
extends ActionService

# scene.apply applies an ordered list of node mutations as one editor undo
# step. Every batch is first replayed against a temporary copy of the active
# scene; only a fully valid batch touches the live scene. Granular node actions
# and batch operations run through the same NodeService methods.

const ID_PATTERN := "^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$"

const OPERATION_FIELDS := {
    "node.add": ["operation", "id", "type", "parent", "name", "properties"],
    "node.set": ["operation", "id", "node", "properties"],
    "node.rename": ["operation", "id", "node", "name"],
    "node.duplicate": ["operation", "id", "node", "name"],
    "node.move": [
        "operation", "id", "node", "parent", "index", "keep_global_transform",
    ],
    "node.delete": ["operation", "id", "node"],
    "node.instantiate": [
        "operation", "id", "scene", "parent", "name", "properties",
    ],
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
    var size_error := validate_batch_operations(operations)

    if size_error != null:
        return size_error

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

    var session := SceneEditSession.new(
        undo_redo,
        "godot-cli: Apply scene batch",
        root
    )
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
        var rollback := session.rollback()

        if rollback["ok"]:
            return _failure_with_context(
                outcome["error"],
                outcome,
                (
                    "Batch failed and was rolled back"
                    if applied_before_rollback
                    else "Batch failed before any operation was applied"
                )
            )

        return _failure_with_context(
            Response.failure(
                ErrorCode.INTERNAL_ERROR,
                "Rollback was incomplete (%d of %d undo steps restored): %s" % [
                    rollback["restored"],
                    rollback["total"],
                    outcome["error"].message,
                ]
            ),
            outcome,
            "Batch failed and rollback was incomplete"
        )

    session.finish()

    return Response.success(
        _result(scene, outcome["results"], true, false)
    )

func _preview(root: Node, operations: Array) -> Dictionary:
    var packed := PackedScene.new()
    var pack_error := packed.pack(root)

    if pack_error != OK:
        return _preview_error(
            "Could not build a temporary scene copy for prevalidation: %s"
            % error_string(pack_error)
        )

    var copy := packed.instantiate(PackedScene.GEN_EDIT_STATE_INSTANCE)

    if copy == null or not is_instance_valid(copy):
        return _preview_error(
            "Could not instantiate a temporary scene copy for prevalidation"
        )

    copy.scene_file_path = root.scene_file_path

    var seen_nodes := {}
    _remember_nodes(copy, seen_nodes)

    var context := scene_context.for_root(copy)
    var session := SceneEditSession.new(null)
    var node_service := NodeService.new(context, session)
    var outcome := _run(
        operations,
        copy,
        context,
        node_service.get_actions(),
        session,
        seen_nodes
    )

    # A script can free the temporary root while its properties are applied.
    # Never report a successful dry run from a freed preview root.
    var root_freed := not is_instance_valid(copy)
    var orphans := _orphaned_nodes(seen_nodes.keys(), copy)

    if not root_freed:
        copy.free()

    _free_orphans(orphans)

    if root_freed:
        return _preview_error(
            "The temporary scene root was freed during prevalidation; "
            + "the live scene was not modified"
        )

    return outcome

func _preview_error(message: String) -> Dictionary:
    return {
        "error": Response.failure(ErrorCode.INTERNAL_ERROR, message),
        "failed_index": -1,
        "failed_operation": {},
        "failed_target": "",
        "results": [],
    }

func _run(
    operations: Array,
    root: Node,
    context: EditorSceneContext,
    actions: Dictionary,
    session: SceneEditSession,
    seen_nodes: Dictionary = {}
) -> Dictionary:
    var results: Array[Dictionary] = []
    var ids: Dictionary = {}

    for index in operations.size():
        var operation: Dictionary = operations[index]
        var operation_name: String = operation.get("operation", "")
        var id: String = operation.get("id", "")
        var target := _operation_target(operation)

        if not is_instance_valid(root):
            session.discard_pending()

            return _outcome_error(
                index,
                operation,
                target,
                Response.failure(
                    ErrorCode.INTERNAL_ERROR,
                    "The scene root was freed while the batch was running"
                ),
                results
            )

        var resolved := _operation_params(operation, ids, root, context)
        var error: Response = resolved["error"]

        if error != null:
            session.discard_pending()

            return _outcome_error(index, operation, target, error, results)

        var handler: ActionHandler = actions[operation_name]
        var params: Dictionary = resolved["params"]
        var response = handler.callable.call(Request.new(operation_name, params))

        if not response is Response:
            session.discard_pending()

            return _outcome_error(
                index,
                operation,
                target,
                Response.failure(
                    ErrorCode.INTERNAL_ERROR,
                    "Operation returned an invalid response"
                ),
                results
            )

        var execution_error := session.take_execution_error()

        if not execution_error.is_empty():
            return _outcome_error(
                index,
                operation,
                target,
                Response.failure(ErrorCode.INTERNAL_ERROR, execution_error),
                results
            )

        if not response.code.is_empty():
            session.discard_pending()

            return _outcome_error(index, operation, target, response, results)

        if not is_instance_valid(root):
            session.discard_pending()

            return _outcome_error(
                index,
                operation,
                target,
                Response.failure(
                    ErrorCode.INTERNAL_ERROR,
                    "The scene root was freed while the batch was running"
                ),
                results
            )

        var result: Dictionary = response.result
        var node := _result_node(operation_name, result, resolved, context)
        var entry := _operation_result(index, operation, result, node, resolved)
        results.append(entry)

        if not seen_nodes.is_empty() and is_instance_valid(root):
            _remember_nodes(root, seen_nodes)

        if not id.is_empty():
            ids[id] = {
                "node": node,
                "deleted": operation_name == "node.delete",
            }

    return {
        "error": null,
        "failed_index": -1,
        "failed_operation": {},
        "failed_target": "",
        "results": results,
    }

func _operation_params(
    operation: Dictionary,
    ids: Dictionary,
    root: Node,
    context: EditorSceneContext
) -> Dictionary:
    var operation_name: String = operation.get("operation", "")
    var params := {}
    var resolved_node: Node = null
    var resolved_parent: Node = null

    match operation_name:
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
                    "Unsupported operation: %s" % operation_name
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
    operation_name: String,
    result: Dictionary,
    resolved: Dictionary,
    context: EditorSceneContext
) -> Node:
    match operation_name:
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
    var operation_name: String = operation.get("operation", "")
    var entry := {
        "index": index,
        "operation": operation_name,
        "path": "",
        "name": "",
        "type": "",
        "changed": false,
        "summary": "",
    }

    if operation.has("id"):
        entry["id"] = operation["id"]

    match operation_name:
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

        match entry["operation"]:
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
    operation: Dictionary,
    target: String,
    error: Response,
    results: Array
) -> Dictionary:
    return {
        "error": error,
        "failed_index": index,
        "failed_operation": operation,
        "failed_target": target,
        "results": results,
    }

func _failure_with_context(
    error: Response,
    outcome: Dictionary,
    prefix: String
) -> Response:
    var index := int(outcome.get("failed_index", -1))

    if index < 0:
        return Response.failure(error.code, "%s: %s" % [prefix, error.message])

    return batch_operation_failure(
        index,
        outcome.get("failed_operation", {}),
        str(outcome.get("failed_target", "")),
        error,
        prefix
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
            return batch_operation_failure(
                index,
                operation,
                "",
                Response.failure(
                    ErrorCode.INVALID_ARGUMENT,
                    "Each operation must be an object"
                )
            )

        var operation_name = operation.get("operation")

        if not operation_name is String or operation_name.is_empty():
            return batch_operation_failure(
                index,
                operation,
                _operation_target(operation),
                Response.failure(
                    ErrorCode.INVALID_ARGUMENT,
                    "An operation is required"
                )
            )

        if not OPERATION_FIELDS.has(operation_name):
            return batch_operation_failure(
                index,
                operation,
                _operation_target(operation),
                Response.failure(
                    ErrorCode.INVALID_ARGUMENT,
                    "Unsupported operation: %s" % operation_name
                )
            )

        var field_error := validate_operation_fields(
            index,
            operation,
            OPERATION_FIELDS
        )

        if field_error != null:
            return batch_operation_failure(
                index,
                operation,
                _operation_target(operation),
                field_error
            )

        if operation.has("id"):
            var id = operation["id"]

            if not id is String or id.is_empty():
                return batch_operation_failure(
                    index,
                    operation,
                    _operation_target(operation),
                    Response.failure(
                        ErrorCode.INVALID_ARGUMENT,
                        "An id must be a nonempty string"
                    )
                )

            if pattern.search(id) == null:
                return batch_operation_failure(
                    index,
                    operation,
                    _operation_target(operation),
                    Response.failure(
                        ErrorCode.INVALID_ARGUMENT,
                        "Invalid id: %s" % id
                    )
                )

            if ids.has(id):
                return batch_operation_failure(
                    index,
                    operation,
                    _operation_target(operation),
                    Response.failure(
                        ErrorCode.INVALID_ARGUMENT,
                        "Duplicate operation id: %s" % id
                    )
                )

            ids[id] = true

        for field in ["node", "parent"]:
            if not operation.has(field):
                continue

            var reference = operation[field]

            if not reference is String or not reference.begins_with("@"):
                continue

            var reference_id: String = reference.substr(1)

            if reference_id.is_empty():
                return batch_operation_failure(
                    index,
                    operation,
                    _operation_target(operation),
                    Response.failure(
                        ErrorCode.INVALID_ARGUMENT,
                        "%s has an empty operation reference" % field
                    )
                )

            if not ids.has(reference_id):
                return batch_operation_failure(
                    index,
                    operation,
                    _operation_target(operation),
                    Response.failure(
                        ErrorCode.INVALID_ARGUMENT,
                        "%s refers to unknown or not-yet-defined operation id %s"
                        % [field, reference]
                    )
                )

    return null

func _operation_target(operation: Dictionary) -> String:
    var node = operation.get("node", "")

    if node is String and not node.is_empty():
        return node

    var parent = operation.get("parent", "")

    if parent is String and not parent.is_empty():
        return parent

    var id = operation.get("id", "")

    if id is String:
        return id

    return ""

func _remember_nodes(node: Node, nodes: Dictionary) -> void:
    if node == null or not is_instance_valid(node):
        return

    nodes[node] = true

    for child in node.get_children():
        _remember_nodes(child, nodes)

func _orphaned_nodes(nodes: Array, root: Node) -> Array[Node]:
    var orphans: Array[Node] = []

    for value in nodes:
        var node := value as Node

        if node == null or not is_instance_valid(node):
            continue

        if is_instance_valid(root):
            if node == root:
                continue

            if root.is_ancestor_of(node):
                continue

        orphans.append(node)

    return orphans

func _free_orphans(orphans: Array[Node]) -> void:
    for node in orphans:
        if not is_instance_valid(node):
            continue

        if node.get_parent() != null:
            continue

        node.free()
