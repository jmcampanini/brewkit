package cli

import (
	"github.com/jmcampanini/brewkit/internal/profile"
	"github.com/spf13/cobra"
)

func newTapCmd() *cobra.Command {
	return newApplyCmd(
		"tap [TAP]",
		"Apply Tapfile entries for active profiles",
		profile.KindTap,
	)
}
