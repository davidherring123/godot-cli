class_name ResourceService
extends ActionService

var editor: EditorInterface
var dry_run := false
var dry_run_resources: Dictionary = {}
var dry_run_deleted: Dictionary = {}

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

    var property_error := PropertyCodec.apply_properties(resource, values)

    if property_error != null:
        return property_error

    if dry_run:
        dry_run_deleted.erase(path)
        dry_run_resources[path] = resource
        return Response.success({
            "resource": path,
            "type": _type_name(resource),
            "properties": _serialize_properties(resource, values),
        })

    var save_error := _save(resource, path)

    if save_error != null:
        return save_error

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

    var decoded := PropertyCodec.decode_properties(resource, values)

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

    if dry_run:
        dry_run_resources[path] = resource

    var result := {}

    for property_name in names:
        result[property_name] = VariantSerializer.serialize(resource.get(property_name))

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

    if dry_run and dry_run_resources.has(path):
        dry_run_resources.erase(path)
        dry_run_deleted[path] = true
        return Response.success({
            "resource": path,
            "deleted": true,
        })

    var referencing := _referencing_paths(path)

    if not referencing.is_empty():
        return Response.failure(
            ErrorCode.CONFLICT,
            "Resource is still referenced by: %s" % ", ".join(referencing)
        )

    if dry_run:
        dry_run_deleted[path] = true
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
            applied.code,
            "%s; rollback failed: %s" % [applied.message, rollback_error]
        )

    return Response.success(_apply_summary(applied.result, false, true))

func _run_operations(operations: Array, preview: bool) -> Response:
    dry_run = preview
    dry_run_resources.clear()
    dry_run_deleted.clear()

    var summaries: Array = []

    for index in operations.size():
        var operation = operations[index]

        if not operation is Dictionary:
            var invalid := Response.failure(
                ErrorCode.INVALID_ARGUMENT,
                "Each operation must be an object"
            )

            _reset_dry_run_state()
            return batch_operation_failure(index, operation, "", invalid)

        var kind := str(operation.get("operation", ""))
        var target := str(operation.get("resource", ""))
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
            _reset_dry_run_state()
            return batch_operation_failure(index, operation, target, response)

        summaries.append(_operation_summary(index, operation, response.result))

    _reset_dry_run_state()

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
    for path in snapshot:
        var state: Dictionary = snapshot[path]

        if bool(state.get("exists", false)):
            var file := FileAccess.open(path, FileAccess.WRITE)

            if file == null:
                return "%s: %s" % [path, error_string(FileAccess.get_open_error())]

            file.store_buffer(state.get("data", PackedByteArray()))
            file.close()
        elif FileAccess.file_exists(path):
            var error := DirAccess.remove_absolute(path)

            if error != OK:
                return "%s: %s" % [path, error_string(error)]

        _release_cached(path)
        _notify_filesystem(path)

    return ""

func _reset_dry_run_state() -> void:
    dry_run = false
    dry_run_resources.clear()
    dry_run_deleted.clear()

func _resource_exists(path: String) -> bool:
    if dry_run:
        if dry_run_deleted.has(path):
            return false

        if dry_run_resources.has(path):
            return true

    return FileAccess.file_exists(path)

func _resource_for_operation(path: String) -> Resource:
    if dry_run and dry_run_resources.has(path):
        var pending: Resource = dry_run_resources[path]
        return pending

    var resource := _load(path)

    if dry_run and resource != null:
        return resource.duplicate()

    return resource

func _serialize_properties(resource: Resource, values: Dictionary) -> Dictionary:
    var serialized := {}

    for property_name in values:
        serialized[property_name] = VariantSerializer.serialize(resource.get(property_name))

    return serialized

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
