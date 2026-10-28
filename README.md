# godot-mcp

[![CI](https://github.com/davidherring123/godot-mcp/actions/workflows/ci.yml/badge.svg?branch=master)](https://github.com/davidherring123/godot-mcp/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/davidherring123/godot-mcp?include_prereleases)](https://github.com/davidherring123/godot-mcp/releases)
[![Godot 4](https://img.shields.io/badge/Godot-4-478CBF?logo=godotengine&logoColor=white)](https://godotengine.org/)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

Connect your AI coding assistant to the Godot 4 editor through the Model Context
Protocol (MCP). Inspect and edit scenes, manage resources, and run your game—all
with access to the live project, including unsaved scene changes.

Built-in setup for **OpenCode**, **Claude Code**, and **Codex**. The addon manages
its bundled MCP host from the editor; no separate host installation is needed.

> [!NOTE]
> Early development.

## Installation

Requires **Godot 4** on macOS (arm64/x86_64), Linux (x86_64/arm64), or Windows
(x86_64). Go is only needed when building from source.

### GodotEnv (recommended)

Install [GodotEnv](https://github.com/chickensoft-games/GodotEnv#-installation),
then add this entry to your project's `addons.json` or `addons.jsonc`:

```json
{
  "addons": {
    "godot_mcp": {
      "url": "https://github.com/davidherring123/godot-mcp/releases/download/v0.1.0/godot-mcp-addon-v0.1.0.zip",
      "source": "zip",
      "subfolder": "addons/godot_mcp"
    }
  }
}
```

Run from the project root (the folder containing `project.godot`):

```sh
godotenv addons install
```

To update, change the version in the URL and run the command again.

### Manual installation

1. Download [godot-mcp-addon-v0.1.0.zip](https://github.com/davidherring123/godot-mcp/releases/download/v0.1.0/godot-mcp-addon-v0.1.0.zip).
2. Extract the archive into your project root. The plugin should be at
   `addons/godot_mcp/plugin.cfg`.

Use the addon archive, not GitHub's **Source code** ZIP, which lacks the bundled
host binaries.

### Connect your AI client

1. Open the project in Godot and enable **Godot MCP** under
   **Project Settings → Plugins**. The bundled host starts automatically.
2. Open the **Godot MCP** bottom panel and select **Integrations**.
3. Click **Install** for your client, or **Install All**. This adds project-scoped
   MCP configuration and a workflow skill.
4. Open your AI client in the same project and check that its MCP tools are available.

Keep the project open in Godot while using the tools. Use **MCP Status** to check
the connection or restart the host, and **Repair** to refresh client setup.

## What you can do

- **Build scenes:** create and edit nodes, instantiate scenes, and batch changes
  into a single editor undo step.
- **Configure your project:** manage resources, project settings, and input actions.
- **Check your work:** validate the active scene, run the game, inspect logs, and
  capture screenshots from a windowed session.

Try asking your assistant:

> “Inspect the current scene and explain its node structure.”
>
> “Add a Camera2D under the player and a jump action bound to Space.”
>
> “Validate this scene, then run the game and take a screenshot.”

See the [reference guide](docs/REFERENCE.md) for batch behavior, dry-run and
rollback limits, runtime details, client configuration paths, and migration notes.

## Development

[Contributing](CONTRIBUTING.md) · [Architecture](ARCHITECTURE.md) ·
[Releasing](RELEASING.md) · [MIT License](LICENSE)

## MCP tools

| Category | Tools | Purpose |
| --- | --- | --- |
| Editor | `editor_status` | Check the connected editor. |
| Scenes | `scene_list`, `scene_current`, `scene_tree`, `scene_open`, `scene_create`, `scene_save`, `scene_apply` | Browse, create, save, and batch-edit scenes. |
| Nodes | `node_inspect`, `node_add`, `node_set`, `node_rename`, `node_duplicate`, `node_move`, `node_delete`, `node_instantiate` | Inspect and edit the scene tree. |
| Resources | `resource_inspect`, `resource_create`, `resource_set`, `resource_delete`, `resource_apply` | Inspect and edit resources, individually or in batches. |
| Project settings | `project_settings_list`, `project_settings_get`, `project_settings_set`, `project_settings_unset`, `project_settings_apply` | Read and update project settings. |
| Input Map | `input_list`, `input_inspect`, `input_add`, `input_set`, `input_rename`, `input_delete`, `input_event_add`, `input_event_delete`, `input_map_apply` | Manage input actions and event bindings. |
| Validation | `validate_scene` | Check the active scene for script and serialization errors. |
| Runtime | `runtime_start`, `runtime_status`, `runtime_capture`, `runtime_stop` | Run the game in a separate process, inspect logs, and capture screenshots. |
