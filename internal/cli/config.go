package cli

import "github.com/spf13/cobra"

func newConfigCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "config",
		Short: "Print the effective brewkit configuration as TOML",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConfig(cmd.Context())
		},
	}
}
