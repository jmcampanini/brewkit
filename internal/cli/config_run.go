package cli

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/jmcampanini/brewkit/internal/profile"
	"github.com/jmcampanini/go-config-loader/configreporter"
)

var filepathAbs = filepath.Abs

func runConfig(_ context.Context) error {
	cfg, report, profiles, err := loadEffectiveConfig()
	if err != nil {
		return err
	}

	effectiveDir, err := filepathAbs(cfg.Dir)
	if err != nil {
		return fmt.Errorf("resolve effective dir %q: %w", cfg.Dir, err)
	}

	reporter := configreporter.New(cfg, report)
	out, err := reporter.TOML()
	if err != nil {
		return err
	}

	w := stdoutWriter()
	if _, err := w.Write(out); err != nil {
		return err
	}
	if len(out) > 0 && out[len(out)-1] != '\n' {
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
	}

	if _, err := fmt.Fprintln(w, "# provenance:"); err != nil {
		return err
	}
	headers := reporter.ProvenanceHeaders()
	if _, err := fmt.Fprintf(w, "# %s\n", strings.Join(headers, "\t")); err != nil {
		return err
	}
	for _, row := range reporter.ProvenanceRows() {
		if _, err := fmt.Fprintf(w, "# %s\n", strings.Join(row, "\t")); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(w, "# loaded_files = [%s]\n", quoteList(report.LoadedFiles)); err != nil {
		return err
	}

	if _, err := fmt.Fprintln(w, "# effective:"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "# effective_dir = %q\n", filepath.Clean(effectiveDir)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "# effective_profiles = [%s]\n", quoteList(profiles)); err != nil {
		return err
	}
	if slices.Contains(profiles, profile.LocalName) {
		if _, err := fmt.Fprintln(w, "# note: 'local' profile auto-appended (*file.local present)"); err != nil {
			return err
		}
	}
	return nil
}

func quoteList(values []string) string {
	quoted := make([]string, len(values))
	for i, value := range values {
		quoted[i] = fmt.Sprintf("%q", value)
	}
	return strings.Join(quoted, ", ")
}
