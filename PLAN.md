# Progress Spinner Plan

## Intention

Add lightweight progress feedback so `brewkit` does not appear frozen while it is checking Homebrew state, installing, upgrading, tapping, or handling HEAD formulas.

The spinner is intended to communicate “work is currently happening” without changing the existing result-oriented output model.

## Goals

- Show progress feedback during all Homebrew subprocess work.
- Keep normal command output clean: spinner text should be transient, while final item lines and summaries remain the durable output.
- Preserve existing behavior for:
  - `--quiet`
  - `--verbose`
  - non-interactive / redirected output
  - errors and failure output
  - final summaries
- Use user-facing, high-level messages rather than raw `brew` command details.
- Avoid expanding the public CLI surface for now; no new spinner flag initially.

## Non-goals

- Do not turn `brewkit` into an interactive TUI.
- Do not stream raw Homebrew output live as part of this change.
- Do not change how successful, verbose, or failed Homebrew output is captured and rendered.
- Do not add a spinner to `--quiet`, `--verbose`, or redirected output.

## User-facing API

No new command-line flags or config keys are planned initially.

Spinner behavior is automatic:

| Mode | Spinner? |
| --- | --- |
| Normal interactive terminal output | Yes |
| `--quiet` | No |
| `--verbose` | No |
| stdout redirected / non-TTY | No |
| `NO_COLOR` set | No |

The existing command API remains unchanged:

```sh
brewkit tap
brewkit brew
brewkit head
brewkit cask
```

## Spinner message style

Messages should describe the user-level operation currently in progress.

Examples:

- `Checking Homebrew state…`
- `Tapping charmbracelet/tap…`
- `Installing ripgrep…`
- `Upgrading neovim…`
- `Checking HEAD tmux…`
- `Installing HEAD tmux…`
- `Reinstalling HEAD tmux…`
- `Checking latest HEAD tmux…`

Messages should avoid exposing low-level command details such as `brew outdated --json=v2` unless a future diagnostic mode explicitly asks for that.

## Expected output behavior

In normal interactive mode, users should see a transient spinner while work is running, followed by the same durable result lines they see today, for example:

```text
+ ripgrep
↑ neovim 0.10.0 → 0.10.2

Summary: 1 added, 1 upgraded
```

The spinner line should not remain in the captured final output after the operation completes.

In non-spinner modes, output should match the current stable behavior.

## Validation

Validate with automated tests and manual terminal checks.

### Automated validation

- Existing tests should continue passing unchanged where spinner is disabled.
- Non-TTY test captures should not include spinner frames or transient messages.
- `--quiet` should still show only errors and final summary.
- `--verbose` should still print raw captured Homebrew output after operations and should not show spinner output.
- Failure paths should still print full captured Homebrew output.
- Summaries should remain accurate.

### Manual validation

Run in an interactive terminal and confirm a spinner appears during slow work:

```sh
brewkit brew
brewkit cask
brewkit head
brewkit tap
```

Confirm no spinner appears for:

```sh
brewkit --quiet brew
brewkit --verbose brew
brewkit brew > out.txt
NO_COLOR=1 brewkit brew
```

Confirm final output remains clean after spinner completion and does not leave partial spinner frames behind.

## Success criteria

- A user can tell `brewkit` is actively working during long checks or installs.
- Scripts and redirected output remain stable and machine-friendly.
- Existing result lines, summaries, and error reporting semantics are preserved.
- The public CLI remains unchanged.
