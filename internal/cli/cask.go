package cli

import (
	"github.com/jmcampanini/brewkit/internal/profile"
	"github.com/spf13/cobra"
)

func newCaskCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "cask [CASK]",
		Short: "Apply Caskfile entries for active profiles (greedy upgrades)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runApply(cmd.Context(), profile.KindCask, args)
		},
	}
}
