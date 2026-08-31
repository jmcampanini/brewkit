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
		kindHelp: `Register every tap listed in Tapfile.<profile> for the active profiles. A
Tapfile line is '<tap>' or '<tap> <url>'; the URL is passed to brew for
taps outside Homebrew's default index. A tap already present in the
'brew tap' output is reported ✓; otherwise brewkit runs 'brew tap <tap>
[<url>]' and reports +. Taps are never untapped.

` + brewStateHelp,
		example: `  brewkit tap
  brewkit tap --dry-run
  brewkit tap jmcampanini/overlay`,
	})
}
