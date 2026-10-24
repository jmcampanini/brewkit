## Build and validate

- Install prerequisites with `brew install git mise`, then run `mise trust` and `mise install`.
- Use `mise run <task>`; do not invoke pinned tools directly.
- Run `mise tasks` to discover available tasks. Key tasks:
  - Run `mise run build` to compile to `build/brewkit` with version ldflags.
  - Run `mise run test` to run every test uncached with the race detector.
  - Run `mise run check` to verify formatting, module tidiness, lint, uncached race tests, the production build, version output, and vulnerabilities. Run this before declaring work done.

## Tool updates

- Run `mise upgrade --bump <tool>`, then `mise lock` and `mise run check`. Commit `mise.toml` and `mise.lock` together.

## Conventions

- The binary is built to `build/brewkit`, not the repo root. `./build/` is gitignored.
- Keep `internal/docs/manual.txt` in sync when changing CLI behavior, config files, or user-facing workflows.
- This CLI shells out to Homebrew; treat config/profile inputs as untrusted and validate paths, command args, and parser behavior defensively.
- Keep command help the canonical user-facing documentation and the README a landing page; keep both consistent with observable behavior.

## Before committing

- Always run `mise run check`; it is the single source of truth for "this is ready".
