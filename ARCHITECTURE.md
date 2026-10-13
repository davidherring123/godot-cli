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
  bridge/         connect + one request/response over TCP
  ──────────────── wire ────────────────
  bridge.gd       TCP server (127.0.0.1, ports 49152–49252), polled each frame
  requests/       request_handler.gd routes by action name → *Service
  core/*.gd       editor_scene_context.gd validates; Scene/NodeService executes
```

Editor discovery: on enable, the addon writes
`.godot/godot-cli/instances/<pid>.json` (pid, port, project); the CLI reads it
and connects. If no editor is reachable, commands fail. There is no headless
path yet — assume a running, addon-enabled editor.

## Layers and invariants

- `internal/cli` — cobra commands ONLY. No editor logic, no direct use of
  `internal/bridge`.
- `internal/core/<scene|node|resource>` — one action per file: constant name,
  `Params`, `Result`, thin function that validates and calls `bridge.Call`.
  Registered in `core/schema.go` (`define[...]` + `All()`); schemas feed the
  agent skill docs.
- `internal/core/property` — property metadata shared by the node and resource
  domains.
- `internal/agent/skill` — generates `references/commands.md` + JSON schemas from
  cobra commands / `core` definitions; `godot-cli init --agent` installs them.
- `internal/addon/godot_cli/` — `plugin.gd` (lifecycle) → `requests/`
  (transport + routing) → `core/` (Godot logic). `node_service.gd` is the
  largest file; `node_builder.gd`/`node_path_util.gd` are its helpers.
  `property_codec.gd` holds the property validation, application, and
  serialization shared by the scene, node, and resource services;
  `resource/resource_service.gd` manages `.tres`/`.res` files on disk.
- Mutations go through `EditorUndoRedoManager` — every command is one undo step.
- `expected_scene` is validated centrally in `request_handler.gd`
  (`supports_expected_scene`), except `scene.current`, which handles it itself so
  it can return `active:false` when no scene is open.
- `--save` is a post-effect in `request_handler.gd`: run action → save scene.
  Save once per edit batch, not per command.
- Node property values use `{"resource":"res://..."}` references; decoded in
  `variant_deserializer.gd`, not by the Go side.

## Extension recipe

New action: `internal/core/<domain>/<verb>.go` (name + params/result + call) →
`internal/core/schema.go` (`define` + `All()`) → GDScript handler in the
matching `*_service.gd` + register in its `get_actions()` →
CLI command in `internal/cli/` (set `skill.ActionKey` annotation) →
fixture in `tests/fixtures/core/cases/` + test in `tests/core/`.

## Testing

- `make check` — gofmt, vet, golangci-lint (repo config in `.golangci.yml`).
- `make test` — spins up a real headless Godot editor and drives the addon over
  the bridge (same wire as the CLI). Skips if Godot is missing. CI runs this
  with Godot 4.7.
- Fixtures each own a directory under `tests/fixtures/core/cases/`; shared
  scenes live in `tests/fixtures/core/scenes/`.
