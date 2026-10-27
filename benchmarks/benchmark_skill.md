---
name: godot-mcp
description: Optional workflow guidance for building and editing Godot projects with the godot-mcp MCP server.
---

# godot-mcp workflow

Use godot-mcp MCP tools for supported semantic inspection and mutation of scenes,
nodes, resources, project settings, and the Input Map. Tool descriptions and
schemas are authoritative for exact names, arguments, and results.

Use file tools for scripts, shaders, documentation, repository search, exact
serialization, and line references. Use Git for diffs and history. Prefer MCP
over directly editing `.tscn`, `.tres`, or `project.godot`: text edits bypass
Godot validation and can conflict with an open editor.

## Work deliberately

- Call the targeted tool directly when the target and desired values are known.
  Use the narrowest inspection needed only when information is missing.
- Pass the known scene as `expected_scene` for active-scene work. This guards
  against editing the wrong tab. Use `scene_tree` when the active scene or node
  path is unknown.
- Treat mutation results as confirmation of applied values. Do not re-inspect
  solely to confirm them; inspect again only to answer a new question.
- If an operation reports that no editor is available, open this project in
  Godot with the addon enabled, then retry. Scene validation requires the editor.

Batch related scene and node edits. They are editor undo steps; save once on the
last mutation or with `scene_save`, rather than after every call. Save intended
changes before reviewing Git because live editor state may differ from disk.
Resource, project-setting, and Input Map mutations persist immediately and are
not editor undo steps.

When instancing a scene, save its source first. Keep linked instances reusable:
put shared structure and behavior in the source scene and reserve per-instance
overrides for genuine variation.

## Godot structure

Match architecture to the request and the existing project. For gameplay work,
prefer Godot's native composition where appropriate: reusable scenes, standard
body and collision nodes, resources or exported properties for configuration,
and signals for decoupled communication. Put behavior and procedural generation
in scripts rather than constructing every reusable node tree at runtime.

“Small” or “minimal” usually means less scope, not forcing everything into one
script or node. Avoid hand-rolled physics, a single owner for unrelated systems,
or custom drawing as a substitute for ordinary scene structure unless the task
or project calls for it. Do not restructure an intentional existing design just
to impose these defaults.

## Validate the right claim

Use `validate_scene` to check the active editor scene, including unsaved edits,
for script and serialization errors. It does not run the game or prove that
gameplay is correct. Use the runtime tools when behavior matters, and report
that distinction clearly.
