package brew

import (
	"context"
	"encoding/json"
	"testing"
)

func TestFake_State(t *testing.T) {
	f := NewFake()
	f.TapsSet["charmbracelet/tap"] = true
	f.FormulasMap["git"] = FormulaState{Installed: true, Version: "2.45.0"}
	f.CasksMap["ghostty"] = CaskState{Installed: true, Version: "1.0.0"}

	st, err := f.State(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !st.Taps["charmbracelet/tap"] {
		t.Error("missing tap")
	}
	if !st.Formulas["git"].Installed {
		t.Error("missing formula")
	}
	if !st.Casks["ghostty"].Installed {
		t.Error("missing cask")
	}
}

func TestFake_Tap_AddsAndRecords(t *testing.T) {
	f := NewFake()
	if _, err := f.Tap(context.Background(), "user/repo", "https://example.com"); err != nil {
		t.Fatal(err)
	}
	if !f.TapsSet["user/repo"] {
		t.Error("tap not registered")
	}
	if len(f.Calls) != 1 || f.Calls[0].Op != OpTap || f.Calls[0].Name != "user/repo" || f.Calls[0].Arg != "https://example.com" {
		t.Errorf("unexpected calls: %+v", f.Calls)
	}
}

func TestFake_BrewInstall(t *testing.T) {
	f := NewFake()
	if _, err := f.BrewInstall(context.Background(), "ripgrep"); err != nil {
		t.Fatal(err)
	}
	if !f.FormulasMap["ripgrep"].Installed {
		t.Error("formula not installed")
	}
}

func TestFake_BrewUpgrade(t *testing.T) {
	f := NewFake()
	f.FormulasMap["neovim"] = FormulaState{
		Installed: true, Version: "0.10.0", Outdated: true, OutdatedTo: "0.10.2",
	}
	r, err := f.BrewUpgrade(context.Background(), "neovim")
	if err != nil {
		t.Fatal(err)
	}
	if r.From != "0.10.0" || r.To != "0.10.2" {
		t.Errorf("Result = %+v", r)
	}
	if f.FormulasMap["neovim"].Version != "0.10.2" {
		t.Errorf("post-upgrade version: %s", f.FormulasMap["neovim"].Version)
	}
}

func TestFake_HeadInstall(t *testing.T) {
	f := NewFake()
	f.HeadLatest["tmux"] = "abcdef0"
	if _, err := f.HeadInstall(context.Background(), "tmux"); err != nil {
		t.Fatal(err)
	}
	if f.HeadInstalls["tmux"] != "abcdef0" {
		t.Errorf("HeadInstalls[tmux] = %q", f.HeadInstalls["tmux"])
	}
	fs := f.FormulasMap["tmux"]
	if !fs.IsHead || fs.Version != "HEAD-abcdef0" {
		t.Errorf("formula state = %+v", fs)
	}
}

func TestFake_HeadInstalledSHA(t *testing.T) {
	f := NewFake()
	f.HeadInstalls["tmux"] = "abcdef0"
	f.FormulasMap["tmux"] = FormulaState{Installed: true, Version: "HEAD-abcdef0", IsHead: true}

	sha, asHead, installed, err := f.HeadInstalledSHA(context.Background(), "tmux")
	if err != nil {
		t.Fatal(err)
	}
	if sha != "abcdef0" || !asHead || !installed {
		t.Errorf("got (%q, %v, %v)", sha, asHead, installed)
	}

	// Not installed at all.
	sha, asHead, installed, err = f.HeadInstalledSHA(context.Background(), "missing")
	if err != nil {
		t.Fatal(err)
	}
	if installed || asHead || sha != "" {
		t.Errorf("got (%q, %v, %v) for missing", sha, asHead, installed)
	}

	// Installed but not as head.
	f.FormulasMap["direnv"] = FormulaState{Installed: true, Version: "2.37.1"}
	sha, asHead, installed, err = f.HeadInstalledSHA(context.Background(), "direnv")
	if err != nil {
		t.Fatal(err)
	}
	if !installed || asHead {
		t.Errorf("got (%q, %v, %v) for non-head install", sha, asHead, installed)
	}
}

func TestOutdatedJSON_CaskInstalledVersionsIsArray(t *testing.T) {
	// Realistic shape from `brew outdated --cask --greedy --json=v2`:
	// installed_versions is an ARRAY, not a string. A previous version
	// of the parser declared it as a string and would fail on any
	// real outdated cask payload.
	payload := []byte(`{
        "casks": [
            {
                "name": "ghostty",
                "installed_versions": ["1.0.0"],
                "current_version": "1.1.0"
            }
        ]
    }`)
	var got outdatedJSON
	if err := json.Unmarshal(payload, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(got.Casks) != 1 {
		t.Fatalf("got %d casks, want 1", len(got.Casks))
	}
	c := got.Casks[0]
	if c.Name != "ghostty" || c.CurrentVersion != "1.1.0" {
		t.Errorf("unexpected cask fields: %+v", c)
	}
	if len(c.InstalledVersions) != 1 || c.InstalledVersions[0] != "1.0.0" {
		t.Errorf("InstalledVersions = %v, want [1.0.0]", c.InstalledVersions)
	}
}

func TestFake_FailureInjection(t *testing.T) {
	f := NewFake()
	f.FailOps[OpBrewInstall] = map[string]bool{"borked": true}
	if _, err := f.BrewInstall(context.Background(), "borked"); err == nil {
		t.Error("expected error")
	}
	if f.FormulasMap["borked"].Installed {
		t.Error("formula should not be installed after failure")
	}
}
