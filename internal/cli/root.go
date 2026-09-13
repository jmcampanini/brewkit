// Package cli wires the cobra commands for the brewkit binary.
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
	outputPrefix  string
	hideUnchanged bool
}

var flags globalFlags
var configFlagSet *pflag.FlagSet

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "brewkit",
		Short: "Multi-profile Homebrew manager",
		Long: `Apply Homebrew taps, formulas, HEAD formulas, and casks from per-profile
files.

A profile is a name such as work or personal, and each profile has up to
four files in the configured directory: Tapfile.<profile> (one tap per
line with an optional URL), Brewfile.<profile> (brew "name" lines),
Headfile.<profile> (one HEAD-tracked formula per line), and
Caskfile.<profile> (one cask per line). Every file accepts '# comment'
lines and trailing '  # description' comments. 'brewkit tap', 'brewkit
brew', 'brewkit head', and 'brewkit cask' each apply one file kind for
every active profile in order, and a full run is those four commands in
that order. 'brewkit lint' checks the files' style, 'brewkit config'
prints the configuration in effect, and 'brewkit docs' prints the
embedded manual.

` + configDiscoveryHelp + `

The global flags apply to every command. --dry-run makes the apply
commands report what they would change without running any brew command
that changes the system. --verbose (-v) adds brew's output under each
changed entry and the offending line under each lint violation; --quiet
(-q) limits the apply commands and lint to errors; the two cannot be
combined. --output-prefix TEXT prefixes every line brewkit writes,
including errors and progress, except help and version output.
--config, --profiles, and --profile are described under 'brewkit config
--help'.

Command payloads go to stdout: per-entry results and the summary from the
apply commands, lint findings, the config report, and the manual. Errors
and transient progress go to stderr, and a failure ends with 'brewkit:
<message>' on stderr. Color is decided per stream: a stream is colored
only when it is a terminal, a nonempty NO_COLOR disables color on both,
and the light or dark palette follows COLORFGBG when set, otherwise a
terminal background query when stdin and the stream are terminals and
--quiet is not set, otherwise the dark palette. brewkit never prompts.
It runs Homebrew's brew from PATH with stdin disconnected and output
captured until each command ends, so brew cannot read an answer from
brewkit's stdin and any prompt text appears only in the captured output;
'brewkit head' also runs git. No other program is run.

Run 'brewkit config --help' for configuration precedence and the
brewkit.toml keys, 'brewkit help exit-codes' for exit-status meanings,
and 'brewkit docs' for the file-format and lint-rule manual.`,
		Example: `  brewkit tap && brewkit brew && brewkit head && brewkit cask
  brewkit --profiles work,personal brew --dry-run
  BREWKIT_PROFILES=work brewkit cask ghostty`,
		Version:       Version,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	persistentFlags := root.PersistentFlags()
	persistentFlags.StringVar(&flags.configPath, "config", "", "config file (default ./brewkit.toml when present)")
	if err := config.RegisterFlags(persistentFlags); err != nil {
		panic(err)
	}
	configFlagSet = persistentFlags
	persistentFlags.BoolVar(&flags.dryRun, "dry-run", false, "compute changes without applying them")
	persistentFlags.BoolVarP(&flags.verbose, "verbose", "v", false, "add brew output and lint line text; not with -q")
	persistentFlags.BoolVarP(&flags.quiet, "quiet", "q", false, "errors only (apply commands, lint); not with -v")
	persistentFlags.StringVar(&flags.outputPrefix, "output-prefix", "", "prefix output lines (not help or version)")
	root.MarkFlagsMutuallyExclusive("quiet", "verbose")

	root.AddCommand(newTapCmd())
	root.AddCommand(newBrewCmd())
	root.AddCommand(newHeadCmd())
	root.AddCommand(newCaskCmd())
	root.AddCommand(newLintCmd())
	root.AddCommand(newConfigCmd())
	root.AddCommand(newDocsCmd())
	root.AddCommand(newExitCodesTopic())

	return root
}

// applyCommandSpec is the per-kind part of an apply command. newApplyCmd
// appends the operand, profile, and output contract every apply command
// shares to kindHelp.
type applyCommandSpec struct {
	example  string
	kind     profile.Kind
	kindHelp string
	short    string
	use      string
}

func newApplyCmd(spec applyCommandSpec) *cobra.Command {
	cmd := &cobra.Command{
		Use:   spec.use,
		Short: spec.short,
		Long: spec.kindHelp + `

` + applyOperandHelp + `

` + profileSelectionHelp + `

` + applyOutputHelp,
		Example: spec.example,
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runApply(cmd.Context(), spec.kind, args)
		},
	}
	cmd.Flags().BoolVar(&flags.hideUnchanged, "hide-unchanged", false, "hide lines for entries already satisfied")
	return cmd
}

// Execute is the entrypoint invoked from cmd/brewkit/main.go.
func Execute() error {
	return newRootCmd().Execute()
}
