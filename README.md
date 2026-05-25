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

## Quick start

Create `brewkit.toml` and one or more profile files in the same directory:

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

Run `brewkit docs` for the full file format and config reference.
