package ui

import "os"

// NewTerminalPrinter configures a printer for the process output streams.
func NewTerminalPrinter(opts PrinterOptions) *Printer {
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
