## Build and validate

- Use `mise run <task>`; do not invoke pinned tools directly.
- Run `mise run check` before declaring work done.


## Tool updates

- Run `mise upgrade --bump <tool>`, then `mise lock` and `mise run check`.
- Commit `mise.toml` and `mise.lock` together.


## Documentation and input handling

- Keep `internal/docs/manual.txt` in sync when changing CLI behavior, config files, or user-facing workflows.
- This CLI shells out to Homebrew; treat config/layer inputs as untrusted and validate paths, command args, and parser behavior defensively.
- Keep command help the canonical user-facing documentation and the README a landing page; keep both consistent with observable behavior.
