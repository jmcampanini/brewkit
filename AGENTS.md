## Build and validate

- This repository uses mise for tool versions and task execution. Run tasks with `mise run <task>`; do not call `go`, `golangci-lint`, or similar tools directly.
- Common tasks (`mise tasks` lists all of them):
  - `mise run check` runs the complete verification contract. **Run this before declaring work done.**
  - `mise run build` compiles `build/brewkit` with version ldflags.
  - `mise run test` runs `go test -count=1 -race ./...`.
  - `mise run fmt` and `mise run lint-fix` apply formatting and lint fixes.
- `mise.toml` pins Go, golangci-lint, and govulncheck; `mise.lock` records their checksums. Bump a tool with `mise upgrade --bump <tool>`, run `mise lock` and `mise run check`, and commit both files.

## Conventions

- The binary is built to `build/brewkit`, not the repo root. `./build/` is gitignored.
- Keep `internal/docs/manual.txt` in sync when changing CLI behavior, config files, or user-facing workflows.
- This CLI shells out to Homebrew; treat config/profile inputs as untrusted and validate paths, command args, and parser behavior defensively.
- Keep command help the canonical user-facing documentation and the README a landing page; keep both consistent with observable behavior.

## Before committing

- Always run `mise run check`; it is the single source of truth for "this is ready".
