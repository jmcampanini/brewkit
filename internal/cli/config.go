package cli

import "github.com/spf13/cobra"

func newConfigCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "config",
		Short: "Print loaded config, provenance, and effective runtime values",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runConfig(cmd.Context())
		},
	}
}
