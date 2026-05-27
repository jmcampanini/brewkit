package cli

import (
	"io"
	"os"

	"golang.org/x/term"
)

type fdWriter interface{ Fd() uintptr }

// isTerminal reports whether w is connected to a terminal AND the user
// has not requested colors off via NO_COLOR.
func isTerminal(w io.Writer) bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	fd, ok := writerFD(w)
	return ok && term.IsTerminal(fd)
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
