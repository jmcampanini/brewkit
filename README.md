# brewkit

`brewkit` manages Homebrew taps, formulas, HEAD formulas, and casks across layered profile files.

## Install

```sh
brew tap jmcampanini/brewkit https://github.com/jmcampanini/brewkit
brew install --HEAD jmcampanini/brewkit/brewkit
```

To update a HEAD install:

```sh
brew upgrade --fetch-HEAD brewkit
```

For a source/dev build:

```sh
git clone https://github.com/jmcampanini/brewkit
cd brewkit
make build
./build/brewkit --version
```

## Quick start

Create `brewkit.toml` and one or more profile files:

```toml
# brewkit.toml
profiles = ["common"]
```

```ruby
# Brewfile.common
brew "git"  # version control
```

```text
# Caskfile.common
ghostty  # terminal emulator

# Headfile.common
direnv  # shell environment loader

# Tapfile.common
jmcampanini/overlay https://github.com/jmcampanini/overlay
```

Then run:

```sh
brewkit lint
brewkit tap
brewkit brew
brewkit head
brewkit cask
```

There is no implicit `common` profile; set `profiles` in `brewkit.toml`, use `BREWKIT_PROFILES=work,personal`, or pass `--profiles work,personal`.

Use `--hide-unchanged` with `tap`, `brew`, `head`, or `cask` to hide already-satisfied per-item lines while keeping the final summary accurate.

Use `--output-prefix "  "` when nesting brewkit under a task runner and you want indented output without piping stdout/stderr, which keeps spinners and colors available.

Use `-q, --quiet` for errors-only operational runs; successful quiet `tap`, `brew`, `head`, `cask`, and `lint` commands are silent, while `lint -q` still prints violations. `--quiet` and `--verbose` cannot be combined.

Run `brewkit docs` for the full file format and config reference.
