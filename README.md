# godot-cli

A command-line interface for inspecting, controlling, and automating Godot projects.

`godot-cli` is designed to make Godot projects easier to work with from terminals, scripts, CI systems, and AI coding agents without requiring a separate automation protocol.

> [!NOTE]
> `godot-cli` is currently in early development.

## Goals

- Provide a simple CLI for common Godot project operations.
- Work naturally from anywhere inside a Godot project.
- Support both headless workflows and running Godot editors.
- Expose structured output for scripts, CI, and coding agents.
- Keep installation and configuration minimal.
- Stay as close to native Godot APIs and workflows as possible.

## Architecture

`godot-cli` consists of two main parts:

- **Go CLI** — handles commands, project discovery, process management, installation, and communication with Godot.
- **Godot addon** — provides access to live editor state and Godot editor APIs.

The CLI will automatically install and manage its matching addon inside supported Godot projects.

When an editor is running, commands can be routed to the live editor. When no editor is running, commands can use Godot's headless capabilities instead.

## Editor lifecycle

`godot-cli init` installs the addon and enables it in `project.godot`. From there:

```shell
godot-cli editor open     # start the editor and wait for the addon bridge
godot-cli editor status   # check whether an editor is running
```

`editor open` reuses an existing editor when one is already registered, so it is safe to call before live commands. It prints JSON with `opened`, `already_running`, `pid`, `port`, and `version`.

## Runtime feedback

`runtime` tools start the project's game process itself (no editor or addon required) and report what happened:

```shell
godot-cli runtime start                  # windowed, so screenshots work
godot-cli runtime start --headless       # logs, errors, and frame counts only
godot-cli runtime status                 # state, errors, frames, logs, last capture
godot-cli runtime capture                # PNG saved under .godot/godot-cli/runtime
godot-cli runtime stop                   # graceful quit, then kill after timeout
```

`runtime start` loads the configured main scene unless a scene path is supplied, waits until the scene has rendered, and reports parsed `ERROR:`/`SCRIPT ERROR:` lines from stdout and stderr. `runtime capture` returns a `res://` artifact path plus image metadata; MCP calls also receive the PNG as image content. Captures require a windowed runtime — a headless session returns an `unsupported` error instead of a blank image.

Runtime state is scoped to one project and one active session at a time. Sessions, logs, and captures live under `.godot/godot-cli/runtime/sessions/<session>/`, and every artifact path stays inside that directory. `runtime stop` is idempotent and refuses to kill an unresponsive process unless `force=true` is passed.

## MCP server

`godot-cli mcp` exposes the project's actions as a [Model Context Protocol](https://modelcontextprotocol.io) server over stdio. It serves one Godot project per process: start it from anywhere inside the project, or pass `--project`.

Live-editor tools require Godot 4 to be running with the godot-cli addon installed. Run `godot-cli init` once in the project first; it installs and enables the addon. If no editor is running, the `editor_open` tool (or `godot-cli editor open`) starts it and waits for the bridge. Tools that work on saved files, such as `scene_list` and `validate_scene`, do not need a running editor.

```json
{
  "mcpServers": {
    "godot": {
      "command": "godot-cli",
      "args": ["mcp", "--project", "C:/Projects/MyGame"]
    }
  }
}
```

For OpenCode, `godot-cli init --agent opencode` prints the project-local registration command:

```shell
opencode mcp add godot -- godot-cli mcp
```

`godot-cli` does not modify OpenCode configuration itself; the command above adds only the `godot` MCP server to the current project.

Tool names use underscores: `editor_open`, `editor_status`, `scene_list`, `scene_apply`, `node_add`, `resource_set`, `input_event_add`, `validate_scene`, `runtime_start`, `runtime_capture`, and so on. Every successful call returns JSON text plus structured content. `runtime_capture` also returns an MCP image content block when the PNG fits the embedding limit; otherwise its structured result keeps the artifact path, dimensions, and byte size. Tool failures set `isError` and preserve the core error code (`invalid_argument`, `not_found`, `failed_precondition`, `unsupported`, and others) so the client can recover.

For ordered multi-step edits within one domain, use `resource_apply`, `project_settings_apply`, and `input_map_apply` (the CLI equivalents are `resource apply`, `project settings apply`, and `input apply`; the underlying action names are `resource.apply`, `project.settings.apply`, and `input.map_apply`). Each accepts 1–100 operations, applies them in order, returns a compact per-operation summary, and supports `dry_run` to validate the full batch without writing. Later resource create/set operations may reference resources created earlier in the same batch. Batch failures identify the exact operation. Resource batches attempt a best-effort restore of every affected file written by earlier operations; project-setting and Input Map batches apply in memory and save `project.godot` once, skipping the save when every operation is a no-op. Rollback is best-effort: if it fails, the error code is `internal_error` because the editor may be partially changed. `resource_apply` tracks references introduced earlier in a batch conservatively, so removing a reference earlier in the same batch can still block a later delete. `resource_apply --dry-run` can still instantiate resources and run custom Resource scripts and property setters in memory, so scripts with external side effects are not side-effect free. Atomicity is per domain only: these tools never make a resource, project-setting, or Input Map batch atomic with each other, and they do not promise crash recovery mid-write.

`scene_apply` is the batch tool. It accepts 1–100 ordered `operations` of node mutations (`node.add`, `node.set`, `node.rename`, `node.duplicate`, `node.move`, `node.delete`, `node.instantiate`), validates the whole batch on a temporary copy of the active scene, and commits it as one editor undo step. Each operation uses the `operation` field and only the fields valid for that kind; operations can carry an `id` and later operations can target the resulting node with `@id`. Set `dry_run` to return best-effort planned paths and summaries without editing. Prevalidation uses a packed copy of the scene, so editor-only and unowned runtime nodes are not modeled; the live pass still validates and rolls back on such drift. If any operation fails during the live pass, earlier operations are reversed directly without adding the failed batch to undo history, so a previously clean scene stays clean; an incomplete rollback is reported as `internal_error`. Result paths are snapshots at the time each operation completed, not final paths after later renames, moves, or deletes, and an id cannot be referenced after its node is deleted. `save` runs once after the batch succeeds; if the save fails, the batch remains applied and undoable. The result names the exact operation index, `operation`, and id on failure. The granular node tools remain available and behave as before.

stdout is reserved for MCP messages; diagnostics are written to stderr.

## Installation

Installation instructions will be made available once the first version is released.

## License

This project is licensed under the MIT License. See [LICENSE](LICENSE).
