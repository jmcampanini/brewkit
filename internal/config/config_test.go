package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/jmcampanini/go-config-loader/configloader"
	"github.com/jmcampanini/go-config-loader/pflagloader"
	"github.com/spf13/pflag"
)

func TestDefaults(t *testing.T) {
	got := Defaults()
	want := Config{
		Dir:       ".",
		Layers:    []string{},
		EnvLayers: []string{},
		FailFast:  true,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Defaults() = %+v, want %+v", got, want)
	}
}

func TestLoad_MissingImplicitFallsBackToDefaults(t *testing.T) {
	unsetEnv(t, "BREWKIT_LAYERS")
	dir := t.TempDir()
	t.Chdir(dir)

	got, err := Load(nil)
	if err != nil {
		t.Fatalf("Load() returned err: %v", err)
	}
	if len(got.Report.LoadedFiles) != 0 {
		t.Errorf("LoadedFiles = %v, want none", got.Report.LoadedFiles)
	}
	if !reflect.DeepEqual(got.Config, Defaults()) {
		t.Errorf("config = %+v, want defaults %+v", got.Config, Defaults())
	}
	if got.Report.Updates["layers"] != configloader.SourceDefault {
		t.Errorf("layers source = %q, want default", got.Report.Updates["layers"])
	}
	if want := mustAbs(t, dir); got.Dir != want {
		t.Errorf("Dir = %q, want %q", got.Dir, want)
	}
	if len(got.Layers) != 0 || len(got.LayerOrder) != 0 {
		t.Errorf("Layers, LayerOrder = %v, %v, want both empty", got.Layers, got.LayerOrder)
	}
}

func TestLoad_ConfigFileErrors(t *testing.T) {
	unsetEnv(t, "BREWKIT_LAYERS")
	for _, tt := range []struct {
		name    string
		setup   func(t *testing.T) *pflag.FlagSet
		wantErr string
	}{
		{
			name: "implicit config is a directory",
			setup: func(t *testing.T) *pflag.FlagSet {
				t.Chdir(t.TempDir())
				if err := os.Mkdir("brewkit.toml", 0o755); err != nil {
					t.Fatal(err)
				}
				return nil
			},
			wantErr: "directory",
		},
		{
			name: "explicit config is missing",
			setup: func(t *testing.T) *pflag.FlagSet {
				return newTestFlagSet(t, "--config", "/nonexistent/brewkit.toml")
			},
			wantErr: "brewkit.toml",
		},
		{
			name: "explicit config is empty",
			setup: func(t *testing.T) *pflag.FlagSet {
				return newTestFlagSet(t, "--config", "")
			},
			wantErr: "--config: path must not be empty",
		},
		{
			name: "explicit config is a directory",
			setup: func(t *testing.T) *pflag.FlagSet {
				path := filepath.Join(t.TempDir(), "brewkit.toml")
				if err := os.Mkdir(path, 0o755); err != nil {
					t.Fatal(err)
				}
				return newTestFlagSet(t, "--config", path)
			},
			wantErr: "directory",
		},
		{
			name: "malformed toml",
			setup: func(t *testing.T) *pflag.FlagSet {
				return newTestFlagSet(t, "--config", writeConfig(t, "dir = "))
			},
			wantErr: "",
		},
		{
			name: "unknown key",
			setup: func(t *testing.T) *pflag.FlagSet {
				return newTestFlagSet(t, "--config", writeConfig(t, "verbose = true\n"))
			},
			wantErr: "verbose",
		},
		{
			name: "empty dir",
			setup: func(t *testing.T) *pflag.FlagSet {
				return newTestFlagSet(t, "--config", writeConfig(t, `dir = ""`))
			},
			wantErr: "dir must not be empty",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			flags := tt.setup(t)

			_, err := Load(flags)
			if err == nil {
				t.Fatal("Load() = nil error, want error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Load() error = %v, want it to contain %q", err, tt.wantErr)
			}
		})
	}
}

func TestLoad_ParsesFile(t *testing.T) {
	unsetEnv(t, "BREWKIT_LAYERS")
	path := writeConfig(t, `
dir = "./homebrew"
layers = ["common", "work"]
env_layers = ["DOTFILES_LAYERS"]
fail_fast = false
`)
	cwd := t.TempDir()
	t.Chdir(cwd)

	got, err := Load(newTestFlagSet(t, "--config", path))
	if err != nil {
		t.Fatalf("Load() returned err: %v", err)
	}
	want := Config{
		Dir:       "./homebrew",
		Layers:    []string{"common", "work"},
		EnvLayers: []string{"DOTFILES_LAYERS"},
		FailFast:  false,
	}
	if !reflect.DeepEqual(got.Config, want) {
		t.Errorf("Config = %+v, want %+v", got.Config, want)
	}
	if wantDir := filepath.Join(mustAbs(t, cwd), "homebrew"); got.Dir != wantDir {
		t.Errorf("Dir = %q, want %q (relative to the working directory, not the config file)", got.Dir, wantDir)
	}
	if !reflect.DeepEqual(got.Report.LoadedFiles, []string{path}) {
		t.Errorf("LoadedFiles = %v, want [%s]", got.Report.LoadedFiles, path)
	}
	if got.Report.Updates["layers"] != path {
		t.Errorf("layers source = %q, want file path", got.Report.Updates["layers"])
	}
}

func TestLoad_FileContentsKeepDefaults(t *testing.T) {
	unsetEnv(t, "BREWKIT_LAYERS")
	for _, tt := range []struct {
		name     string
		contents string
		want     func(Config) Config
	}{
		{name: "empty file", contents: "", want: func(c Config) Config { return c }},
		{name: "comment only", contents: "# disabled, no keys\n", want: func(c Config) Config { return c }},
		{name: "partial file", contents: `layers = ["work"]`, want: func(c Config) Config {
			c.Layers = []string{"work"}
			return c
		}},
		{name: "absolute dir", contents: `dir = "/abs/path"`, want: func(c Config) Config {
			c.Dir = "/abs/path"
			return c
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			path := writeConfig(t, tt.contents)

			got, err := Load(newTestFlagSet(t, "--config", path))
			if err != nil {
				t.Fatalf("Load() returned err: %v", err)
			}
			if want := tt.want(Defaults()); !reflect.DeepEqual(got.Config, want) {
				t.Errorf("Config = %+v, want %+v", got.Config, want)
			}
			if !reflect.DeepEqual(got.Report.LoadedFiles, []string{path}) {
				t.Errorf("LoadedFiles = %v, want [%s]", got.Report.LoadedFiles, path)
			}
		})
	}
}

func TestLoad_LayersPrecedence(t *testing.T) {
	path := writeConfig(t, `layers = ["file"]`)
	for _, tt := range []struct {
		name       string
		env        string
		args       []string
		wantLayers []string
		wantSource string
	}{
		{name: "env replaces file", env: "work, personal", wantLayers: []string{"work", "personal"}, wantSource: configloader.SourceEnv},
		{name: "empty env clears file", env: "", wantLayers: []string{}, wantSource: configloader.SourceEnv},
		{name: "layers flag replaces env", env: "env", args: []string{"--layers", "work,personal"}, wantLayers: []string{"work", "personal"}, wantSource: pflagloader.SourcePFlag},
		{name: "layer flags replace env", env: "env", args: []string{"--layer", "work", "--layer", "personal"}, wantLayers: []string{"work", "personal"}, wantSource: pflagloader.SourcePFlag},
		{name: "empty layers flag clears", env: "env", args: []string{"--layers", ""}, wantLayers: []string{}, wantSource: pflagloader.SourcePFlag},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("BREWKIT_LAYERS", tt.env)
			flags := newTestFlagSet(t, append([]string{"--config", path}, tt.args...)...)

			got, err := Load(flags)
			if err != nil {
				t.Fatalf("Load() returned err: %v", err)
			}
			if !reflect.DeepEqual(got.Layers, tt.wantLayers) {
				t.Errorf("Layers = %v, want %v", got.Layers, tt.wantLayers)
			}
			if got.Report.Updates["layers"] != tt.wantSource {
				t.Errorf("layers source = %q, want %q", got.Report.Updates["layers"], tt.wantSource)
			}
		})
	}
}

func TestLoad_EffectiveLayers(t *testing.T) {
	unsetEnv(t, "BREWKIT_LAYERS")
	for _, tt := range []struct {
		name     string
		contents string
		env      map[string]string
		args     []string
		want     []string
		wantErr  string
	}{
		{name: "raw layers", contents: `layers = ["work", "personal"]`, want: []string{"work", "personal"}},
		{name: "env_layers append in order", contents: "layers = [\"work\"]\nenv_layers = [\"A\", \"B\"]",
			env: map[string]string{"A": "personal, laptop", "B": "home"}, want: []string{"work", "personal", "laptop", "home"}},
		{name: "env_layers unset or empty add nothing", contents: "layers = [\"work\"]\nenv_layers = [\"UNSET_LAYERS\", \"EMPTY_LAYERS\"]",
			env: map[string]string{"EMPTY_LAYERS": ""}, want: []string{"work"}},
		{name: "env_layers blanks dropped", contents: `env_layers = ["A"]`,
			env: map[string]string{"A": "  work , , personal  "}, want: []string{"work", "personal"}},
		{name: "env_layers apply after flags clear", contents: "layers = [\"file\"]\nenv_layers = [\"A\"]",
			env: map[string]string{"A": "laptop"}, args: []string{"--layers", ""}, want: []string{"laptop"}},
		{name: "last occurrence wins", contents: "layers = [\"work\", \"personal\", \"work\"]\nenv_layers = [\"A\"]",
			env: map[string]string{"A": "personal,laptop,work"}, want: []string{"personal", "laptop", "work"}},
		{name: "blank raw items dropped", contents: `layers = ["work", "", " ", "laptop"]`, want: []string{"work", "laptop"}},
		{name: "padded TOML item is not trimmed", contents: `layers = [" work "]`, wantErr: `invalid layer name " work "`},
		{name: "env blank item dropped", args: []string{"--layers", "work,,laptop"}, want: []string{"work", "laptop"}},
		{name: "whitespace env value is empty", env: map[string]string{"BREWKIT_LAYERS": " "}, want: []string{}},
		{name: "local rejected", contents: `layers = ["local"]`, wantErr: `layer "local" is automatic`},
		{name: "base rejected", contents: `layers = ["base"]`, wantErr: `layer "base" is automatic`},
		{name: "local rejected from env_layers", contents: `env_layers = ["A"]`, env: map[string]string{"A": "local"}, wantErr: `layer "local" is automatic`},
		{name: "uppercase rejected", contents: `layers = ["Work"]`, wantErr: `invalid layer name "Work"`},
		{name: "underscore rejected from flag", args: []string{"--layer", "work_laptop"}, wantErr: `invalid layer name "work_laptop"`},
		{name: "blank env_layers name rejected", contents: `env_layers = [""]`, wantErr: `env_layers entry ""`},
		{name: "padded env_layers name rejected", contents: `env_layers = [" A"]`, wantErr: `env_layers entry " A"`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			for key, value := range tt.env {
				t.Setenv(key, value)
			}
			flags := newTestFlagSet(t, append([]string{"--config", writeConfig(t, tt.contents)}, tt.args...)...)

			got, err := Load(flags)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("Load() error = %v, want it to contain %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Load() returned err: %v", err)
			}
			if !reflect.DeepEqual(got.Layers, tt.want) {
				t.Errorf("Layers = %v, want %v", got.Layers, tt.want)
			}
		})
	}
}

func TestLoad_LayerOrder(t *testing.T) {
	unsetEnv(t, "BREWKIT_LAYERS")
	for _, tt := range []struct {
		name      string
		files     []string
		layers    string
		wantOrder []string
	}{
		{name: "no automatic layers", layers: `["work"]`, wantOrder: []string{"work"}},
		{name: "base from unsuffixed file", files: []string{"Tapfile"}, layers: `["work"]`, wantOrder: []string{"base", "work"}},
		{name: "local from suffixed file", files: []string{"Brewfile.local"}, layers: `["work"]`, wantOrder: []string{"work", "local"}},
		{name: "both around the layers", files: []string{"Caskfile", "Headfile.local"}, layers: `["work", "laptop"]`, wantOrder: []string{"base", "work", "laptop", "local"}},
		{name: "empty selection keeps automatic layers", files: []string{"Brewfile", "Brewfile.local"}, layers: `[]`, wantOrder: []string{"base", "local"}},
		{name: "suffixed base file is not base", files: []string{"Brewfile.base"}, layers: `[]`, wantOrder: []string{}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			for _, name := range tt.files {
				if err := os.WriteFile(filepath.Join(dir, name), nil, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			path := writeConfig(t, "dir = "+quote(dir)+"\nlayers = "+tt.layers+"\n")

			got, err := Load(newTestFlagSet(t, "--config", path))
			if err != nil {
				t.Fatalf("Load() returned err: %v", err)
			}
			if !reflect.DeepEqual(got.LayerOrder, tt.wantOrder) {
				t.Errorf("LayerOrder = %v, want %v", got.LayerOrder, tt.wantOrder)
			}
		})
	}
}

func newTestFlagSet(t *testing.T, args ...string) *pflag.FlagSet {
	t.Helper()
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	if err := RegisterFlags(flags); err != nil {
		t.Fatalf("RegisterFlags: %v", err)
	}
	if err := flags.Parse(args); err != nil {
		t.Fatalf("Parse(%v): %v", args, err)
	}
	return flags
}

func unsetEnv(t *testing.T, key string) {
	t.Helper()
	old, had := os.LookupEnv(key)
	if err := os.Unsetenv(key); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if had {
			_ = os.Setenv(key, old)
		} else {
			_ = os.Unsetenv(key)
		}
	})
}

// writeConfig writes contents to a brewkit.toml in a fresh temp directory
// and returns its path.
func writeConfig(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "brewkit.toml")
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func quote(s string) string {
	return `"` + s + `"`
}

func mustAbs(t *testing.T, path string) string {
	t.Helper()
	abs, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	return abs
}
