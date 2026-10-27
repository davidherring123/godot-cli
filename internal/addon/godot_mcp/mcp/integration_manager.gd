@tool
extends RefCounted

signal changed

const CLIENT_ORDER := ["opencode", "claude", "codex"]
const CLIENT_LABELS := {
    "opencode": "OpenCode",
    "claude": "Claude Code",
    "codex": "Codex",
}
const INSTALL_TARGETS := ["opencode", "claude", "codex", "all"]

const STATE_INSTALLED := "installed"
const STATE_MISSING := "missing"
const STATE_REPAIR := "repair"
const STATE_UNSUPPORTED := "unsupported"
const STATE_ERROR := "error"
const STATE_UNKNOWN := "unknown"

var last_result := ""
var last_error := ""
var busy := false

var _executable_path := ""
var _project_root := ""
var _statuses := {}
var _report_project_root := ""
var _report_host_path := ""

func _init() -> void:
    _reset_statuses()

func configure(executable_path: String, project_root: String) -> void:
    _executable_path = _normalize_executable(executable_path)
    _project_root = _normalize_project_root(project_root)

func is_available() -> bool:
    return not _executable_path.is_empty()

func is_busy() -> bool:
    return busy

func get_status(client_id: String) -> Dictionary:
    if _statuses.has(client_id):
        return _statuses[client_id]

    return _unknown_status()

func refresh() -> bool:
    if not is_available():
        last_result = ""
        last_error = "The bundled MCP host is unavailable."
        changed.emit()
        return false

    return _run_command(
        ["integrations", "status", "--project", _project_root],
        "status",
        ""
    )

func install(target: String) -> bool:
    if not INSTALL_TARGETS.has(target):
        last_result = ""
        last_error = "Unknown integration target: %s" % target
        changed.emit()
        return false

    if not is_available():
        last_result = ""
        last_error = "The bundled MCP host is unavailable."
        changed.emit()
        return false

    var installed := _run_command(
        ["integrations", "install", target, "--project", _project_root],
        "install",
        target
    )
    var install_result := last_result
    var install_error := last_error

    # Reflect partial installs even when the host reports a failure, but keep the
    # install result/error as the most recent actionable feedback.
    refresh()
    last_result = install_result

    if not install_error.is_empty():
        last_error = install_error
    elif installed and last_error.is_empty():
        last_error = ""

    changed.emit()
    return installed

func status_text(state: String) -> String:
    match state:
        STATE_INSTALLED:
            return "Installed"
        STATE_MISSING:
            return "Missing"
        STATE_REPAIR:
            return "Repair needed"
        STATE_UNSUPPORTED:
            return "Unsupported"
        STATE_ERROR:
            return "Error"
        _:
            return "Unknown"

func button_text(client_id: String) -> String:
    var state := str(get_status(client_id).get("mcp", STATE_UNKNOWN))

    if state == STATE_INSTALLED \
    or state == STATE_REPAIR \
    or state == STATE_ERROR:
        return "Repair"

    return "Install"

func _run_command(arguments: Array, action: String, target: String) -> bool:
    busy = true
    changed.emit()

    var output_lines: Array = []
    var exit_code := OS.execute(
        _executable_path,
        PackedStringArray(arguments),
        output_lines,
        true,
        false
    )
    var output := _join_output(output_lines)
    var data := _parse_json_object(output)
    var command_error := _error_message(data, output, exit_code)

    busy = false

    if exit_code != 0:
        last_result = ""
        last_error = command_error
        changed.emit()
        return false

    if data.is_empty():
        last_result = ""
        last_error = "The bundled MCP host did not return one JSON object."
        changed.emit()
        return false

    if action == "status":
        _apply_statuses(data)

        if not command_error.is_empty():
            last_result = ""
            last_error = command_error
        else:
            last_result = "Integration status refreshed."
            last_error = ""

        changed.emit()
        return command_error.is_empty()

    last_result = _result_message(data, target)
    _apply_statuses(data)

    if command_error.is_empty():
        last_error = ""
    else:
        last_result = ""
        last_error = command_error

    changed.emit()
    return command_error.is_empty()

func _result_message(data: Dictionary, target: String) -> String:
    var message := _first_string(data, ["message", "summary", "detail"])

    if not message.is_empty():
        return message

    var results := data.get("results", [])

    if results is Array:
        var messages: Array[String] = []

        for result in results:
            if result is Dictionary:
                var result_message := _first_string(
                    result,
                    ["message", "summary", "detail"]
                )

                if not result_message.is_empty():
                    messages.append(result_message)

        if not messages.is_empty():
            return "; ".join(PackedStringArray(messages))

    var clients := data.get("clients", [])

    if clients is Array and not clients.is_empty():
        var changed_labels: Array[String] = []
        var installed_labels: Array[String] = []

        for client in clients:
            if not client is Dictionary:
                continue

            var client_id := _entry_id(client)
            var label := str(CLIENT_LABELS.get(client_id, client_id))

            if not label.is_empty():
                installed_labels.append(label)

            if _truthy(client.get("config_changed", false)) \
            or _truthy(client.get("skill_changed", false)):
                if not label.is_empty():
                    changed_labels.append(label)

        if changed_labels.is_empty():
            if target == "all":
                return "All integrations were already up to date."

            if not installed_labels.is_empty():
                return "%s was already up to date." % installed_labels[0]
        elif changed_labels.size() > 1 or target == "all":
            return "Updated %d integration%s." % [
                changed_labels.size(),
                "" if changed_labels.size() == 1 else "s",
            ]

        return "Updated %s." % changed_labels[0]

    if target == "all":
        return "Install completed for all integrations."

    return "Install completed for %s." % CLIENT_LABELS.get(target, target)

func _error_message(
    data: Dictionary,
    output: String,
    exit_code: int
) -> String:
    if not data.is_empty():
        var ok_value = data.get("ok", null)
        var error_text := _string_value(data.get("error", ""))

        if not error_text.is_empty():
            return error_text

        if ok_value == false:
            var failure := _first_string(
                data,
                ["message", "detail", "reason", "summary"]
            )

            if not failure.is_empty():
                return failure

        var errors = data.get("errors", [])

        if errors is Array and not errors.is_empty():
            var first_error = errors[0]

            if first_error is Dictionary:
                var detail := _first_string(
                    first_error,
                    ["message", "detail", "error"]
                )

                if not detail.is_empty():
                    return detail
            elif not str(first_error).is_empty():
                return str(first_error)

    if exit_code != 0:
        var text := output.strip_edges()

        if text.is_empty():
            return "The bundled MCP host exited with code %d." % exit_code

        return "The bundled MCP host exited with code %d: %s" % [
            exit_code,
            _truncate(text, 600),
        ]

    return ""

func _apply_statuses(data: Dictionary) -> bool:
    var found := false

    if data.has("project_root"):
        _report_project_root = _normalize_project_root(
            _string_value(data["project_root"])
        )

    if data.has("host_path"):
        _report_host_path = _normalize_executable(
            _string_value(data["host_path"])
        )

    for key in [
        "integrations",
        "clients",
        "statuses",
        "results",
        "targets",
        "agents",
        "items",
    ]:
        if data.has(key):
            if _consume_container(data[key], ""):
                found = true

    for key in [
        "mcp",
        "mcp_servers",
        "mcpServers",
        "mcp_status",
        "mcpStatus",
    ]:
        if data.has(key):
            if _consume_component_map(data[key], "mcp"):
                found = true

    for key in ["skills", "skill", "skill_status", "skillStatus"]:
        if data.has(key):
            if _consume_component_map(data[key], "skill"):
                found = true

    for key in data.keys():
        var client_id := _client_id(str(key))

        if client_id.is_empty():
            continue

        if _consume_entry(data[key], client_id):
            found = true

    for key in ["data", "result"]:
        if data.has(key) and data[key] is Dictionary:
            if _apply_statuses(data[key]):
                found = true

    return found

func _consume_container(value, key: String) -> bool:
    var found := false

    if value is Array:
        for entry in value:
            if _consume_entry(entry, key):
                found = true

        return found

    if value is Dictionary:
        var map_id := _client_id(key)

        if not map_id.is_empty():
            return _consume_entry(value, map_id)

        var entry_id := _entry_id(value)

        if not entry_id.is_empty() or _has_component(value):
            return _consume_entry(value, "")

        for child_key in value.keys():
            if _consume_entry(value[child_key], str(child_key)):
                found = true

    return found

func _consume_component_map(value, component: String) -> bool:
    if not value is Dictionary:
        return false

    var found := false

    for key in value.keys():
        var client_id := _client_id(str(key))

        if client_id.is_empty():
            continue

        var state := _status_from(value[key])

        if state == STATE_UNKNOWN:
            continue

        _statuses[client_id][component] = state
        found = true

    return found

func _consume_entry(entry, key: String) -> bool:
    var client_id := _client_id(key)

    if client_id.is_empty() and entry is Dictionary:
        client_id = _entry_id(entry)

    if client_id.is_empty():
        return false

    if not entry is Dictionary:
        var state := _status_from(entry)

        if state == STATE_UNKNOWN:
            return false

        _statuses[client_id]["mcp"] = state
        _statuses[client_id]["skill"] = state
        return true

    var found := false

    if _has_client_status_fields(entry):
        var client_mcp_state := _client_mcp_state(entry)
        var client_skill_state := _client_skill_state(entry)

        if not client_mcp_state.is_empty():
            _statuses[client_id]["mcp"] = client_mcp_state
            found = true

        if not client_skill_state.is_empty():
            _statuses[client_id]["skill"] = client_skill_state
            found = true
    else:
        var mcp_value = _entry_component(entry, [
            "mcp",
            "mcp_status",
            "mcpStatus",
            "mcp_server",
            "mcpServer",
            "mcp_installed",
            "mcpInstalled",
            "server",
        ])
        var skill_value = _entry_component(entry, [
            "skill",
            "skills",
            "skill_status",
            "skillStatus",
            "workflow_skill",
            "workflowSkill",
            "skill_installed",
            "skillInstalled",
        ])

        if mcp_value == null and skill_value == null:
            var shared = _entry_component(entry, [
                "installed",
                "configured",
                "present",
                "current",
                "status",
                "state",
                "ok",
            ])

            if shared != null:
                mcp_value = shared
                skill_value = shared

        if mcp_value != null:
            var mcp_state := _status_from(mcp_value)

            if mcp_state != STATE_UNKNOWN:
                _statuses[client_id]["mcp"] = mcp_state
                found = true

        if skill_value != null:
            var skill_state := _status_from(skill_value)

            if skill_state != STATE_UNKNOWN:
                _statuses[client_id]["skill"] = skill_state
                found = true

    var detail := _entry_detail(entry)

    if not detail.is_empty():
        _statuses[client_id]["detail"] = detail

    return found

func _has_client_status_fields(entry: Dictionary) -> bool:
    for key in [
        "configured",
        "up_to_date",
        "config_exists",
        "skill_installed",
        "skill_current",
    ]:
        if entry.has(key):
            return true

    return false

func _client_mcp_state(entry: Dictionary) -> String:
    if entry.has("configured"):
        if not _truthy(entry["configured"]):
            return STATE_MISSING

        var command = entry.get("command", null)

        if command is Array and not _report_host_path.is_empty() \
        and not _report_project_root.is_empty():
            var expected := PackedStringArray([
                _report_host_path,
                "connect",
                "--project",
                _report_project_root,
            ])

            if PackedStringArray(command) == expected:
                return STATE_INSTALLED

            return STATE_REPAIR

        if entry.has("up_to_date"):
            return STATE_INSTALLED if _truthy(entry["up_to_date"]) else STATE_REPAIR

        return STATE_INSTALLED

    if entry.has("up_to_date"):
        return STATE_INSTALLED if _truthy(entry["up_to_date"]) else STATE_REPAIR

    if entry.has("config_exists"):
        return STATE_INSTALLED if _truthy(entry["config_exists"]) else STATE_MISSING

    return ""

func _client_skill_state(entry: Dictionary) -> String:
    if entry.has("skill_installed"):
        if not _truthy(entry["skill_installed"]):
            return STATE_MISSING

        if entry.has("skill_current") and not _truthy(entry["skill_current"]):
            return STATE_REPAIR

        return STATE_INSTALLED

    if entry.has("skill_current"):
        return STATE_INSTALLED if _truthy(entry["skill_current"]) else STATE_MISSING

    return ""

func _entry_id(entry: Dictionary) -> String:
    for key in ["id", "client", "target", "agent", "key", "slug", "name"]:
        var client_id := _client_id(_string_value(entry.get(key, "")))

        if not client_id.is_empty():
            return client_id

    return ""

func _entry_component(entry: Dictionary, keys: Array) -> Variant:
    for key in keys:
        if entry.has(key):
            return entry[key]

    return null

func _entry_detail(entry: Dictionary) -> String:
    return _first_string(entry, [
        "error",
        "message",
        "detail",
        "path",
        "config_path",
        "configPath",
        "file",
    ])

func _has_component(entry: Dictionary) -> bool:
    return _entry_component(entry, [
        "mcp",
        "mcp_status",
        "mcpStatus",
        "mcp_installed",
        "mcpInstalled",
        "skill",
        "skills",
        "skill_status",
        "skillStatus",
        "skill_installed",
        "skillInstalled",
    ]) != null

func _status_from(value) -> String:
    if value is bool:
        return STATE_INSTALLED if value else STATE_MISSING

    if value is int or value is float:
        return STATE_INSTALLED if int(value) != 0 else STATE_MISSING

    if value is String or value is StringName:
        return _status_from_text(str(value))

    if value is Array:
        return STATE_INSTALLED if not value.is_empty() else STATE_MISSING

    if not value is Dictionary:
        return STATE_UNKNOWN

    if value.has("supported") and value["supported"] is bool \
    and not value["supported"]:
        return STATE_UNSUPPORTED

    if value.has("applicable") and value["applicable"] is bool \
    and not value["applicable"]:
        return STATE_UNSUPPORTED

    for key in ["needs_repair", "needs_update", "outdated", "stale"]:
        if value.has(key) and _truthy(value[key]):
            return STATE_REPAIR

    for key in ["status", "state", "result"]:
        if value.has(key):
            var state := _status_from(value[key])

            if state != STATE_UNKNOWN:
                return state

    if value.has("error") and not _string_value(value["error"]).is_empty():
        return STATE_ERROR

    if value.has("missing") and _truthy(value["missing"]):
        return STATE_MISSING

    for key in [
        "installed",
        "configured",
        "registered",
        "present",
        "exists",
        "current",
        "ready",
        "ok",
    ]:
        if not value.has(key):
            continue

        var flag = value[key]

        if flag is bool:
            return STATE_INSTALLED if flag else STATE_MISSING

        var state := _status_from(flag)

        if state != STATE_UNKNOWN:
            return state

    for key in ["path", "config_path", "configPath", "file"]:
        if value.has(key) and not _string_value(value[key]).is_empty():
            return STATE_INSTALLED

    return STATE_UNKNOWN

func _status_from_text(text: String) -> String:
    var normalized := text.to_lower().strip_edges()
    normalized = normalized.replace("-", "_").replace(" ", "_")

    if normalized.is_empty():
        return STATE_UNKNOWN

    match normalized:
        "installed", "configured", "present", "ready", "current", \
        "up_to_date", "uptodate", "ok", "true", "yes", "complete", \
        "completed", "enabled", "available", "found", "synced", "linked":
            return STATE_INSTALLED
        "missing", "not_installed", "notinstalled", "absent", "none", \
        "unavailable", "not_found", "notfound", "disabled", \
        "not_configured", "unconfigured", "required", "pending", \
        "needs_install", "install_required", "false", "no":
            return STATE_MISSING
        "outdated", "stale", "needs_repair", "repair", \
        "repair_required", "update_available", "needs_update", "invalid", \
        "mismatch", "conflict", "broken", "drift", "out_of_date", "partial":
            return STATE_REPAIR
        "unsupported", "not_supported", "not_applicable", "na", "n/a", \
        "skipped":
            return STATE_UNSUPPORTED
        "error", "failed", "failure":
            return STATE_ERROR
        _:
            pass

    if normalized.contains("not_installed") or normalized.contains("missing"):
        return STATE_MISSING

    if normalized.contains("repair") or normalized.contains("outdated"):
        return STATE_REPAIR

    if normalized.contains("install"):
        return STATE_INSTALLED

    return STATE_UNKNOWN

func _truthy(value) -> bool:
    if value is bool:
        return value

    if value is int or value is float:
        return int(value) != 0

    return _string_value(value).to_lower() in ["true", "yes", "1"]

func _first_string(data: Dictionary, keys: Array) -> String:
    for key in keys:
        if not data.has(key):
            continue

        var text := _string_value(data[key])

        if not text.is_empty():
            return text

    return ""

func _string_value(value) -> String:
    if value is String or value is StringName:
        return str(value)

    return ""

func _parse_json_object(text: String) -> Dictionary:
    var trimmed := text.strip_edges()

    if trimmed.is_empty():
        return {}

    var parsed = JSON.parse_string(trimmed)

    if parsed is Dictionary:
        return parsed

    var start := trimmed.find("{")
    var end := trimmed.rfind("}")

    if start >= 0 and end > start:
        parsed = JSON.parse_string(trimmed.substr(start, end - start + 1))

        if parsed is Dictionary:
            return parsed

    return {}

func _join_output(lines: Array) -> String:
    var parts: PackedStringArray = []

    for line in lines:
        parts.append(_string_value(line))

    return "\n".join(parts)

func _normalize_executable(path: String) -> String:
    var executable := path.strip_edges()

    if executable.begins_with("res://") or executable.begins_with("user://"):
        executable = ProjectSettings.globalize_path(executable)

    return executable

func _normalize_project_root(path: String) -> String:
    var root := path.strip_edges()

    if root.length() > 1 and (root.ends_with("/") or root.ends_with("\\")):
        var trimmed := root.substr(0, root.length() - 1)

        if not trimmed.ends_with(":"):
            root = trimmed

    return root

func _truncate(text: String, limit: int) -> String:
    if text.length() <= limit:
        return text

    return text.substr(0, limit) + "..."

func _unknown_status() -> Dictionary:
    return {
        "mcp": STATE_UNKNOWN,
        "skill": STATE_UNKNOWN,
        "detail": "",
    }

func _reset_statuses() -> void:
    _statuses = {}

    for client_id in CLIENT_ORDER:
        _statuses[client_id] = _unknown_status()

func _client_id(value: String) -> String:
    var normalized := value.to_lower().strip_edges()
    normalized = normalized.replace("_", "-").replace(" ", "-")

    match normalized:
        "opencode", "open-code":
            return "opencode"
        "claude", "claude-code", "claudecode", "claude-ai":
            return "claude"
        "codex", "openai-codex":
            return "codex"
        _:
            return ""
