package ui

import "os"

// TerminalOptions carries the caller-chosen printer settings. NewTerminalPrinter
// derives everything else from the process output streams.
type TerminalOptions struct {
	Level         Level
	DryRun        bool
	HideUnchanged bool
	OutputPrefix  string
}

// NewTerminalPrinter builds a Printer on stdout and stderr, detecting color
// profiles, the theme, and spinner support from the terminals.
func NewTerminalPrinter(opts TerminalOptions) *Printer {
	level := opts.Level
	stdoutTTY := isTerminal(os.Stdout)
	stderrTTY := isTerminal(os.Stderr)
	stdoutProfile := terminalColorProfile(os.Stdout)
	stderrProfile := terminalColorProfile(os.Stderr)
	stdoutColorTerminal := stdoutTTY && terminalColorsEnabled(stdoutProfile)
	stderrColorTerminal := stderrTTY && terminalColorsEnabled(stderrProfile)
	themeOutput := os.Stdout
	themeProfile := stdoutProfile
	if !stdoutColorTerminal && stderrColorTerminal {
		themeOutput = os.Stderr
		themeProfile = stderrProfile
	}
	allowThemeQuery := level != LevelQuiet
	theme := ThemeForBackground(terminalHasDarkBackground(themeOutput, themeProfile, allowThemeQuery))
	spinner := level == LevelNormal && stdoutTTY && stderrTTY
	spinnerWidth := 0
	if spinner {
		spinnerWidth = terminalWidth(os.Stderr)
		if spinnerWidth <= 1 {
			spinner = false
		}
	}
	return New(os.Stdout, os.Stderr, PrinterOptions{
		Level:         level,
		OutputProfile: stdoutProfile,
		ErrorProfile:  stderrProfile,
		Theme:         theme,
		DryRun:        opts.DryRun,
		HideUnchanged: opts.HideUnchanged,
		OutputPrefix:  opts.OutputPrefix,
		Spinner:       spinner,
		SpinnerWidth:  spinnerWidth,
	})
}
