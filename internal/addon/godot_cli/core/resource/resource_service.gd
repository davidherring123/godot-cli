class_name ResourceService
extends ActionService

var editor: EditorInterface
var dry_run := false
var batch_active := false
var batch_resources: Dictionary = {}
var batch_deleted: Dictionary = {}
var batch_references: Dictionary = {}
var batch_paths: Dictionary = {}

func _init(editor_interface: EditorInterface) -> void:
    editor = editor_interface

func get_actions() -> Dictionary:
    return {
        "resource.create": ActionHandler.new(create, false, false, false),
        "resource.inspect": ActionHandler.new(inspect, false, false, false),
        "resource.set": ActionHandler.new(set_properties, false, false, false),
        "resource.delete": ActionHandler.new(delete, false, false, false),
        "resource.apply": ActionHandler.new(apply, false, false, false),
    }

func create(request: Request) -> Response:
    var path_response := _validated_path(request.params.get("resource"))

    if not path_response.code.is_empty():
        return path_response

    var path: String = path_response.result

    var resource_type = request.params.get("type")

    if not resource_type is String or resource_type.is_empty():
        return Response.failure(
            ErrorCode.INVALID_ARGUMENT,
            "A resource type is required"
        )

    var values = request.params.get("properties", {})

    if not values is Dictionary:
        return Response.failure(
            ErrorCode.INVALID_ARGUMENT,
            "properties must be an object"
        )

    if _resource_exists(path):
        return Response.failure(
            ErrorCode.CONFLICT,
            "A file already exists at: %s" % path
        )

    var resource := _instantiate(resource_type)

    if resource == null:
        return Response.failure(
            ErrorCode.INVALID_ARGUMENT,
            "Unknown or non-instantiable resource type: %s" % resource_type
        )

    var property_error := PropertyCodec.apply_properties(
        resource,
        values,
        _resource_overrides()
    )

    if property_error != null:
        return property_error

    if dry_run:
        _remember_created_resource(path, resource, values)
        return Response.success({
            "resource": path,
            "type": _type_name(resource),
            "properties": _serialize_properties(resource, values),
        })

    var save_error := _save(resource, path)

    if save_error != null:
        return save_error

    _remember_created_resource(path, resource, values)

    return Response.success({
        "resource": path,
        "type": _type_name(resource),
        "properties": _serialize_properties(resource, values),
    })

func inspect(request: Request) -> Response:
    var path_response := _validated_path(request.params.get("resource"))

    if not path_response.code.is_empty():
        return path_response

    var path: String = path_response.result

    if not FileAccess.file_exists(path):
        return Response.failure(
            ErrorCode.NOT_FOUND,
            "Resource not found: %s" % path
        )

    var resource := _load(path)

    if resource == null:
        return Response.failure(
            ErrorCode.ACTION_FAILED,
            "Could not load resource: %s" % path
        )

    var requested = request.params.get("properties", [])

    if not requested is Array:
        return Response.failure(
            ErrorCode.INVALID_ARGUMENT,
            "properties must be an array of property names"
        )

    var filters := {}

    for property_name in requested:
        if not property_name is String or property_name.is_empty():
            return Response.failure(
                ErrorCode.INVALID_ARGUMENT,
                "Property filters must be nonempty strings"
            )

        filters[property_name] = false

    var result := {
        "resource": path,
        "type": _type_name(resource),
        "properties": PropertyCodec.collect_properties(resource, filters),
    }

    for property_name in filters:
        if not filters[property_name]:
            return Response.failure(
                ErrorCode.NOT_FOUND,
                "Unknown or non-editor-visible property: %s" % property_name
            )

    return Response.success(result)

func set_properties(request: Request) -> Response:
    var path_response := _validated_path(request.params.get("resource"))

    if not path_response.code.is_empty():
        return path_response

    var path: String = path_response.result

    var values = request.params.get("properties")

    if not values is Dictionary or values.is_empty():
        return Response.failure(
            ErrorCode.INVALID_ARGUMENT,
            "A nonempty properties object is required"
        )

    if not _resource_exists(path):
        return Response.failure(
            ErrorCode.NOT_FOUND,
            "Resource not found: %s" % path
        )

    var resource := _resource_for_operation(path)

    if resource == null:
        return Response.failure(
            ErrorCode.ACTION_FAILED,
            "Could not load resource: %s" % path
        )

    var decoded := PropertyCodec.decode_properties(
        resource,
        values,
        _resource_overrides()
    )

    if decoded.error != null:
        return decoded.error

    var converted: Dictionary = decoded.converted
    var previous: Dictionary = decoded.previous
    var names: Array = converted.keys()

    var changed := false

    for property_name in names:
        if converted[property_name] != previous[property_name]:
            changed = true
            break

    if changed:
        for property_name in names:
            resource.set(property_name, converted[property_name])

        resource.emit_changed()

        if not dry_run:
            var save_error := _save(resource, path)

            if save_error != null:
                for property_name in names:
                    resource.set(property_name, previous[property_name])

                resource.emit_changed()

                return save_error

        if batch_active:
            _remember_set_references(path, converted)

    if dry_run or batch_active:
        batch_resources[path] = resource
        batch_paths[resource.get_instance_id()] = path

    var result := {}

    for property_name in names:
        result[property_name] = VariantSerializer.serialize(
            resource.get(property_name),
            batch_paths
        )

    return Response.success({
        "resource": path,
        "type": _type_name(resource),
        "properties": result,
        "changed": changed,
    })

func delete(request: Request) -> Response:
    var path_response := _validated_path(request.params.get("resource"))

    if not path_response.code.is_empty():
        return path_response

    var path: String = path_response.result

    if not _resource_exists(path):
        return Response.failure(
            ErrorCode.NOT_FOUND,
            "Resource not found: %s" % path
        )

    var referencing := _referencing_paths_for_delete(path)

    if not referencing.is_empty():
        return Response.failure(
            ErrorCode.CONFLICT,
            "Resource is still referenced by: %s" % ", ".join(referencing)
        )

    if dry_run:
        if batch_active:
            _forget_resource(path)
            batch_deleted[path] = true

        return Response.success({
            "resource": path,
            "deleted": true,
        })

    var error := DirAccess.remove_absolute(path)

    if error != OK:
        return Response.failure(
            ErrorCode.ACTION_FAILED,
            "Could not delete %s: %s" % [path, error_string(error)]
        )

    if batch_active:
        _forget_resource(path)
        batch_deleted[path] = true

    _release_cached(path)
    _notify_filesystem(path)

    return Response.success({
        "resource": path,
        "deleted": true,
    })

func apply(request: Request) -> Response:
    var operations = request.params.get("operations")
    var size_error := validate_batch_operations(operations)

    if size_error != null:
        return size_error

    var dry_run_requested = request.params.get("dry_run", false)

    if not dry_run_requested is bool:
        return Response.failure(
            ErrorCode.INVALID_ARGUMENT,
            "dry_run must be a boolean"
        )

    var preview := _run_operations(operations, true)

    if not preview.code.is_empty():
        return preview

    if dry_run_requested:
        return Response.success(_apply_summary(preview.result, true, false))

    var snapshot := _snapshot_resources(operations)
    var applied := _run_operations(operations, false)

    if not applied.code.is_empty():
        var rollback_error := _restore_resources(snapshot)

        if rollback_error.is_empty():
            return Response.failure(
                applied.code,
                "%s; rolled back earlier operations" % applied.message
            )

        return Response.failure(
            ErrorCode.INTERNAL_ERROR,
            "%s; rollback failed: %s" % [applied.message, rollback_error]
        )

    return Response.success(_apply_summary(applied.result, false, true))

func _run_operations(operations: Array, preview: bool) -> Response:
    _reset_batch_state()
    dry_run = preview
    batch_active = true

    var summaries: Array = []
    var allowed_fields := {
        "create": ["operation", "resource", "type", "properties"],
        "set": ["operation", "resource", "properties"],
        "delete": ["operation", "resource"],
    }

    for index in operations.size():
        var operation = operations[index]

        if not operation is Dictionary:
            var invalid := Response.failure(
                ErrorCode.INVALID_ARGUMENT,
                "Each operation must be an object"
            )

            _reset_batch_state()
            return batch_operation_failure(index, operation, "", invalid)

        var kind := str(operation.get("operation", ""))
        var target := str(operation.get("resource", ""))
        var field_error := validate_operation_fields(
            index,
            operation,
            allowed_fields
        )

        if field_error != null:
            _reset_batch_state()
            return batch_operation_failure(index, operation, target, field_error)

        var response: Response

        match kind:
            "create":
                response = create(Request.new("resource.create", operation))
            "set":
                response = set_properties(Request.new("resource.set", operation))
            "delete":
                response = delete(Request.new("resource.delete", operation))
            _:
                response = Response.failure(
                    ErrorCode.INVALID_ARGUMENT,
                    "Unsupported operation: %s" % kind
                )

        if not response.code.is_empty():
            _reset_batch_state()
            return batch_operation_failure(index, operation, target, response)

        summaries.append(_operation_summary(index, operation, response.result))

    _reset_batch_state()

    return Response.success(summaries)

func _operation_summary(
    index: int,
    operation: Dictionary,
    result: Dictionary
) -> Dictionary:
    var kind := str(operation.get("operation", ""))
    var summary := {
        "index": index,
        "operation": kind,
        "resource": str(result.get("resource", operation.get("resource", ""))),
        "status": kind,
        "changed": true,
    }

    match kind:
        "create":
            summary["status"] = "created"
        "set":
            var changed := bool(result.get("changed", false))
            summary["changed"] = changed
            summary["status"] = "updated" if changed else "unchanged"
        "delete":
            summary["status"] = "deleted"

    if result.has("type"):
        summary["type"] = result["type"]

    return summary

func _apply_summary(
    results: Array,
    preview: bool,
    applied: bool
) -> Dictionary:
    var changed := 0

    for result in results:
        if bool(result.get("changed", false)):
            changed += 1

    return {
        "dry_run": preview,
        "applied": applied,
        "operation_count": results.size(),
        "changed": changed,
        "unchanged": results.size() - changed,
        "operations": results,
    }

func _snapshot_resources(operations: Array) -> Dictionary:
    var snapshot := {}

    for operation in operations:
        if not operation is Dictionary:
            continue

        var value = operation.get("resource")

        if not value is String or not value.begins_with("res://"):
            continue

        var path: String = value.simplify_path()

        if snapshot.has(path):
            continue

        var exists := FileAccess.file_exists(path)

        snapshot[path] = {
            "exists": exists,
            "data": FileAccess.get_file_as_bytes(path) if exists else PackedByteArray(),
        }

    return snapshot

func _restore_resources(snapshot: Dictionary) -> String:
    var failures: Array[String] = []

    for path in snapshot:
        var state: Dictionary = snapshot[path]

        if bool(state.get("exists", false)):
            var file := FileAccess.open(path, FileAccess.WRITE)

            if file == null:
                failures.append(
                    "%s: %s" % [path, error_string(FileAccess.get_open_error())]
                )
                continue

            file.store_buffer(state.get("data", PackedByteArray()))
            file.close()
        elif FileAccess.file_exists(path):
            var error := DirAccess.remove_absolute(path)

            if error != OK:
                failures.append("%s: %s" % [path, error_string(error)])
                continue

        _release_cached(path)
        _notify_filesystem(path)

    return "; ".join(failures)

func _reset_batch_state() -> void:
    dry_run = false
    batch_active = false
    batch_resources.clear()
    batch_deleted.clear()
    batch_references.clear()
    batch_paths.clear()

func _resource_exists(path: String) -> bool:
    if batch_deleted.has(path):
        return false

    if batch_resources.has(path):
        return true

    return FileAccess.file_exists(path)

func _resource_for_operation(path: String) -> Resource:
    if batch_resources.has(path):
        var pending: Resource = batch_resources[path]
        return pending

    var resource := _load(path)

    if dry_run and resource != null:
        return resource.duplicate()

    return resource

func _serialize_properties(resource: Resource, values: Dictionary) -> Dictionary:
    var serialized := {}

    for property_name in values:
        serialized[property_name] = VariantSerializer.serialize(
            resource.get(property_name),
            batch_paths
        )

    return serialized

func _resource_overrides() -> Dictionary:
    if not batch_active:
        return {}

    var overrides := {}

    for path in batch_resources:
        overrides[path] = batch_resources[path]

    for path in batch_deleted:
        overrides[path] = null

    return overrides

func _remember_created_resource(
    path: String,
    resource: Resource,
    values: Dictionary
) -> void:
    if not batch_active:
        return

    batch_deleted.erase(path)
    batch_resources[path] = resource
    batch_paths[resource.get_instance_id()] = path
    batch_references[path] = _json_references(values)

func _forget_resource(path: String) -> void:
    if not batch_active:
        return

    batch_resources.erase(path)
    batch_references.erase(path)

    for instance_id in batch_paths.keys():
        if batch_paths[instance_id] == path:
            batch_paths.erase(instance_id)

func _json_references(values: Dictionary) -> Dictionary:
    var references := {}
    _collect_json_references(values, references)
    return references

func _collect_json_references(value: Variant, references: Dictionary) -> void:
    if value is Dictionary:
        var reference = value.get("resource")

        if reference is String and reference.begins_with("res://"):
            references[reference.simplify_path()] = true

        for key in value:
            _collect_json_references(value[key], references)
    elif value is Array:
        for item in value:
            _collect_json_references(item, references)

# _remember_set_references keeps a conservative batch-local set of references
# introduced by set operations. References removed by a set are not subtracted:
# without inspecting every untouched property (which can run custom getters), a
# removed reference could still be held elsewhere on the resource. Keeping the
# reference may block a later delete in the same batch, but it never allows a
# delete that would drop a resource introduced earlier in the batch.
func _remember_set_references(path: String, converted: Dictionary) -> void:
    if not batch_references.has(path):
        batch_references[path] = {}

    var references: Dictionary = batch_references[path]

    for property_name in converted:
        _collect_value_references(converted[property_name], references)

func _collect_value_references(value: Variant, references: Dictionary) -> void:
    if value is Resource:
        var path := _resource_path(value)

        if not path.is_empty():
            references[path] = true
    elif value is Dictionary:
        var reference = value.get("resource")

        if reference is String and reference.begins_with("res://"):
            references[reference.simplify_path()] = true

        for key in value:
            _collect_value_references(value[key], references)
    elif value is Array:
        for item in value:
            _collect_value_references(item, references)

func _resource_path(resource: Resource) -> String:
    var path := str(resource.resource_path)

    if not path.is_empty():
        return path

    var instance_id := resource.get_instance_id()

    if batch_paths.has(instance_id):
        return str(batch_paths[instance_id])

    for candidate_path in batch_resources:
        if is_same(resource, batch_resources[candidate_path]):
            return candidate_path

    return ""

func _referencing_paths_for_delete(path: String) -> Array[String]:
    var referencing := {}

    for candidate in _referencing_paths(path):
        if candidate == path or batch_deleted.has(candidate):
            continue

        referencing[candidate] = true

    for candidate in batch_references:
        if candidate == path or batch_deleted.has(candidate):
            continue

        if batch_references[candidate].has(path):
            referencing[candidate] = true

    var sorted: Array[String] = []

    for candidate in referencing:
        sorted.append(candidate)

    sorted.sort()

    return sorted

func _validated_path(value: Variant) -> Response:
    if not value is String or not value.begins_with("res://"):
        return Response.failure(
            ErrorCode.INVALID_ARGUMENT,
            "A res:// resource path is required"
        )

    var path: String = value.simplify_path()

    if not path.begins_with("res://"):
        return Response.failure(
            ErrorCode.INVALID_ARGUMENT,
            "Resource must be inside the project"
        )

    var extension := path.get_extension().to_lower()

    if extension != "tres" and extension != "res":
        return Response.failure(
            ErrorCode.INVALID_ARGUMENT,
            "Resource path must end with .tres or .res: %s" % path
        )

    return Response.success(path)

func _instantiate(resource_type: String) -> Resource:
    if ClassDB.class_exists(resource_type):
        if not ClassDB.can_instantiate(resource_type):
            return null

        var instance = ClassDB.instantiate(resource_type)

        if instance is Resource:
            return instance

        _free_instance(instance)

        return null

    for entry in ProjectSettings.get_global_class_list():
        if str(entry.get("class", "")) != resource_type:
            continue

        if bool(entry.get("is_abstract", false)):
            return null

        var script: Script = load(entry.get("path", ""))

        if script == null:
            return null

        var instance = script.new()

        if instance is Resource:
            return instance

        _free_instance(instance)

        return null

    return null

func _free_instance(instance: Variant) -> void:
    if instance is Node:
        instance.free()

func _load(path: String) -> Resource:
    if not ResourceLoader.exists(path):
        return null

    var resource := ResourceLoader.load(path)

    if resource == null or not resource is Resource:
        return null

    return resource

func _save(resource: Resource, path: String) -> Response:
    var directory := path.get_base_dir()

    if not DirAccess.dir_exists_absolute(directory):
        var directory_error := DirAccess.make_dir_recursive_absolute(directory)

        if directory_error != OK:
            return Response.failure(
                ErrorCode.ACTION_FAILED,
                "Could not create %s: %s" % [directory, error_string(directory_error)]
            )

    if resource.resource_path != path:
        resource.take_over_path(path)

    var error := ResourceSaver.save(resource, path)

    if error != OK:
        return Response.failure(
            ErrorCode.ACTION_FAILED,
            "Could not save resource %s: %s" % [path, error_string(error)]
        )

    _notify_filesystem(path)

    return null

func _notify_filesystem(path: String) -> void:
    var filesystem := editor.get_resource_filesystem()

    if filesystem.get_filesystem_path(path.get_base_dir()) == null:
        filesystem.scan()
        return

    filesystem.update_file(path)

func _release_cached(path: String) -> void:
    var cached := ResourceLoader.get_cached_ref(path)

    if cached != null:
        cached.take_over_path(path + ".godot-cli-deleted")

func _referencing_paths(path: String) -> Array[String]:
    var root := editor.get_resource_filesystem().get_filesystem()

    if root == null:
        return []

    var files: Array[String] = []

    _collect_files(root, files)

    var referencing: Array[String] = []

    for candidate in files:
        if candidate == path:
            continue

        for dependency in ResourceLoader.get_dependencies(candidate):
            if _dependency_path(dependency) == path:
                referencing.append(candidate)
                break

    referencing.sort()

    return referencing

func _collect_files(directory: EditorFileSystemDirectory, files: Array[String]) -> void:
    for index in directory.get_file_count():
        var file_type: StringName = directory.get_file_type(index)

        if file_type == "TextFile" or file_type == "OtherFile":
            continue

        files.append(directory.get_file_path(index))

    for index in directory.get_subdir_count():
        _collect_files(directory.get_subdir(index), files)

func _dependency_path(dependency: String) -> String:
    if not dependency.contains("::"):
        return dependency

    var fallback := dependency.get_slice("::", 2)

    if not fallback.is_empty():
        return fallback

    var uid_text := dependency.get_slice("::", 0)

    if uid_text.begins_with("uid://"):
        var uid := ResourceUID.text_to_id(uid_text)

        if ResourceUID.has_id(uid):
            return ResourceUID.get_id_path(uid)

    return ""

func _type_name(resource: Resource) -> String:
    var script: Script = resource.get_script()

    while script != null:
        var global_name := str(script.get_global_name())

        if not global_name.is_empty():
            return global_name

        script = script.get_base_script()

    return resource.get_class()
