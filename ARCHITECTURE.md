# Architecture

godot-cli is an MCP-first bridge between AI agents and a Godot 4 project. Its
primary agent interface is a Model Context Protocol server; the CLI is a
supported interface for terminals, scripts, CI, and debugging. Both share the
same typed actions; live operations drive a running editor through a TCP bridge
exposed by an embedded GDScript addon. Read this file before touching `internal/`
or `internal/addon/`.

## Surfaces

| Surface | Entry point | Role |
|---|---|---|
| MCP | `godot-cli mcp` | Stdio server exposing typed actions as tools. One project per process. Primary agent interface. |
| CLI | `godot-cli <command>` | Cobra commands over the same actions. Fully supported for direct use, scripts, and CI. |
| Skill | `godot-cli init --agent` | Optional short workflow guidance installed at `.agents/skills/godot-cli/`. Not a protocol and not the schema source. |

MCP and the CLI invoke the same core actions in `internal/core`. The skill
carries workflow judgment — routing between MCP, files, and Git, batching edits,
and save/validation semantics — while MCP schemas define the exact tool contract.
Clients may not auto-load the skill, and subagents may not inherit it, so nothing
required to operate the tool may live only in the skill.

## Two halves

| | Go CLI (`cmd/`, `internal/`) | Godot addon (`internal/addon/godot_cli/`) |
|---|---|---|
| Runs | outside the editor, any cwd inside the project | inside `godot --editor` as an `EditorPlugin` |
| Talks | discovers the project, sends one JSON action, returns JSON | executes the action against live editor APIs |
| Ships | compiled binary | `//go:embed`ded; `godot-cli init` copies it to `addons/godot_cli/` |

The addon is embedded at build time. There is no version negotiation: the CLI
copies its matching addon on `init`; a running editor picks it up on next launch.

## Request flow

MCP is the primary path:

```
MCP client → godot-cli mcp → internal/agent/mcp → core.Invoke → core/bridge → addon
```

The CLI takes the same route through cobra:

```
godot-cli node move Player World --save
  cli/*.go        cobra parsing, flags, project discovery, PrintJSON
  core/<domain>/  typed params+result, pre-bridge validation, action name
  core/bridge/    connect + one request/response over TCP
  ──────────────── wire ────────────────
  bridge.gd       TCP server (127.0.0.1, ports 49152–49252), polled each frame
  requests/       request_handler.gd routes by action name → *Service
  core/*.gd       editor_scene_context.gd validates; Scene/NodeService executes
```

MCP arguments are raw JSON decoded into the action's typed `Params`; unknown
fields are rejected. Successful results are returned as both JSON text and
structured content. Tool failures use `CallToolResult.isError`, preserving
`bridge.APIError` codes and mapping argument, missing-editor, multiple-editor,
and missing-Godot failures to stable codes so the model can recover. Calls are
serialized so the editor handles one operation at a time. `godot-cli mcp` serves
stdio and resolves one project for the process lifetime.

Editor discovery: on enable, the addon writes
`.godot/godot-cli/instances/<pid>.json` (pid, port, project); the CLI reads it
and connects. If no editor is reachable, live commands fail — assume a running,
addon-enabled editor. `validate` is the exception: by default it runs the saved
project in a fresh `godot --headless` process and never talks to the editor.
`validate --live` uses the editor like any other live command.

## Layers and invariants

- `internal/cli` — cobra commands ONLY. No editor logic, no direct use of
  `internal/core/bridge`.
- `internal/core/<scene|node|resource|input>` and `internal/core/project/settings`
  — one action per file: constant name, `Params`, `Result`, thin function that
  validates and calls `bridge.Call`. Registered in `core/schema.go`
  (`define[...]` + `All()`) with typed schemas and an `Invoke` function. This
  registry is the single source of truth for MCP tools, tool input/output
  schemas, and read-only/destructive annotations; the CLI reuses the same actions.
- `internal/core/bridge` — editor discovery and TCP request/response transport
  used by core actions; defines the API errors also translated by MCP.
- `internal/agent/mcp` — exposes `core.All()` over the Model Context Protocol.
  Tool names replace dots with underscores (`scene.list` → `scene_list`). Tool
  annotations mark read-only and destructive actions. Server instructions carry
  the constraints an agent needs even when no skill is loaded.
- `internal/core/project` — the `Project` type, `Resolve` path handling, and
  `FindProjectRoot` project discovery used by every command.
- `internal/core/editor` — Go-only `editor.open` and `editor.status` actions.
  `open` launches `godot --editor` detached, waits for the addon's instance file,
  and reuses an already-registered editor; `status` reports the registered
  instance without starting one.
- `internal/core/godot` — locates the Godot executable for `validate.scene` and
  `editor.open` (`GODOT_BIN`, then `godot`/`godot4` on PATH).
- `internal/core/property` — property metadata shared by the node and resource
  domains.
- `internal/core/validate` — saved-project validation spawns `godot --headless`
  with an embedded `runner.gd`, merges the runner's JSON report with Godot's
  `ERROR:`/`SCRIPT ERROR:` output, and never writes to the project.
  `validate.live` packs and re-instantiates the active editor scene in memory:
  it detects scripts that no longer parse and serialization failures without
  saving anything.
- `internal/agent/skill` — installs the optional short workflow skill to
  `.agents/skills/godot-cli/SKILL.md`. That path is kept stable for
  compatibility. `SKILL.md` is the only installed artifact; skill content must
  not duplicate MCP schemas, claim that every client auto-loads it, or become
  required for MCP operation. For OpenCode,
  `init --agent` also prints the project-local `opencode mcp add` command;
  godot-cli never writes OpenCode configuration itself.
- `internal/addon` — embeds the GDScript addon, installs/updates it, and enables
  `res://addons/godot_cli/plugin.cfg` in `project.godot` during `init`.
- `internal/addon/godot_cli/` — `plugin.gd` (lifecycle) → `requests/`
  (transport + routing) → `core/` (Godot logic). `node_service.gd` is the
  largest file; `node_builder.gd`/`node_path_util.gd` are its helpers.
  `property_codec.gd` holds the property validation, application, and
  serialization shared by the scene, node, and resource services;
  `resource/resource_service.gd` manages `.tres`/`.res` files on disk.
  `project/settings/settings_service.gd` edits `project.godot` settings and
  `input/input_service.gd` (with `input/input_event_codec.gd`) edits Input Map
  actions, both saving the project file immediately rather than as undo steps.
- Mutations go through `EditorUndoRedoManager` — every command is one undo step.
  Project settings and Input Map commands are the exception: they change
  `project.godot` directly and are not undoable.
- `expected_scene` is validated centrally in `request_handler.gd`
  (`supports_expected_scene`), except `scene.current`, which handles it itself so
  it can return `active:false` when no scene is open.
- `--save` is a post-effect in `request_handler.gd`: run action → save scene.
  Save once per edit batch, not per command. The MCP tool argument is `save`.
- Node property values use `{"resource":"res://..."}` references; decoded in
  `variant_deserializer.gd`, not by the Go side.

## Documentation ownership

- `core/schema.go` action descriptions and reflected schemas are the canonical
  MCP contract. Keep them precise enough to use without the skill.
- `internal/agent/mcp` server instructions should state prerequisites, save
  batching, and error behavior. They must stand alone when no skill is loaded.
- `internal/agent/skill` is short workflow guidance only. It should explain
  routing between MCP, files, and Git without restating tool signatures or acting
  as a command catalog.

## Extension recipe

New action: `internal/core/<domain>/<verb>.go` (name + params/result + call) →
`internal/core/schema.go` (`define` with the function + `All()`) → GDScript
handler in the matching `*_service.gd` + register in its `get_actions()` →
CLI command in `internal/cli/` →
fixture in `tests/fixtures/core/cases/` + test in `tests/core/`. Registering the
action in `All()` exposes it as an MCP tool and schema automatically; write the
description for an agent that sees only the tool list.

## Testing

- `tests/core` — live Godot action tests; `tests/editor` — isolated editor
  lifecycle tests, including launch behavior with a fake executable.
- `tests/cli` — CLI interface tests, including `init`; `tests/mcp` — MCP
  interface tests, including live calls with their own Godot sessions.
- `tests/harness` — Godot session management shared by live integration tests.
- `make check` — gofmt, vet, golangci-lint (repo config in `.golangci.yml`).
- `make test` — runs Go unit tests, then spins up a real headless Godot editor
  and drives the addon over the bridge (same wire as the CLI). Skips the editor
  suite if Godot is missing. CI runs this with Godot 4.7.
- `benchmarks/` — model-driven MCP versus direct-file comparisons.
  `make benchmark-smoke` validates the full MCP path without invoking a model.
- Fixtures each own a directory under `tests/fixtures/core/cases/`; shared
  scenes live in `tests/fixtures/core/scenes/`.
