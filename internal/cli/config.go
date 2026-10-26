package cli

import "github.com/spf13/cobra"

func newConfigCmd(app *commandContext) *cobra.Command {
	var provenance bool
	cmd := &cobra.Command{
		GroupID: commandGroupID,
		Use:     "config",
		Short:   "Print loaded config and effective values",
		Long: `Print the configuration in effect as TOML on stdout, followed by a
commented effective-value section and, with --provenance, a commented
provenance section. Nothing is modified and no brew command runs.

Settings load in this order, and a later input replaces a value an
earlier one sets: built-in defaults (dir = ".", layers = [],
env_layers = [], fail_fast = true), brewkit.toml, the environment
variable BREWKIT_LAYERS, then the flags --layers and --layer, which
every command accepts. Only layers has an environment variable and
flags; dir, env_layers, and fail_fast come from the file alone.

` + configDiscoveryHelp + `

Keys: dir (the directory holding the layer files, relative to the
current working directory; must not be empty), layers (the layer names
to select; each must satisfy the layer name grammar below), env_layers
(the names of environment variables whose comma-separated values
append to layers; each entry must be a variable name without
surrounding whitespace, so a blank or padded entry is an error), and
fail_fast (stop at the first failing entry when true; attempt every
entry and report all failures at the end when false). Any other key is
an error.

` + layerSelectionHelp + `

The TOML section reloads as brewkit.toml. The '# effective:' block gives
effective_dir (dir made absolute), effective_layers (the selected layers
after env_layers additions and deduplication, never base or local), and
layer_order (the list the apply commands walk: the effective layers,
then local when a *file.local exists). With --provenance the
'# provenance:' block lists each field as a commented, tab-separated
path, value, and source, where source is <default>, the path of
brewkit.toml, <env>, or <pflag>, and '# loaded_files = [...]' names the
files read. Every line after the TOML is a comment, so the whole report
reloads as brewkit.toml. Nothing is redacted; the configuration holds
no secret fields. --quiet does not suppress the report and --dry-run
has no effect.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return app.runConfig(cmd.Context(), provenance)
		},
	}
	cmd.Flags().BoolVar(&provenance, "provenance", false, "append the source of every field")
	return cmd
}
