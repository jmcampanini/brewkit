// Package config loads brewkit's configuration from defaults, an optional
// brewkit.toml file, BREWKIT_* environment variables, and config-backed CLI
// flags, then derives and validates the values the commands consume.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jmcampanini/brewkit/internal/layer"
	"github.com/jmcampanini/go-config-loader/configloader"
	"github.com/jmcampanini/go-config-loader/pflagloader"
	"github.com/spf13/pflag"
)

const (
	appName        = "brewkit"
	configFileName = "brewkit.toml"
	configFlagName = "config"
)

// Config is brewkit's raw loaded configuration, the value the config
// command prints as TOML. Effective carries the values derived from it.
type Config struct {
	// Dir is the directory holding the layer files
	// (Brewfile[.<layer>], Caskfile[.<layer>], Headfile[.<layer>],
	// Tapfile[.<layer>]). Relative paths are resolved by the process
	// current working directory, not by the location of brewkit.toml.
	Dir string `toml:"dir"`

	// Layers is the raw layer list. BREWKIT_LAYERS, --layers, and repeated
	// --layer flags each replace it.
	Layers []string `toml:"layers" config:"layers" pflag_singular:"layer" help:"layers to select, comma-separated; replaces brewkit.toml and BREWKIT_LAYERS"`

	// EnvLayers names environment variables whose comma-separated values
	// append layers to Layers. It is set from the file only.
	EnvLayers []string `toml:"env_layers"`

	// FailFast stops execution at the first error when true; collects and
	// reports all errors at the end when false.
	FailFast bool `toml:"fail_fast"`
}

// Effective is the validated configuration every command consumes.
type Effective struct {
	// Config is the raw configuration after defaults, the file, the
	// environment, and flags have been applied.
	Config Config

	// Dir is Config.Dir made absolute.
	Dir string

	// Layers is the effective layers: the raw layers plus the env_layers
	// additions, keeping the last occurrence of each name. It never holds
	// base or local.
	Layers []string

	// LayerOrder is the list the apply commands walk: base when an
	// unsuffixed file exists, then Layers, then local when a *file.local
	// exists.
	LayerOrder []string

	// Report records which source set each field.
	Report LoadReport
}

// LoadReport is the provenance report returned by the shared config loader.
type LoadReport = configloader.LoadReport

// Defaults returns the zero-config defaults used when no brewkit.toml is
// found and no environment variables or flags override.
func Defaults() Config {
	return Config{
		Dir:       ".",
		Layers:    []string{},
		EnvLayers: []string{},
		FailFast:  true,
	}
}

// RegisterFlags registers --config and brewkit's config-backed flags on flags.
func RegisterFlags(flags *pflag.FlagSet) error {
	flags.String(configFlagName, "", "config file (default ./brewkit.toml when present)")
	return pflagloader.Register[Config](flags)
}

// Load applies defaults, brewkit.toml, BREWKIT_* environment variables, and
// the flags registered by RegisterFlags, then derives and validates the
// effective values. A --config path the person supplied must name a readable
// file, even when it is empty; otherwise ./brewkit.toml is read when present.
// flags may be nil, in which case no flag is applied.
func Load(flags *pflag.FlagSet) (Effective, error) {
	cfg, report, err := loadRaw(flags)
	if err != nil {
		return Effective{}, err
	}

	if cfg.Dir == "" {
		return Effective{}, fmt.Errorf("dir must not be empty")
	}
	dir, err := filepath.Abs(cfg.Dir)
	if err != nil {
		return Effective{}, fmt.Errorf("resolve effective dir %q: %w", cfg.Dir, err)
	}

	layers, err := effectiveLayers(cfg)
	if err != nil {
		return Effective{}, err
	}
	order, err := layerOrder(dir, layers)
	if err != nil {
		return Effective{}, err
	}

	return Effective{
		Config:     cfg,
		Dir:        dir,
		Layers:     layers,
		LayerOrder: order,
		Report:     report,
	}, nil
}

func loadRaw(flags *pflag.FlagSet) (Config, LoadReport, error) {
	fileLoader, err := newFileLoader(flags)
	if err != nil {
		return Config{}, LoadReport{}, err
	}
	envLoader, err := configloader.NewEnvironmentLoader[Config](appName, configloader.OSEnv())
	if err != nil {
		return Config{}, LoadReport{}, err
	}

	loaders := []configloader.ConfigLoader[Config]{fileLoader, envLoader}
	if flags != nil {
		flagLoader, err := pflagloader.NewLoader[Config](flags)
		if err != nil {
			return Config{}, LoadReport{}, err
		}
		loaders = append(loaders, flagLoader)
	}

	return configloader.Load(Defaults(), loaders...)
}

func newFileLoader(flags *pflag.FlagSet) (configloader.ConfigLoader[Config], error) {
	if flags != nil && flags.Changed(configFlagName) {
		path := flags.Lookup(configFlagName).Value.String()
		if path == "" {
			return nil, fmt.Errorf("--%s: path must not be empty", configFlagName)
		}
		return configloader.NewRequiredFileLoader[Config](path)
	}

	abs, err := filepath.Abs(configFileName)
	if err != nil {
		return nil, fmt.Errorf("resolve %s: %w", configFileName, err)
	}
	info, err := os.Stat(abs)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("stat implicit config file %q: %w", abs, err)
	}
	if err == nil && info.IsDir() {
		return nil, fmt.Errorf("config file %q is a directory", abs)
	}
	return configloader.NewPickLastFileLoader[Config](configloader.File(abs))
}

// effectiveLayers combines the raw layers with the env_layers additions,
// drops blank items, keeps the last occurrence of each name, and validates
// every survivor. Items from the environment and flags arrive trimmed; a
// TOML item is used exactly as written.
func effectiveLayers(cfg Config) ([]string, error) {
	combined := append([]string(nil), cfg.Layers...)
	for _, variable := range cfg.EnvLayers {
		if variable == "" || strings.TrimSpace(variable) != variable {
			return nil, fmt.Errorf("env_layers entry %q must be a variable name without surrounding whitespace", variable)
		}
		for _, item := range strings.Split(os.Getenv(variable), ",") {
			combined = append(combined, strings.TrimSpace(item))
		}
	}

	layers := keepLastOccurrence(dropBlank(combined))
	for _, name := range layers {
		if err := layer.ValidateSelected(name); err != nil {
			return nil, err
		}
	}
	return layers, nil
}

func layerOrder(dir string, layers []string) ([]string, error) {
	hasBase, err := layer.HasFiles(dir, layer.BaseName)
	if err != nil {
		return nil, err
	}
	hasLocal, err := layer.HasFiles(dir, layer.LocalName)
	if err != nil {
		return nil, err
	}

	order := make([]string, 0, len(layers)+2)
	if hasBase {
		order = append(order, layer.BaseName)
	}
	order = append(order, layers...)
	if hasLocal {
		order = append(order, layer.LocalName)
	}
	return order, nil
}

// dropBlank returns the items of values that hold more than whitespace.
func dropBlank(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			out = append(out, value)
		}
	}
	return out
}

func keepLastOccurrence(values []string) []string {
	lastIndex := make(map[string]int, len(values))
	for i, value := range values {
		lastIndex[value] = i
	}

	out := make([]string, 0, len(lastIndex))
	for i, value := range values {
		if lastIndex[value] == i {
			out = append(out, value)
		}
	}
	return out
}
