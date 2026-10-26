// Command brewkit manages Homebrew taps, formulas, HEAD formulas, and casks
// across layer files.
package main

import (
	"os"

	"github.com/jmcampanini/brewkit/internal/cli"
)

func main() {
	os.Exit(cli.Execute())
}
