## Build and validate

- Use `make`; do not invoke `go build` / `go test` / `golangci-lint` directly.
- Run `make help` to discover available tasks. Key tasks:
  - Run `make build` to compile to `build/brewkit` with version ldflags.
  - Run `make test` to execute `go test -race ./...`.
  - Run `make check` to execute `fmt-check` + `tidy-check` + `lint` + `test`. Run this before declaring work done.

## Conventions

- The binary is built to `build/brewkit`, not the repo root. `./build/` is gitignored.
- Skip comments in `.go` files unless explaining why a non-obvious choice was necessary.
- Keep `internal/docs/manual.txt` in sync when changing CLI behavior, config files, or user-facing workflows.
- This CLI shells out to Homebrew; treat config/profile inputs as untrusted and validate paths, command args, and parser behavior defensively.

## Before committing

- Always run `make check`; it is the single source of truth for "this is ready".
