package cli

import (
	"image/color"
	"io"
	"math"
	"os"
	"runtime"
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
	return colorprofile.Detect(w, os.Environ())
}

func terminalColorsEnabled(profile colorprofile.Profile) bool {
	return profile > colorprofile.ASCII
}

func terminalHasDarkBackground(out *os.File, profile colorprofile.Profile, allowQuery bool) bool {
	var query func() (color.Color, bool)
	if allowQuery {
		query = func() (color.Color, bool) {
			return queryTerminalBackground(out)
		}
	}
	return detectDarkBackground(profile, query, os.Getenv("COLORFGBG"))
}

func queryTerminalBackground(out *os.File) (color.Color, bool) {
	if out == nil || !isTerminal(out) {
		return nil, false
	}
	if isTerminal(os.Stdin) || runtime.GOOS == "windows" {
		bg, err := lipgloss.BackgroundColor(os.Stdin, out)
		return bg, err == nil && bg != nil
	}

	// Query the controlling terminal when stdin was redirected.
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return nil, false
	}
	defer tty.Close() //nolint:errcheck
	bg, err := lipgloss.BackgroundColor(tty, out)
	return bg, err == nil && bg != nil
}

func detectDarkBackground(
	profile colorprofile.Profile,
	query func() (color.Color, bool),
	colorFGBG string,
) bool {
	if profile <= colorprofile.ASCII {
		return true
	}
	if query != nil {
		if bg, ok := query(); ok {
			return isDarkColor(bg)
		}
	}
	if dark, ok := darkBackgroundFromEnv(colorFGBG); ok {
		return dark
	}
	return true
}

func darkBackgroundFromEnv(value string) (dark bool, ok bool) {
	parts := strings.Split(value, ";")
	if len(parts) < 2 {
		return false, false
	}
	bg, err := strconv.Atoi(parts[len(parts)-1])
	if err != nil || bg < 0 {
		return false, false
	}
	return bg <= 6 || bg == 8, true
}

func isDarkColor(c color.Color) bool {
	if c == nil {
		return true
	}
	r, g, b, _ := c.RGBA()
	linear := func(channel uint32) float64 {
		value := float64(channel) / 65535
		if value <= 0.04045 {
			return value / 12.92
		}
		return math.Pow((value+0.055)/1.055, 2.4)
	}
	luminance := 0.2126*linear(r) + 0.7152*linear(g) + 0.0722*linear(b)
	return luminance < 0.5
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
