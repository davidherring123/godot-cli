# Architecture

godot-cli is a Go CLI that drives a running Godot 4 editor through a TCP bridge
exposed by an embedded GDScript addon. Read this file before touching
`internal/` or `internal/addon/`.

## Two halves

| | Go CLI (`cmd/`, `internal/`) | Godot addon (`internal/addon/godot_cli/`) |
|---|---|---|
| Runs | outside the editor, any cwd inside the project | inside `godot --editor` as an `EditorPlugin` |
| Talks | discovers the project, sends one JSON action, prints JSON | executes the action against live editor APIs |
| Ships | compiled binary | `//go:embed`ded; `godot-cli init` copies it to `addons/godot_cli/` |

The addon is embedded at build time. There is no version negotiation: the CLI
copies its matching addon on `init`; a running editor picks it up on next launch.

## Request flow

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

Editor discovery: on enable, the addon writes
`.godot/godot-cli/instances/<pid>.json` (pid, port, project); the CLI reads it
and connects. If no editor is reachable, live commands fail — assume a running,
addon-enabled editor. `validate` is the exception: by default it runs the saved
project in a fresh `godot --headless` process and never talks to the editor.
`validate --live` uses the editor like any other live command.

`runtime.*` is also Go-only. It starts the game itself instead of asking the
editor to play the scene, so it works without the addon and without an editor:

```
godot-cli runtime start  →  godot --path <project> --script <runner.gd> -- ...
                            runner.gd extends SceneTree, loads the scene,
                            writes ready/heartbeat/exit JSON, polls requests
godot-cli runtime capture →  request file → runner saves PNG → response file
godot-cli runtime status  →  session files + process liveness + log scan
godot-cli runtime stop    →  graceful stop.request, then kill after timeout
```

Runtime state lives under `.godot/godot-cli/runtime/sessions/<session>/`:
`runner.gd`, `session.json`, `stdout.log`, `stderr.log`, `ready.json`,
`heartbeat.json`, `exit.json`, `requests/`, `responses/`, and `captures/`.
Go never accepts a user-supplied artifact path; screenshot names are generated
by the runtime action and must stay inside the runtime directory, which the
runner re-checks. Captures are `res://` paths so paths stay inside the project
even when the host path contains symbolic links.

`runtime.start` defaults to a window because screenshots need a rendering
display. `headless=true` is supported for CI and error/log feedback; capture
fails with `unsupported` rather than writing a blank image. `runtime.status`
reports parsed `ERROR:`, `SCRIPT ERROR:`, `USER ERROR:`, and `FATAL:` lines from
the log tail, heartbeat frame counts, exit codes, and the last artifact. Only one
runtime session may be active per project. `runtime.stop` is idempotent, and
refuses to kill an unresponsive process unless `force=true`.

For editor actions, the MCP server reuses the same path without a subprocess:

```
godot-cli mcp  →  internal/agent/mcp  →  core action Invoke  →  core/bridge  →  addon
```

MCP arguments are raw JSON decoded into the action's typed `Params`; successful
results are returned as both JSON text and structured content. Tool failures use
`CallToolResult.isError` so the model can read the error code and recover.

## Layers and invariants

- `internal/cli` — cobra commands ONLY. No editor logic, no direct use of
  `internal/core/bridge`.
- `internal/core/<scene|node|resource|input>` and `internal/core/project/settings`
  — one action per file: constant name, `Params`, `Result`, thin function that
  validates and calls `bridge.Call`. Registered in `core/schema.go`
  (`define[...]` + `All()`) with typed schemas and an `Invoke` function; the
  registry feeds the agent skill docs and the MCP server.
- Domain batch actions — `resource.apply`, `project.settings.apply`, and
  `input.map_apply` (MCP `resource_apply`, `project_settings_apply`,
  `input_map_apply`) — sit beside the granular actions and reuse their addon
  handlers. They accept ordered operations (1–100), validate and apply in order,
  support `dry_run`, and report the failed operation index. The resource batch
  resolves references to resources created earlier in the batch, checks deletes
  against references introduced by earlier create/set operations, and restores
  every affected file on a best-effort basis when a later write fails. The
  settings and Input Map batches apply in memory and save `project.godot` once,
  simulating setting override state during the batch and skipping the save for
  all-no-op batches; Input Map dry runs do not reload the live map. Rollback
  failures return `internal_error` because the editor may be partially changed.
  Atomicity is per domain only, and none of the batches promise crash recovery
  mid-write; never describe a resource, settings, and Input Map batch as one
  transaction. Resource `dry_run` still instantiates resources and runs custom
  Resource scripts and property setters in memory; batch-local reference
  tracking derives dependencies from requested property values, so dependencies
  synthesized inside a custom script are only visible after a real write.
  Introduced references are remembered conservatively for the rest of the batch,
  so a set that removes an earlier in-batch reference can still block a later
  delete rather than risk inspecting untouched custom properties. The reference
  check for unmodified files relies on the editor filesystem's dependency cache,
  which can lag external edits. Document these as preview limitations rather than
  promising a side-effect-free validation pass.
- `internal/core/bridge` — editor discovery and TCP request/response transport
  used by core actions; defines the API errors also translated by MCP.
- `internal/agent/mcp` — exposes `core.All()` over the Model Context Protocol.
  Tool names replace dots with underscores (`scene.list` → `scene_list`);
  arguments are decoded into the action's `Params`, results become both JSON
  text and structured content, and `bridge.APIError` and `runtime.Error` codes
  are preserved in `isError` tool responses. Results that implement
  `artifact.ImageSource` (currently `runtime.capture`) also add an MCP image
  content block while the structured result keeps only metadata. Tool calls are
  serialized so the editor only handles one operation at a time. `godot-cli mcp`
  serves stdio and resolves one project for the process lifetime.
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
- `internal/core/scenepath` — resolves explicit scenes and the configured main
  scene for `validate` and `runtime`, rejecting paths outside the project.
- `internal/core/artifact` — protocol-neutral image payload interface used by
  action results and rendered as MCP image content.
- `internal/core/runtime` — Go-only game process lifecycle: detached launch,
  readiness/heartbeat/exit files, log error extraction, screenshot request
  channel, and safe stop. The runner script is embedded, like the validation
  runner, and runs as a custom `SceneTree`; autoloads still initialize normally.
- `internal/core/validate` — saved-project validation spawns `godot --headless`
  with an embedded `runner.gd`, merges the runner's JSON report with Godot's
  `ERROR:`/`SCRIPT ERROR:` output, and never writes to the project.
  `validate.live` packs and re-instantiates the active editor scene in memory:
  it detects scripts that no longer parse and serialization failures without
  saving anything.
- `internal/agent/skill` — generates `references/commands.md` + JSON schemas from
  cobra commands / `core` definitions; `godot-cli init --agent` installs them.
  For OpenCode it also prints the project-local `opencode mcp add` command;
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
  Those services also implement their domain batch action by calling the same
  granular handlers with deferred saves and rollback; resource batches reuse
  `resource_service.gd` handlers with an in-memory preview pass, batch-local
  resource references, and no file writes until apply.
- `scene/scene_apply_service.gd` adds `scene.apply`, the batch entry point for
  `node.add`/`set`/`rename`/`duplicate`/`move`/`delete`/`instantiate`
  operations. It resolves operation ids (`@id`) and reuses the granular
  `NodeService` handlers, so single-node commands and batches share the same
  validation and mutation logic. A batch accepts 1–100 operations and first
  runs against a temporary packed-scene copy (the dry-run path); only a fully
  valid batch edits the live scene. `scene/scene_edit_session.gd` adapts the
  node handlers' `EditorUndoRedoManager` calls into one editor action: do
  methods execute immediately for later operation validation, while do and undo
  methods accumulate without touching editor history. On success the whole
  batch is registered as one action and committed without re-running do
  methods. If the live pass fails, rollback directly executes the accumulated
  undo methods in reverse order and never commits the failed batch to history,
  so an originally clean scene stays clean. If an undo method cannot run, the
  error is `internal_error` and reports how many undo steps were restored.
  `dry_run` is best-effort: packed copies omit editor-only and unowned runtime
  nodes, so the live pass can still fail and roll back. Result paths are
  snapshots from each operation, ids cannot be referenced after their node is
  deleted, and a save failure leaves the applied batch undoable.
- Mutations go through `EditorUndoRedoManager` — every command is one undo step,
  and a whole `scene.apply` batch is one undo step. Project settings and Input
  Map commands are the exception: they change `project.godot` directly and are
  not undoable.
- `expected_scene` is validated centrally in `request_handler.gd`
  (`supports_expected_scene`), except `scene.current`, which handles it itself so
  it can return `active:false` when no scene is open.
- `--save` is a post-effect in `request_handler.gd`: run action → save scene.
  Save once per edit batch, not per command. `scene.apply` supports the same
  post-effect and rejects `save` together with `dry_run`. Previews run against a
  preview `EditorSceneContext` whose `save()` fails closed, so no code path can
  save the live tab during prevalidation. If the final save fails, the batch
  remains applied in the editor and undoable; the error says the action
  completed but the scene could not be saved.
- Node property values use `{"resource":"res://..."}` references; decoded in
  `variant_deserializer.gd`, not by the Go side.

## Extension recipe

New action: `internal/core/<domain>/<verb>.go` (name + params/result + call) →
`internal/core/schema.go` (`define` with the function + `All()`) → GDScript
handler in the matching `*_service.gd` + register in its `get_actions()` →
CLI command in `internal/cli/` (set `skill.ActionKey` annotation) →
fixture in `tests/fixtures/core/cases/` + test in `tests/core/`. Registering the
action in `All()` also exposes it as an MCP tool automatically.

## Testing

- `tests/core` — live Godot action tests; `tests/runtime` — live runtime
  lifecycle/capture tests (windowed capture is opt-in with
  `GODOT_MODE=graphical`); `tests/editor` — isolated editor lifecycle tests,
  including launch behavior with a fake executable.
- `tests/cli` — CLI interface tests, including `init`; `tests/mcp` — MCP
  interface tests, including image content, actionable runtime errors, and live
  calls with their own Godot sessions.
- `tests/addon` checks addon installation and runs a small headless Godot script
  that verifies deferred session history plus one-step undo/redo ordering.
- `tests/harness` — Godot session management shared by live integration tests.
- `make check` — gofmt, vet, golangci-lint (repo config in `.golangci.yml`).
- `make test` — runs Go unit tests, then spins up a real headless Godot editor
  and drives the addon over the bridge (same wire as the CLI). Skips the editor
  suite if Godot is missing. CI runs this with Godot 4.7.
- Fixtures each own a directory under `tests/fixtures/core/cases/`; shared
  scenes live in `tests/fixtures/core/scenes/`.
- `scene.apply` tests live in `tests/core/scene_apply_test.go`; MCP tool tests
  live in `tests/mcp/scene_apply_test.go`. Keep batch operation names and field
  rules in sync between `internal/core/scene/apply.go` and
  `scene_apply_service.gd`.
