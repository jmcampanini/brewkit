# CLAUDE.md

Guidance for Claude Code when working in this repo. Keep it short.

## Build and validate

Use `make` — do not invoke `go build` / `go test` / `golangci-lint` directly.

Run `make help` for the task list. Key tasks:

- `make build` — compile to `build/brewkit` with version ldflags.
- `make test` — `go test -race ./...`.
- `make check` — `fmt-check` + `tidy-check` + `lint` + `test`. **Run this before declaring work done.**

## Conventions

- The binary is built to `build/brewkit`, not the repo root. `./build/` is gitignored.
- Scratch/smoke output goes under `.claude-sandbox/<scenario>/`, not `/tmp/`. This directory is gitignored.
- Skip comments in `.go` files unless explaining why a non-obvious choice was necessary.
- Keep `internal/docs/manual.txt` in sync when changing CLI behavior, config files, or user-facing workflows.
- This CLI shells out to Homebrew; treat config/profile inputs as untrusted and validate paths, command args, and parser behavior defensively.

## Before committing

Always run `make check`. It is the single source of truth for "this is ready".
