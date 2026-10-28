# Contributing to godot-mcp

Contributions, bug reports, and feature suggestions are welcome.

## Getting Started

- Check existing issues and pull requests to avoid duplicate work.
- Read [ARCHITECTURE.md](ARCHITECTURE.md) to understand the project's structure and design decisions.

## Issues and Pull Requests

For major features, breaking changes, or architectural changes, open an issue to discuss the proposed approach before implementation. This helps ensure changes align with the project's direction.

Keep each pull request focused on one coherent change. Descriptions have no required format, but should clearly communicate the motivation, changes made, relevant testing, and any known limitations concisely.

## Development

The project uses Go and Godot 4.7.2.

Before submitting code changes, run:

```sh
make check
make test
```

`make check` runs formatting verification, `go vet`, and `golangci-lint`.

`make test` runs the Go test suite, including integration tests requiring Godot. Some tests may be skipped if Godot is unavailable.

Add or update tests for behavioral changes when appropriate. Keep tests under `tests/`. Avoid unrelated refactoring or weakening existing checks to make a change pass.

Report any checks that were not run or could not complete in the pull request description. Do not run model-backed benchmarks without explicit authorization because they may incur costs.

## Branches

Create contributions on a focused branch rather than committing directly to `master`.

Keep the branch up to date with `master` before requesting review. Do not force-push shared branches or rewrite history after review has started unless necessary and clearly communicated.

## Commits

Use Conventional Commit-style messages:

```text
type(scope): description
```

Common types include:

- `feat` — new functionality
- `fix` — bug fixes
- `refactor` — internal restructuring
- `test` — test additions or changes
- `docs` — documentation
- `ci` — continuous integration
- `build` — build system or dependencies
- `chore` — maintenance
- `perf` — performance improvements

Scopes are optional. Prefer existing domains such as `node`, `scene`, `resource`, `mcp`, or `runtime` when applicable.

Keep the description concise and imperative, such as `add` rather than `added`. Use the same convention for pull request titles.

## Compatibility

Preserve existing public behavior unless intentionally introducing a breaking change.

Changes to action names, JSON contracts, MCP interfaces, or error codes should be documented and tested. Breaking changes require prior discussion and must be clearly identified in the pull request.
