package cli

import (
	"github.com/jmcampanini/brewkit/internal/profile"
	"github.com/spf13/cobra"
)

func newBrewCmd() *cobra.Command {
	return newApplyCmd(
		"brew [FORMULA]",
		"Apply Brewfile entries for active profiles",
		profile.KindBrew,
	)
}
