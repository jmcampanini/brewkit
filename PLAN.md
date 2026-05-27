# Plan: Adopt `go-config-loader` for brewkit configuration

## Intention

Use `github.com/jmcampanini/go-config-loader` as brewkit's configuration-loading boundary. The CLI should rely on the shared library for raw config layering, provenance, environment-variable loading, flag-backed overrides, and effective-config reporting.

Brewkit should retain ownership of brewkit-specific runtime derivation: profile semantics, `local` profile handling, and how loaded values are interpreted when running commands.

## Goals

- Replace brewkit's bespoke TOML/default/flag/env config loading with the shared config library.
- Align brewkit's config UX with overlay where appropriate.
- Make config provenance visible through the library's reporting API.
- Keep raw config loading separate from derived runtime behavior.
- Error on unknown config keys so typos are surfaced early.
- Remove the implicit `common` profile default.
- Make an empty active profile list an error for apply commands.

## Config API

### Config file

Brewkit continues to use `brewkit.toml`.

Implicit config lookup remains:

```sh
brewkit <command>
```

Explicit config path remains:

```sh
brewkit --config path/to/brewkit.toml <command>
```

An implicit missing config file is allowed. An explicit missing config file is an error.

### Raw config shape

```toml
dir = "."
profiles = []
env_profiles = ""
fail_fast = true
```

Fields:

- `dir`: directory containing profile files, resolved relative to the process current working directory.
- `profiles`: raw active profiles.
- `env_profiles`: optional environment variable name whose comma-separated values append to `profiles`.
- `fail_fast`: stop on first operation error when true; collect/report failures when false.

### Environment API

Config-backed environment variables use the `BREWKIT_` prefix.

Primary profile override:

```sh
BREWKIT_PROFILES=work,personal brewkit brew
```

`env_profiles` is separate: it names an additional user-controlled environment variable whose values append to the raw profile list.

Example:

```toml
profiles = ["work"]
env_profiles = "DOTFILES_PROFILES"
```

```sh
DOTFILES_PROFILES=personal brewkit brew
```

Effective profiles become:

```text
work, personal
```

### Flag API

Profile flags mirror overlay:

```sh
brewkit --profiles work,personal brew
```

The old singular `--profile` flag is intentionally not preserved.

Other existing global flags keep their current purpose unless they become config-backed explicitly.

## Runtime semantics

### `dir`

`dir` is a loaded config value. It is not anchored to the config file location.

Example:

```sh
cd /tmp/session
brewkit --config ~/dotfiles/brewkit.toml brew
```

With:

```toml
dir = "homebrew"
profiles = ["common"]
```

Brewkit looks for:

```text
/tmp/session/homebrew/Brewfile.common
```

not:

```text
~/dotfiles/homebrew/Brewfile.common
```

If `dir` is omitted, the default `.` also means the process current working directory.

### Profiles

Effective profiles are derived from:

1. raw `profiles`
2. appended values from the environment variable named by `env_profiles`
3. de-duplication while preserving first occurrence
4. brewkit validation
5. automatic `local` append when any `*file.local` exists

The profile name `local` remains reserved and cannot be listed explicitly.

Apply commands error when the effective profile list is empty.

### Zero-byte config files

Zero-byte config files do not need special semantics. They are valid empty config files and therefore load defaults.

Because `dir` is resolved relative to the process current working directory, a zero-byte explicit config such as `/dev/null` does not accidentally anchor `dir` to `/dev`.

## Reporting API

`brewkit config` should use the config library's reporting API.

The command should show:

1. raw loaded config as TOML
2. provenance from the config library
3. effective runtime values as comments

The raw config section represents what the library loaded. The effective comments represent what brewkit will use after runtime derivation, including effective `dir`, effective profiles, and any auto-appended `local` note.

## Non-goals

- Preserve the old `--profile` flag.
- Preserve `profiles_env`; the new field is `env_profiles`.
- Keep the implicit `common` profile default.
- Anchor `dir` to the config file location.
- Add custom config loaders for `env_profiles`.
- Auto-discover profiles for apply commands when no profiles are selected.

## Validation

### Automated tests

Validation should cover:

- Missing implicit config uses defaults.
- Missing explicit config errors.
- Unknown TOML keys error.
- Default `profiles` is empty.
- `BREWKIT_PROFILES` overrides raw profiles.
- `--profiles` overrides file/env raw profiles.
- `env_profiles` appends profiles after raw loading.
- Duplicate effective profiles are de-duplicated while preserving order.
- Explicit `local` profile is rejected from file, env, flags, and `env_profiles`.
- `local` is auto-appended when any `*file.local` exists.
- Apply commands error when no effective profiles exist.
- `dir` resolves relative to the process current working directory, not the config file directory.
- Zero-byte config files load defaults without special path anchoring.
- `brewkit config` includes raw TOML, provenance, and effective comments.

### Manual checks

Run representative commands:

```sh
brewkit config
brewkit --profiles work config
BREWKIT_PROFILES=work brewkit config
brewkit --config /dev/null config
brewkit brew
```

Expected high-level outcomes:

- Config output shows library-backed raw config and provenance.
- Effective comments match runtime behavior.
- No profile selection causes apply commands to fail clearly.
- Profile selection via `--profiles`, `BREWKIT_PROFILES`, and `env_profiles` behaves predictably.

### Compatibility checks

- Update README/docs to remove the implicit `common` default.
- Update docs from `profiles_env` to `env_profiles`.
- Update docs from `--profile` to `--profiles`.
- Confirm generated config output remains understandable and copyable as TOML aside from comments.
