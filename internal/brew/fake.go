package brew

import (
	"context"
	"errors"
	"fmt"
)

// Fake is an in-memory Brewer for tests. Tests construct a Fake, populate
// the maps with the desired starting state, then pass it to a runtime
// that calls action methods. Action methods mutate the in-memory state
// and record what was called.
type Fake struct {
	// Initial / current state.
	TapsSet      map[string]bool
	FormulasMap  map[string]FormulaState
	CasksMap     map[string]CaskState
	HeadInstalls map[string]string // formula → installed SHA (empty if none)
	HeadLatest   map[string]string // formula → latest SHA available
	HeadHasURL   map[string]bool   // formula → has HEAD source

	// Recording of calls (in order).
	Calls []FakeCall

	// Optional failure injection: if a name is in FailOps[op], the
	// corresponding action returns an error and is not applied.
	FailOps map[FakeOp]map[string]bool
}

// FakeOp is the typed set of operations a Fake records. Using a named
// string type means test assertions can't silently mismatch on typos.
type FakeOp string

// One FakeOp per mutating Brewer method.
const (
	OpTap           FakeOp = "tap"
	OpBrewInstall   FakeOp = "brew-install"
	OpBrewUpgrade   FakeOp = "brew-upgrade"
	OpHeadInstall   FakeOp = "head-install"
	OpHeadReinstall FakeOp = "head-reinstall"
	OpCaskInstall   FakeOp = "cask-install"
	OpCaskUpgrade   FakeOp = "cask-upgrade"
)

// FakeCall records one Brewer method invocation.
type FakeCall struct {
	Op   FakeOp
	Name string
	Arg  string // url for tap, empty otherwise
}

// NewFake returns a Fake with empty state.
func NewFake() *Fake {
	return &Fake{
		TapsSet:      map[string]bool{},
		FormulasMap:  map[string]FormulaState{},
		CasksMap:     map[string]CaskState{},
		HeadInstalls: map[string]string{},
		HeadLatest:   map[string]string{},
		HeadHasURL:   map[string]bool{},
		FailOps:      map[FakeOp]map[string]bool{},
	}
}

func (f *Fake) shouldFail(op FakeOp, name string) bool {
	return f.FailOps[op][name]
}

func (f *Fake) record(op FakeOp, name, arg string) {
	f.Calls = append(f.Calls, FakeCall{Op: op, Name: name, Arg: arg})
}

// State returns a copy of the fake's in-memory snapshot.
func (f *Fake) State(_ context.Context) (*State, error) {
	out := EmptyState()
	for k, v := range f.TapsSet {
		out.Taps[k] = v
	}
	for k, v := range f.FormulasMap {
		out.Formulas[k] = v
	}
	for k, v := range f.CasksMap {
		out.Casks[k] = v
	}
	return out, nil
}

// Tap records the call and marks the tap as registered.
func (f *Fake) Tap(_ context.Context, name, url string) (Result, error) {
	f.record(OpTap, name, url)
	if f.shouldFail(OpTap, name) {
		return Result{}, fmt.Errorf("fake: tap %s failed", name)
	}
	f.TapsSet[name] = true
	return Result{To: name}, nil
}

// BrewInstall records the call and marks the formula installed.
func (f *Fake) BrewInstall(_ context.Context, name string) (Result, error) {
	f.record(OpBrewInstall, name, "")
	if f.shouldFail(OpBrewInstall, name) {
		return Result{}, fmt.Errorf("fake: brew install %s failed", name)
	}
	f.FormulasMap[name] = FormulaState{Installed: true, Version: "1.0.0"}
	return Result{To: name}, nil
}

// BrewUpgrade records the call and moves the formula to its OutdatedTo version.
func (f *Fake) BrewUpgrade(_ context.Context, name string) (Result, error) {
	f.record(OpBrewUpgrade, name, "")
	if f.shouldFail(OpBrewUpgrade, name) {
		return Result{}, fmt.Errorf("fake: brew upgrade %s failed", name)
	}
	prev := f.FormulasMap[name]
	upgraded := FormulaState{Installed: true, Version: prev.OutdatedTo}
	f.FormulasMap[name] = upgraded
	return Result{From: prev.Version, To: upgraded.Version}, nil
}

// HeadInstall records the call and installs the configured latest HEAD SHA.
func (f *Fake) HeadInstall(_ context.Context, name string) (Result, error) {
	f.record(OpHeadInstall, name, "")
	if f.shouldFail(OpHeadInstall, name) {
		return Result{}, fmt.Errorf("fake: head install %s failed", name)
	}
	sha := f.HeadLatest[name]
	if sha == "" {
		sha = "newhead"
	}
	f.HeadInstalls[name] = sha
	f.FormulasMap[name] = FormulaState{
		Installed: true,
		Version:   "HEAD-" + sha,
		IsHead:    true,
	}
	return Result{To: "HEAD-" + sha}, nil
}

// HeadReinstall records the call and replaces the installed HEAD SHA with the latest.
func (f *Fake) HeadReinstall(_ context.Context, name string) (Result, error) {
	f.record(OpHeadReinstall, name, "")
	if f.shouldFail(OpHeadReinstall, name) {
		return Result{}, fmt.Errorf("fake: head reinstall %s failed", name)
	}
	prev := f.HeadInstalls[name]
	sha := f.HeadLatest[name]
	if sha == "" {
		sha = "newhead"
	}
	f.HeadInstalls[name] = sha
	f.FormulasMap[name] = FormulaState{
		Installed: true,
		Version:   "HEAD-" + sha,
		IsHead:    true,
	}
	return Result{From: "HEAD-" + prev, To: "HEAD-" + sha}, nil
}

// HeadInstalledSHA reports the fake's installed HEAD SHA, if any.
func (f *Fake) HeadInstalledSHA(_ context.Context, name string) (string, bool, bool, error) {
	if sha, ok := f.HeadInstalls[name]; ok && sha != "" {
		return sha, true, true, nil
	}
	if fs, ok := f.FormulasMap[name]; ok && fs.Installed {
		// Installed but not as HEAD.
		return "", false, true, nil
	}
	return "", false, false, nil
}

// HeadLatestSHA reports the fake's configured latest HEAD SHA, if any.
func (f *Fake) HeadLatestSHA(_ context.Context, name string) (string, bool, error) {
	if !f.HeadHasURL[name] {
		// Default: assume head source exists if we know a latest sha.
		if _, ok := f.HeadLatest[name]; !ok {
			return "", false, nil
		}
	}
	sha, ok := f.HeadLatest[name]
	if !ok {
		return "", true, errors.New("fake: no latest sha configured")
	}
	return sha, true, nil
}

// CaskInstall records the call and marks the cask installed.
func (f *Fake) CaskInstall(_ context.Context, name string) (Result, error) {
	f.record(OpCaskInstall, name, "")
	if f.shouldFail(OpCaskInstall, name) {
		return Result{}, fmt.Errorf("fake: cask install %s failed", name)
	}
	f.CasksMap[name] = CaskState{Installed: true, Version: "1.0.0"}
	return Result{To: name}, nil
}

// CaskUpgrade records the call and moves the cask to its OutdatedTo version.
func (f *Fake) CaskUpgrade(_ context.Context, name string) (Result, error) {
	f.record(OpCaskUpgrade, name, "")
	if f.shouldFail(OpCaskUpgrade, name) {
		return Result{}, fmt.Errorf("fake: cask upgrade %s failed", name)
	}
	prev := f.CasksMap[name]
	upgraded := CaskState{Installed: true, Version: prev.OutdatedTo}
	f.CasksMap[name] = upgraded
	return Result{From: prev.Version, To: upgraded.Version}, nil
}
