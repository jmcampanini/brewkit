package ui

import "os"

// TerminalOptions carries the caller-chosen printer settings. NewTerminalPrinter
// derives everything else from the process output streams.
type TerminalOptions struct {
	Color         ColorMode
	Level         Level
	DryRun        bool
	HideUnchanged bool
	OutputPrefix  string
}

// NewTerminalPrinter builds a Printer on stdout and stderr, resolving each
// stream's color profile from opts.Color, the theme from the terminal
// background, and spinner support from the terminals.
func NewTerminalPrinter(opts TerminalOptions) *Printer {
	level := opts.Level
	env := os.Environ()
	stdoutTTY := isTerminal(os.Stdout)
	stderrTTY := isTerminal(os.Stderr)
	stdoutProfile := streamColorProfile(opts.Color, os.Stdout, stdoutTTY, env)
	stderrProfile := streamColorProfile(opts.Color, os.Stderr, stderrTTY, env)
	stdoutColorTerminal := stdoutTTY && terminalColorsEnabled(stdoutProfile)
	stderrColorTerminal := stderrTTY && terminalColorsEnabled(stderrProfile)

	// The background is queried at most once, on one stream, and only when
	// that stream is a terminal whose resolved profile renders color; both
	// streams then share the theme. Otherwise the dark palette is the
	// deterministic fallback.
	themeOutput := os.Stdout
	themeProfile := stdoutProfile
	if !stdoutColorTerminal && stderrColorTerminal {
		themeOutput = os.Stderr
		themeProfile = stderrProfile
	}
	allowThemeQuery := level != LevelQuiet
	theme := ThemeForBackground(terminalHasDarkBackground(themeOutput, themeProfile, allowThemeQuery))

	// The spinner is transient progress drawn with cursor control on
	// stderr, so it needs both streams to be terminals and stderr to
	// accept cursor control; the color decision does not affect it.
	spinner := level == LevelNormal && stdoutTTY && stderrTTY && terminalCursorControl(env)
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
