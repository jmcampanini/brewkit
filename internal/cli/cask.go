package cli

import (
	"github.com/jmcampanini/brewkit/internal/profile"
	"github.com/spf13/cobra"
)

func newCaskCmd() *cobra.Command {
	return newApplyCmd(applyCommandSpec{
		use:   "cask [CASK]",
		short: "Apply Caskfile entries for active profiles (greedy upgrades)",
		kind:  profile.KindCask,
		kindHelp: `Install or upgrade every cask listed in Caskfile.<profile> for the active
profiles. For each entry brewkit runs 'brew install --cask <name>' when it
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
}
