package cli

import "github.com/spf13/cobra"

func newLayersTopic() *cobra.Command {
	return newHelpTopic("layers", "Layers, their order, and the reserved names",
		`A layer is a named set of Homebrew entries: the taps, formulas, HEAD
formulas, and casks one machine, role, or person needs. The layer named
work has up to four files in the layer directory, Tapfile.work,
Brewfile.work, Headfile.work, and Caskfile.work, and may omit any of
them. Keep the entries every machine needs in one layer and the
machine-specific or role-specific ones in others, then select on each
machine the layers it needs. 'brewkit help layer-files' describes the
files themselves.

Several layers can be selected at once. The apply commands walk the
selected layers in the order they were selected, then the automatic
local layer, and process each file's entries in file order. An entry
listed by more than one layer is applied where it first appears; a
later layer that lists it sees it as already satisfied. The selected
layers come from brewkit.toml, the environment, and flags; 'brewkit
config --help' describes those inputs, the layer name grammar, and how
duplicates are resolved.

The local layer is the *file.local files (Tapfile.local, Brewfile.local,
Headfile.local, and Caskfile.local). It applies last whenever one of
those files exists in the layer directory, so it holds the overrides of
one machine, and by convention it is not committed to version control.
The names base and local are reserved and cannot be selected. brewkit
never removes a tap, formula, or cask that a layer no longer lists.`)
}
