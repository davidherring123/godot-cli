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

Common node work:

```shell
godot-cli node inspect <node> --property <name> --expect-scene <scene>
godot-cli node set <node> '{"position":{"x":64,"y":96}}' --save --expect-scene <scene>
```

`node set` takes a JSON object of property names to values. Vectors are
`{"x":..,"y":..}` (add `z`/`w` as needed), colors `{"r":..,"g":..,"b":..,"a":..}`,
and rectangles `{"position":{...},"size":{...}}`; other scalars and strings are
literal.

Try the targeted command directly when using `--expect-scene`; open the scene
only if it reports a different scene is active. `--save` persists the scene and
groups the change into one editor undo step. A mutation response that reports the
resulting values with `saved: true` is enough validation, including when asked to
verify the result; do not re-inspect solely to confirm it.

Scene and node commands print JSON; setup, status, and help print text. Failures
use stderr and a nonzero exit status. `status` only prints the project root.

The complete command list with signatures is in `references/commands.md`; one
read covers every command, so prefer it over repeated `godot-cli --help`. Use
`godot-cli <command> --help` only when a command fails and you need its exact
flags.
