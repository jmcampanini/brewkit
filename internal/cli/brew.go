package cli

import (
	"github.com/jmcampanini/brewkit/internal/profile"
	"github.com/spf13/cobra"
)

func newBrewCmd() *cobra.Command {
	return newApplyCmd(applyCommandSpec{
		use:   "brew [FORMULA]",
		short: "Apply Brewfile entries for active profiles",
		kind:  profile.KindBrew,
		kindHelp: `Install or upgrade every formula listed as brew "name" in
Brewfile.<profile> for the active profiles. For each entry brewkit runs
'brew install --formula <name>' when it is not installed, 'brew upgrade
--formula <name>' when it is outdated, and otherwise reports ✓ with the
installed version. A name may be qualified as user/tap/name; it also
matches the installed short name. Formulas are never uninstalled.

` + brewStateHelp,
		example: `  brewkit brew
  brewkit --profiles work,personal brew --dry-run
  brewkit brew ripgrep`,
	})
}
