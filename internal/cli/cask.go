package cli

import (
	"github.com/jmcampanini/brewkit/internal/layer"
	"github.com/spf13/cobra"
)

func newCaskCmd(app *commandContext) *cobra.Command {
	cmd := newApplyCmd(app, applyCommandSpec{
		use:   "cask [CASK]",
		short: "Apply Caskfile entries for the layers in order (greedy upgrades)",
		kind:  layer.KindCask,
		kindHelp: `Install or upgrade every cask listed in the Caskfile of each layer in
order. For each entry brewkit runs 'brew install --cask <name>' when it
is not installed, 'brew upgrade --cask --greedy <name>' when it is
outdated, and otherwise reports ✓ with the installed version. --greedy,
also used for the outdated query, includes casks marked auto_updates or
version :latest, which brew would otherwise never report as outdated.
After the run, upgraded casks are listed on stdout under 'Restart these
apps to apply upgrades' unless --quiet is set. Casks are never
uninstalled.

` + brewStateHelp,
		example: `  brewkit cask
  brewkit cask --dry-run
  brewkit cask ghostty`,
	})
	cmd.Flags().BoolVar(&app.flags.hideUnchanged, "hide-unchanged", false, "hide lines for entries already satisfied")
	return cmd
}
