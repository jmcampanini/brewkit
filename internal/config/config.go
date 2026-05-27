// Package config loads brewkit's raw configuration from defaults,
// an optional brewkit.toml file, B... environment variables, and
// config-backed CLI flags.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	configloader "github.com/jmcampanini/go-config-loader"
	"github.com/jmcampanini/go-config-loader/pflagloader"
	"github.com/spf13/pflag"
)

const (
	appName        = "brewkit"
	configFileName = "brewkit.toml"
)

// Config is brewkit's raw loaded configuration. Runtime behavior such as
// env_profiles expansion, local-profile auto-append, and empty-profile apply
// validation is derived outside this package.
type Config struct {
	// Dir is the directory holding the profile files
	// (Brewfile.<p>, Caskfile.<p>, Headfile.<p>, Tapfile.<p>).
	// Relative paths are intentionally resolved by the process current working
	// directory at use time, not by the location of brewkit.toml.
	Dir string `toml:"dir"`

	// Profiles is the raw active profile list. It may be overridden by
	// BREWKIT_PROFILES or by --profiles.
	Profiles []string `toml:"profiles" config:"profiles" help:"active profiles as a comma-separated list; overrides file/env profiles"`

	// EnvProfiles names an additional environment variable whose comma-separated
	// values append to Profiles during runtime derivation.
	EnvProfiles string `toml:"env_profiles"`

	// FailFast stops execution at the first error when true; collects and
	// reports all errors at the end when false.
	FailFast bool `toml:"fail_fast"`
}

// LoadReport is the provenance report returned by the shared config loader.
type LoadReport = configloader.LoadReport

// Defaults returns the zero-config defaults used when no brewkit.toml is
// found and no environment variables or flags override.
func Defaults() Config {
	return Config{
		Dir:         ".",
		Profiles:    []string{},
		EnvProfiles: "",
		FailFast:    true,
	}
}

// RegisterFlags registers brewkit's config-backed flags on flags.
func RegisterFlags(flags *pflag.FlagSet) error {
	return pflagloader.Register[Config](flags)
}

// Load reads brewkit.toml from path (or, if path is empty, from
// ./brewkit.toml in the current working directory) and overlays B... env vars.
// If path is empty and no file exists, Defaults is used. A non-empty path that
// does not exist is an error.
func Load(path string) (Config, error) {
	cfg, _, err := LoadWithReport(path, nil)
	return cfg, err
}

// LoadWithReport loads config and returns the shared library's provenance
// report. When flags is non-nil, changed config-backed pflags override file and
// environment values.
func LoadWithReport(path string, flags *pflag.FlagSet) (Config, LoadReport, error) {
	fileLoader, err := newFileLoader(path)
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

	cfg, report, err := configloader.Load(Defaults(), loaders...)
	if err != nil {
		return Config{}, LoadReport{}, err
	}
	if cfg.Dir == "" {
		return Config{}, LoadReport{}, fmt.Errorf("dir must not be empty")
	}
	return cfg, report, nil
}

func newFileLoader(path string) (configloader.ConfigLoader[Config], error) {
	if path != "" {
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
