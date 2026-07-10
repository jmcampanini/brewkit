package cli

import (
	"io"
	"os"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
	"golang.org/x/term"
)

type fdWriter interface{ Fd() uintptr }

func isTerminal(w io.Writer) bool {
	fd, ok := writerFD(w)
	return ok && term.IsTerminal(fd)
}

func terminalColorProfile(w io.Writer) colorprofile.Profile {
	if os.Getenv("NO_COLOR") != "" {
		return colorprofile.ASCII
	}
	return colorprofile.Detect(w, terminalColorEnvironment())
}

func terminalColorEnvironment() []string {
	// colorprofile.Detect runs tmux info without a timeout when TMUX is present.
	env := os.Environ()
	filtered := make([]string, 0, len(env))
	for _, entry := range env {
		key, _, _ := strings.Cut(entry, "=")
		if key != "TMUX" {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

func terminalColorsEnabled(profile colorprofile.Profile) bool {
	return profile > colorprofile.ASCII
}

func terminalHasDarkBackground(out *os.File, profile colorprofile.Profile, allowQuery bool) bool {
	return detectDarkBackground(
		profile,
		allowQuery && terminalCanQueryBackground(out),
		func() bool { return lipgloss.HasDarkBackground(os.Stdin, out) },
		os.Getenv("COLORFGBG"),
	)
}

func terminalCanQueryBackground(out *os.File) bool {
	return out != nil && isTerminal(os.Stdin) && isTerminal(out)
}

func detectDarkBackground(
	profile colorprofile.Profile,
	canQuery bool,
	query func() bool,
	colorFGBG string,
) bool {
	if dark, ok := darkBackgroundFromEnv(colorFGBG); ok {
		return dark
	}
	if profile <= colorprofile.ASCII || !canQuery || query == nil {
		return true
	}
	return query()
}

func darkBackgroundFromEnv(value string) (dark bool, ok bool) {
	parts := strings.Split(value, ";")
	if len(parts) < 2 {
		return false, false
	}
	bg, err := strconv.Atoi(parts[len(parts)-1])
	if err != nil {
		return false, false
	}
	return bg < 7 || bg == 8, true
}

func terminalWidth(w io.Writer) int {
	fd, ok := writerFD(w)
	if !ok {
		return 0
	}
	width, _, err := term.GetSize(fd)
	if err != nil || width <= 0 {
		return 0
	}
	return width
}

func writerFD(w io.Writer) (int, bool) {
	f, ok := w.(fdWriter)
	if !ok {
		return 0, false
	}
	return int(f.Fd()), true
}
