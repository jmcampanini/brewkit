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

There is no implicit `common` profile; set `profiles` in `brewkit.toml`, use `BREWKIT_PROFILES=work,personal`, pass `--profiles work,personal`, or repeat `--profile work --profile personal`.

Use `--hide-unchanged` with `tap`, `brew`, `head`, or `cask` to hide already-satisfied per-item lines while keeping the final summary accurate.

Use `--output-prefix TEXT` to prefix output emitted by brewkit commands, including durable command output, errors, and transient progress frames. Help and version output are intentionally not prefixed.

Use `-q, --quiet` for errors-only operational runs; successful quiet `tap`, `brew`, `head`, `cask`, and `lint` commands are silent, while `lint -q` still prints violations. `--quiet` and `--verbose` cannot be combined.

Run `brewkit docs` for the full file format and config reference.
