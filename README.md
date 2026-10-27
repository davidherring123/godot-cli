# godot-mcp

A Godot 4 editor addon that exposes live project automation over the Model Context Protocol (MCP).

`godot-mcp` runs inside Godot. Its **Godot MCP** bottom panel manages a private,
bundled MCP host and configures supported AI clients for the current project.
There is no public command-line interface to install, configure, or drive the
addon: installation, client setup, and day-to-day use all start in the editor.

> [!NOTE]
> `godot-mcp` is currently in early development.

## What it provides

- MCP tools for inspecting and mutating scenes, nodes, resources, project
  settings, and the Input Map.
- Live editor access, including the active scene and unsaved changes.
- Ordered batch operations with best-effort `dry_run` previews and rollback
  limits documented below.
- Scene validation that reports script and serialization errors without saving,
  opening, or running the scene.
- Runtime tools that start the game, report logs and frame counts, and capture
  screenshots from a windowed session.
- Project-scoped client setup for OpenCode, Claude Code, and Codex from the
  editor panel, including the portable workflow skill.

## Requirements

- Godot 4.
- A supported platform with a bundled host: macOS arm64/x86_64, Linux
  x86_64/arm64, or Windows x86_64.

Go is only required to build the addon host from source. End users do not need Go,
a separately installed binary, or any terminal setup.

## Installation

Install the addon with one of these methods:

### Godot Asset Library

Install `godot-mcp` from the editor's **AssetLib** tab.

### GodotEnv

Reference the release ZIP as a `source: "zip"` addon and install it with
GodotEnv:

```jsonc
{
  "addons": {
    "godot_mcp": {
      "url": "https://github.com/davidherring123/godot-cli/releases/download/v0.1.0/godot-mcp-addon-v0.1.0.zip",
      "source": "zip",
      "subfolder": "addons/godot_mcp"
    }
  }
}
```

```sh
godotenv addons install
```

### Release ZIP

Download `godot-mcp-addon-<version>.zip` from the releases page and extract it
at the project root. Do not use the repository's auto-generated "Source code"
ZIP; it does not contain the host binaries.

### Enable the addon

After installing, enable **Godot MCP** in **Project Settings > Plugins** if it
is not already enabled. The **Godot MCP** panel is added to the bottom panel and
the bundled backend starts automatically. If the panel reports that the backend
is not included, use a release archive rather than a development checkout.

When upgrading from a pre-rename `godot-cli` build, disable that plugin and
remove `addons/godot_cli/` before installing this addon. Then use **Install** or
**Repair** in the Godot MCP panel so client configurations point at the renamed
host path. The new addon uses `addons/godot_mcp/`, `.godot/godot-mcp/`, and
`godot-mcp` skill directories; old generated state and skill directories may be
removed after the migration.

Release archives place the addon at `addons/godot_mcp/` and the matching host at
`addons/godot_mcp/bin/<os>-<arch>/godot-mcp-host` (`godot-mcp-host.exe` on
Windows). See [RELEASING.md](RELEASING.md) for archive layout and release
instructions.

## Configure AI clients

Open the project in Godot and use the **Godot MCP** bottom panel:

1. Use **MCP Status** for backend health, endpoint details, and lifecycle
   controls. It warns when no AI client integration is configured.
2. Open **Integrations** to manage client registration and workflow skills.
3. **Refresh Status** reads the current MCP registration and workflow-skill
   state for each client.
4. **Install** or **Repair** updates one client; **Install All** updates every
   supported client.
5. **Start**, **Restart**, and **Stop** control the bundled backend. **Copy
   Endpoint** copies the loopback Streamable HTTP endpoint when a client needs
   to connect to it directly.

Installation writes project-scoped configuration and the portable skill. It
preserves unrelated configuration, and repeated installs are no-ops when the
project is already current.

| Client | MCP configuration | Skill |
|---|---|---|
| OpenCode | `mcp.servers` in `opencode.json(c)` or `.opencode/opencode.json(c)` | `.agents/skills/godot-mcp/SKILL.md` |
| Claude Code | `.mcp.json` | `.claude/skills/godot-mcp/SKILL.md` |
| Codex | `.codex/config.toml` | `.agents/skills/godot-mcp/SKILL.md` |

The generated client configuration launches the bundled host with its private
`connect --project <absolute-project-root>` entrypoint over stdio. The host is
an implementation detail of the addon, not a supported user-facing command-line
interface. Its only commands are the internal `serve`, `connect`,
`integrations`, and `version` commands used by the panel and generated client
configuration.

## Editor and runtime lifecycle

Live editor tools require this project to be open in Godot with the addon
enabled. The addon registers the editor instance when it starts; if no editor is
registered, open the project in Godot and retry. There is no command to launch
or inspect the editor.

Runtime tools start a separate game process instead of asking the editor to play
a scene. They use the Godot executable reported by the connected editor and
therefore also require an editor registered for the project. Runtime sessions do
not fall back to `GODOT_BIN` or a `godot` executable on `PATH`.

Runtime state is scoped to one project and one active session at a time.
Sessions, logs, and captures live under
`.godot/godot-mcp/runtime/sessions/<session>/`, and every artifact path stays
inside that directory. Start defaults to a windowed session so screenshots work;
`headless` remains available for logs and error feedback, and capture returns an
`unsupported` error instead of a blank image. Stop is idempotent and refuses to
kill an unresponsive process unless `force` is set.

## MCP tools

Tool names use underscores: `editor_status`, `scene_list`, `scene_apply`,
`node_add`, `resource_set`, `input_event_add`, `validate_scene`, `runtime_start`,
`runtime_capture`, and so on. Every successful call returns JSON text plus
structured content. `runtime_capture` also returns an MCP image content block
when the PNG fits the embedding limit; otherwise its structured result keeps the
artifact path, dimensions, and byte size. Tool failures set `isError` and
preserve the core error code (`invalid_argument`, `not_found`,
`failed_precondition`, `unsupported`, and others) so the client can recover.

### Scene editing

`scene_apply` is the batch tool. It accepts 1–100 ordered `operations` of node
mutations (`node.add`, `node.set`, `node.rename`, `node.duplicate`, `node.move`,
`node.delete`, `node.instantiate`), validates the whole batch on a temporary
copy of the active scene, and commits it as one editor undo step. Each operation
uses the `operation` field and only the fields valid for that kind; operations
can carry an `id`, and later operations can target the resulting node with
`@id`. Set `dry_run` to return best-effort planned paths and summaries without
editing.

The preview never writes remapped paths into live shared subresources, so
dry-run and failed prevalidation do not alter animation tracks or
NodePath-bearing resources. Prevalidation still uses a packed copy, so
editor-only and unowned runtime nodes are not modeled; the live pass validates
and rolls back on such drift. If any operation fails during the live pass,
earlier operations are reversed directly without adding the failed batch to undo
history, so a previously clean scene stays clean. Nodes created by the failed
batch are freed, and an incomplete rollback is reported as `internal_error` with
restored/total counts and failure details.

Result paths are snapshots at the time each operation completed, not final paths
after later renames, moves, or deletes, and an id cannot be referenced after its
node is deleted. `save` runs once after the batch succeeds; if the save fails,
the batch remains applied and undoable, so retry with `scene_save` rather than
re-running the batch. The result names the exact operation index, `operation`,
and id on failure. The granular node tools remain available and behave as
before.

### Resources, project settings, and Input Map

For ordered multi-step edits within one domain, use `resource_apply`,
`project_settings_apply`, and `input_map_apply` (the underlying action names are
`resource.apply`, `project.settings.apply`, and `input.map_apply`). Each accepts
1–100 operations, applies them in order, returns a compact per-operation
summary, and supports `dry_run` to validate the full batch without writing.
Later resource create/set operations may reference resources created earlier in
the same batch. Batch failures identify the exact operation.

Resource batches attempt a best-effort restore of every affected file written by
earlier operations; project-setting and Input Map batches apply in memory and
save `project.godot` once, skipping the save when every operation is a no-op.
Rollback is best-effort: if it fails, the error code is `internal_error` because
the editor may be partially changed. `resource_apply` tracks references
introduced earlier in a batch conservatively, so removing a reference earlier in
the same batch can still block a later delete.

A resource `dry_run` can still instantiate resources and run custom Resource
scripts and property setters in memory, so scripts with external side effects
are not side-effect free. Atomicity is per domain only: these tools never make a
resource, project-setting, or Input Map batch atomic with each other, and they
do not promise crash recovery mid-write.

### Validation

`validate_scene` checks the active editor scene, including unsaved changes,
without saving, opening, switching, or running it. It reports script and
serialization errors but does not prove gameplay correctness. Use the runtime
tools when behavior matters.

## Project layout

- **Godot addon** — provides the editor plugin, TCP bridge, live editor
  operations, the **Godot MCP** panel, and the backend supervisor.
- **Private Go host** — bundles the MCP server, core actions, editor/runtime
  adapters, client integration installer, and the portable skill. It is shipped
  inside the addon and launched by the panel or generated client configuration.

See [ARCHITECTURE.md](ARCHITECTURE.md) for package boundaries, request flow, and
the action extension recipe.

## Contributing

Development, test, and release commands are documented in
[CONTRIBUTING.md](CONTRIBUTING.md) and [RELEASING.md](RELEASING.md). The
contributor-only release flow uses `make release-addon`; it is not a user
installation step.

## License

This project is licensed under the MIT License. See [LICENSE](LICENSE).
