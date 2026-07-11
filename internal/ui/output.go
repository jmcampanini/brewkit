// Package ui implements the brewkit terminal output: a streaming printer
// that emits per-item changes during a command, plus a final summary line
// outside quiet mode.
package ui

import (
	"fmt"
	"io"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// Level controls output verbosity.
type Level int

// Verbosity levels, from default to most detailed.
const (
	LevelNormal Level = iota
	LevelQuiet
	LevelVerbose
)

// Symbol classifies a per-item line and drives the glyph, color, and
// summary counter used for it.
type Symbol int

// The per-item outcome symbols.
const (
	SymUpToDate Symbol = iota
	SymAdded
	SymUpgraded
	SymError
	SymNotice
)

const (
	detailIndent         = "    "
	spinnerInterval      = 100 * time.Millisecond
	defaultSpinnerWidth  = 80
	spinnerTruncateTail  = "…"
	spinnerClearSequence = "\r\033[2K"
)

// Summary accumulates per-item outcomes for the final summary line.
type Summary struct {
	Added    int
	Upgraded int
	UpToDate int
	Errors   int
	Skipped  int
}

// PrinterOptions configures a Printer.
type PrinterOptions struct {
	Level         Level
	Color         bool
	DryRun        bool
	HideUnchanged bool
	OutputPrefix  string
	Spinner       bool
	SpinnerWidth  int
}

// Printer streams per-item output lines and tracks the running Summary.
type Printer struct {
	out           io.Writer
	err           io.Writer
	spinnerErr    io.Writer
	level         Level
	dryRun        bool
	hideUnchanged bool
	color         bool
	outputPrefix  string
	spinner       bool
	spinnerWidth  int
	summary       Summary
	bodyWritten   bool // any non-summary line was written
}

// New builds a Printer that writes items to out and errors to errOut.
func New(out, errOut io.Writer, opts PrinterOptions) *Printer {
	spinnerWidth := opts.SpinnerWidth
	if opts.Spinner && spinnerWidth <= 0 {
		spinnerWidth = defaultSpinnerWidth
	}
	durableOut := NewLinePrefixWriter(out, opts.OutputPrefix)
	durableErr := NewLinePrefixWriter(errOut, opts.OutputPrefix)
	return &Printer{
		out:           durableOut,
		err:           durableErr,
		spinnerErr:    errOut,
		level:         opts.Level,
		dryRun:        opts.DryRun,
		hideUnchanged: opts.HideUnchanged,
		color:         opts.Color,
		outputPrefix:  opts.OutputPrefix,
		spinner:       opts.Spinner,
		spinnerWidth:  spinnerWidth,
	}
}

// Item prints one per-item line and bumps the matching summary counter.
func (p *Printer) Item(sym Symbol, name, detail string) {
	switch sym {
	case SymUpToDate:
		p.summary.UpToDate++
	case SymAdded:
		p.summary.Added++
	case SymUpgraded:
		p.summary.Upgraded++
	case SymError:
		p.summary.Errors++
	}

	if p.level == LevelQuiet && sym != SymError {
		return
	}
	if p.hideUnchanged && sym == SymUpToDate {
		return
	}
	prefix := p.symbol(sym)
	line := prefix + " " + name
	if detail != "" {
		line += " " + p.styleDim(detail)
	}
	if p.dryRun && sym != SymUpToDate && sym != SymError {
		line += " " + p.styleDim("(dry-run)")
	}
	p.writeBodyOut(line)
}

// Notice is used for missing-file messages like "⊘ work: no Headfile,
// skipping" — informational, not an error, but counted as a skip.
func (p *Printer) Notice(msg string) {
	p.summary.Skipped++
	if p.level == LevelQuiet {
		return
	}
	p.writeBodyOut(p.symbol(SymNotice) + " " + p.styleDim(msg))
}

// Verbose is a no-op outside LevelVerbose; it indents raw brew output
// under the most recent item.
func (p *Printer) Verbose(content string) {
	if p.level != LevelVerbose || strings.TrimSpace(content) == "" {
		return
	}
	for _, line := range outputLines(content) {
		p.writeBodyOut(detailIndent + p.styleDim(line))
	}
}

// Error writes to stderr regardless of verbosity, and always includes
// the captured brew output (the user explicitly asked for "print all
// the information" on failure).
func (p *Printer) Error(name, msg, output string) {
	p.summary.Errors++
	prefix := p.symbol(SymError)
	p.writeBodyErr(prefix + " " + name + ": " + msg)
	if strings.TrimSpace(output) == "" {
		return
	}
	for _, line := range outputLines(output) {
		p.writeBodyErr(detailIndent + line)
	}
}

// WithSpinner runs fn while rendering a transient progress line to stderr.
// The spinner is intentionally best-effort: it is disabled by default and
// never contributes to bodyWritten, summaries, or durable stdout output.
func (p *Printer) WithSpinner(message string, fn func() error) error {
	if !p.spinner || p.level != LevelNormal || strings.TrimSpace(message) == "" {
		return fn()
	}

	done := make(chan struct{})
	stopped := make(chan struct{})
	go p.runSpinner(message, done, stopped)

	defer func() {
		close(done)
		<-stopped
	}()
	return fn()
}

func (p *Printer) runSpinner(message string, done <-chan struct{}, stopped chan<- struct{}) {
	defer close(stopped)
	ticker := time.NewTicker(spinnerInterval)
	defer ticker.Stop()

	wrote := false
	frame := 0
	for {
		select {
		case <-done:
			if wrote {
				p.clearSpinner()
			}
			return
		default:
		}

		p.writeSpinnerFrame(spinnerFrames[frame%len(spinnerFrames)], message)
		wrote = true
		frame++

		select {
		case <-done:
			p.clearSpinner()
			return
		case <-ticker.C:
		}
	}
}

func outputLines(content string) []string {
	return strings.Split(strings.TrimRight(content, "\n"), "\n")
}

func (p *Printer) writeBodyOut(line string) {
	_, _ = fmt.Fprintln(p.out, line)
	p.bodyWritten = true
}

func (p *Printer) writeBodyErr(line string) {
	_, _ = fmt.Fprintln(p.err, line)
	p.bodyWritten = true
}

func (p *Printer) writeSpinnerFrame(frame, message string) {
	line := p.outputPrefix + frame + " " + message
	line = p.truncateSpinnerLine(line)
	line = p.styleDim(line)
	_, _ = fmt.Fprintf(p.spinnerErr, "%s%s", spinnerClearSequence, line)
}

func (p *Printer) truncateSpinnerLine(line string) string {
	if p.spinnerWidth <= 0 {
		return line
	}
	// Leave the final column unused. Writing into the last column can trigger
	// terminal autowrap on some emulators, which would make clearSpinner clear
	// only the wrapped physical line and leave stale spinner fragments behind.
	maxWidth := p.spinnerWidth - 1
	if maxWidth <= 0 {
		return ""
	}
	return ansi.Truncate(line, maxWidth, spinnerTruncateTail)
}

func (p *Printer) clearSpinner() {
	_, _ = fmt.Fprint(p.spinnerErr, spinnerClearSequence)
}

// Footer emits the final summary outside quiet mode. In quiet mode,
// operational commands communicate success or failure through their exit code
// and any error output already emitted by Error.
func (p *Printer) Footer() {
	if p.level == LevelQuiet {
		return
	}

	var parts []string
	if p.summary.Added > 0 {
		parts = append(parts, fmt.Sprintf("%d added", p.summary.Added))
	}
	if p.summary.Upgraded > 0 {
		parts = append(parts, fmt.Sprintf("%d upgraded", p.summary.Upgraded))
	}
	if p.summary.UpToDate > 0 {
		parts = append(parts, fmt.Sprintf("%d up-to-date", p.summary.UpToDate))
	}
	if p.summary.Skipped > 0 {
		parts = append(parts, fmt.Sprintf("%d skipped", p.summary.Skipped))
	}
	if p.summary.Errors > 0 {
		parts = append(parts, fmt.Sprintf("%d failed", p.summary.Errors))
	}
	if len(parts) == 0 {
		parts = append(parts, "nothing to do")
	}
	if p.bodyWritten {
		_, _ = fmt.Fprintln(p.out)
	}
	_, _ = fmt.Fprintln(p.out, "Summary: "+strings.Join(parts, ", "))
}

// RestartAppsNotice prints the "restart these apps" block after cask upgrades.
func (p *Printer) RestartAppsNotice(names []string) {
	if len(names) == 0 || p.level == LevelQuiet {
		return
	}
	if p.bodyWritten {
		_, _ = fmt.Fprintln(p.out)
	}
	p.writeBodyOut(p.styleWarn("⚠ Restart these apps to apply upgrades"))
	for _, name := range names {
		p.writeBodyOut("  " + name)
	}
}

func (p *Printer) symbol(sym Symbol) string {
	plain := symbolText(sym)
	if !p.color {
		return plain
	}
	switch sym {
	case SymUpToDate:
		return styleOK.Render(plain)
	case SymAdded:
		return styleAdded.Render(plain)
	case SymUpgraded:
		return styleUpgraded.Render(plain)
	case SymError:
		return styleErr.Render(plain)
	case SymNotice:
		return styleDim.Render(plain)
	}
	return plain
}

func symbolText(sym Symbol) string {
	switch sym {
	case SymUpToDate:
		return "✓"
	case SymAdded:
		return "+"
	case SymUpgraded:
		return "↑"
	case SymError:
		return "✗"
	case SymNotice:
		return "⊘"
	}
	return "?"
}

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

var (
	styleOK       = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	styleAdded    = lipgloss.NewStyle().Foreground(lipgloss.Color("12"))
	styleUpgraded = lipgloss.NewStyle().Foreground(lipgloss.Color("13"))
	styleErr      = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	styleDim      = lipgloss.NewStyle().Faint(true)
	styleWarn     = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
)

func (p *Printer) styleDim(s string) string {
	if !p.color {
		return s
	}
	return styleDim.Render(s)
}

func (p *Printer) styleWarn(s string) string {
	if !p.color {
		return s
	}
	return styleWarn.Render(s)
}
