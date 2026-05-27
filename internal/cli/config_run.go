package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jmcampanini/brewkit/internal/profile"
	"github.com/jmcampanini/go-config-loader/configreporter"
)

func runConfig(_ context.Context) error {
	cfg, report, profiles, err := loadEffectiveConfig()
	if err != nil {
		return err
	}

	reporter := configreporter.New(cfg, report)
	out, err := reporter.TOML()
	if err != nil {
		return err
	}
	if _, err := fmt.Fprint(os.Stdout, string(out)); err != nil {
		return err
	}
	if len(out) > 0 && out[len(out)-1] != '\n' {
		if _, err := fmt.Fprintln(os.Stdout); err != nil {
			return err
		}
	}

	if _, err := fmt.Fprintln(os.Stdout, "# provenance:"); err != nil {
		return err
	}
	headers := reporter.ProvenanceHeaders()
	if _, err := fmt.Fprintf(os.Stdout, "# %s\n", strings.Join(headers, "\t")); err != nil {
		return err
	}
	for _, row := range reporter.ProvenanceRows() {
		if _, err := fmt.Fprintf(os.Stdout, "# %s\n", strings.Join(row, "\t")); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(os.Stdout, "# loaded_files = [%s]\n", quoteList(report.LoadedFiles)); err != nil {
		return err
	}

	effectiveDir := cfg.Dir
	if abs, err := filepath.Abs(cfg.Dir); err == nil {
		effectiveDir = abs
	}
	if _, err := fmt.Fprintln(os.Stdout, "# effective:"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(os.Stdout, "# effective_dir = %q\n", filepath.Clean(effectiveDir)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(os.Stdout, "# effective_profiles = [%s]\n", quoteList(profiles)); err != nil {
		return err
	}
	if hasProfile(profiles, profile.LocalName) {
		if _, err := fmt.Fprintln(os.Stdout, "# note: 'local' profile auto-appended (*file.local present)"); err != nil {
			return err
		}
	}
	return nil
}

func quoteList(values []string) string {
	if len(values) == 0 {
		return ""
	}
	quoted := make([]string, len(values))
	for i, value := range values {
		quoted[i] = fmt.Sprintf("%q", value)
	}
	return strings.Join(quoted, ", ")
}

func hasProfile(values []string, profileName string) bool {
	for _, value := range values {
		if value == profileName {
			return true
		}
	}
	return false
}
