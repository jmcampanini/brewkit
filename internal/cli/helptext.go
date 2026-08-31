package cli

// Shared help fragments compose command long descriptions so repeated
// contract text cannot drift between commands. Fragments carry no leading or
// trailing newline; compose them with explicit separators.

const configDiscoveryHelp = `brewkit reads brewkit.toml from the current directory when it exists and
uses the built-in defaults when it does not. --config PATH reads PATH
instead and fails when PATH does not exist. Profile files are looked up
in the directory named by dir (default '.'), resolved against the current
working directory rather than the location of brewkit.toml.`

const profileSelectionHelp = `The active profiles come from the highest layer that sets them: the flags
--profiles LIST (comma-separated) and --profile NAME (repeatable), which
combine into one list, override BREWKIT_PROFILES (comma-separated), which
overrides profiles in brewkit.toml. When env_profiles names an environment
variable that is set, its comma-separated value is appended. Environment
and flag values are split on commas and trimmed. Duplicates are dropped
keeping the first occurrence, and an empty name is an error except in
the env_profiles variable, where blanks are dropped. The name local is
reserved: listing it is an error, and it is appended last whenever a
Tapfile.local, Brewfile.local, Headfile.local, or Caskfile.local exists
in dir. 'brewkit config' prints the resulting effective_profiles.`

const brewStateHelp = `Before the first entry that needs it, brewkit reads Homebrew's state once
with 'brew tap', 'brew list --formula --versions', 'brew list --cask
--versions', 'brew outdated --formula --json=v2', and 'brew outdated
--cask --greedy --json=v2'. A failing query is an error ('brew state:
...') for the entry that needed it.`

const applyOperandHelp = `An optional operand limits the run to one entry: it must equal an entry's
full name or the part after its last '/', and every matching entry across
the active profiles' files is applied. When nothing matches, nothing is
applied and the command fails. Without an operand every entry is applied,
and a profile that has no file of this kind is reported as skipped.`

const applyOutputHelp = `Profiles are processed in order and entries in file order. Each entry
prints one line on stdout: '+ name' for an install or tap, '↑ name old →
new' for an upgrade, '✓ name' when already satisfied, and '⊘ profile: no
Brewfile, skipping' (or Tapfile, Headfile, Caskfile) when a profile has no
file of this kind. A failure prints '✗ name: reason' on stderr followed by
brew's captured output, whatever the verbosity, and a line that is not a
valid entry for the file kind is a failure. A final 'Summary: ...' line
on stdout counts added, upgraded, up-to-date, skipped, and failed
entries. --hide-unchanged omits the ✓ lines; the summary still counts
them. --verbose adds brew's output, indented, under each changed entry.
--quiet prints only the ✗ lines and their brew output, so a successful
quiet run prints nothing. When neither is set and stdout and stderr are
both terminals, a progress line shows on stderr while brew runs and is
erased afterwards.

With --dry-run no brew command that changes the system runs. Lines that
would change carry a '(dry-run)' suffix, and a later profile that lists
the same entry sees it as already satisfied. The state and info queries
still run. fail_fast (default true) stops at the first failure; when
false every entry is attempted and all failures are listed at the end.
A run with any failure exits 1.`
