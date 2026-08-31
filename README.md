# brewkit

brewkit applies Homebrew taps, formulas, HEAD formulas, and casks from per-profile files. A profile is a name such as `work` or `personal`; its `Tapfile.<profile>`, `Brewfile.<profile>`, `Headfile.<profile>`, and `Caskfile.<profile>` list what that profile needs, several profiles can be active at once, and `brewkit lint` keeps the files sorted and commented consistently. brewkit never removes a tap, formula, or cask that is not listed.

Command help is the canonical reference: `brewkit --help` and each command's `--help` describe every user-facing contract, `brewkit config --help` describes configuration precedence and the `brewkit.toml` keys, and `brewkit help exit-codes` describes exit statuses. `brewkit docs` prints the longer manual.

## Install

brewkit distributes from HEAD only; there is no release channel or tagged binary.

### Homebrew

```sh
brew tap jmcampanini/brewkit https://github.com/jmcampanini/brewkit
brew install --HEAD jmcampanini/brewkit/brewkit
```

Upgrade to the latest commit:

```sh
brew upgrade --fetch-HEAD brewkit
```

### From source

```sh
git clone https://github.com/jmcampanini/brewkit
cd brewkit
make build
./build/brewkit --version
```

## Representative commands

| Command | Result |
|---|---|
| `brewkit lint` | Check every profile file's sort order and comment style; exit 1 on violations. |
| `brewkit tap` | Register the taps listed for the active profiles. |
| `brewkit brew [--dry-run]` | Install or upgrade the listed formulas; `--dry-run` prints the plan instead. |
| `brewkit head` | Install the listed HEAD formulas and rebuild those whose upstream commit moved. |
| `brewkit cask` | Install or upgrade the listed casks, including ones that auto-update. |
| `brewkit brew ripgrep` | Apply only the listed entry named `ripgrep`. |
| `brewkit config` | Print the configuration in effect with the source of each value. |

A full run is `brewkit lint`, then `brewkit tap`, `brewkit brew`, `brewkit head`, and `brewkit cask` in that order. Selecting profiles for one run looks like `brewkit --profiles work,personal brew` or `BREWKIT_PROFILES=work brewkit cask`.

## Required external programs

Homebrew's `brew` must be on `PATH` for `tap`, `brew`, `head`, and `cask`; `head` also needs `git`. `lint`, `config`, and `docs` run on their own.

## Configuration

brewkit reads `brewkit.toml` from the current directory when it exists (`--config PATH` names another file) and looks for profile files in the directory its `dir` key names, `.` by default, relative to the current directory. There is no implicit profile: set `profiles` in `brewkit.toml`, or pass `--profiles work,personal`, `--profile work`, or `BREWKIT_PROFILES=work`. A `Brewfile.local` (or any other `*file.local`) adds a `local` profile automatically for machine-specific entries. `brewkit config --help` documents the precedence and every key, and `brewkit config` prints the values in effect.

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
