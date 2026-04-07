package cli

import (
	"github.com/jmcampanini/brewkit/internal/profile"
	"github.com/spf13/cobra"
)

func newHeadCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "head [FORMULA]",
		Short: "Apply Headfile entries for active profiles (SHA-idempotent)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runApply(cmd.Context(), profile.KindHead, args)
		},
	}
}
