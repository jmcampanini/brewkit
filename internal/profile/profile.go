// Package profile resolves the effective list of active brewkit profiles
// and locates the profile files (Brewfile/Caskfile/Headfile/Tapfile) on disk.
package profile

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jmcampanini/brewkit/internal/config"
)

// LocalName is reserved: users may not list it explicitly. It is
// auto-appended whenever any *file.local exists in the configured dir.
const LocalName = "local"

type Kind int

const (
	KindTap Kind = iota
	KindBrew
	KindHead
	KindCask
)

// AllKinds is the canonical processing order: taps must be registered
// before formulas can pull from them.
var AllKinds = []Kind{KindTap, KindBrew, KindHead, KindCask}

func (k Kind) String() string {
	switch k {
	case KindTap:
		return "tap"
	case KindBrew:
		return "brew"
	case KindHead:
		return "head"
	case KindCask:
		return "cask"
	}
	return "unknown"
}

func (k Kind) FilenamePrefix() string {
	switch k {
	case KindTap:
		return "Tapfile"
	case KindBrew:
		return "Brewfile"
	case KindHead:
		return "Headfile"
	case KindCask:
		return "Caskfile"
	}
	return ""
}

func FilenameFor(k Kind, profile string) string {
	return k.FilenamePrefix() + "." + profile
}

func PathFor(dir string, k Kind, profile string) string {
	return filepath.Join(dir, FilenameFor(k, profile))
}

// Resolve computes the effective list of active profiles given the loaded raw
// config. Config loading has already applied file, B...PROFILES, and
// --profiles precedence to cfg.Profiles. Runtime derivation appends profiles
// from the environment variable named by cfg.EnvProfiles, de-duplicates while
// preserving the first occurrence, validates profile names, then auto-appends
// the reserved "local" profile if any *file.local exists in cfg.Dir.
func Resolve(cfg config.Config) ([]string, error) {
	effective := append([]string(nil), cfg.Profiles...)
	if cfg.EnvProfiles != "" {
		if val, ok := os.LookupEnv(cfg.EnvProfiles); ok {
			effective = append(effective, parseEnvList(val)...)
		}
	}
	effective = dedupePreserveOrder(effective)

	for _, p := range effective {
		if p == LocalName {
			return nil, fmt.Errorf("profile %q is reserved and applied automatically when *file.local exists; remove it from your profile list", LocalName)
		}
		if p == "" {
			return nil, fmt.Errorf("empty profile name in active list")
		}
	}

	hasLocal, err := HasLocalFiles(cfg.Dir)
	if err != nil {
		return nil, err
	}
	if hasLocal {
		effective = append(effective, LocalName)
	}

	return effective, nil
}

// HasLocalFiles reports whether any *file.local exists in dir, which
// triggers the reserved "local" profile.
func HasLocalFiles(dir string) (bool, error) {
	for _, k := range AllKinds {
		path := PathFor(dir, k, LocalName)
		_, err := os.Stat(path)
		switch {
		case err == nil:
			return true, nil
		case errors.Is(err, fs.ErrNotExist):
			continue
		default:
			return false, fmt.Errorf("stat %s: %w", path, err)
		}
	}
	return false, nil
}

// Discover returns every profile name found in dir, sorted, including
// the reserved "local" name when present. brewkit lint uses this to
// scan every file regardless of which profiles are currently active.
//
// A file matches a profile only when its suffix is a "clean" name —
// alphanumerics, underscores, and hyphens. This rejects editor swap
// files (Brewfile.foo.swp), backups (Brewfile.foo.bak, Brewfile.foo~),
// and similar leftovers from being silently treated as profiles.
func Discover(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", dir, err)
	}

	seen := make(map[string]struct{})
	prefixes := make(map[string]struct{}, len(AllKinds))
	for _, k := range AllKinds {
		prefixes[k.FilenamePrefix()] = struct{}{}
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		prefix, suffix, ok := strings.Cut(entry.Name(), ".")
		if !ok || !validProfileName(suffix) {
			continue
		}
		if _, known := prefixes[prefix]; !known {
			continue
		}
		seen[suffix] = struct{}{}
	}

	out := make([]string, 0, len(seen))
	for p := range seen {
		out = append(out, p)
	}
	sort.Strings(out)
	return out, nil
}

func validProfileName(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		switch {
		case c >= 'a' && c <= 'z':
		case c >= 'A' && c <= 'Z':
		case c >= '0' && c <= '9':
		case c == '_' || c == '-':
		default:
			return false
		}
	}
	return true
}

func dedupePreserveOrder(values []string) []string {
	if len(values) < 2 {
		return values
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func parseEnvList(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
