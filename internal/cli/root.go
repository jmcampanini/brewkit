// Package cli wires the cobra commands for the brewkit binary.
package cli

import (
	"fmt"
	"os"

	"github.com/jmcampanini/brewkit/internal/config"
	"github.com/jmcampanini/brewkit/internal/docs"
	"github.com/jmcampanini/brewkit/internal/ui"
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

type commandContext struct {
	configFlagSet *pflag.FlagSet
	flags         globalFlags
}

func newRootCmd(app *commandContext) *cobra.Command {
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

` + docs.ConfigDiscoveryHelp + `

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
		Version: Version,
		// An unknown command reports only unknown command "x" for "brewkit",
		// the same message the subcommands give, with no suggestion list.
		DisableSuggestions: true,
		SilenceUsage:       true,
		SilenceErrors:      true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	persistentFlags := root.PersistentFlags()
	persistentFlags.StringVar(&app.flags.configPath, "config", "", "config file (default ./brewkit.toml when present)")
	if err := config.RegisterFlags(persistentFlags); err != nil {
		panic(err)
	}
	app.configFlagSet = persistentFlags
	persistentFlags.BoolVar(&app.flags.dryRun, "dry-run", false, "compute changes without applying them")
	persistentFlags.BoolVarP(&app.flags.verbose, "verbose", "v", false, "add brew output and lint line text; not with -q")
	persistentFlags.BoolVarP(&app.flags.quiet, "quiet", "q", false, "errors only (apply commands, lint); not with -v")
	persistentFlags.StringVar(&app.flags.outputPrefix, "output-prefix", "", "prefix output lines (not help or version)")
	root.MarkFlagsMutuallyExclusive("quiet", "verbose")

	root.AddCommand(newTapCmd(app))
	root.AddCommand(newBrewCmd(app))
	root.AddCommand(newHeadCmd(app))
	root.AddCommand(newCaskCmd(app))
	root.AddCommand(newLintCmd(app))
	root.AddCommand(newConfigCmd(app))
	root.AddCommand(newDocsCmd(app))
	root.AddCommand(newExitCodesTopic(app))

	// Cobra registers --help and --version inside Execute, after Find has
	// already stripped flags to locate the subcommand. Without the flags
	// known up front, Find treats the value of a flag that follows --help
	// or --version as an operand, so `brewkit --help --config x` fails with
	// unknown command "x". Registering them here keeps Find's parse exact.
	root.InitDefaultHelpFlag()
	root.InitDefaultVersionFlag()

	return root
}

// Execute runs the command and returns its process exit status.
func Execute() int {
	app := &commandContext{}
	if err := newRootCmd(app).Execute(); err != nil {
		printError(err, app.flags.outputPrefix)
		return 1
	}
	return 0
}

func printError(err error, prefix string) {
	_, _ = fmt.Fprintln(ui.NewLinePrefixWriter(os.Stderr, prefix), "brewkit:", err)
}
