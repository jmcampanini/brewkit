package cli

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/jmcampanini/brewkit/internal/config"
	"github.com/jmcampanini/brewkit/internal/lint"
	"github.com/jmcampanini/brewkit/internal/parse"
	"github.com/jmcampanini/brewkit/internal/profile"
)

func runLint(_ context.Context) error {
	cfg, err := config.Load(flags.configPath)
	if err != nil {
		return err
	}

	// --profile narrows the scan; otherwise lint scans every discoverable
	// profile in the directory regardless of what's currently active.
	profiles := flags.profiles
	if len(profiles) == 0 {
		discovered, err := profile.Discover(cfg.Dir)
		if err != nil {
			return err
		}
		profiles = discovered
	}

	var violations []lint.Violation
	for _, prof := range profiles {
		for _, k := range profile.AllKinds {
			path := profile.PathFor(cfg.Dir, k, prof)
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

	w := os.Stdout
	if len(violations) == 0 {
		if !flags.quiet {
			if _, err := fmt.Fprintln(w, "no violations"); err != nil {
				return err
			}
		}
		return nil
	}

	if !flags.quiet {
		for _, v := range violations {
			if _, err := fmt.Fprintf(w, "%s:%d: [%s] %s\n", v.Path, v.Line, v.RuleID, v.Message); err != nil {
				return err
			}
			if flags.verbose && v.Raw != "" {
				if _, err := fmt.Fprintf(w, "      %s\n", v.Raw); err != nil {
					return err
				}
			}
		}
	}
	if _, err := fmt.Fprintf(w, "Summary: %d violation(s)\n", len(violations)); err != nil {
		return err
	}
	return fmt.Errorf("lint failed: %d violation(s)", len(violations))
}
