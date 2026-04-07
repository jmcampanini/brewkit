package cli

import (
	"github.com/jmcampanini/brewkit/internal/profile"
	"github.com/spf13/cobra"
)

func newBrewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "brew [FORMULA]",
		Short: "Apply Brewfile entries for active profiles",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runApply(cmd.Context(), profile.KindBrew, args)
		},
	}
}
