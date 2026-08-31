package cli

import (
	"github.com/jmcampanini/brewkit/internal/profile"
	"github.com/spf13/cobra"
)

func newHeadCmd() *cobra.Command {
	return newApplyCmd(applyCommandSpec{
		use:   "head [FORMULA]",
		short: "Apply Headfile entries for active profiles (SHA-idempotent)",
		kind:  profile.KindHead,
		kindHelp: `Keep every formula listed in Headfile.<profile> installed from its HEAD
source. For each entry brewkit runs 'brew info --json=v2 --formula <name>'
and then: a formula installed but not as HEAD, or without a HEAD source,
fails without being changed (✗); one that is not installed is installed
with 'brew install --head --formula <name>' (+); one installed as HEAD is
refreshed with 'brew fetch --HEAD --formula <name>', the newest commit of
the cached source repository is read with git, and when it differs from
the installed HEAD-<sha> brewkit runs 'brew uninstall --formula <name>'
then 'brew install --head --formula <name>' (↑), otherwise it reports ✓.
Install, uninstall, and fetch run with HOMEBREW_NO_GITHUB_API=1 and
HOMEBREW_NO_AUTO_UPDATE=1. If the uninstall succeeds and the reinstall
fails, the formula is left uninstalled; the error says so and the next
run installs it again. A formula listed by more than one active profile
is processed once. --dry-run runs every check, including brew fetch
--HEAD, which updates Homebrew's source cache, but skips uninstall and
install. This command does not run the brew tap, list, and outdated
queries the other apply commands use.`,
		example: `  brewkit head
  brewkit head --dry-run
  brewkit head direnv`,
	})
}
