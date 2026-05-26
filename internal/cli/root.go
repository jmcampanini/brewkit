package cli

import (
	"github.com/jmcampanini/brewkit/internal/profile"
	"github.com/spf13/cobra"
)

type globalFlags struct {
	configPath    string
	profiles      []string
	dryRun        bool
	verbose       bool
	quiet         bool
	hideUnchanged bool
}

var flags globalFlags

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "brewkit",
		Short:         "Multi-profile Homebrew manager",
		Long:          "brewkit manages Homebrew taps, formulas, HEAD formulas, and casks across multiple layered profiles defined in profile files.",
		Version:       Version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.PersistentFlags().StringVar(&flags.configPath, "config", "", "path to brewkit.toml (defaults to ./brewkit.toml if present)")
	root.PersistentFlags().StringSliceVar(&flags.profiles, "profile", nil, "active profile (repeatable; replaces config/env)")
	root.PersistentFlags().BoolVar(&flags.dryRun, "dry-run", false, "compute changes without applying them")
	root.PersistentFlags().BoolVarP(&flags.verbose, "verbose", "v", false, "stream raw brew output for every operation")
	root.PersistentFlags().BoolVarP(&flags.quiet, "quiet", "q", false, "suppress per-item output; show only errors and final summary")

	root.AddCommand(newTapCmd())
	root.AddCommand(newBrewCmd())
	root.AddCommand(newHeadCmd())
	root.AddCommand(newCaskCmd())
	root.AddCommand(newLintCmd())
	root.AddCommand(newConfigCmd())
	root.AddCommand(newDocsCmd())

	return root
}

func newApplyCmd(use, short string, kind profile.Kind) *cobra.Command {
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runApply(cmd.Context(), kind, args)
		},
	}
	cmd.Flags().BoolVar(&flags.hideUnchanged, "hide-unchanged", false, "suppress per-item output for entries that are already satisfied")
	return cmd
}

// Execute is the entrypoint invoked from cmd/brewkit/main.go.
func Execute() error {
	return newRootCmd().Execute()
}
