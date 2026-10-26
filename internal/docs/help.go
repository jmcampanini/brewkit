package docs

// Shared help fragments compose command long descriptions so repeated
// contract text cannot drift between commands. Fragments carry no leading or
// trailing newline; compose them with explicit separators.

// ConfigDiscoveryHelp describes how brewkit.toml and the layer directory
// are located; the root, config, and lint commands include it.
const ConfigDiscoveryHelp = `brewkit reads brewkit.toml from the current directory when it exists and
uses the built-in defaults when it does not. --config PATH reads PATH
instead and fails when PATH is empty or does not exist. Layer files are
looked up in the directory named by dir (default '.'), resolved against
the current working directory rather than the location of brewkit.toml.`

// LayerSelectionHelp describes how the layers are chosen and ordered; the
// config and apply commands include it.
const LayerSelectionHelp = `The layers come from the highest-priority input that sets them: the
flags --layers LIST (comma-separated) and --layer NAME (repeatable),
which combine into one list, replace BREWKIT_LAYERS (comma-separated),
which replaces layers in brewkit.toml. --layers and BREWKIT_LAYERS are
split on commas, --layer takes one name, and every flag and environment
item is trimmed; --layers "" or an empty BREWKIT_LAYERS clears the
list. Each environment variable named in env_layers is then read in
order; its value is split on commas, trimmed, and appended with blank
items dropped. Duplicates are dropped keeping the last occurrence, so
work,laptop,work selects laptop then work. A layer name is 1 to 64
characters of lowercase letters and digits in groups separated by
single hyphens, such as work or work-laptop; any other name, including
a blank item, is an error. The names base and local
are reserved and cannot be selected; local is the *file.local files,
which apply whenever one of them exists in dir. The apply commands walk
the selected layers in order, then local; a selection with no layers
still applies local. 'brewkit config' prints the resulting
effective_layers and layer_order.`

// BrewStateHelp describes the Homebrew state queries; the brew and cask
// commands include it.
const BrewStateHelp = `Before the first entry that needs it, brewkit reads Homebrew's state once
with 'brew list --formula --versions', 'brew list --cask
--versions', 'brew outdated --formula --json=v2', and 'brew outdated
--cask --greedy --json=v2'. A failing query is an error ('brew state:
...') for the entry that needed it.`

// ApplyOperandHelp describes the optional entry operand every apply
// command accepts.
const ApplyOperandHelp = `An optional operand limits the run to one entry: it must equal an entry's
full name or the part after its last '/', and every matching entry across
the layers' files is applied. When nothing matches, nothing is applied
and the command fails. Without an operand every entry is applied, and a
layer that has no file of this kind is reported as skipped.`

// ApplyOutputHelp describes the per-entry output, summary, and dry-run
// contract every apply command shares.
const ApplyOutputHelp = `Layers are processed in order and entries in file order. Each entry
prints one line on stdout: '+ name' for an install, tap, or trust change,
'↑ name old → new' for an upgrade, and '✓ name' when already satisfied.
A layer with no file of this kind prints '⊘ layer: no Brewfile,
skipping' (or Tapfile, Headfile, Caskfile).
A failure prints '✗ name: reason' on stderr followed by
brew's captured output, whatever the verbosity, and a line that is not a
valid entry for the file kind is a failure. A final 'Summary: ...' line
on stdout counts added, upgraded, trusted, up-to-date, skipped, and failed
entries. --hide-unchanged omits the ✓ lines; the summary still counts
them. --verbose adds brew's output, indented, under each changed entry.
--quiet prints only the ✗ lines and their brew output, so a successful
quiet run prints nothing. When neither is set and stdout and stderr are
both terminals, a progress line shows on stderr while brew runs and is
erased afterwards.

With --dry-run no brew command that changes the system runs. Lines that
would change carry a '(dry-run)' suffix, and a later layer that lists
the same entry sees it as already satisfied. The state and info queries
still run. fail_fast (default true) stops at the first failure; when
false every entry is attempted and all failures are listed at the end.
A run with any failure exits 1.`
