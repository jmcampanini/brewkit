package main

import (
	"fmt"
	"os"

	"github.com/jmcampanini/brewkit/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "brewkit:", err)
		os.Exit(1)
	}
}
