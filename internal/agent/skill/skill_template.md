---
name: godot-cli
description: Work with Godot projects through godot-cli: scenes, nodes, resources, project settings, Input Map actions, editor state, and gameplay scaffolding.
---

# godot-cli

Use `godot-cli` for Godot scenes, nodes, resources, and editor state.

If a live command fails because no editor is running, start it with
`godot-cli editor open` and retry. `godot-cli editor status` reports whether an
editor is registered. `godot-cli init` installs and enables the addon.

Use the narrowest command that completes the task. Do not rediscover information
already supplied:

- When the scene is known, use `--expect-scene` instead of `scene list` or
  `scene current`.
- When the node path is known, skip `scene tree`.
- When the property and value are known, mutate directly.
- When inspecting, pass `--property <name>` rather than returning everything.
- When the scene or node location is unknown, start with `scene tree`; it reports
  the active scene and every node path in one call.

Structure defaults for new or extended gameplay:

Match structure to the request, not to file or node count. Prefer, where
appropriate: reusable objects as scenes (`Player.tscn`, `Platform.tscn`) that
you instantiate with `node instantiate` instead of rebuilding them in code;
standard nodes for behavior and collisions (`CharacterBody2D` for the player,
`StaticBody2D` for fixed platforms, `AnimatableBody2D` for moving ones,
`CollisionShape2D`, `Camera2D`, `CanvasLayer` for HUD); configuration as
exported properties or `.tres` resources created with `resource create`, not
magic numbers in code; signals or a small manager to decouple objects; scripts
for behavior and procedural spawning.

Avoid these shapes unless asked: one script that `_draw()`s all visuals; one
script owning player, platforms, generation, camera, and HUD; hand-rolled
physics instead of `CharacterBody2D` + `CollisionShape2D`; runtime-built node
trees that could be `.tscn` files.

Read "small", "quick", or "minimal" as less scope — fewer features, boxes
instead of art, fewer levels — not fewer files or nodes. Keep architecture
proportional; do not create scenes for trivial one-off nodes. Code-only or
custom `_draw()` implementations are fine when explicitly requested or clearly
better (custom rendering, heavy procedural work, throwaway experiments); if
you deviate, say why in one line before building, and do not restructure an
existing project's intentional design just to match this.

Common scene and node work:

```shell
godot-cli scene create res://levels/arena.tscn --root-type Node2D --name Arena
godot-cli node inspect <node> --property <name> --expect-scene <scene>
godot-cli node set <node> --properties '{"position":{"x":64,"y":96}}' --expect-scene <scene>
godot-cli node add <type> <parent> --name <name> --properties '{"position":{"x":64,"y":96}}' --expect-scene <scene>
godot-cli node instantiate <source> <parent> --name <name> --expect-scene <scene>
godot-cli node rename <node> <name> --expect-scene <scene>
godot-cli node duplicate <node> --name <name> --expect-scene <scene>
godot-cli node move <node> <parent> --index <n> --expect-scene <scene>
godot-cli node delete <node> --expect-scene <scene>
godot-cli scene save --expect-scene <scene>
```

For several ordered node edits, use one `scene apply` call instead of one command
per node. A batch accepts 1–100 operations. Operations run in order and commit
as a single editor undo step. Each operation uses the `operation` field and only
the fields valid for that kind. The batch is prevalidated on a temporary copy of
the scene, so a failed operation leaves the live scene untouched and the error
names the failed operation index, `operation`, and id. Prevalidation does not
write remapped paths into live shared subresources, so it cannot alter animation
tracks or NodePath-bearing resources. A live failure reverses earlier
operations directly without adding the failed batch to undo history, so a
previously clean scene stays clean; nodes created by the failed batch are freed,
and an incomplete rollback is reported as an `internal_error` with restored and
total undo counts plus failure details. Give an operation an `id` and later
operations can target its resulting node with `@id` in `node` or `parent`; an id
cannot be referenced after its node is deleted. Use `--dry-run` to preview paths
and summaries without editing. Dry-run is best-effort topology validation because
packed copies omit editor-only and unowned runtime nodes. Result paths are
snapshots from when each operation completed, not final paths after later
renames, moves, or deletes. `--properties` is not used here; each operation
carries its own `properties` object. Add `--save` once to save after the batch
succeeds; if the save fails, the batch remains applied and undoable. Retry with
`scene save` instead of re-running the batch.

```shell
godot-cli scene apply --expect-scene <scene> --operations '[
  {"operation":"node.add","id":"spawn","type":"Marker2D","parent":"World","name":"Spawn","properties":{"position":{"x":32,"y":48}}},
  {"operation":"node.set","node":"@spawn","properties":{"position":{"x":64,"y":96}}},
  {"operation":"node.rename","node":"@spawn","name":"SpawnPoint"},
  {"operation":"node.instantiate","id":"player","scene":"res://player.tscn","parent":"@spawn","name":"Player"}
]'
```

Common resource file work:

```shell
godot-cli resource create <path> --type <type> --properties '{"...":"..."}'
godot-cli resource inspect <path> --property <name>
godot-cli resource set <path> --properties '{"...":"..."}'
godot-cli resource delete <path>
godot-cli resource apply --operations '[{"operation":"...","resource":"res://..."}]' [--dry-run]
```

`resource` commands manage `.tres` and `.res` files on disk and need no active
scene. Mutations are written immediately and are not editor undo steps, so there
is no `--save` and no reason to re-inspect to confirm a change. `resource
create` accepts native resource classes such as `StandardMaterial3D` and global
script classes extending `Resource`; unknown, abstract, and non-resource types
are rejected, and an existing file is never overwritten. `resource delete`
fails and lists the project files that still reference the resource, so clear
those references first. `resource apply` runs 1–100 ordered create, set, and
delete operations, validates and applies them in order, and restores files
written by earlier operations on a best-effort basis if a later write fails
(rollback failures return `internal_error`). Later create/set operations may
reference resources created earlier in the same batch, and deletes check the
references introduced by those earlier operations as well as references already
on disk. `--dry-run` validates without writing, but previews still instantiate
resources and run property setters in memory, so custom Resource scripts with
external side effects are not side-effect free.

Project settings and the Input Map:

```shell
godot-cli project settings list --prefix <group>
godot-cli project settings get <setting>
godot-cli project settings set <setting> --value '<json>'
godot-cli project settings unset <setting>
godot-cli input list
godot-cli input inspect <action>
godot-cli input add <action> --deadzone <0..1>
godot-cli input set <action> --deadzone <0..1>
godot-cli input rename <action> <new-name>
godot-cli input delete <action>
godot-cli input event add <action> --event '{"type":"key","physical_keycode":32}'
godot-cli input event delete <action> --index <n>
godot-cli project settings apply --operations '[{"operation":"...","setting":"..."}]' [--dry-run]
godot-cli input apply --operations '[{"operation":"...","action":"..."}]' [--dry-run]
```

`project settings` and `input` commands edit `project.godot` and save it
immediately; they need no active scene and are not editor undo steps. `--value`
and `--event` take JSON matching the setting's or event's Godot type. `project
settings unset` removes an override and restores the engine default; binding
indexes come from `input inspect`. `input event add` accepts `key`,
`mouse_button`, `joypad_button`, and `joypad_motion`, for example
`'{"type":"joypad_motion","axis":0,"axis_value":-1}'`.

`project settings apply` and `input apply` run 1–100 ordered operations, validate
the whole batch in order, apply changes in memory, and save `project.godot` once.
Set and unset operations in one batch see each other's override state (a set
followed by unset removes the override, and a second unset reports unchanged).
A failed batch attempts to restore in-memory state and `project.godot`;
rollback failures return `internal_error`. All-no-op batches do not rewrite
`project.godot`, and `input apply --dry-run` does not reload the Input Map.
Input Map binding indexes refer to the state after earlier operations in the same
batch. These batches are domain-local — they do not make resource,
project-setting, and Input Map changes atomic with each other, and they do not
promise crash recovery mid-write. Batch errors include the zero-based operation
index and target.

Every mutation applies to the live editor scene immediately, so run related
edits back to back and save once when they are done: add `--save` to the scene's
final mutation or to `scene apply`, or run `scene save` afterwards. Do not save
between steps. Each granular mutation is its own editor undo step and a whole
`scene apply` batch is one undo step. `saved: false` in a response only means the
scene still has unsaved changes; do not re-inspect to confirm a change. If
`--save` fails, the edits remain applied and undoable, and the error says the
action completed before the save failure. Retry with `scene save`; do not re-run
the mutation or batch.

`--properties` takes a JSON object of property names to values. Vectors are
`{"x":..,"y":..}` (add `z`/`w` as needed), colors `{"r":..,"g":..,"b":..,"a":..}`,
and rectangles `{"position":{...},"size":{...}}`; other scalars and strings are
literal. Control theme overrides use their full property paths, for example
`theme_override_font_sizes/font_size` and `theme_override_colors/font_color`.

Resource-valued properties (textures, materials, scripts) use
`{"resource": "res://..."}` references, for example
`'{"script":{"resource":"res://player.gd"}}'`. `node inspect` returns resources
in the same shape with their type. `node add`, `node instantiate`, and
`scene create` report the final node path, name, type, and resulting property
values; omit `--name` for a generated unique name.

`node instantiate` adds a linked instance of an existing scene. Instancing reads
the source scene from disk, so save the source scene before instancing it.
Instances stay connected to their source: change shared nodes in the source scene
instead of overriding them per instance, and note that only the instance root's
properties can be set. Save the active scene before `scene open` if its edits
must persist.

`scene create` writes a new scene file with one root node and opens it; the path
must be new, and the file is written immediately, so later edits only need the
usual final save. `node rename` keeps NodePath references and animation tracks
pointing at the node, and `node duplicate` copies a subtree, preserving scripts,
scene instances, and connections. `node move` reparents or reorders a node with
the same reference updates, so build a hierarchy bottom-up and move nodes into
place, or fix a mistaken order, without deleting and re-adding.

Try the targeted command directly when using `--expect-scene`; open the scene
only if it reports a different scene is active. A mutation response that reports
the resulting values is enough validation, including when asked to verify the
result; do not re-inspect solely to confirm it.

Validate that a scene loads and runs:

```shell
godot-cli validate
godot-cli validate <scene>
godot-cli validate --live --expect-scene <scene>
```

`validate` loads a scene in a fresh headless Godot process and runs it briefly,
returning `valid`, `scene`, and `errors` and exiting nonzero when validation
fails; without a scene argument it uses the configured main scene. `validate
--live` checks the active editor scene instead, including unsaved state, and
reports `unsaved`. Neither mode saves or modifies the project. Use it to confirm
that a scene loads and runs; it does not replace the mutation responses above,
which remain the way to confirm individual property values.

See and debug the running game:

```shell
godot-cli runtime start [scene]
godot-cli runtime start --headless
godot-cli runtime status [--session <id>]
godot-cli runtime capture [--session <id>]
godot-cli runtime stop [--session <id>] [--force]
```

`runtime start` launches the project's game process, waits until the scene is
ready, and prints the session, state, frames, errors, and log paths. It defaults
to a window because `runtime capture` needs a rendering display; use
`--headless` in CI, where the same session reports parsed engine/script errors
but capture fails with `unsupported`. `runtime status` reports the active or most
recent session, including `error_count` and distinct `errors`; read it before
guessing why a scene exited. Captures are written under
`.godot/godot-cli/runtime` and returned as `res://` artifact paths; MCP calls
also receive the PNG as image content. Stop a session when finished;
`runtime stop` is idempotent and requires `--force` only when the heartbeat has
stopped responding. Do not start a second session while one is active.

Command results print JSON; setup, status, and help print text. Failures
use stderr and a nonzero exit status. `status` prints the project root, and
`scene current` succeeds with `active: false` when no scene is being edited.

The complete command list with signatures is in `references/commands.md`; one
read covers every command, so prefer it over repeated `godot-cli --help`. Use
`godot-cli <command> --help` only when a command fails and you need its exact
flags.
