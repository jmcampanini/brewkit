package cli

import (
	"github.com/jmcampanini/brewkit/internal/apply"
	"github.com/jmcampanini/brewkit/internal/brew"
	"github.com/jmcampanini/brewkit/internal/config"
	"github.com/jmcampanini/brewkit/internal/docs"
	"github.com/jmcampanini/brewkit/internal/layer"
	"github.com/jmcampanini/brewkit/internal/ui"
	"github.com/spf13/cobra"
)

// brewerFactory is overridden in tests to inject a Fake brewer.
var brewerFactory = func() brew.Brewer { return brew.NewExec() }

// applyCommandSpec is the per-kind part of an apply command. newApplyCmd
// appends the operand, layer, and output contract every apply command
// shares to kindHelp.
type applyCommandSpec struct {
	example  string
	kind     layer.Kind
	kindHelp string
	short    string
	use      string
}

func newApplyCmd(app *commandContext, spec applyCommandSpec) *cobra.Command {
	return &cobra.Command{
		Use:   spec.use,
		Short: spec.short,
		Long: spec.kindHelp + `

` + docs.ApplyOperandHelp + `

` + docs.LayerSelectionHelp + `

` + docs.ApplyOutputHelp,
		Example: spec.example,
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			effective, err := config.Load(app.configFlagSet)
			if err != nil {
				return err
			}

			level := ui.LevelNormal
			switch {
			case app.flags.verbose:
				level = ui.LevelVerbose
			case app.flags.quiet:
				level = ui.LevelQuiet
			}
			return apply.Run(cmd.Context(), spec.kind, args, apply.Options{
				Brewer:   brewerFactory(),
				Dir:      effective.Dir,
				DryRun:   app.flags.dryRun,
				FailFast: effective.Config.FailFast,
				Layers:   effective.LayerOrder,
				Printer: ui.NewTerminalPrinter(ui.TerminalOptions{
					Level:         level,
					DryRun:        app.flags.dryRun,
					HideUnchanged: app.flags.hideUnchanged,
					OutputPrefix:  app.flags.outputPrefix,
				}),
			})
		},
	}
}
