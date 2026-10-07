---
name: godot-cli
description: Work with Godot projects through godot-cli.
---

# godot-cli

Use `godot-cli` when working with Godot scenes, nodes, resources,
project settings, or editor state.

Prefer inspecting actual Godot state instead of guessing.

Use `scene current` to identify the active editor scene and `scene open <scene>`
to open or activate a scene. `scene tree`, `node inspect <node>`, and
`node set <node> <properties-json>` operate on that live scene, including unsaved
changes. Use `--expect-scene res://path.tscn` to catch unexpected tab switches.
Use repeatable `--property <name>` flags with `node inspect` when only specific
properties are needed. `node set` groups property changes into one editor undo
step. Pass `--save` to mutation commands to persist the active scene as part of
the action; otherwise use `scene save`. A scene without a path must first be
saved using the editor's Save As.

Scene and node commands return JSON. Setup, status, and help output are text.
Failures use stderr and a nonzero exit code.

For changes, prefer:

`inspect -> change -> validate -> inspect`

Read `references/commands.md` for the commands supported by the
installed version of godot-cli.

The reference links to JSON input and output schemas for supported commands.
Use CLI usage for argument order and flags; input schemas describe the named
parameters supplied by each command. Load schemas only when their field details
are needed.

When more information is needed, use:

`godot-cli <command> --help`
