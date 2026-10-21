# godot-cli

A command-line interface for inspecting, controlling, and automating Godot projects.

`godot-cli` is designed to make Godot projects easier to work with from terminals, scripts, CI systems, and AI coding agents without requiring a separate automation protocol.

> [!NOTE]
> `godot-cli` is currently in early development.

## Goals

- Provide a simple CLI for common Godot project operations.
- Work naturally from anywhere inside a Godot project.
- Support both headless workflows and running Godot editors.
- Expose structured output for scripts, CI, and coding agents.
- Keep installation and configuration minimal.
- Stay as close to native Godot APIs and workflows as possible.

## Architecture

`godot-cli` consists of two main parts:

- **Go CLI** — handles commands, project discovery, process management, installation, and communication with Godot.
- **Godot addon** — provides access to live editor state and Godot editor APIs.

The CLI will automatically install and manage its matching addon inside supported Godot projects.

When an editor is running, commands can be routed to the live editor. When no editor is running, commands can use Godot's headless capabilities instead.

## MCP server

`godot-cli mcp` exposes the project's actions as a [Model Context Protocol](https://modelcontextprotocol.io) server over stdio. It serves one Godot project per process: start it from anywhere inside the project, or pass `--project`.

Live-editor tools require Godot 4 to be running with the godot-cli addon installed. Run `godot-cli init` once in the project first. Tools that work on saved files, such as `scene_list`, `resource_inspect`, and `validate_scene`, do not need a running editor.

```json
{
  "mcpServers": {
    "godot": {
      "command": "godot-cli",
      "args": ["mcp", "--project", "C:/Projects/MyGame"]
    }
  }
}
```

Tool names use underscores: `scene_list`, `node_add`, `resource_set`, `input_event_add`, `validate_scene`, and so on. Every successful call returns JSON text plus structured content. Tool failures set `isError` and preserve the core error code (`invalid_argument`, `not_found`, `failed_precondition`, and others) so the client can recover.

stdout is reserved for MCP messages; diagnostics are written to stderr.

## Installation

Installation instructions will be made available once the first version is released.

## License

This project is licensed under the MIT License. See [LICENSE](LICENSE).
