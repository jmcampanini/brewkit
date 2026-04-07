// Package brew provides the Brewer interface that brewkit uses to talk to
// Homebrew, plus a real implementation that shells out to `brew` and a
// fake implementation suitable for tests.
package brew

import (
	"context"
)

// State is a bulk snapshot of the local Homebrew installation, fetched
// once at the start of a command so per-item idempotency checks don't
// shell out per entry.
type State struct {
	Taps     map[string]bool
	Formulas map[string]FormulaState
	Casks    map[string]CaskState
}

type FormulaState struct {
	Installed  bool
	Version    string
	IsHead     bool
	Outdated   bool
	OutdatedTo string
}

type CaskState struct {
	Installed  bool
	Version    string
	Outdated   bool
	OutdatedTo string
}

// Result captures combined stdout/stderr (for --verbose rendering and
// failure reporting) plus optional From/To version info.
type Result struct {
	From   string
	To     string
	Output string
}

// Brewer is the surface brewkit needs from Homebrew. The Exec
// implementation shells out to `brew`; Fake satisfies the same interface
// with in-memory state for tests.
type Brewer interface {
	State(ctx context.Context) (*State, error)

	// Tap accepts an optional url for taps not in Homebrew's default index.
	Tap(ctx context.Context, name, url string) (Result, error)

	BrewInstall(ctx context.Context, name string) (Result, error)
	BrewUpgrade(ctx context.Context, name string) (Result, error)

	HeadInstall(ctx context.Context, name string) (Result, error)
	// HeadReinstall removes the current install and reinstalls from
	// HEAD. Invoked both when the installed HEAD SHA has moved and when
	// the formula is currently installed at a non-HEAD version. If the
	// uninstall step succeeds and the install step fails, the formula
	// is left MISSING from the system — implementations must make that
	// clear in the returned error.
	HeadReinstall(ctx context.Context, name string) (Result, error)
	// HeadInstalledSHA returns the short SHA of the installed HEAD build.
	// installedAsHead is false if a non-HEAD version is installed;
	// installed is false if nothing is installed.
	HeadInstalledSHA(ctx context.Context, name string) (sha string, installedAsHead bool, installed bool, err error)
	// HeadLatestSHA returns hasHead=false with nil error if the formula
	// has no HEAD source defined upstream. On any other error, hasHead
	// is unspecified and callers must check err first.
	HeadLatestSHA(ctx context.Context, name string) (sha string, hasHead bool, err error)

	CaskInstall(ctx context.Context, name string) (Result, error)
	// CaskUpgrade applies --greedy.
	CaskUpgrade(ctx context.Context, name string) (Result, error)
}

// Compile-time guarantees that the production implementations satisfy
// Brewer. Placed here (not in brew_test.go) so a non-test `go build`
// catches interface drift at the package level.
var (
	_ Brewer = (*Exec)(nil)
	_ Brewer = (*Fake)(nil)
)

// EmptyState returns a State with non-nil empty maps.
func EmptyState() *State {
	return &State{
		Taps:     map[string]bool{},
		Formulas: map[string]FormulaState{},
		Casks:    map[string]CaskState{},
	}
}
