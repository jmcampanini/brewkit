package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/jmcampanini/brewkit/internal/config"
	"github.com/jmcampanini/brewkit/internal/ui"
	"github.com/jmcampanini/go-config-loader/configreporter"
)

func (app *commandContext) runConfig(_ context.Context, provenance bool) error {
	effective, err := config.Load(app.configFlagSet)
	if err != nil {
		return err
	}

	reporter := configreporter.New(effective.Config, effective.Report)
	out, err := reporter.TOML()
	if err != nil {
		return err
	}

	w := ui.NewLinePrefixWriter(os.Stdout, app.flags.outputPrefix)
	if _, err := w.Write(out); err != nil {
		return err
	}
	if len(out) > 0 && out[len(out)-1] != '\n' {
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
	}

	if _, err := fmt.Fprintln(w, "# effective:"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "# effective_dir = %q\n", effective.Dir); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "# effective_layers = [%s]\n", quoteList(effective.Layers)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "# layer_order = [%s]\n", quoteList(effective.LayerOrder)); err != nil {
		return err
	}
	if !provenance {
		return nil
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
	_, err = fmt.Fprintf(w, "# loaded_files = [%s]\n", quoteList(effective.Report.LoadedFiles))
	return err
}

func quoteList(values []string) string {
	quoted := make([]string, len(values))
	for i, value := range values {
		quoted[i] = fmt.Sprintf("%q", value)
	}
	return strings.Join(quoted, ", ")
}
