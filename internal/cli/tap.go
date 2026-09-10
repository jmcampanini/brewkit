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
optional URL is used when registering a missing tap.

Before the first selected entry, brewkit reads installed taps and their
trust with 'brew tap-info --installed --json=v1'. It does not query
package inventory or available upgrades. Homebrew must report each tap's
trust status; missing or invalid trust data is an error. Update Homebrew
with 'brew update' if its JSON lacks trust information.

For missing taps, brewkit runs 'brew tap <tap> [<url>]', then
'brew trust --tap <tap>'. Installed but untrusted taps need only the
trust command. Homebrew resolves custom remotes and persists trust;
official taps are already trusted. Installed taps keep their remote.

A newly registered and trusted tap prints '+ name registered and trusted'
and counts as added. A trust-only change prints '+ name trusted' and
counts as trusted. Only an installed, trusted tap is reported ✓.
If registration succeeds but trust fails, the tap remains registered;
the entry fails and a later attempt retries trust. --dry-run previews
registration and trust without changing either. Removing an entry never
untaps it or revokes trust.`,
		example: `  brewkit tap
  brewkit tap --dry-run
  brewkit tap jmcampanini/overlay`,
	})
}
