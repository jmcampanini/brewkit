package ui

import (
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
	"golang.org/x/term"
)

// ColorMode is the --color setting: when the output brewkit itself renders
// is colored. It never affects the output of brew, which passes through
// unchanged. Its zero value is auto, and it implements the flag value
// interface so an unknown mode is rejected while the flag is parsed.
type ColorMode string

// The color modes, in the order a person reaches for them.
const (
	// ColorAuto colors a stream only when it is a terminal, TERM is not
	// dumb, and NO_COLOR is empty.
	ColorAuto ColorMode = "auto"
	// ColorAlways colors every stream, even one that is not a terminal
	// or when NO_COLOR is set.
	ColorAlways ColorMode = "always"
	// ColorNever disables color on every stream.
	ColorNever ColorMode = "never"
)

// Set stores the mode named by value and rejects any other value.
func (m *ColorMode) Set(value string) error {
	switch mode := ColorMode(value); mode {
	case ColorAuto, ColorAlways, ColorNever:
		*m = mode
		return nil
	}
	return fmt.Errorf("must be auto, always, or never")
}

// String returns the mode name, auto for the zero value.
func (m ColorMode) String() string {
	if m == "" {
		return string(ColorAuto)
	}
	return string(m)
}

// Type names the flag value in usage text.
func (ColorMode) Type() string { return "MODE" }

type fdWriter interface{ Fd() uintptr }

func isTerminal(w io.Writer) bool {
	fd, ok := writerFD(w)
	return ok && term.IsTerminal(fd)
}

// streamColorProfile resolves the color profile for one output stream.
// terminal reports whether w is a terminal and env is the process
// environment. ColorAuto disables color unless w is a terminal, TERM is
// not dumb, and NO_COLOR is empty; ColorAlways takes the profile the
// environment describes, ignoring NO_COLOR, and raises it to at least
// ANSI; ColorNever disables color.
func streamColorProfile(mode ColorMode, w io.Writer, terminal bool, env []string) colorprofile.Profile {
	switch mode {
	case ColorNever:
		return colorprofile.ASCII
	case ColorAlways:
		return max(colorprofile.Env(colorDetectionEnvironment(env, "NO_COLOR")), colorprofile.ANSI)
	}
	if !terminal || envValue(env, "NO_COLOR") != "" || envValue(env, "TERM") == "dumb" {
		return colorprofile.ASCII
	}
	return colorprofile.Detect(w, colorDetectionEnvironment(env))
}

// colorDetectionEnvironment returns env without TMUX, which makes
// colorprofile run tmux info without a timeout, and without the named
// keys.
func colorDetectionEnvironment(env []string, dropped ...string) []string {
	filtered := make([]string, 0, len(env))
	for _, entry := range env {
		key, _, _ := strings.Cut(entry, "=")
		if key != "TMUX" && !slices.Contains(dropped, key) {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

func envValue(env []string, name string) string {
	for _, entry := range env {
		if key, value, _ := strings.Cut(entry, "="); key == name {
			return value
		}
	}
	return ""
}

func terminalColorsEnabled(profile colorprofile.Profile) bool {
	return profile > colorprofile.ASCII
}

// terminalCursorControl reports whether a terminal described by env accepts
// the cursor movement and line erase the spinner uses. It is independent
// of the color profile: an uncolored terminal still carries the spinner,
// and TERM=dumb or an unset TERM carries neither.
func terminalCursorControl(env []string) bool {
	value := envValue(env, "TERM")
	return value != "" && value != "dumb"
}

func terminalHasDarkBackground(out *os.File, profile colorprofile.Profile, allowQuery bool) bool {
	var query func() bool
	if allowQuery && terminalCanQueryBackground(out) {
		query = func() bool { return lipgloss.HasDarkBackground(os.Stdin, out) }
	}
	return detectDarkBackground(profile, query, os.Getenv("COLORFGBG"))
}

func terminalCanQueryBackground(out *os.File) bool {
	return out != nil && isTerminal(os.Stdin) && isTerminal(out)
}

func detectDarkBackground(profile colorprofile.Profile, query func() bool, colorFGBG string) bool {
	if dark, ok := darkBackgroundFromEnv(colorFGBG); ok {
		return dark
	}
	if profile <= colorprofile.ASCII || query == nil {
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
