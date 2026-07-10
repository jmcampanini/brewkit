package cli

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/jmcampanini/brewkit/internal/brew"
	"github.com/jmcampanini/brewkit/internal/config"
	"github.com/jmcampanini/brewkit/internal/parse"
	"github.com/jmcampanini/brewkit/internal/profile"
	"github.com/jmcampanini/brewkit/internal/ui"
)

// brewerFactory is overridden in tests to inject a Fake brewer.
var brewerFactory = func() brew.Brewer { return brew.NewExec() }

func loadEffectiveConfig() (config.Config, config.LoadReport, []string, error) {
	cfg, report, err := config.LoadWithReport(flags.configPath, configFlagSet)
	if err != nil {
		return config.Config{}, config.LoadReport{}, nil, err
	}
	resolved, err := profile.Resolve(cfg)
	if err != nil {
		return config.Config{}, config.LoadReport{}, nil, err
	}
	return cfg, report, resolved, nil
}

func newPrinter() *ui.Printer {
	level := ui.LevelNormal
	switch {
	case flags.verbose:
		level = ui.LevelVerbose
	case flags.quiet:
		level = ui.LevelQuiet
	}
	stdoutTTY := isTerminal(os.Stdout)
	stderrTTY := isTerminal(os.Stderr)
	stdoutProfile := terminalColorProfile(os.Stdout)
	stderrProfile := terminalColorProfile(os.Stderr)
	themeOutput := os.Stdout
	themeProfile := stdoutProfile
	if (!stdoutTTY || !terminalColorsEnabled(stdoutProfile)) && stderrTTY && terminalColorsEnabled(stderrProfile) {
		themeOutput = os.Stderr
		themeProfile = stderrProfile
	}
	allowThemeQuery := level != ui.LevelQuiet
	theme := ui.ThemeForBackground(terminalHasDarkBackground(themeOutput, themeProfile, allowThemeQuery))
	spinner := level == ui.LevelNormal && stdoutTTY && stderrTTY
	spinnerWidth := 0
	if spinner {
		spinnerWidth = terminalWidth(os.Stderr)
		if spinnerWidth <= 1 {
			spinner = false
		}
	}
	return ui.New(os.Stdout, os.Stderr, ui.PrinterOptions{
		Level:         level,
		OutputProfile: stdoutProfile,
		ErrorProfile:  stderrProfile,
		Theme:         theme,
		DryRun:        flags.dryRun,
		HideUnchanged: flags.hideUnchanged,
		OutputPrefix:  flags.outputPrefix,
		Spinner:       spinner,
		SpinnerWidth:  spinnerWidth,
	})
}

// runApply is the shared implementation for `brewkit tap|brew|head|cask`.
func runApply(ctx context.Context, t profile.Kind, args []string) error {
	cfg, _, profiles, err := loadEffectiveConfig()
	if err != nil {
		return err
	}
	if len(profiles) == 0 {
		return fmt.Errorf("no active profiles selected; set profiles in brewkit.toml, BREWKIT_PROFILES, --profiles/--profile, env_profiles, or add a *file.local")
	}

	printer := newPrinter()
	rc := &runContext{
		ctx:      ctx,
		brewer:   newProgressBrewer(brewerFactory(), printer),
		printer:  printer,
		dryRun:   flags.dryRun,
		failFast: cfg.FailFast,
	}
	defer rc.printer.Footer()

	var filter string
	if len(args) == 1 {
		filter = args[0]
	}

	var (
		matched  bool
		failures []error
	)
	for _, prof := range profiles {
		path := profile.PathFor(cfg.Dir, t, prof)
		_, statErr := os.Stat(path)
		if errors.Is(statErr, fs.ErrNotExist) {
			if filter == "" {
				rc.printer.Notice(fmt.Sprintf("%s: no %s, skipping", prof, t.FilenamePrefix()))
			}
			continue
		}
		if statErr != nil {
			return fmt.Errorf("stat %s: %w", path, statErr)
		}
		f, err := parse.Parse(path, t)
		if err != nil {
			return err
		}
		// Surface LineUnknown lines as failures rather than silently
		// skipping them — `f.Entries()` filters to LineEntry only, so a
		// malformed line would otherwise produce a clean exit and a
		// partially applied profile.
		for _, l := range f.Lines {
			if l.Kind != parse.LineUnknown {
				continue
			}
			err := fmt.Errorf("%s:%d: invalid %s entry: %s", path, l.Number, t, l.Raw)
			rc.printer.Error(path, fmt.Sprintf("line %d: invalid entry", l.Number), l.Raw)
			if rc.failFast {
				return err
			}
			failures = append(failures, err)
		}
		for _, e := range f.Entries() {
			if filter != "" && !entryMatches(e, filter) {
				continue
			}
			if filter != "" {
				matched = true
			}
			if err := rc.apply(t, e); err != nil {
				if rc.failFast {
					return err
				}
				failures = append(failures, err)
			}
		}
	}

	if filter != "" && !matched {
		return fmt.Errorf("%q not found in any active profile %s", filter, t)
	}

	if t == profile.KindCask {
		rc.printer.RestartAppsNotice(rc.restartApps)
	}

	if len(failures) > 0 {
		// errors.Join preserves every failure for errors.Is/As walking
		// via Go 1.20+'s Unwrap() []error, and prints them
		// newline-separated after the "N op(s) failed:" prefix.
		return fmt.Errorf("%d %s operation(s) failed: %w",
			len(failures), t, errors.Join(failures...))
	}
	return nil
}

func entryMatches(e *parse.Entry, filter string) bool {
	return e.Name == filter || shortName(e.Name) == filter
}

// shortName returns the substring after the final '/' in name, or name
// itself if no '/' is present. Qualified Brewfile entries like
// "user/tap/freeze" must match the short name "freeze" that brew uses
// in its state output.
func shortName(name string) string {
	if i := strings.LastIndex(name, "/"); i >= 0 {
		return name[i+1:]
	}
	return name
}

// formulaState looks up the FormulaState for an entry, falling back to
// the short name when the entry uses a fully qualified `user/tap/name`.
func (rc *runContext) formulaState(name string) brew.FormulaState {
	if fs, ok := rc.state.Formulas[name]; ok {
		return fs
	}
	if short := shortName(name); short != name {
		if fs, ok := rc.state.Formulas[short]; ok {
			return fs
		}
	}
	return brew.FormulaState{}
}

// caskState looks up CaskState with the same short-name fallback.
func (rc *runContext) caskState(name string) brew.CaskState {
	if cs, ok := rc.state.Casks[name]; ok {
		return cs
	}
	if short := shortName(name); short != name {
		if cs, ok := rc.state.Casks[short]; ok {
			return cs
		}
	}
	return brew.CaskState{}
}

type runContext struct {
	ctx      context.Context
	brewer   brew.Brewer
	printer  *ui.Printer
	state    *brew.State
	dryRun   bool
	failFast bool

	restartApps []string
	headSeen    map[string]struct{} // names already processed by applyHead this run
}

// ensureState lazily fetches the bulk Homebrew state on the first call.
// The two things this laziness enables:
//  1. `brewkit head` never invokes State at all (applyHead uses only
//     per-formula probes and skips the ensureState call).
//  2. A run that finds no matching profile files — runApply stats each
//     path and `continue`s on ENOENT before ever entering apply* — can
//     complete without shelling out to brew, which matters on machines
//     where brew is unavailable or transiently broken.
func (rc *runContext) ensureState() error {
	if rc.state != nil {
		return nil
	}
	state, err := rc.brewer.State(rc.ctx)
	if err != nil {
		return fmt.Errorf("brew state: %w", err)
	}
	rc.state = state
	return nil
}

// cacheFormula records a formula's post-action state under both its
// fully qualified name (e.g. user/tap/foo) and its short name (foo) so a
// later profile that references the same formula via either form sees
// the install and skips it.
func (rc *runContext) cacheFormula(name string, fs brew.FormulaState) {
	rc.state.Formulas[name] = fs
	if short := shortName(name); short != name {
		rc.state.Formulas[short] = fs
	}
}

func (rc *runContext) cacheCask(name string, cs brew.CaskState) {
	rc.state.Casks[name] = cs
	if short := shortName(name); short != name {
		rc.state.Casks[short] = cs
	}
}

func (rc *runContext) apply(t profile.Kind, e *parse.Entry) error {
	switch t {
	case profile.KindTap:
		return rc.applyTap(e)
	case profile.KindBrew:
		return rc.applyBrew(e)
	case profile.KindHead:
		return rc.applyHead(e)
	case profile.KindCask:
		return rc.applyCask(e)
	}
	return fmt.Errorf("unknown profile kind %d", t)
}

func (rc *runContext) applyTap(e *parse.Entry) error {
	if err := rc.ensureState(); err != nil {
		return err
	}
	if rc.state.Taps[e.Name] {
		rc.printer.Item(ui.SymUpToDate, e.Name, "")
		return nil
	}
	if rc.dryRun {
		rc.printer.Item(ui.SymAdded, e.Name, "")
		rc.state.Taps[e.Name] = true
		return nil
	}
	res, err := rc.brewer.Tap(rc.ctx, e.Name, e.Extra)
	if err != nil {
		rc.printer.Error(e.Name, "tap failed", res.Output)
		return err
	}
	rc.printer.Item(ui.SymAdded, e.Name, "")
	rc.printer.Verbose(res.Output)
	rc.state.Taps[e.Name] = true
	return nil
}

func (rc *runContext) applyBrew(e *parse.Entry) error {
	if err := rc.ensureState(); err != nil {
		return err
	}
	fs := rc.formulaState(e.Name)
	switch {
	case !fs.Installed:
		if rc.dryRun {
			rc.printer.Item(ui.SymAdded, e.Name, "")
			// Project the install into the cache so a later layered
			// profile referencing the same formula sees it as up-to-date
			// in the preview, matching real-run behavior.
			rc.cacheFormula(e.Name, brew.FormulaState{Installed: true, Version: e.Name})
			return nil
		}
		res, err := rc.brewer.BrewInstall(rc.ctx, e.Name)
		if err != nil {
			rc.printer.Error(e.Name, "install failed", res.Output)
			return err
		}
		rc.printer.Item(ui.SymAdded, e.Name, "")
		rc.printer.Verbose(res.Output)
		rc.cacheFormula(e.Name, brew.FormulaState{Installed: true, Version: res.To})
	case fs.Outdated:
		if rc.dryRun {
			rc.printer.Item(ui.SymUpgraded, e.Name, fs.Version+" → "+fs.OutdatedTo)
			rc.cacheFormula(e.Name, brew.FormulaState{Installed: true, Version: fs.OutdatedTo})
			return nil
		}
		res, err := rc.brewer.BrewUpgrade(rc.ctx, e.Name)
		if err != nil {
			rc.printer.Error(e.Name, "upgrade failed", res.Output)
			return err
		}
		rc.printer.Item(ui.SymUpgraded, e.Name, fs.Version+" → "+fs.OutdatedTo)
		rc.printer.Verbose(res.Output)
		rc.cacheFormula(e.Name, brew.FormulaState{Installed: true, Version: fs.OutdatedTo})
	default:
		rc.printer.Item(ui.SymUpToDate, e.Name, "("+fs.Version+")")
	}
	return nil
}

// applyHead does NOT call ensureState — HEAD operations are entirely
// per-formula (HeadInstalledSHA / HeadLatestSHA), so the bulk
// `brew tap`/`brew list`/`brew outdated` probes from State are wasted
// work and would also abort the run if any unrelated probe (e.g. a
// flaky `brew outdated --cask`) failed.
func (rc *runContext) applyHead(e *parse.Entry) error {
	if rc.headSeen == nil {
		rc.headSeen = map[string]struct{}{}
	}
	// Same formula listed in two layered profiles (under either qualified
	// or short form) is reported once. Key on shortName so e.g.
	// "homebrew/core/tmux" and "tmux" collide. Mark the entry as seen
	// only after the operation succeeds; if the first invocation errors
	// in fail_fast=false mode, a later profile still gets a chance to
	// retry instead of silently being reported as "already processed".
	key := shortName(e.Name)
	if _, ok := rc.headSeen[key]; ok {
		rc.printer.Item(ui.SymUpToDate, e.Name, "(already processed)")
		return nil
	}
	if err := rc.applyHeadEntry(e.Name); err != nil {
		return err
	}
	rc.headSeen[key] = struct{}{}
	return nil
}

func (rc *runContext) applyHeadEntry(name string) error {
	installedSHA, asHead, installed, err := rc.brewer.HeadInstalledSHA(rc.ctx, name)
	if err != nil {
		rc.printer.Error(name, "head install check failed", err.Error())
		return err
	}

	if !installed {
		return rc.installHead(name)
	}
	if !asHead {
		// A stable install is already an invalid Headfile state. Report it
		// directly instead of fetching the latest HEAD SHA, so transient
		// network/cache failures cannot mask the actionable error.
		headErr := fmt.Errorf("%s: installed but not as HEAD", name)
		rc.printer.Error(name, "installed but not as HEAD", "")
		return headErr
	}
	return rc.updateHead(name, installedSHA)
}

func (rc *runContext) installHead(name string) error {
	// Confirm the formula actually exposes a HEAD source before trying to
	// install it. A Headfile entry without a HEAD source is invalid and
	// must be fixed by the user rather than silently accepted as up-to-date.
	if _, err := rc.requireHeadSource(name); err != nil {
		return err
	}
	if rc.dryRun {
		rc.printer.Item(ui.SymAdded, name, "(HEAD)")
		return nil
	}
	res, err := rc.brewer.HeadInstall(rc.ctx, name)
	if err != nil {
		rc.printer.Error(name, "head install failed", res.Output)
		return err
	}
	rc.printer.Item(ui.SymAdded, name, res.To)
	rc.printer.Verbose(res.Output)
	return nil
}

func (rc *runContext) updateHead(name, installedSHA string) error {
	latestSHA, err := rc.requireHeadSource(name)
	if err != nil {
		return err
	}
	if latestSHA == installedSHA {
		rc.printer.Item(ui.SymUpToDate, name, "(HEAD-"+installedSHA+")")
		return nil
	}
	if rc.dryRun {
		rc.printer.Item(ui.SymUpgraded, name, "HEAD-"+installedSHA+" → HEAD-"+latestSHA)
		return nil
	}
	res, err := rc.brewer.HeadReinstall(rc.ctx, name)
	if err != nil {
		rc.printer.Error(name, "head reinstall failed", res.Output)
		return err
	}
	rc.printer.Item(ui.SymUpgraded, name, "HEAD-"+installedSHA+" → HEAD-"+latestSHA)
	rc.printer.Verbose(res.Output)
	return nil
}

func (rc *runContext) requireHeadSource(name string) (string, error) {
	latestSHA, hasHead, err := rc.brewer.HeadLatestSHA(rc.ctx, name)
	if err != nil {
		rc.printer.Error(name, "head latest check failed", err.Error())
		return "", err
	}
	if !hasHead {
		err := fmt.Errorf("%s: no HEAD source", name)
		rc.printer.Error(name, "no HEAD source", "")
		return "", err
	}
	return latestSHA, nil
}

func (rc *runContext) applyCask(e *parse.Entry) error {
	if err := rc.ensureState(); err != nil {
		return err
	}
	cs := rc.caskState(e.Name)
	switch {
	case !cs.Installed:
		if rc.dryRun {
			rc.printer.Item(ui.SymAdded, e.Name, "")
			rc.cacheCask(e.Name, brew.CaskState{Installed: true, Version: e.Name})
			return nil
		}
		res, err := rc.brewer.CaskInstall(rc.ctx, e.Name)
		if err != nil {
			rc.printer.Error(e.Name, "cask install failed", res.Output)
			return err
		}
		rc.printer.Item(ui.SymAdded, e.Name, "")
		rc.printer.Verbose(res.Output)
		rc.cacheCask(e.Name, brew.CaskState{Installed: true, Version: res.To})
	case cs.Outdated:
		if rc.dryRun {
			rc.printer.Item(ui.SymUpgraded, e.Name, cs.Version+" → "+cs.OutdatedTo)
			rc.cacheCask(e.Name, brew.CaskState{Installed: true, Version: cs.OutdatedTo})
			return nil
		}
		res, err := rc.brewer.CaskUpgrade(rc.ctx, e.Name)
		if err != nil {
			rc.printer.Error(e.Name, "cask upgrade failed", res.Output)
			return err
		}
		rc.printer.Item(ui.SymUpgraded, e.Name, cs.Version+" → "+cs.OutdatedTo)
		rc.printer.Verbose(res.Output)
		rc.cacheCask(e.Name, brew.CaskState{Installed: true, Version: cs.OutdatedTo})
		rc.restartApps = append(rc.restartApps, e.Name)
	default:
		rc.printer.Item(ui.SymUpToDate, e.Name, "("+cs.Version+")")
	}
	return nil
}
