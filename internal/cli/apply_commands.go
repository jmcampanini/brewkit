package cli

import (
	"fmt"

	"github.com/jmcampanini/brewkit/internal/apply"
	"github.com/jmcampanini/brewkit/internal/brew"
	"github.com/jmcampanini/brewkit/internal/docs"
	"github.com/jmcampanini/brewkit/internal/profile"
	"github.com/jmcampanini/brewkit/internal/ui"
	"github.com/spf13/cobra"
)

// brewerFactory is overridden in tests to inject a Fake brewer.
var brewerFactory = func() brew.Brewer { return brew.NewExec() }

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

func newApplyCmd(app *commandContext, spec applyCommandSpec) *cobra.Command {
	return &cobra.Command{
		Use:   spec.use,
		Short: spec.short,
		Long: spec.kindHelp + `

` + docs.ApplyOperandHelp + `

` + docs.ProfileSelectionHelp + `

` + docs.ApplyOutputHelp,
		Example: spec.example,
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, profiles, err := profile.LoadConfig(app.flags.configPath, app.configFlagSet)
			if err != nil {
				return err
			}
			if len(profiles) == 0 {
				return fmt.Errorf("no active profiles selected; set profiles in brewkit.toml, BREWKIT_PROFILES, --profiles/--profile, env_profiles, or add a *file.local")
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
				Config:   cfg,
				DryRun:   app.flags.dryRun,
				Profiles: profiles,
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
