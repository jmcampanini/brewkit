package cli

import "github.com/spf13/cobra"

func newLintCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "lint",
		Short: "Validate profile files (sort order + comment style)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runLint(cmd.Context())
		},
	}
}
