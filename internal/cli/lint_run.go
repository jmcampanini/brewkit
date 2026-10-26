package cli

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/jmcampanini/brewkit/internal/config"
	"github.com/jmcampanini/brewkit/internal/layer"
	"github.com/jmcampanini/brewkit/internal/lint"
	"github.com/jmcampanini/brewkit/internal/parse"
	"github.com/jmcampanini/brewkit/internal/ui"
)

func (app *commandContext) layerFlagChanged() bool {
	if app.configFlagSet == nil {
		return false
	}
	for _, name := range []string{"layers", "layer"} {
		flag := app.configFlagSet.Lookup(name)
		if flag != nil && flag.Changed {
			return true
		}
	}
	return false
}

func (app *commandContext) runLint(_ context.Context) error {
	effective, err := config.Load(app.configFlagSet)
	if err != nil {
		return err
	}

	// --layers/--layer narrows the scan to the effective layers; otherwise
	// lint scans every layer that has a file in the directory.
	layers := effective.Layers
	if !app.layerFlagChanged() {
		layers, err = layer.Discover(effective.Dir)
		if err != nil {
			return err
		}
	}

	var violations []lint.Violation
	for _, name := range layers {
		for _, k := range layer.AllKinds {
			path := layer.PathFor(effective.Dir, k, name)
			_, statErr := os.Stat(path)
			if errors.Is(statErr, fs.ErrNotExist) {
				continue
			}
			if statErr != nil {
				return fmt.Errorf("stat %s: %w", path, statErr)
			}
			f, err := parse.Parse(path, k)
			if err != nil {
				return err
			}
			violations = append(violations, lint.Check(f)...)
		}
	}

	w := ui.NewLinePrefixWriter(os.Stdout, app.flags.outputPrefix)
	if len(violations) == 0 {
		if !app.flags.quiet {
			if _, err := fmt.Fprintln(w, "no violations"); err != nil {
				return err
			}
		}
		return nil
	}

	for _, v := range violations {
		if _, err := fmt.Fprintf(w, "%s:%d: [%s] %s\n", v.Path, v.Line, v.RuleID, v.Message); err != nil {
			return err
		}
		if app.flags.verbose && !app.flags.quiet && v.Raw != "" {
			if _, err := fmt.Fprintf(w, "      %s\n", v.Raw); err != nil {
				return err
			}
		}
	}
	if !app.flags.quiet {
		if _, err := fmt.Fprintf(w, "Summary: %d violation(s)\n", len(violations)); err != nil {
			return err
		}
	}
	return fmt.Errorf("lint failed: %d violation(s)", len(violations))
}
