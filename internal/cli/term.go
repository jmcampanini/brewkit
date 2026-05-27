package cli

import (
	"io"
	"os"

	"golang.org/x/term"
)

// isTerminal reports whether w is connected to a terminal AND the user
// has not requested colors off via NO_COLOR.
func isTerminal(w io.Writer) bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	type fder interface{ Fd() uintptr }
	f, ok := w.(fder)
	if !ok {
		return false
	}
	return term.IsTerminal(int(f.Fd()))
}

func terminalWidth(w io.Writer) int {
	type fder interface{ Fd() uintptr }
	f, ok := w.(fder)
	if !ok {
		return 0
	}
	width, _, err := term.GetSize(int(f.Fd()))
	if err != nil || width <= 0 {
		return 0
	}
	return width
}
