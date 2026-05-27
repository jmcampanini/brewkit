# Plan: Revisit `--quiet`

## Intention

Make `--quiet` mean what users expect: operational commands should emit nothing unless there is an error. A successful quiet run should be silent and communicate success through its exit code.

## CLI API

### Global output flags

- `-q, --quiet`: errors-only mode for operational commands.
- `-v, --verbose`: verbose mode for raw Homebrew output.
- `--quiet` and `--verbose` are mutually exclusive.

If both are provided, brewkit should fail with a clear usage/configuration error that says the flags cannot be used together.

### Operational commands

For these commands, `--quiet` means errors only:

- `brewkit tap`
- `brewkit brew`
- `brewkit head`
- `brewkit cask`
- `brewkit lint`

Expected quiet behavior:

- No per-item success output.
- No skipped/missing-file notices.
- No restart notices.
- No final success or failure summary line.
- Errors remain visible.
- Apply-command errors continue to include captured Homebrew output.
- Apply-command failures may still also print the final top-level `brewkit:` error line.
- `lint -q` still prints individual violations because violations are actionable errors.

### Output-producing commands

These commands should keep printing their requested primary output even when `--quiet` is present:

- `brewkit config`
- `brewkit docs`

For these commands, quiet should not suppress the command payload.

## Goals

- Make quiet mode predictable: successful operational runs are silent.
- Preserve actionable failure information.
- Avoid hiding lint violations in quiet mode.
- Clarify that quiet and verbose are contradictory modes.
- Keep docs and help text aligned with the CLI behavior.
- Preserve exit-code semantics: scripts can rely on status even when stdout is silent.

## Non-goals

- Do not change default output behavior.
- Do not change verbose output behavior except for rejecting `--quiet --verbose` together.
- Do not suppress `config` or `docs` command payloads.
- Do not introduce a new summary-specific flag.

## Validation

Behavioral checks:

- A successful `brewkit -q brew` run produces no stdout/stderr output.
- A quiet apply run with missing profile files produces no skipped notice and no summary.
- A quiet apply run with a Homebrew failure prints the item error and captured Homebrew output.
- A quiet apply run with failures still exits non-zero.
- `brewkit -q lint` prints individual lint violations and exits non-zero.
- `brewkit -q lint` with no violations prints nothing and exits zero.
- `brewkit -q config` still prints config output.
- `brewkit -q docs` still prints docs output.
- Passing both `--quiet` and `--verbose` fails with a clear mutual-exclusion error.
- Help text and `internal/docs/manual.txt` describe the new quiet contract and mutual exclusion.

Project validation:

- Run `make check` before considering the change complete.
