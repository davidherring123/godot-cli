# Releasing the addon

godot-cli releases are installable Godot addon archives. They contain the
canonical GDScript addon and prebuilt `godot-mcp-host` binaries, so **end users
do not need Go** or a separate CLI installation.

## Build

```sh
make release-addon VERSION=v0.1.0
```

Artifacts are written to `dist/` (ignored by git; do not commit them):

| File | Use |
|---|---|
| `godot-cli-addon-<version>.zip` | Godot Asset Library, GitHub ZIP installs, GodotEnv. Contains every supported platform. |
| `godot-cli-addon-<version>-<os>-<arch>.tar.gz` | Per-platform macOS/Linux download. |
| `godot-cli-addon-<version>-windows-amd64.zip` | Per-platform Windows download. |
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
addons/godot_cli/
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

The GDScript files come from `internal/addon/godot_cli`, the same canonical
tree `godot-cli init` installs. Universal archives contain every binary above;
per-platform archives contain only their own `bin/<os>-<arch>/` directory. Host
binaries are executable (`0755`) in both tar and ZIP archives.

A launcher or MCP client can invoke the host for the current machine directly:

```sh
addons/godot_cli/bin/<os>-<arch>/godot-mcp-host mcp --project /path/to/game
```

On Windows the file is `godot-mcp-host.exe`.

## Install options

### Godot Asset Library

Upload `godot-cli-addon-<version>.zip` with the release and use its URL for the
Asset Library entry. The all-platform ZIP is the asset to submit so every
supported OS and architecture works after installation.

### GitHub ZIP

Attach all archives from `dist/` to the GitHub release. Users download
`godot-cli-addon-<version>.zip` (not the repository's auto-generated "Source
code" ZIP, which has no binaries) and extract it at the project root, leaving
`addons/godot_cli/` in place. Per-platform `.tar.gz` files are extracted the
same way.

### GodotEnv

Reference the release ZIP from `addons.jsonc` and install it with GodotEnv:

```jsonc
{
  "addons": {
    "godot_cli": {
      "url": "https://github.com/davidherring123/godot-cli/releases/download/v0.1.0/godot-cli-addon-v0.1.0.zip",
      "source": "zip",
      "subfolder": "addons/godot_cli"
    }
  }
}
```

```sh
godotenv addons install
```

GodotEnv copies the archive's addon tree into the project; Go is not required.

## Limitations

Release archives are **not code-signed or notarized**. macOS Gatekeeper may
block downloaded host binaries and Windows SmartScreen may warn about them.
Before publishing widely, sign the macOS binaries with a Developer ID and
notarize them, and sign the Windows executable with Authenticode. `SHA256SUMS`
lets users check downloads, but checksums do not replace operating-system
signing.
