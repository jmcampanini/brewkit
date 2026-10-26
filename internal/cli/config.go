package cli

import (
	"github.com/jmcampanini/brewkit/internal/docs"
	"github.com/spf13/cobra"
)

func newConfigCmd(app *commandContext) *cobra.Command {
	var provenance bool
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Print loaded config and effective values",
		Long: `Print the configuration in effect as TOML on stdout, followed by a
commented effective-value section and, with --provenance, a commented
provenance section. Nothing is modified and no brew command runs.

Settings load in this order, and a later input replaces a value an
earlier one sets: built-in defaults (dir = ".", layers = [],
env_layers = [], fail_fast = true), brewkit.toml, the environment
variable BREWKIT_LAYERS, then the flags --layers and --layer, which
every command accepts. Only layers has an environment variable and
flags; dir, env_layers, and fail_fast come from the file alone. An
unknown key in brewkit.toml is an error, and dir must not be empty.

` + docs.ConfigDiscoveryHelp + `

Keys: dir (directory holding the layer files), layers (list of layer
names), env_layers (list of environment variable names whose
comma-separated values append to layers), and fail_fast (stop at the
first failing entry when true; attempt every entry and report all
failures at the end when false).

` + docs.LayerSelectionHelp + `

The TOML section reloads as brewkit.toml. The '# effective:' block gives
effective_dir (dir made absolute), effective_layers (the selected layers
after env_layers additions and deduplication, without base and local),
and layer_order (the list the apply commands walk: base when an
unsuffixed file exists, the effective layers, then local when a
*file.local exists). With --provenance the '# provenance:' block lists
each field as a commented, tab-separated path, value, and source, where
source is <default>, the path of brewkit.toml, <env>, or <pflag>, and
'# loaded_files = [...]' names the files read. Every line after the TOML
is a comment, so the whole report reloads as brewkit.toml. Nothing is
redacted; the configuration holds no secret fields. --quiet does not
suppress the report and --dry-run has no effect.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return app.runConfig(cmd.Context(), provenance)
		},
	}
	cmd.Flags().BoolVar(&provenance, "provenance", false, "append the source of every field")
	return cmd
}
