# brewkit

brewkit applies Homebrew taps, formulas, HEAD formulas, and casks from per-layer files. A layer is a name such as `work` or `personal`; its `Tapfile.<layer>`, `Brewfile.<layer>`, `Headfile.<layer>`, and `Caskfile.<layer>` list what that layer needs, several layers can be selected at once, and `brewkit lint` keeps the files sorted and commented consistently. The `*file.local` files are the automatic `local` layer, which applies whenever one of them exists. brewkit never removes a tap, formula, or cask that is not listed.


## Install

brewkit distributes from HEAD only; there is no release channel or tagged binary.


### Homebrew

```sh
brew trust --tap https://github.com/jmcampanini/brewkit
brew tap jmcampanini/brewkit https://github.com/jmcampanini/brewkit
brew install --HEAD jmcampanini/brewkit/brewkit
```

Upgrade to the latest commit:

```sh
brew upgrade --fetch-HEAD brewkit
```


### From source

```sh
brew install git mise
git clone https://github.com/jmcampanini/brewkit
cd brewkit
mise trust
mise install
mise run build
# then copy ./build/brewkit to a directory on your PATH
```


## Representative commands

| Command | Result |
|---|---|
| `brewkit lint` | Check every layer file's sort order and comment style; exit 1 on violations. |
| `brewkit tap` | Register and trust the taps listed by the layers in order. |
| `brewkit brew [--dry-run]` | Install or upgrade the listed formulas; `--dry-run` prints the plan instead. |
| `brewkit head` | Install the listed HEAD formulas and rebuild those whose upstream commit moved. |
| `brewkit cask` | Install or upgrade the listed casks, including ones that auto-update. |
| `brewkit brew ripgrep` | Apply only the listed entry named `ripgrep`. |
| `brewkit config` | Print the configuration in effect and the derived layer order; `--provenance` adds the source of each value. |

`brewkit --help`, each command's `--help`, and the help topics it lists (`brewkit help layers`, `brewkit help layer-files`, and `brewkit help exit-codes`) are the canonical reference.

A full run is `brewkit tap`, `brewkit brew`, `brewkit head`, and `brewkit cask` in that order, usually after `brewkit lint` has checked the files.


## Required external programs

- `brew`: `tap`, `brew`, `head`, and `cask` run Homebrew's `brew` from `PATH`, and `tap` needs a Homebrew with `brew trust --tap`.
- `git`: `head` runs it against the cached HEAD source to find each formula's latest upstream commit.


## Configuration

brewkit reads `brewkit.toml` from the current directory when it exists and looks for layer files in the directory its `dir` key names, `.` by default, relative to the current directory. `brewkit config --help` documents layer selection, the precedence between flags, environment, and file, and every key.

```toml
# brewkit.toml
layers = ["common"]
```
