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

## Installation

Installation instructions will be made available once the first version is released.

## License

This project is licensed under the MIT License. See [LICENSE](LICENSE).
