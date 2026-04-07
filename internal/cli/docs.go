package cli

import "github.com/spf13/cobra"

func newDocsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "docs",
		Short: "Print the embedded brewkit manual as plain text",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDocs(cmd.Context())
		},
	}
}
