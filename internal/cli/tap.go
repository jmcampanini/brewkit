package cli

import (
	"github.com/jmcampanini/brewkit/internal/profile"
	"github.com/spf13/cobra"
)

func newTapCmd() *cobra.Command {
	return newApplyCmd(applyCommandSpec{
		use:   "tap [TAP]",
		short: "Apply Tapfile entries for active profiles",
		kind:  profile.KindTap,
		kindHelp: `Register and trust every tap listed in Tapfile.<profile> for the active
profiles. Listing a tap authorizes trust for the whole tap, including its
formulae, casks, and commands. A line is '<tap>' or '<tap> <url>'; the
optional URL is used when trusting and registering a missing tap.

Before the first selected entry, brewkit reads installed taps and their
trust with 'brew tap-info --installed --json=v1'. It does not query
package inventory or available upgrades. The query sets
HOMEBREW_NO_GITHUB_API=1 to skip GitHub metadata requests; Homebrew may
still need its formula-index cache. Homebrew must report each tap's
trust status; missing or invalid trust data is an error. Update Homebrew
with 'brew update' if its JSON lacks trust information.

For missing taps, brewkit first runs 'brew trust --tap <url>' when a URL
is supplied, or 'brew trust --tap <tap>' otherwise. It then runs
'brew tap <tap> [<url>]'. Homebrew requires trust before it can verify
the tap's formulas and casks during registration. Installed but untrusted
taps need only 'brew trust --tap <tap>'; Homebrew resolves their existing
remote and persists trust. Official taps are already trusted.

A newly registered and trusted tap prints '+ name registered and trusted'
and counts as added. A trust-only change prints '+ name trusted' and
counts as trusted. Only an installed, trusted tap is reported ✓.
If trust fails, registration is not attempted. If trust succeeds but
registration fails, trust is retained and the entry fails; a later run
can retry registration. --dry-run previews trust and registration without
changing either. Removing an entry never untaps it or revokes trust.`,
		example: `  brewkit tap
  brewkit tap --dry-run
  brewkit tap jmcampanini/overlay`,
	})
}
