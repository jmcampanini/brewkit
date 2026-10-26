// Package layer names brewkit's layers, validates layer names, and locates
// the layer files (Brewfile/Caskfile/Headfile/Tapfile) on disk.
package layer

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// The automatic layers apply when their files exist and cannot be selected.
const (
	// BaseName is the layer an unsuffixed file (Brewfile, Caskfile,
	// Headfile, Tapfile) belongs to.
	BaseName = "base"
	// LocalName is the layer the *file.local files belong to.
	LocalName = "local"
)

// MaxNameLength is the longest layer name brewkit accepts.
const MaxNameLength = 64

var namePattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// Kind identifies one of the layer file types brewkit manages.
type Kind int

// The four layer file kinds, in declaration order only; use AllKinds
// for processing order.
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

// FilenamePrefix returns the file name prefix for the kind, e.g. "Tapfile".
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

// FilenameFor returns the file name for a kind/layer pair, e.g. "Tapfile.work".
// The base layer's file carries no suffix: FilenameFor(KindTap, BaseName) is
// "Tapfile".
func FilenameFor(k Kind, name string) string {
	if name == BaseName {
		return k.FilenamePrefix()
	}
	return k.FilenamePrefix() + "." + name
}

// PathFor returns the full path of a kind/layer file inside dir.
func PathFor(dir string, k Kind, name string) string {
	return filepath.Join(dir, FilenameFor(k, name))
}

// ValidName reports whether name matches the layer name grammar: 1 to 64
// ASCII characters of lowercase letters or digits in nonempty groups
// separated by single hyphens. The automatic names base and local match
// the grammar; callers that select layers reject them separately.
func ValidName(name string) bool {
	return len(name) <= MaxNameLength && namePattern.MatchString(name)
}

// ValidateSelected returns an error when name cannot be selected as a
// layer: it fails the grammar or is one of the automatic layers.
func ValidateSelected(name string) error {
	if !ValidName(name) {
		return fmt.Errorf("invalid layer name %q: use 1 to 64 lowercase letters or digits, with single hyphens between groups, such as work or work-laptop", name)
	}
	if name == BaseName || name == LocalName {
		return fmt.Errorf("layer %q is automatic and applies whenever its files exist; remove it from the layers", name)
	}
	return nil
}

// HasFiles reports whether any path of the named layer exists in dir. A
// directory counts, so the command that reads it reports the error.
func HasFiles(dir, name string) (bool, error) {
	for _, k := range AllKinds {
		path := PathFor(dir, k, name)
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

// Discover returns every layer name that has a file in dir, sorted,
// including base for an unsuffixed file and local when present. brewkit
// lint uses this to scan every file regardless of which layers are
// selected.
//
// A suffixed file declares a layer only when its suffix matches the layer
// name grammar, which keeps editor swap files (Brewfile.foo.swp), backups
// (Brewfile.foo.bak, Brewfile.foo~), and similar leftovers out. A file
// suffixed with base is not a layer file: the base layer is the unsuffixed
// file.
func Discover(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", dir, err)
	}

	prefixes := make(map[string]struct{}, len(AllKinds))
	for _, k := range AllKinds {
		prefixes[k.FilenamePrefix()] = struct{}{}
	}

	seen := make(map[string]struct{})
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		prefix, suffix, suffixed := strings.Cut(entry.Name(), ".")
		if _, known := prefixes[prefix]; !known {
			continue
		}
		switch {
		case !suffixed:
			seen[BaseName] = struct{}{}
		case suffix != BaseName && ValidName(suffix):
			seen[suffix] = struct{}{}
		}
	}

	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}
