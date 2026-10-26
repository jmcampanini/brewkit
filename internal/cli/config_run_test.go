package cli

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/jmcampanini/brewkit/internal/config"
	"github.com/spf13/pflag"
)

func TestRunConfig_Report(t *testing.T) {
	for _, tt := range []struct {
		name       string
		files      map[string]string
		provenance bool
		quiet      bool
		want       []string
		wantAbsent []string
	}{
		{
			name: "toml then effective block",
			want: []string{
				`layers = ["common"]`,
				`env_layers = []`,
				"# effective:",
				`# effective_layers = ["common"]`,
				`# layer_order = ["common"]`,
			},
			wantAbsent: []string{"# provenance:", "loaded_files"},
		},
		{
			name:       "provenance appended on request",
			provenance: true,
			want: []string{
				`# effective_layers = ["common"]`,
				"# provenance:",
				"# Path\tValue\tSource",
				"# loaded_files = [",
			},
		},
		{
			name:  "quiet still prints the report",
			quiet: true,
			want:  []string{`layers = ["common"]`, "# effective:"},
		},
		{
			name:  "local appears only in the layer order",
			files: map[string]string{"Brewfile.local": ""},
			want: []string{
				`layers = ["common"]`,
				`# effective_layers = ["common"]`,
				`# layer_order = ["common", "local"]`,
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			resetFlags()
			defer resetFlags()
			dir := fixtureRepo(t, tt.files)
			useConfigFile(t, filepath.Join(dir, "brewkit.toml"))
			testApp.flags.quiet = tt.quiet

			out := captureStdout(t, func() {
				if err := testApp.runConfig(context.Background(), tt.provenance); err != nil {
					t.Errorf("runConfig err: %v", err)
				}
			})

			for _, want := range tt.want {
				if !strings.Contains(out, want) {
					t.Errorf("config output missing %q:\n%s", want, out)
				}
			}
			for _, absent := range tt.wantAbsent {
				if strings.Contains(out, absent) {
					t.Errorf("config output should not contain %q:\n%s", absent, out)
				}
			}
			if tt.provenance && strings.Index(out, "# effective:") > strings.Index(out, "# provenance:") {
				t.Errorf("effective block should precede provenance:\n%s", out)
			}
		})
	}
}

// TestRunConfig_ReportReloads redirects the full report, provenance included,
// into a file and loads it as brewkit.toml: the raw settings must come back
// unchanged and the effective blocks must stay comments.
func TestRunConfig_ReportReloads(t *testing.T) {
	resetFlags()
	defer resetFlags()
	dir := fixtureRepo(t, map[string]string{"Brewfile.local": ""})
	useConfigFile(t, filepath.Join(dir, "brewkit.toml"))
	t.Setenv("EXTRA_LAYERS", "work")
	if err := os.WriteFile(filepath.Join(dir, "brewkit.toml"), []byte(`dir = "`+dir+`"`+"\n"+`layers = ["common"]`+"\n"+`env_layers = ["EXTRA_LAYERS"]`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out := captureStdout(t, func() {
		if err := testApp.runConfig(context.Background(), true); err != nil {
			t.Errorf("runConfig err: %v", err)
		}
	})

	reportPath := filepath.Join(dir, "report.toml")
	if err := os.WriteFile(reportPath, []byte(out), 0o644); err != nil {
		t.Fatal(err)
	}
	flags := pflag.NewFlagSet("reload", pflag.ContinueOnError)
	if err := config.RegisterFlags(flags); err != nil {
		t.Fatal(err)
	}
	if err := flags.Parse([]string{"--config", reportPath}); err != nil {
		t.Fatal(err)
	}
	reloaded, err := config.Load(flags)
	if err != nil {
		t.Fatalf("reload report as brewkit.toml: %v\n%s", err, out)
	}
	want := config.Config{Dir: dir, Layers: []string{"common"}, EnvLayers: []string{"EXTRA_LAYERS"}, FailFast: true}
	if !reflect.DeepEqual(reloaded.Config, want) {
		t.Errorf("reloaded config = %+v, want %+v\n%s", reloaded.Config, want, out)
	}
	if !reflect.DeepEqual(reloaded.LayerOrder, []string{"common", "work", "local"}) {
		t.Errorf("reloaded layer order = %v, want [common work local]", reloaded.LayerOrder)
	}
}
