// Command brewkit manages Homebrew taps, formulas, HEAD formulas, and casks
// across layered profile files.
package main

import (
	"os"

	"github.com/jmcampanini/brewkit/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		cli.PrintError(err)
		os.Exit(1)
	}
}
