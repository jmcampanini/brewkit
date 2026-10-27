// Package cli wires the cobra commands for the brewkit binary.
package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/jmcampanini/brewkit/internal/config"
	"github.com/jmcampanini/brewkit/internal/ui"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// Process exit statuses; the exit-codes topic documents them.
const (
	exitSuccess = 0
	exitFailure = 1
	exitUsage   = 2
)

type globalFlags struct {
	color         ui.ColorMode
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
		Short: "Layered Homebrew manager",
		Long: `Apply Homebrew taps, formulas, HEAD formulas, and casks from per-layer
files.

A layer is a name such as work or personal, and each layer has up to
four files in the configured directory: Tapfile.<layer>,
Brewfile.<layer>, Headfile.<layer>, and Caskfile.<layer>. The
*file.local files belong to the automatic local layer. The apply
commands each apply one file kind for every selected layer in order,
then local, and never remove a tap, formula, or cask that is not
listed. 'brewkit help layers' explains the layer scheme and 'brewkit
help layer-files' the file formats and editing conventions.

` + configDiscoveryHelp + `

The global flags apply to every command. --dry-run makes the apply
commands report what they would change without running any brew command
that changes the system. --verbose (-v) adds brew's output under each
changed entry and the offending line under each lint violation; --quiet
(-q) limits the apply commands and lint to errors; the two cannot be
combined. --output-prefix TEXT prefixes every line brewkit writes,
including errors and progress, except help and version output and an
invocation rejected before the flag is parsed.
--color MODE decides whether the output brewkit itself renders is
colored: auto (the default) colors a stream only when it is a terminal,
TERM is not dumb, and NO_COLOR is empty; always colors every stream
even when it is not a terminal or NO_COLOR is set; never disables
color. The mode governs only the styling brewkit adds; brew's captured
text is never altered, and the config report and lint findings never
carry color. --config, --layers, and --layer are described under
'brewkit config --help'.

Command payloads go to stdout: per-entry results and the summary from
the apply commands, lint findings, and the config report. Errors,
warnings, brew's output under --verbose, and transient progress go to
stderr, and a failure ends with 'brewkit: <message>' on stderr. Color is
decided per stream, and the light or dark palette follows COLORFGBG when
set, otherwise one terminal background query when a colored stream and
stdin are terminals, TERM is set and not dumb, and --quiet is not set,
otherwise the dark palette. The progress line needs stdout and stderr to
be terminals and a TERM that is set and not dumb; without them the apply
commands print only their durable lines.
brewkit never prompts. It runs Homebrew's brew from PATH with stdin
disconnected and output captured until each command ends, so brew
cannot read an answer from brewkit's stdin and any prompt text appears
only in the captured output; 'brewkit head' also runs git. No other
program is run.

Run 'brewkit config --help' for configuration precedence and the
brewkit.toml keys and 'brewkit help exit-codes' for the exit statuses.

Common tasks:
  Lint the files:    brewkit lint
  Apply all files:   brewkit tap && brewkit brew && brewkit head && brewkit cask
  Preview changes:   brewkit --layers work,personal brew --dry-run
  Apply one entry:   BREWKIT_LAYERS=work brewkit cask ghostty
  Print the config:  brewkit config`,
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
	if err := config.RegisterFlags(persistentFlags); err != nil {
		panic(err)
	}
	app.configFlagSet = persistentFlags
	persistentFlags.Var(&app.flags.color, "color", "color output: auto|always|never")
	persistentFlags.BoolVar(&app.flags.dryRun, "dry-run", false, "compute changes without applying them")
	persistentFlags.BoolVarP(&app.flags.verbose, "verbose", "v", false, "add brew output and lint line text; not with -q")
	persistentFlags.BoolVarP(&app.flags.quiet, "quiet", "q", false, "errors only (apply commands, lint); not with -v")
	persistentFlags.StringVar(&app.flags.outputPrefix, "output-prefix", "", "prefix output lines (not help or version)")
	root.MarkFlagsMutuallyExclusive("quiet", "verbose")

	root.AddGroup(
		&cobra.Group{ID: commandGroupID, Title: "Commands:"},
		&cobra.Group{ID: helpTopicGroupID, Title: "Help topics:"},
	)
	root.AddCommand(newTapCmd(app))
	root.AddCommand(newBrewCmd(app))
	root.AddCommand(newHeadCmd(app))
	root.AddCommand(newCaskCmd(app))
	root.AddCommand(newLintCmd(app))
	root.AddCommand(newConfigCmd(app))
	root.AddCommand(newLayersTopic())
	root.AddCommand(newLayerFilesTopic())
	root.AddCommand(newExitCodesTopic())

	// Cobra would add its help and completion commands inside Execute,
	// where 'help <unknown>' prints usage and 'completion <unknown>' prints
	// help, both exiting 0. Registering them here lets each reject an
	// unknown operand before any runner starts, so it exits 2 like every
	// other usage error. The completion command needs a runner because
	// Cobra prints help for a non-runnable command before validating its
	// operands.
	root.InitDefaultHelpCmd()
	root.InitDefaultCompletionCmd()
	for _, command := range root.Commands() {
		switch command.Name() {
		case "help":
			command.Args = func(cmd *cobra.Command, args []string) error {
				target, rest, err := cmd.Root().Find(args)
				if err != nil {
					return err
				}
				if len(rest) > 0 {
					return fmt.Errorf("unknown command %q for %q", rest[0], target.CommandPath())
				}
				return nil
			}
		case "completion":
			command.RunE = func(cmd *cobra.Command, _ []string) error {
				return cmd.Help()
			}
		}
	}

	// Cobra registers --help and --version inside Execute, after Find has
	// already stripped flags to locate the subcommand. Without the flags
	// known up front, Find treats the value of a flag that follows --help
	// or --version as an operand, so `brewkit --help --config x` fails with
	// unknown command "x". Registering them here keeps Find's parse exact.
	root.InitDefaultHelpFlag()
	root.InitDefaultVersionFlag()

	return root
}

// commandGroupID is the cobra group of the commands that do work; the root
// help lists them before the help topics.
const commandGroupID = "commands"

// Execute runs the command and returns its process exit status: 0 on
// success, 2 when the invocation was rejected before any command ran, and
// 1 for every other failure.
func Execute() int {
	app := &commandContext{}
	root := newRootCmd(app)
	started := false
	markCommandStart(root, &started)
	if err := root.Execute(); err != nil {
		printError(err, app.flags.outputPrefix)
		if !started {
			return exitUsage
		}
		return exitFailure
	}
	return exitSuccess
}

// markCommandStart wraps every application-owned runner in the tree so
// started records that cobra accepted the invocation. Cobra validates the
// flag groups after the pre-run hooks, so entering a runner is the only
// point that proves command lookup, flag parsing, the positional grammar,
// and the flag groups all passed; an error before it is a usage error.
func markCommandStart(cmd *cobra.Command, started *bool) {
	if run := cmd.RunE; run != nil {
		cmd.RunE = func(cmd *cobra.Command, args []string) error {
			*started = true
			return run(cmd, args)
		}
	}
	for _, child := range cmd.Commands() {
		markCommandStart(child, started)
	}
}

// printError writes the failure as one 'brewkit: <message>' line. An error
// that joins several failures renders them on separate lines, so the
// newlines are folded into one line.
func printError(err error, prefix string) {
	message := strings.NewReplacer("\r\n", "; ", "\n", "; ").Replace(err.Error())
	_, _ = fmt.Fprintln(ui.NewLinePrefixWriter(os.Stderr, prefix), "brewkit:", message)
}
