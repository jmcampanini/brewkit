// Package ui implements the brewkit terminal output: a streaming printer
// that emits per-item changes during a command, plus a final summary line.
package ui

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type Level int

const (
	LevelNormal Level = iota
	LevelQuiet
	LevelVerbose
)

type Symbol int

const (
	SymUpToDate Symbol = iota
	SymAdded
	SymUpgraded
	SymError
	SymNotice
)

type Summary struct {
	Added    int
	Upgraded int
	UpToDate int
	Errors   int
	Skipped  int
}

type Printer struct {
	out     io.Writer
	err     io.Writer
	level   Level
	dryRun  bool
	color   bool
	summary Summary
}

func New(out, errOut io.Writer, level Level, color, dryRun bool) *Printer {
	return &Printer{out: out, err: errOut, level: level, dryRun: dryRun, color: color}
}

func (p *Printer) Group(name string) {
	if p.level == LevelQuiet {
		return
	}
	_, _ = fmt.Fprintln(p.out, p.styleHeader(name))
}

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
	prefix := p.symbol(sym)
	line := "  " + prefix + " " + name
	if detail != "" {
		line += " " + p.styleDim(detail)
	}
	if p.dryRun && sym != SymUpToDate && sym != SymError {
		line += " " + p.styleDim("(dry-run)")
	}
	_, _ = fmt.Fprintln(p.out, line)
}

// Notice is used for missing-file messages like "∘ work: no Headfile,
// skipping" — informational, not an error, but counted as a skip.
func (p *Printer) Notice(msg string) {
	p.summary.Skipped++
	if p.level == LevelQuiet {
		return
	}
	_, _ = fmt.Fprintln(p.out, "  "+p.symbol(SymNotice)+" "+p.styleDim(msg))
}

// Verbose is a no-op outside LevelVerbose; it indents raw brew output
// under the most recent item.
func (p *Printer) Verbose(content string) {
	if p.level != LevelVerbose || strings.TrimSpace(content) == "" {
		return
	}
	for _, line := range strings.Split(strings.TrimRight(content, "\n"), "\n") {
		_, _ = fmt.Fprintln(p.out, "      "+p.styleDim(line))
	}
}

// Error writes to stderr regardless of verbosity, and always includes
// the captured brew output (the user explicitly asked for "print all
// the information" on failure).
func (p *Printer) Error(name, msg, output string) {
	p.summary.Errors++
	prefix := p.symbol(SymError)
	_, _ = fmt.Fprintln(p.err, "  "+prefix+" "+name+": "+msg)
	if strings.TrimSpace(output) == "" {
		return
	}
	for _, line := range strings.Split(strings.TrimRight(output, "\n"), "\n") {
		_, _ = fmt.Fprintln(p.err, "      "+line)
	}
}

// Footer is emitted even in quiet mode so scripts have something to grep.
func (p *Printer) Footer() {
	parts := []string{}
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
	if p.level != LevelQuiet {
		_, _ = fmt.Fprintln(p.out)
	}
	_, _ = fmt.Fprintln(p.out, "Summary: "+strings.Join(parts, ", "))
}

func (p *Printer) RestartAppsNotice(names []string) {
	if len(names) == 0 || p.level == LevelQuiet {
		return
	}
	_, _ = fmt.Fprintln(p.out)
	_, _ = fmt.Fprintln(p.out, p.styleWarn("⚠ Restart these apps to apply upgrades"))
	for _, n := range names {
		_, _ = fmt.Fprintln(p.out, "  "+n)
	}
}

func (p *Printer) symbol(sym Symbol) string {
	if !p.color {
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
			return "∘"
		}
		return "?"
	}
	switch sym {
	case SymUpToDate:
		return styleOK.Render("✓")
	case SymAdded:
		return styleAdded.Render("+")
	case SymUpgraded:
		return styleUpgraded.Render("↑")
	case SymError:
		return styleErr.Render("✗")
	case SymNotice:
		return styleDim.Render("∘")
	}
	return "?"
}

var (
	styleOK       = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	styleAdded    = lipgloss.NewStyle().Foreground(lipgloss.Color("12"))
	styleUpgraded = lipgloss.NewStyle().Foreground(lipgloss.Color("13"))
	styleErr      = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	styleDim      = lipgloss.NewStyle().Faint(true)
	styleWarn     = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	styleHeaderHl = lipgloss.NewStyle().Bold(true)
)

func (p *Printer) styleHeader(s string) string {
	if !p.color {
		return s
	}
	return styleHeaderHl.Render(s)
}

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
