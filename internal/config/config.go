// Package config loads and merges brewkit configuration from defaults,
// an optional brewkit.toml file, and CLI flag overrides.
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
)

// Config is the effective brewkit configuration after merging all layers.
type Config struct {
	// Dir is the directory holding the profile files
	// (Brewfile.<p>, Caskfile.<p>, Headfile.<p>, Tapfile.<p>).
	Dir string `toml:"dir"`

	// Profiles is the default list of active profiles. May be overridden
	// by the env var named in ProfilesEnv or by --profile flags.
	Profiles []string `toml:"profiles"`

	// ProfilesEnv is the name of the environment variable brewkit reads
	// for profile overrides. Empty disables env-based override.
	ProfilesEnv string `toml:"profiles_env"`

	// FailFast stops execution at the first error when true; collects and
	// reports all errors at the end when false.
	FailFast bool `toml:"fail_fast"`

	// Source records where the config came from for debugging.
	Source string `toml:"-"`
}

// Defaults returns the zero-config defaults used when no brewkit.toml is
// found and no flags override.
func Defaults() Config {
	return Config{
		Dir:         ".",
		Profiles:    []string{"common"},
		ProfilesEnv: "BREWKIT_PROFILES",
		FailFast:    true,
		Source:      "defaults",
	}
}

// fileConfig mirrors Config with pointer fields so we can tell which keys
// the user explicitly set vs. which keys are simply absent.
type fileConfig struct {
	Dir         *string   `toml:"dir"`
	Profiles    *[]string `toml:"profiles"`
	ProfilesEnv *string   `toml:"profiles_env"`
	FailFast    *bool     `toml:"fail_fast"`
}

// Load reads brewkit.toml from path (or, if path is empty, from
// ./brewkit.toml in the current working directory). If path is empty
// and no file exists, returns Defaults() with no error. A non-empty
// path that does not exist returns an error.
//
// Non-empty config files always resolve their `dir` key relative to
// the directory that contains the config file, not the process CWD —
// whether `dir` is set explicitly (`dir = "./homebrew"`) or omitted
// (treated as `dir = "."`). This makes `--config /elsewhere/foo.toml`
// locate profile files next to the config, which is the common
// production case.
//
// A completely empty file (0 bytes, e.g. `/dev/null` or a stubbed-out
// placeholder) short-circuits to pure Defaults() so users can still
// run `brewkit --config /dev/null config` to inspect the documented
// defaults without the dir resolution kicking in.
func Load(path string) (Config, error) {
	cfg := Defaults()

	resolved, explicit := path, path != ""
	if !explicit {
		cwd, err := os.Getwd()
		if err != nil {
			return Config{}, fmt.Errorf("getwd: %w", err)
		}
		resolved = filepath.Join(cwd, "brewkit.toml")
	}

	data, err := os.ReadFile(resolved)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) && !explicit {
			return cfg, nil
		}
		return Config{}, fmt.Errorf("read %s: %w", resolved, err)
	}

	// Zero-byte file → pure Defaults (preserves the /dev/null idiom).
	// Whitespace/comment-only files do NOT short-circuit here — once
	// the user has bothered to write a non-empty file we treat it as
	// real config and resolve `dir` against the file's location, even
	// if every TOML key is absent.
	if len(data) == 0 {
		cfg.Source = resolved
		return cfg, nil
	}

	var raw fileConfig
	if err := toml.Unmarshal(data, &raw); err != nil {
		return Config{}, fmt.Errorf("parse %s: %w", resolved, err)
	}
	cfg.Source = resolved

	dirStr := "."
	if raw.Dir != nil && *raw.Dir != "" {
		dirStr = *raw.Dir
	}
	if filepath.IsAbs(dirStr) {
		cfg.Dir = dirStr
	} else {
		cfg.Dir = filepath.Clean(filepath.Join(filepath.Dir(resolved), dirStr))
	}

	if raw.Profiles != nil {
		cfg.Profiles = *raw.Profiles
	}
	if raw.ProfilesEnv != nil {
		cfg.ProfilesEnv = *raw.ProfilesEnv
	}
	if raw.FailFast != nil {
		cfg.FailFast = *raw.FailFast
	}
	return cfg, nil
}

// MarshalTOML renders the effective config as a TOML document for
// `brewkit config`. Only fields with explicit TOML tags are emitted.
func (c Config) MarshalTOML() ([]byte, error) {
	return toml.Marshal(c)
}
