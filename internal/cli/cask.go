package cli

import (
	"github.com/jmcampanini/brewkit/internal/profile"
	"github.com/spf13/cobra"
)

func newCaskCmd() *cobra.Command {
	return newApplyCmd(
		"cask [CASK]",
		"Apply Caskfile entries for active profiles (greedy upgrades)",
		profile.KindCask,
	)
}
