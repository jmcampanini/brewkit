package cli

import (
	"github.com/jmcampanini/brewkit/internal/config"
	"github.com/jmcampanini/brewkit/internal/profile"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type globalFlags struct {
	configPath    string
	dryRun        bool
	verbose       bool
	quiet         bool
	hideUnchanged bool
}

var flags globalFlags
var configFlagSet *pflag.FlagSet

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "brewkit",
		Short:         "Multi-profile Homebrew manager",
		Long:          "brewkit manages Homebrew taps, formulas, HEAD formulas, and casks across multiple layered profiles defined in profile files.\n\nOutput modes --quiet and --verbose are mutually exclusive.",
		Version:       Version,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	persistentFlags := root.PersistentFlags()
	persistentFlags.StringVar(&flags.configPath, "config", "", "path to brewkit.toml (defaults to ./brewkit.toml if present)")
	if err := config.RegisterFlags(persistentFlags); err != nil {
		panic(err)
	}
	configFlagSet = persistentFlags
	persistentFlags.BoolVar(&flags.dryRun, "dry-run", false, "compute changes without applying them")
	persistentFlags.BoolVarP(&flags.verbose, "verbose", "v", false, "stream raw brew output for every operation (mutually exclusive with --quiet)")
	persistentFlags.BoolVarP(&flags.quiet, "quiet", "q", false, "errors-only output for operational commands (mutually exclusive with --verbose)")
	root.MarkFlagsMutuallyExclusive("quiet", "verbose")

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
