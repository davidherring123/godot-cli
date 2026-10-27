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

The bundled `godot-mcp-host` is the primary host. Its private command surface is limited to `serve` for the editor-managed loopback Streamable HTTP endpoint, `connect --project` for stable stdio client configuration, `integrations status|install --project`, and `version`. It deliberately has no scene, node, resource, or other terminal editing commands. `godot-cli connect --project <path>` and `godot-cli mcp` remain available in the legacy CLI during the transition.

`godot-cli connect --project <path>` is the stable stdio entrypoint used by AI client configuration. It serves the same tools as `godot-cli mcp` over stdio for one Godot project. `godot-cli mcp` remains the full server command, including the Streamable HTTP transport.

Use `--transport http` to serve the MCP Streamable HTTP transport instead. The endpoint is `/mcp`. HTTP only binds a loopback `--host` (default `127.0.0.1`) and accepts `--port` (default `0`, which asks the operating system for an available port). With `--state-file`, the server binds first, takes an exclusive lifetime lock for that state path, then atomically writes a discovery document containing `pid`, `transport`, `url`, `version`, `project`, and `started_at`. On shutdown it removes that file only when this process still owns it. A second server cannot publish to the same state path while the first is running.

```shell
godot-cli mcp --transport http --port 8765 --state-file .godot/godot-cli/mcp.json
```

Editor tools require Godot 4 to be running with the godot-cli addon installed. If no editor is running, open the project in Godot and retry. `validate_scene` checks the active scene, including unsaved changes, without saving, opening, switching, or running it. Use the runtime tools for gameplay and early-runtime verification.

```json
{
  "mcpServers": {
    "godot": {
      "command": "/path/to/addons/godot_cli/bin/<os>-<arch>/godot-mcp-host",
      "args": ["connect", "--project", "C:/Projects/MyGame"]
    }
  }
}
```

## AI client integrations

`godot-cli integrations` manages project-scoped configuration for OpenCode, Claude Code, and Codex:

```shell
godot-cli integrations status                         # JSON status for every client
godot-cli integrations install opencode|claude|codex|all
```

Install writes the absolute bundled host path followed by `connect --project <absolute-root>`. OpenCode configuration uses `mcp.servers` in `opencode.json(c)` or `.opencode/opencode.json(c)`; Claude Code uses `.mcp.json`; Codex uses `.codex/config.toml`. Existing unrelated configuration is preserved, and repeated installs are no-ops. The canonical skill is installed at `.agents/skills/godot-cli/SKILL.md` for OpenCode and Codex and `.claude/skills/godot-cli/SKILL.md` for Claude Code.

`godot-cli init --agent opencode` still prints a manual `opencode mcp add` command and does not modify client configuration; use `integrations install` when you want godot-cli to manage it.

Tool names use underscores: `editor_status`, `scene_list`, `scene_apply`, `node_add`, `resource_set`, `input_event_add`, `validate_scene`, `runtime_start`, `runtime_capture`, and so on. Every successful call returns JSON text plus structured content. `runtime_capture` also returns an MCP image content block when the PNG fits the embedding limit; otherwise its structured result keeps the artifact path, dimensions, and byte size. Tool failures set `isError` and preserve the core error code (`invalid_argument`, `not_found`, `failed_precondition`, `unsupported`, and others) so the client can recover.

For ordered multi-step edits within one domain, use `resource_apply`, `project_settings_apply`, and `input_map_apply` (the CLI equivalents are `resource apply`, `project settings apply`, and `input apply`; the underlying action names are `resource.apply`, `project.settings.apply`, and `input.map_apply`). Each accepts 1–100 operations, applies them in order, returns a compact per-operation summary, and supports `dry_run` to validate the full batch without writing. Later resource create/set operations may reference resources created earlier in the same batch. Batch failures identify the exact operation. Resource batches attempt a best-effort restore of every affected file written by earlier operations; project-setting and Input Map batches apply in memory and save `project.godot` once, skipping the save when every operation is a no-op. Rollback is best-effort: if it fails, the error code is `internal_error` because the editor may be partially changed. `resource_apply` tracks references introduced earlier in a batch conservatively, so removing a reference earlier in the same batch can still block a later delete. `resource_apply --dry-run` can still instantiate resources and run custom Resource scripts and property setters in memory, so scripts with external side effects are not side-effect free. Atomicity is per domain only: these tools never make a resource, project-setting, or Input Map batch atomic with each other, and they do not promise crash recovery mid-write.

`scene_apply` is the batch tool. It accepts 1–100 ordered `operations` of node mutations (`node.add`, `node.set`, `node.rename`, `node.duplicate`, `node.move`, `node.delete`, `node.instantiate`), validates the whole batch on a temporary copy of the active scene, and commits it as one editor undo step. Each operation uses the `operation` field and only the fields valid for that kind; operations can carry an `id` and later operations can target the resulting node with `@id`. Set `dry_run` to return best-effort planned paths and summaries without editing. The preview never writes remapped paths into live shared subresources, so dry-run and failed prevalidation do not alter animation tracks or NodePath-bearing resources. Prevalidation still uses a packed copy, so editor-only and unowned runtime nodes are not modeled; the live pass validates and rolls back on such drift. If any operation fails during the live pass, earlier operations are reversed directly without adding the failed batch to undo history, so a previously clean scene stays clean; nodes created by the failed batch are freed, and an incomplete rollback is reported as `internal_error` with restored/total counts and failure details. Result paths are snapshots at the time each operation completed, not final paths after later renames, moves, or deletes, and an id cannot be referenced after its node is deleted. `save` runs once after the batch succeeds; if the save fails, the batch remains applied and undoable, so retry with `scene save` rather than re-running the batch. The result names the exact operation index, `operation`, and id on failure. The granular node tools remain available and behave as before.

stdout is reserved for stdio MCP messages; diagnostics are written to stderr.

### Editor panel

With the addon enabled, the editor's **Godot MCP** bottom panel starts and
stops the bundled host, shows the live bridge endpoint, and manages AI-client
integrations. **Refresh Status** reads the current MCP registration and
workflow-skill state for OpenCode, Claude Code, and Codex. **Install**,
**Repair**, and **Install All** update only the current project's configuration
and skill files; existing unrelated configuration is preserved. The controls
are disabled when the release bundle does not include a host binary.

The panel invokes the bundled host directly, without a shell, and expects one
JSON result object:

```shell
addons/godot_cli/bin/<os>-<arch>/godot-mcp-host integrations status --project /path/to/game
addons/godot_cli/bin/<os>-<arch>/godot-mcp-host integrations install opencode --project /path/to/game
```

Install targets are `opencode`, `claude`, `codex`, and `all`.

## Installation

End users need Godot 4; Go is only required to build godot-cli from source.
Release archives include prebuilt `godot-mcp-host` binaries for macOS, Linux,
and Windows.

- **Godot Asset Library** — install the `godot-cli` addon from the editor's
  AssetLib tab.
- **GitHub** — download `godot-cli-addon-<version>.zip` from the releases page
  and extract it at your project root. Do not use the repository's
  auto-generated "Source code" ZIP; it does not contain the host binaries.
- **GodotEnv** — reference the release ZIP as a `source: "zip"` addon with
  `subfolder: "addons/godot_cli"`, then run `godotenv addons install`.

Release archives place the addon at `addons/godot_cli/` and the matching host at
`addons/godot_cli/bin/<os>-<arch>/godot-mcp-host` (`godot-mcp-host.exe` on
Windows). See [RELEASING.md](RELEASING.md) for archive layout and release
instructions.

## License

This project is licensed under the MIT License. See [LICENSE](LICENSE).
