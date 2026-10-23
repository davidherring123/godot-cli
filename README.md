# godot-cli

MCP-first tooling for Godot 4 projects. `godot-cli` exposes a
[Model Context Protocol](https://modelcontextprotocol.io) server that lets AI
agents inspect and edit live editor scenes, resource files, project settings,
and the Input Map. The same actions remain available through the CLI for
terminals, scripts, CI, and debugging.

> [!NOTE]
> `godot-cli` is currently in early development.

## Goals

- Make MCP the primary interface for AI coding agents.
- Expose one typed action registry to both MCP and the CLI.
- Work naturally from anywhere inside a Godot project.
- Support live-editor and headless workflows.
- Keep setup minimal: `godot-cli init` and `godot-cli mcp`.
- Stay close to native Godot APIs and workflows.

## Setup

Requirements: Godot 4 for live-editor tools, and `godot-cli` on `PATH` (or a
command your MCP client can launch).

1. Install and enable the addon in the project:

   ```shell
   godot-cli init
   ```

   Re-running `init` updates the embedded addon to match the CLI. If the editor
   is already open, restart it so the new addon loads.

2. Register the MCP server with your client. The server speaks stdio and serves
   one project per process:

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

   Start the client from inside the project to omit `--project`.

For OpenCode, `init --agent opencode` installs the optional project skill and
prints the registration command:

```shell
opencode mcp add godot -- godot-cli mcp
```

`godot-cli` does not modify OpenCode configuration; the command adds only the
`godot` MCP server to the current project. `init --agent codex` installs the same
portable skill for Codex. Register MCP in each client using that client's own
configuration.

## Skill and MCP schema

The installed skill is deliberately short workflow guidance: when to use MCP
versus files or Git, how to batch edits, and save/validation semantics. The MCP
tool schemas are the authoritative contract for tool names, arguments, types,
defaults, results, and read-only/destructive hints. If the two disagree, follow
the schema.

Do not assume a skill is loaded automatically. Some clients discover project
skills, others require explicit invocation, and subagents may not inherit them.
Registering the MCP server is the required integration; the server instructions,
tool list, and schemas are sufficient without the skill. The skill installs to
`.agents/skills/godot-cli/`; that path is kept stable for compatibility.

## Agent workflow

Prefer MCP for anything that needs the live editor or typed Godot validation.
Use ordinary files for source text, and Git for history and review:

| Task | Use | Why |
|---|---|---|
| Inspect the scene tree, nodes, resources, settings, or Input Map | **MCP** | Answers supported semantic questions through Godot; scene/node inspection includes live editor state. |
| Add, rename, move, delete, or configure nodes; create or edit resources | **MCP** | Scene and node edits are validated and undoable; resource, setting, and Input Map changes write immediately. |
| Validate a saved or live scene | **MCP** | Runs Godot validation and returns structured errors. |
| Write scripts, shaders, documentation, and other text assets | **Files** | Source is easier to read and edit directly. |
| Search source, inspect exact serialization or line references, and edit non-Godot files | **Dedicated file tools** | Answers questions about source and saved text. |
| Diff, review, commit, branch, or revert | **Git** | MCP does not manage version control. |

Directly editing `.tscn`, `.tres`, or `project.godot` is possible, but it
bypasses Godot validation and can conflict with an open editor. Prefer MCP while
the editor is open; use direct file edits when MCP is unavailable or when the
change is intentionally textual.

Typical loop:

1. Try the targeted tool directly; if it requires an editor and none is running,
   call `editor_open` and retry.
2. When information is missing, use the narrowest read tool, such as `scene_tree`
   or `node_inspect`. Skip discovery when the target and required values are known.
3. Make related mutations back to back. Scene and node changes are undoable
   editor actions; pass `save=true` on the last one, or call `scene_save`.
   Resource, project-setting, and Input Map tools save immediately and have no
   save argument.
4. Validate with `validate_scene` or `validate_live` when the scene must load or
   run.
5. When reviewing persisted changes, save intended scene edits so Git diffs
   include them. Live editor state and saved files can differ.

Mutation results confirm applied values, not gameplay correctness. Avoid repeating
an inspection through files unless it answers a different question or resolves
an ambiguity.

MCP tool calls are serialized, so the editor handles one operation at a time.
A failed tool call sets `isError` and returns a JSON error with a stable `code`
(`invalid_argument`, `not_found`, `failed_precondition`, and others) plus a
message, so the agent can correct the request or recover.

## CLI compatibility

MCP and the CLI invoke the same core actions. Existing CLI commands remain
supported and available: use them when MCP is not connected, inside scripts and
CI, or to debug a bridge or editor problem. CLI commands print JSON for command
results; setup, status, and help print text. Failures use stderr and a nonzero
exit status.

```shell
godot-cli status
godot-cli scene list
godot-cli node inspect Player --property position
godot-cli validate res://levels/arena.tscn
```

## Editor lifecycle

`editor open` starts `godot --editor`, waits for the addon bridge, and reuses an
already registered editor. It prints JSON with `opened`, `already_running`,
`pid`, `port`, and `version`. `editor status` reports the registered instance
without starting one, and the equivalent MCP tools are `editor_open` and
`editor_status`.

Live-editor tools require Godot 4 with the addon installed. Tools that read
saved files, such as `scene_list` and `validate_scene`, work without a running
editor.

## MCP tools

Tool names replace action dots with underscores: `editor_open`, `editor_status`,
`scene_list`, `scene_current`, `scene_open`, `scene_create`, `scene_save`,
`scene_tree`, `node_inspect`, `node_set`, `node_add`, `node_instantiate`,
`node_rename`, `node_duplicate`, `node_move`, `node_delete`, `resource_create`,
`resource_inspect`, `resource_set`, `resource_delete`, `project_settings_*`,
`input_*`, `validate_scene`, and `validate_live`.

Every successful call returns JSON text plus structured content. stdout is
reserved for MCP messages; diagnostics are written to stderr.

For benchmark diagnostics, `godot-cli mcp --trace-file <path>` appends JSONL
records of completed tool calls, including arguments, outcome, duration, and
the invoking session ID when supplied by the client. Tracing is off by default
and works with both native tool calls and Code Mode.

## Benchmark connection check

With Godot and OpenCode installed, verify the full MCP path without a model call:

```shell
make benchmark-smoke
```

This creates an isolated fixture, starts its editor and a private OpenCode server,
checks MCP discovery and session permissions, and calls `scene_list` and
`scene_current` through OpenCode against the expected scene. A temporary plugin
provides the model-free invocation path. The check also exercises the session API
lifecycle: permissions are preserved, the model-free session collects without
messages, the configuration endpoint returns JSON, and the session is deleted.
It verifies the conditions for MCP instruction inclusion, not the assembled model
prompt.

Benchmarks default to `mcp` and the `playable_platformer` task; `--approach both`
compares MCP with `direct-files`, the only other supported approach. Tasks are
defined by JSON under `benchmarks/tasks/` and verified by a generic structural
verifier: the task file selects the scene to load and lists checks, where each
check names a `kind` and supplies its own counts and flags (body types and
minimums, collision or script requirements, input actions, HUD text, main-scene
loadability), and the verifier reports which pass. This is a structural check of
the saved scene, not evidence that gameplay is correct or fun. Connection
preflight runs before the model timer. Results separate setup, agent, and task
failures and use server-side traces to count MCP calls, including Code Mode
calls. Model benchmarks incur provider usage; the smoke check does not.

## Installation

Release packages are not available yet. To build from source:

```shell
make install
```

This installs `godot-cli` to `~/.local/bin` (`%USERPROFILE%\.local\bin` on
Windows) using the Go toolchain. `make build` writes `./bin/godot-cli` instead.

## License

This project is licensed under the MIT License. See [LICENSE](LICENSE).
