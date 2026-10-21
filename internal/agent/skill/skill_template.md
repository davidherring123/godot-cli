---
name: godot-cli
description: Work with Godot projects through godot-cli: scenes, nodes, resources, project settings, Input Map actions, editor state, and gameplay scaffolding.
---

# godot-cli

Use `godot-cli` for Godot scenes, nodes, resources, and editor state.

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

Common resource file work:

```shell
godot-cli resource create <path> --type <type> --properties '{"...":"..."}'
godot-cli resource inspect <path> --property <name>
godot-cli resource set <path> --properties '{"...":"..."}'
godot-cli resource delete <path>
```

`resource` commands manage `.tres` and `.res` files on disk and need no active
scene. Mutations are written immediately and are not editor undo steps, so there
is no `--save` and no reason to re-inspect to confirm a change. `resource
create` accepts native resource classes such as `StandardMaterial3D` and global
script classes extending `Resource`; unknown, abstract, and non-resource types
are rejected, and an existing file is never overwritten. `resource delete`
fails and lists the project files that still reference the resource, so clear
those references first.

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
```

`project settings` and `input` commands edit `project.godot` and save it
immediately; they need no active scene and are not editor undo steps. `--value`
and `--event` take JSON matching the setting's or event's Godot type. `project
settings unset` removes an override and restores the engine default; binding
indexes come from `input inspect`. `input event add` accepts `key`,
`mouse_button`, `joypad_button`, and `joypad_motion`, for example
`'{"type":"joypad_motion","axis":0,"axis_value":-1}'`.

Every mutation applies to the live editor scene immediately, so run related
edits back to back and save once when they are done: add `--save` to the scene's
final mutation, or run `scene save` afterwards. Do not save between steps. Each
mutation is its own editor undo step, and `saved: false` in a response only means
the scene still has unsaved changes; do not re-inspect to confirm a change.

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

Command results print JSON; setup, status, and help print text. Failures
use stderr and a nonzero exit status. `status` prints the project root, and
`scene current` succeeds with `active: false` when no scene is being edited.

The complete command list with signatures is in `references/commands.md`; one
read covers every command, so prefer it over repeated `godot-cli --help`. Use
`godot-cli <command> --help` only when a command fails and you need its exact
flags.
