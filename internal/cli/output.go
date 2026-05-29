package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/jmcampanini/brewkit/internal/ui"
)

func outputWriter(w io.Writer) io.Writer {
	return ui.NewLinePrefixWriter(w, flags.outputPrefix)
}

func stdoutWriter() io.Writer {
	return outputWriter(os.Stdout)
}

func stderrWriter() io.Writer {
	return outputWriter(os.Stderr)
}

// PrintError writes the top-level command error using the same output prefix
// as command-generated durable output.
func PrintError(err error) {
	_, _ = fmt.Fprintln(stderrWriter(), "brewkit:", err)
}
