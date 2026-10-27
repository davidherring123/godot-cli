# Agent Instructions

## Project Context

godot-cli is a Godot 4 addon and private Go MCP host, with a GDScript addon executing live editor operations. Users install the addon and manage the bundled host from the editor's Godot MCP panel; there is no public project CLI.

Read `ARCHITECTURE.md` before modifying code under `internal/`, including the embedded GDScript addon. It defines package ownership, dependency boundaries, action registration, editor behavior, and the procedure for introducing new actions.

## Engineering Rules

- Preserve the existing domain → port → adapter architecture. Do not introduce infrastructure dependencies into domain packages.
- Keep business logic out of the private host command and MCP interfaces. Both must reuse the same core actions.
- Follow the extension recipe in `ARCHITECTURE.md` when introducing or modifying actions. Do not create separate implementations for different interfaces.
- Reuse existing handlers, codecs, and shared models rather than duplicating functionality.
- Keep changes focused. Avoid speculative abstractions, unnecessary helpers, and unrelated refactoring.

## Godot Integration

- Prefer native Godot APIs and existing editor workflows.
- Preserve editor undo/redo behavior, unsaved scene state, and batch mutation semantics.
- Respect the existing distinction between live editor actions and the separate runtime game process.
- Preserve stable action names, error codes, and JSON contracts unless intentionally changing the public API.
- Treat dry-run and rollback guarantees according to their documented limitations. Do not assume preview operations are universally side-effect-free.

## Verification

- Run `make check` for formatting, vetting, and static analysis.
- Run `make test` for behavioral verification. Tests requiring Godot may be skipped when the executable is unavailable.
- Add or update relevant tests and fixtures when changing behavior.
- Keep tests under `tests/`; do not add tests for benchmark-only code.
- Do not suppress lint rules or weaken tests merely to make checks pass.
- Report checks that were not run or could not complete.
- Never run paid model benchmarks without explicit authorization.

## Completion

Summarize the changes, verification results, and any remaining limitations.
