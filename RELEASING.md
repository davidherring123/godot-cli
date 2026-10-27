# Releasing the addon

godot-mcp releases are installable Godot addon archives. They contain the
canonical GDScript addon and prebuilt `godot-mcp-host` binaries, so **end users
do not need Go** or a separate host installation. Users install the archive and
configure AI clients from the editor's **Godot MCP** panel; there is no public
`godot-mcp` command to run.

## Build

```sh
make release-addon VERSION=v0.1.0
```

Artifacts are written to `dist/` (ignored by git; do not commit them):

| File | Use |
|---|---|
| `godot-mcp-addon-<version>.zip` | Godot Asset Library, GitHub ZIP installs, GodotEnv. Contains every supported platform. |
| `godot-mcp-addon-<version>-<os>-<arch>.tar.gz` | Per-platform macOS/Linux download. |
| `godot-mcp-addon-<version>-windows-amd64.zip` | Per-platform Windows download. |
| `SHA256SUMS` | SHA-256 checksums for all archives. |

Supported targets: darwin arm64/amd64, linux amd64/arm64, and windows amd64.
Builds use `CGO_ENABLED=0`, `GOOS`, `GOARCH`, `-trimpath`, `-buildvcs=false`,
and an empty Go build ID. Archive entries are sorted and timestamps are
normalized, so the same source, version, and Go toolchain produce identical
archives.

Set `SOURCE_DATE_EPOCH` to use a fixed Unix timestamp instead of the default
1980-01-01 UTC:

```sh
SOURCE_DATE_EPOCH=$(git log -1 --format=%ct) make release-addon VERSION=v0.1.0
```

Run the packaging tests with:

```sh
go test ./tests/release
```

## Archive layout

```text
addons/godot_mcp/
  plugin.cfg
  bridge/
  core/
  requests/
  bin/macos-arm64/godot-mcp-host
  bin/macos-x86_64/godot-mcp-host
  bin/linux-x86_64/godot-mcp-host
  bin/linux-arm64/godot-mcp-host
  bin/windows-x86_64/godot-mcp-host.exe
```

The GDScript files come from `internal/addon/godot_mcp`, the canonical addon
tree packaged into every archive. Universal archives contain every binary above;
per-platform archives contain only their own `bin/<os>-<arch>/` directory. Host
binaries are executable (`0755`) in both tar and ZIP archives.

The packaged host is a private implementation detail of the addon. Its command
surface is limited to `serve`, `connect`, `integrations`, and `version`:

- `serve` runs the loopback Streamable HTTP backend started by the **Godot MCP**
  panel.
- `connect` runs the stdio MCP server written into generated client
  configuration as `<host> connect --project <absolute-project-root>`.
- `integrations status|install <opencode|claude|codex|all>` manages
  project-scoped client configuration and the portable skill.
- `version` reports the released host version.

No `init`, editor launch, scene, node, resource, project, Input Map, validation,
runtime, or generic MCP command is exposed to users. Client configuration is
installed from the panel, not typed by hand.

On Windows the host file is `godot-mcp-host.exe`.

## Install options

### Godot Asset Library

Upload `godot-mcp-addon-<version>.zip` with the release and use its URL for the
Asset Library entry. The all-platform ZIP is the asset to submit so every
supported OS and architecture works after installation.

### GitHub ZIP

Attach all archives from `dist/` to the GitHub release. Users download
`godot-mcp-addon-<version>.zip` (not the repository's auto-generated "Source
code" ZIP, which has no binaries) and extract it at the project root, leaving
`addons/godot_mcp/` in place. Per-platform `.tar.gz` files are extracted the
same way.

### GodotEnv

Reference the release ZIP from `addons.jsonc` and install it with GodotEnv:

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

GodotEnv copies the archive's addon tree into the project; Go is not required.

## After installation

Enable **Godot MCP** in **Project Settings > Plugins**. The addon starts its
bundled backend when enabled and adds the **Godot MCP** bottom panel. AI client
configuration is project-scoped and is installed from that panel with
**Refresh Status**, **Install**, **Repair**, or **Install All**. No project
installer or public CLI command is part of the user flow.

For upgrades from pre-rename builds, disable the old plugin and remove
`addons/godot_cli/` before extracting the new archive. Use **Install** or
**Repair** afterward to refresh client configurations from the old absolute
host path to `addons/godot_mcp/`.

## Limitations

Release archives are **not code-signed or notarized**. macOS Gatekeeper may
block downloaded host binaries and Windows SmartScreen may warn about them.
Before publishing widely, sign the macOS binaries with a Developer ID and
notarize them, and sign the Windows executable with Authenticode. `SHA256SUMS`
lets users check downloads, but checksums do not replace operating-system
signing.
