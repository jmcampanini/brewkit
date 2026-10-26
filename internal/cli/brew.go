package cli

import (
	"github.com/jmcampanini/brewkit/internal/layer"
	"github.com/spf13/cobra"
)

func newBrewCmd(app *commandContext) *cobra.Command {
	cmd := newApplyCmd(app, applyCommandSpec{
		use:   "brew [FORMULA]",
		short: "Apply Brewfile entries for the layers in order",
		kind:  layer.KindBrew,
		kindHelp: `Install or upgrade every formula listed as brew "name" in the
Brewfile of each layer in order. For each entry brewkit runs
'brew install --formula <name>' when it is not installed, 'brew upgrade
--formula <name>' when it is outdated, and otherwise reports ✓ with the
installed version. A name may be qualified as user/tap/name; it also
matches the installed short name. Formulas are never uninstalled.

` + brewStateHelp,
		example: `  brewkit brew
  brewkit brew --dry-run
  brewkit brew ripgrep`,
	})
	cmd.Flags().BoolVar(&app.flags.hideUnchanged, "hide-unchanged", false, "hide lines for entries already satisfied")
	return cmd
}
