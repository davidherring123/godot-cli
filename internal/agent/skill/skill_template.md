---
name: godot-cli
description: Work with Godot projects through godot-cli.
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

Scene and node commands print JSON; setup, status, and help print text. Failures
use stderr and a nonzero exit status. `status` prints the project root, and
`scene current` succeeds with `active: false` when no scene is being edited.

The complete command list with signatures is in `references/commands.md`; one
read covers every command, so prefer it over repeated `godot-cli --help`. Use
`godot-cli <command> --help` only when a command fails and you need its exact
flags.
