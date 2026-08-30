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
	"github.com/charmbracelet/colorprofile"
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
	minSpinnerWidth      = 5
	spinnerTruncateTail  = "..."
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
	OutputProfile colorprofile.Profile
	ErrorProfile  colorprofile.Profile
	Theme         Theme
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
	outStyles     printerStyles
	errStyles     printerStyles
	outputPrefix  string
	spinner       bool
	spinnerWidth  int
	summary       Summary
	bodyWritten   bool // any non-summary line was written
}

// New builds a Printer that writes items to out and errors to errOut.
func New(out, errOut io.Writer, opts PrinterOptions) *Printer {
	spinner := opts.Spinner
	spinnerWidth := opts.SpinnerWidth
	if spinner && spinnerWidth <= 0 {
		spinnerWidth = defaultSpinnerWidth
	}
	if spinner && spinnerWidth < minSpinnerWidth {
		spinner = false
	}
	theme := opts.Theme
	if theme.name == "" {
		theme = DarkTheme()
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
		outStyles:     newPrinterStyles(opts.OutputProfile, theme),
		errStyles:     newPrinterStyles(opts.ErrorProfile, theme),
		outputPrefix:  opts.OutputPrefix,
		spinner:       spinner,
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
	prefix := p.outStyles.symbol(sym)
	line := prefix + " " + name
	if detail != "" {
		line += " " + p.outStyles.detail(detail)
	}
	if p.dryRun && sym != SymUpToDate && sym != SymError {
		line += " " + p.outStyles.secondary("(dry-run)")
	}
	p.writeBodyOut(line)
}

// Notice is used for missing-file messages like "⊘ work: no Headfile,
// skipping" - informational, not an error, but counted as a skip.
func (p *Printer) Notice(msg string) {
	p.summary.Skipped++
	if p.level == LevelQuiet {
		return
	}
	p.writeBodyOut(p.outStyles.symbol(SymNotice) + " " + p.outStyles.secondary(msg))
}

// Verbose is a no-op outside LevelVerbose; it indents raw brew output
// under the most recent item.
func (p *Printer) Verbose(content string) {
	if p.level != LevelVerbose || strings.TrimSpace(content) == "" {
		return
	}
	for _, line := range outputLines(content) {
		p.writeBodyOut(detailIndent + p.outStyles.secondary(line))
	}
}

// Error writes to stderr regardless of verbosity, and always includes
// the captured brew output (the user explicitly asked for "print all
// the information" on failure).
func (p *Printer) Error(name, msg, output string) {
	p.summary.Errors++
	prefix := p.errStyles.symbol(SymError)
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
	prefixedFrame := p.outputPrefix + frame
	line := p.truncateSpinnerLine(prefixedFrame + " " + message)
	if strings.HasPrefix(line, prefixedFrame) {
		line = p.errStyles.secondary(p.outputPrefix) +
			p.errStyles.spinnerFrame(frame) +
			p.errStyles.secondary(line[len(prefixedFrame):])
	} else {
		line = p.errStyles.secondary(line)
	}
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
	p.writeBodyOut(p.outStyles.warning("⚠") + " Restart these apps to apply upgrades")
	for _, name := range names {
		p.writeBodyOut("  " + name)
	}
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

type printerStyles struct {
	enabled     bool
	ok          lipgloss.Style
	added       lipgloss.Style
	upgraded    lipgloss.Style
	err         lipgloss.Style
	detailStyle lipgloss.Style
	warn        lipgloss.Style
	spinner     lipgloss.Style
}

func newPrinterStyles(profile colorprofile.Profile, theme Theme) printerStyles {
	return printerStyles{
		enabled:     profile > colorprofile.ASCII,
		ok:          lipgloss.NewStyle().Foreground(profile.Convert(theme.ok)),
		added:       lipgloss.NewStyle().Foreground(profile.Convert(theme.added)),
		upgraded:    lipgloss.NewStyle().Foreground(profile.Convert(theme.upgraded)),
		err:         lipgloss.NewStyle().Foreground(profile.Convert(theme.err)),
		detailStyle: lipgloss.NewStyle().Foreground(profile.Convert(theme.detail)),
		warn:        lipgloss.NewStyle().Foreground(profile.Convert(theme.warn)),
		spinner:     lipgloss.NewStyle().Foreground(profile.Convert(theme.spinner)),
	}
}

func (s *printerStyles) symbol(sym Symbol) string {
	plain := symbolText(sym)
	if !s.enabled {
		return plain
	}
	switch sym {
	case SymUpToDate:
		return s.ok.Render(plain)
	case SymAdded:
		return s.added.Render(plain)
	case SymUpgraded:
		return s.upgraded.Render(plain)
	case SymError:
		return s.err.Render(plain)
	case SymNotice:
		return s.detailStyle.Render(plain)
	}
	return plain
}

func (s *printerStyles) secondary(text string) string {
	if !s.enabled || text == "" {
		return text
	}
	return s.detailStyle.Render(text)
}

func (s *printerStyles) spinnerFrame(frame string) string {
	if !s.enabled {
		return frame
	}
	return s.spinner.Render(frame)
}

func (s *printerStyles) detail(text string) string {
	if !s.enabled {
		return text
	}
	parts := strings.Split(text, "→")
	for i := range parts {
		parts[i] = s.detailStyle.Render(parts[i])
	}
	return strings.Join(parts, s.upgraded.Render("→"))
}

func (s *printerStyles) warning(text string) string {
	if !s.enabled {
		return text
	}
	return s.warn.Render(text)
}
