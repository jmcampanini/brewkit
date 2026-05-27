package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jmcampanini/brewkit/internal/brew"
	"github.com/jmcampanini/brewkit/internal/parse"
	"github.com/jmcampanini/brewkit/internal/profile"
)

// resetFlags zeroes the package-level flags var so test cases don't bleed.
func resetFlags() {
	flags = globalFlags{}
}

func useBrewer(t *testing.T, brewer brew.Brewer) {
	t.Helper()
	previous := brewerFactory
	brewerFactory = func() brew.Brewer { return brewer }
	t.Cleanup(func() { brewerFactory = previous })
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	out, _ := captureOutput(t, fn)
	return out
}

func captureOutput(t *testing.T, fn func()) (out string, errOut string) {
	t.Helper()
	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	errR, errW, err := os.Pipe()
	if err != nil {
		_ = outR.Close()
		_ = outW.Close()
		t.Fatal(err)
	}
	oldOut := os.Stdout
	oldErr := os.Stderr
	os.Stdout = outW
	os.Stderr = errW

	outDone := readPipe(outR)
	errDone := readPipe(errR)
	defer func() {
		os.Stdout = oldOut
		os.Stderr = oldErr
		_ = outW.Close()
		_ = errW.Close()
		out = <-outDone
		errOut = <-errDone
		_ = outR.Close()
		_ = errR.Close()
	}()

	fn()
	return out, errOut
}

func readPipe(r *os.File) <-chan string {
	done := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		done <- buf.String()
	}()
	return done
}

// fixtureRepo writes a brewkit.toml plus the requested profile files in a
// temp directory and returns its path.
func fixtureRepo(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	tomlPath := filepath.Join(dir, "brewkit.toml")
	toml := `dir = "` + dir + `"` + "\n" +
		`profiles = ["common"]` + "\n" +
		`profiles_env = ""` + "\n"
	if err := os.WriteFile(tomlPath, []byte(toml), 0o644); err != nil {
		t.Fatal(err)
	}
	for name, contents := range files {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestApplyCommandsAcceptHideUnchangedFlag(t *testing.T) {
	resetFlags()
	defer resetFlags()

	root := newRootCmd()
	for _, name := range []string{"tap", "brew", "cask", "head"} {
		cmd, _, err := root.Find([]string{name})
		if err != nil {
			t.Fatalf("find %s: %v", name, err)
		}
		flag := cmd.Flags().Lookup("hide-unchanged")
		if flag == nil {
			t.Fatalf("%s missing --hide-unchanged", name)
		}
		if err := cmd.Flags().Parse([]string{"--hide-unchanged"}); err != nil {
			t.Fatalf("%s should accept --hide-unchanged: %v", name, err)
		}
	}
}

func TestRunApply_ConfigOmittedDir_ResolvesAgainstConfigPath(t *testing.T) {
	resetFlags()
	defer resetFlags()

	// A brewkit.toml in dir/ that does NOT set `dir` should still
	// cause profile files to be looked up from dir/ when brewkit is
	// invoked with --config dir/brewkit.toml from an unrelated CWD.
	dir := t.TempDir()
	tomlPath := filepath.Join(dir, "brewkit.toml")
	if err := os.WriteFile(tomlPath,
		[]byte(`profiles = ["common"]`+"\n"+`profiles_env = ""`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Brewfile.common"),
		[]byte(`brew "git"  # vcs`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Change to an unrelated CWD to make sure `dir = "."` can't
	// accidentally resolve to the right place.
	cwd := t.TempDir()
	t.Chdir(cwd)

	flags.configPath = tomlPath
	flags.dryRun = true

	fake := brew.NewFake()
	useBrewer(t, fake)

	out := captureStdout(t, func() {
		if err := runApply(context.Background(), profile.KindBrew, nil); err != nil {
			t.Errorf("runApply err: %v", err)
		}
	})

	if !strings.Contains(out, "+ git") {
		t.Errorf("expected '+ git' in output — profile files must be discovered next to the config file:\n%s", out)
	}
}

func TestRunApply_Tap_Adds(t *testing.T) {
	resetFlags()
	defer resetFlags()

	dir := fixtureRepo(t, map[string]string{
		"Tapfile.common": `charmbracelet/tap` + "\n",
	})
	flags.configPath = filepath.Join(dir, "brewkit.toml")

	fake := brew.NewFake()
	useBrewer(t, fake)

	out := captureStdout(t, func() {
		if err := runApply(context.Background(), profile.KindTap, nil); err != nil {
			t.Errorf("runApply err: %v", err)
		}
	})

	if !strings.Contains(out, "+ charmbracelet/tap") {
		t.Errorf("expected '+ charmbracelet/tap' in output:\n%s", out)
	}
	if !fake.TapsSet["charmbracelet/tap"] {
		t.Error("tap not registered in fake")
	}
}

func TestRunApply_Brew_DryRun(t *testing.T) {
	resetFlags()
	defer resetFlags()

	dir := fixtureRepo(t, map[string]string{
		"Brewfile.common": `brew "ripgrep"  # search` + "\n",
	})
	flags.configPath = filepath.Join(dir, "brewkit.toml")
	flags.dryRun = true

	fake := brew.NewFake()
	useBrewer(t, fake)

	out := captureStdout(t, func() {
		if err := runApply(context.Background(), profile.KindBrew, nil); err != nil {
			t.Errorf("runApply err: %v", err)
		}
	})

	want := "+ ripgrep (dry-run)\n\nSummary: 1 added\n"
	if out != want {
		t.Errorf("unexpected dry-run output:\nwant: %q\ngot:  %q", want, out)
	}
	if len(fake.Calls) != 0 {
		t.Errorf("dry-run should not call brewer; got %d calls", len(fake.Calls))
	}
}

func TestRunApply_Brew_UpgradesAndUpToDate(t *testing.T) {
	resetFlags()
	defer resetFlags()

	dir := fixtureRepo(t, map[string]string{
		"Brewfile.common": `brew "neovim"  # editor` + "\n" + `brew "git"     # vcs` + "\n",
	})
	flags.configPath = filepath.Join(dir, "brewkit.toml")

	fake := brew.NewFake()
	fake.FormulasMap["neovim"] = brew.FormulaState{
		Installed: true, Version: "0.10.0", Outdated: true, OutdatedTo: "0.10.2",
	}
	fake.FormulasMap["git"] = brew.FormulaState{Installed: true, Version: "2.45.0"}
	useBrewer(t, fake)

	out := captureStdout(t, func() {
		if err := runApply(context.Background(), profile.KindBrew, nil); err != nil {
			t.Errorf("runApply err: %v", err)
		}
	})

	if !strings.Contains(out, "↑ neovim 0.10.0 → 0.10.2") {
		t.Errorf("expected upgrade line:\n%s", out)
	}
	if !strings.Contains(out, "✓ git") {
		t.Errorf("expected up-to-date line:\n%s", out)
	}
	if !strings.Contains(out, "Summary: 1 upgraded, 1 up-to-date") {
		t.Errorf("expected summary:\n%s", out)
	}
}

func TestRunApply_Brew_HideUnchangedKeepsChangesAndSummary(t *testing.T) {
	resetFlags()
	defer resetFlags()

	dir := fixtureRepo(t, map[string]string{
		"Brewfile.common": `brew "ripgrep"  # search` + "\n" +
			`brew "neovim"   # editor` + "\n" +
			`brew "git"      # vcs` + "\n",
	})
	flags.configPath = filepath.Join(dir, "brewkit.toml")
	flags.hideUnchanged = true

	fake := brew.NewFake()
	fake.FormulasMap["neovim"] = brew.FormulaState{
		Installed: true, Version: "0.10.0", Outdated: true, OutdatedTo: "0.10.2",
	}
	fake.FormulasMap["git"] = brew.FormulaState{Installed: true, Version: "2.45.0"}
	useBrewer(t, fake)

	out := captureStdout(t, func() {
		if err := runApply(context.Background(), profile.KindBrew, nil); err != nil {
			t.Errorf("runApply err: %v", err)
		}
	})

	if !strings.Contains(out, "+ ripgrep") {
		t.Errorf("added formula should stay visible with --hide-unchanged:\n%s", out)
	}
	if !strings.Contains(out, "↑ neovim 0.10.0 → 0.10.2") {
		t.Errorf("upgraded formula should stay visible with --hide-unchanged:\n%s", out)
	}
	if strings.Contains(out, "✓ git") {
		t.Errorf("up-to-date formula should be hidden with --hide-unchanged:\n%s", out)
	}
	if !strings.Contains(out, "Summary: 1 added, 1 upgraded, 1 up-to-date") {
		t.Errorf("summary should count hidden up-to-date entries:\n%s", out)
	}
}

func TestRunApply_Tap_HideUnchanged(t *testing.T) {
	resetFlags()
	defer resetFlags()

	dir := fixtureRepo(t, map[string]string{
		"Tapfile.common": `homebrew/core` + "\n",
	})
	flags.configPath = filepath.Join(dir, "brewkit.toml")
	flags.hideUnchanged = true

	fake := brew.NewFake()
	fake.TapsSet["homebrew/core"] = true
	useBrewer(t, fake)

	out := captureStdout(t, func() {
		if err := runApply(context.Background(), profile.KindTap, nil); err != nil {
			t.Errorf("runApply err: %v", err)
		}
	})

	if strings.Contains(out, "homebrew/core") {
		t.Errorf("up-to-date tap should be hidden with --hide-unchanged:\n%s", out)
	}
	if !strings.Contains(out, "Summary: 1 up-to-date") {
		t.Errorf("summary should count hidden tap:\n%s", out)
	}
}

func TestRunApply_Cask_HideUnchanged(t *testing.T) {
	resetFlags()
	defer resetFlags()

	dir := fixtureRepo(t, map[string]string{
		"Caskfile.common": `iterm2  # terminal` + "\n",
	})
	flags.configPath = filepath.Join(dir, "brewkit.toml")
	flags.hideUnchanged = true

	fake := brew.NewFake()
	fake.CasksMap["iterm2"] = brew.CaskState{Installed: true, Version: "3.5.0"}
	useBrewer(t, fake)

	out := captureStdout(t, func() {
		if err := runApply(context.Background(), profile.KindCask, nil); err != nil {
			t.Errorf("runApply err: %v", err)
		}
	})

	if strings.Contains(out, "iterm2") {
		t.Errorf("up-to-date cask should be hidden with --hide-unchanged:\n%s", out)
	}
	if !strings.Contains(out, "Summary: 1 up-to-date") {
		t.Errorf("summary should count hidden cask:\n%s", out)
	}
}

func TestRunApply_Cask_RestartNotice(t *testing.T) {
	resetFlags()
	defer resetFlags()

	dir := fixtureRepo(t, map[string]string{
		"Caskfile.common": `claude  # AI` + "\n",
	})
	flags.configPath = filepath.Join(dir, "brewkit.toml")

	fake := brew.NewFake()
	fake.CasksMap["claude"] = brew.CaskState{
		Installed: true, Version: "1.0.0", Outdated: true, OutdatedTo: "1.1.0",
	}
	useBrewer(t, fake)

	out := captureStdout(t, func() {
		if err := runApply(context.Background(), profile.KindCask, nil); err != nil {
			t.Errorf("runApply err: %v", err)
		}
	})

	if !strings.Contains(out, "Restart these apps") || !strings.Contains(out, "claude") {
		t.Errorf("expected restart notice with claude:\n%s", out)
	}
}

func TestRunApply_MissingFileNotice(t *testing.T) {
	resetFlags()
	defer resetFlags()

	dir := fixtureRepo(t, map[string]string{}) // no profile files
	flags.configPath = filepath.Join(dir, "brewkit.toml")

	fake := brew.NewFake()
	useBrewer(t, fake)

	out := captureStdout(t, func() {
		if err := runApply(context.Background(), profile.KindBrew, nil); err != nil {
			t.Errorf("runApply err: %v", err)
		}
	})

	want := "⊘ common: no Brewfile, skipping\n\nSummary: 1 skipped\n"
	if out != want {
		t.Errorf("unexpected missing-file notice:\nwant: %q\ngot:  %q", want, out)
	}
}

func TestRunApply_PositionalFilter(t *testing.T) {
	resetFlags()
	defer resetFlags()

	dir := fixtureRepo(t, map[string]string{
		"Brewfile.common": `brew "git"     # vcs` + "\n" + `brew "neovim"  # editor` + "\n",
	})
	flags.configPath = filepath.Join(dir, "brewkit.toml")
	flags.dryRun = true

	fake := brew.NewFake()
	useBrewer(t, fake)

	out := captureStdout(t, func() {
		if err := runApply(context.Background(), profile.KindBrew, []string{"git"}); err != nil {
			t.Errorf("runApply err: %v", err)
		}
	})

	if !strings.Contains(out, "+ git") {
		t.Errorf("expected git in output:\n%s", out)
	}
	if strings.Contains(out, "neovim") {
		t.Errorf("neovim should be filtered out:\n%s", out)
	}
}

func TestRunApply_Tap_FailureAborts(t *testing.T) {
	resetFlags()
	defer resetFlags()

	dir := fixtureRepo(t, map[string]string{
		"Tapfile.common": `broken/tap` + "\n" + `other/tap` + "\n",
	})
	flags.configPath = filepath.Join(dir, "brewkit.toml")

	fake := brew.NewFake()
	fake.FailOps[brew.OpTap] = map[string]bool{"broken/tap": true}
	useBrewer(t, fake)

	captureStdout(t, func() {
		err := runApply(context.Background(), profile.KindTap, nil)
		if err == nil {
			t.Error("expected runApply to error on tap failure")
		}
	})
	if fake.TapsSet["other/tap"] {
		t.Error("fail_fast=true should abort before the second tap")
	}
}

func TestRunApply_Brew_UpgradeFailure(t *testing.T) {
	resetFlags()
	defer resetFlags()

	dir := fixtureRepo(t, map[string]string{
		"Brewfile.common": `brew "neovim"  # editor` + "\n",
	})
	flags.configPath = filepath.Join(dir, "brewkit.toml")

	fake := brew.NewFake()
	fake.FormulasMap["neovim"] = brew.FormulaState{
		Installed: true, Version: "0.10.0", Outdated: true, OutdatedTo: "0.10.2",
	}
	fake.FailOps[brew.OpBrewUpgrade] = map[string]bool{"neovim": true}
	useBrewer(t, fake)

	captureStdout(t, func() {
		if err := runApply(context.Background(), profile.KindBrew, nil); err == nil {
			t.Error("expected runApply to error on upgrade failure")
		}
	})
	// The cache projection should not have happened.
	if fake.FormulasMap["neovim"].Version != "0.10.0" {
		t.Errorf("version mutated on failed upgrade: %s", fake.FormulasMap["neovim"].Version)
	}
}

func TestRunApply_Cask_UpgradeFailure_NoRestartNotice(t *testing.T) {
	resetFlags()
	defer resetFlags()

	dir := fixtureRepo(t, map[string]string{
		"Caskfile.common": `claude  # AI` + "\n",
	})
	flags.configPath = filepath.Join(dir, "brewkit.toml")

	fake := brew.NewFake()
	fake.CasksMap["claude"] = brew.CaskState{
		Installed: true, Version: "1.0.0", Outdated: true, OutdatedTo: "1.1.0",
	}
	fake.FailOps[brew.OpCaskUpgrade] = map[string]bool{"claude": true}
	useBrewer(t, fake)

	out := captureStdout(t, func() {
		if err := runApply(context.Background(), profile.KindCask, nil); err == nil {
			t.Error("expected runApply to error on cask upgrade failure")
		}
	})
	if strings.Contains(out, "Restart these apps") {
		t.Errorf("failed upgrade must not produce restart notice:\n%s", out)
	}
}

func TestRunApply_Cask_InstallFailure(t *testing.T) {
	resetFlags()
	defer resetFlags()

	dir := fixtureRepo(t, map[string]string{
		"Caskfile.common": `antinote  # notes` + "\n",
	})
	flags.configPath = filepath.Join(dir, "brewkit.toml")

	fake := brew.NewFake()
	fake.FailOps[brew.OpCaskInstall] = map[string]bool{"antinote": true}
	useBrewer(t, fake)

	captureStdout(t, func() {
		if err := runApply(context.Background(), profile.KindCask, nil); err == nil {
			t.Error("expected runApply to error on cask install failure")
		}
	})
	if fake.CasksMap["antinote"].Installed {
		t.Error("cask should not be marked installed after failure")
	}
}

func TestRunApply_FailFastTrue_StopsImmediately(t *testing.T) {
	resetFlags()
	defer resetFlags()

	dir := fixtureRepo(t, map[string]string{
		"Brewfile.common": `brew "borked"  # fails` + "\n" + `brew "later"  # should be skipped` + "\n",
	})
	flags.configPath = filepath.Join(dir, "brewkit.toml")

	fake := brew.NewFake()
	fake.FailOps[brew.OpBrewInstall] = map[string]bool{"borked": true}
	useBrewer(t, fake)

	captureStdout(t, func() {
		if err := runApply(context.Background(), profile.KindBrew, nil); err == nil {
			t.Error("expected runApply to error")
		}
	})
	// "later" should not have been attempted
	for _, c := range fake.Calls {
		if c.Name == "later" {
			t.Errorf("fail_fast=true should have stopped before 'later': %+v", c)
		}
	}
}

func TestRunApply_FailFastFalse_ContinuesAndReturnsError(t *testing.T) {
	resetFlags()
	defer resetFlags()

	dir := t.TempDir()
	tomlPath := filepath.Join(dir, "brewkit.toml")
	toml := `dir = "` + dir + `"` + "\n" +
		`profiles = ["common"]` + "\n" +
		`profiles_env = ""` + "\n" +
		`fail_fast = false` + "\n"
	if err := os.WriteFile(tomlPath, []byte(toml), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Brewfile.common"),
		[]byte(`brew "borked"   # broken`+"\n"+`brew "alright"  # ok`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	flags.configPath = tomlPath

	fake := brew.NewFake()
	fake.FailOps[brew.OpBrewInstall] = map[string]bool{"borked": true}
	useBrewer(t, fake)

	captureStdout(t, func() {
		err := runApply(context.Background(), profile.KindBrew, nil)
		if err == nil {
			t.Error("expected aggregated error")
		}
	})
	if !fake.FormulasMap["alright"].Installed {
		t.Error("non-failing entry should still be installed in non-fail-fast mode")
	}
}

func TestRunApply_QualifiedNameCachedUnderShortName(t *testing.T) {
	resetFlags()
	defer resetFlags()

	dir := fixtureRepo(t, map[string]string{
		"Brewfile.common": `brew "user/tap/foo"  # qualified` + "\n" +
			`brew "foo"           # short` + "\n",
	})
	flags.configPath = filepath.Join(dir, "brewkit.toml")

	fake := brew.NewFake()
	useBrewer(t, fake)

	captureStdout(t, func() {
		if err := runApply(context.Background(), profile.KindBrew, nil); err != nil {
			t.Errorf("runApply err: %v", err)
		}
	})

	installs := 0
	for _, c := range fake.Calls {
		if c.Op == brew.OpBrewInstall {
			installs++
		}
	}
	if installs != 1 {
		t.Errorf("expected 1 brew-install call (qualified entry installs, short entry should hit cache); got %d", installs)
	}
}

// headLatestErrFake wraps Fake and forces HeadLatestSHA to return an
// error for a given formula, independent of the HeadLatest/HeadHasURL
// fields.
type headLatestErrFake struct {
	*brew.Fake
	errName string
	errMsg  string
}

func (f *headLatestErrFake) HeadLatestSHA(ctx context.Context, name string) (string, bool, error) {
	if name == f.errName {
		return "", true, errors.New(f.errMsg)
	}
	return f.Fake.HeadLatestSHA(ctx, name)
}

// headInstalledErrFake wraps Fake to force HeadInstalledSHA errors.
type headInstalledErrFake struct {
	*brew.Fake
	errName string
}

func (f *headInstalledErrFake) HeadInstalledSHA(ctx context.Context, name string) (string, bool, bool, error) {
	if name == f.errName {
		return "", false, false, errors.New("boom")
	}
	return f.Fake.HeadInstalledSHA(ctx, name)
}

func TestRunApply_Head_InstalledSHAError(t *testing.T) {
	resetFlags()
	defer resetFlags()

	dir := fixtureRepo(t, map[string]string{
		"Headfile.common": `tmux  # mux` + "\n",
	})
	flags.configPath = filepath.Join(dir, "brewkit.toml")

	fake := &headInstalledErrFake{Fake: brew.NewFake(), errName: "tmux"}
	useBrewer(t, fake)

	captureStdout(t, func() {
		if err := runApply(context.Background(), profile.KindHead, nil); err == nil {
			t.Error("expected runApply to error on HeadInstalledSHA failure")
		}
	})
}

func TestRunApply_Head_LatestSHAError(t *testing.T) {
	resetFlags()
	defer resetFlags()

	dir := fixtureRepo(t, map[string]string{
		"Headfile.common": `tmux  # mux` + "\n",
	})
	flags.configPath = filepath.Join(dir, "brewkit.toml")

	base := brew.NewFake()
	base.HeadInstalls["tmux"] = "abc1234"
	fake := &headLatestErrFake{Fake: base, errName: "tmux", errMsg: "network flake"}
	useBrewer(t, fake)

	captureStdout(t, func() {
		if err := runApply(context.Background(), profile.KindHead, nil); err == nil {
			t.Error("expected runApply to error on HeadLatestSHA failure")
		}
	})
	// No reinstall should have happened.
	for _, c := range base.Calls {
		if c.Op == brew.OpHeadReinstall {
			t.Errorf("HeadReinstall should NOT run after HeadLatestSHA error: %+v", c)
		}
	}
}

func TestRunApply_Head_InstallFailure(t *testing.T) {
	resetFlags()
	defer resetFlags()

	dir := fixtureRepo(t, map[string]string{
		"Headfile.common": `tmux  # mux` + "\n",
	})
	flags.configPath = filepath.Join(dir, "brewkit.toml")

	fake := brew.NewFake()
	fake.HeadLatest["tmux"] = "abc1234"
	fake.HeadHasURL["tmux"] = true
	fake.FailOps[brew.OpHeadInstall] = map[string]bool{"tmux": true}
	useBrewer(t, fake)

	captureStdout(t, func() {
		if err := runApply(context.Background(), profile.KindHead, nil); err == nil {
			t.Error("expected runApply to error on HeadInstall failure")
		}
	})
}

func TestRunApply_Head_SHAMoved_DryRun(t *testing.T) {
	resetFlags()
	defer resetFlags()

	dir := fixtureRepo(t, map[string]string{
		"Headfile.common": `tmux  # mux` + "\n",
	})
	flags.configPath = filepath.Join(dir, "brewkit.toml")
	flags.dryRun = true

	fake := brew.NewFake()
	fake.HeadInstalls["tmux"] = "oldsha0"
	fake.HeadLatest["tmux"] = "newsha1"
	useBrewer(t, fake)

	out := captureStdout(t, func() {
		if err := runApply(context.Background(), profile.KindHead, nil); err != nil {
			t.Errorf("runApply err: %v", err)
		}
	})

	if !strings.Contains(out, "HEAD-oldsha0 → HEAD-newsha1") {
		t.Errorf("expected SHA diff in output:\n%s", out)
	}
	for _, c := range fake.Calls {
		if c.Op == brew.OpHeadReinstall {
			t.Errorf("dry-run should not reinstall: %+v", c)
		}
	}
}

func TestRunApply_Head_SHAMoved_RealRun(t *testing.T) {
	resetFlags()
	defer resetFlags()

	dir := fixtureRepo(t, map[string]string{
		"Headfile.common": `tmux  # mux` + "\n",
	})
	flags.configPath = filepath.Join(dir, "brewkit.toml")

	fake := brew.NewFake()
	fake.HeadInstalls["tmux"] = "oldsha0"
	fake.HeadLatest["tmux"] = "newsha1"
	useBrewer(t, fake)

	out := captureStdout(t, func() {
		if err := runApply(context.Background(), profile.KindHead, nil); err != nil {
			t.Errorf("runApply err: %v", err)
		}
	})

	if !strings.Contains(out, "HEAD-oldsha0 → HEAD-newsha1") {
		t.Errorf("expected SHA diff in output:\n%s", out)
	}
	reinstalls := 0
	for _, c := range fake.Calls {
		if c.Op == brew.OpHeadReinstall && c.Name == "tmux" {
			reinstalls++
		}
	}
	if reinstalls != 1 {
		t.Errorf("expected 1 HeadReinstall, got %d", reinstalls)
	}
}

// stateFailProbe forces State to return an error.
type stateFailProbe struct{ *brew.Fake }

func (p *stateFailProbe) State(ctx context.Context) (*brew.State, error) {
	return nil, fmt.Errorf("brew unavailable")
}

func TestRunApply_EnsureStateFailure(t *testing.T) {
	resetFlags()
	defer resetFlags()

	dir := fixtureRepo(t, map[string]string{
		"Brewfile.common": `brew "git"  # vcs` + "\n",
	})
	flags.configPath = filepath.Join(dir, "brewkit.toml")

	fake := &stateFailProbe{Fake: brew.NewFake()}
	useBrewer(t, fake)

	captureStdout(t, func() {
		err := runApply(context.Background(), profile.KindBrew, nil)
		if err == nil {
			t.Error("expected runApply to error when State fails")
		}
		if !strings.Contains(err.Error(), "brew state") {
			t.Errorf("error should mention 'brew state': %v", err)
		}
	})
}

func TestRunApply_Head_InstalledStableErrorsNoReinstall(t *testing.T) {
	resetFlags()
	defer resetFlags()

	dir := fixtureRepo(t, map[string]string{
		"Headfile.common": `direnv  # stable install only` + "\n",
	})
	flags.configPath = filepath.Join(dir, "brewkit.toml")

	base := brew.NewFake()
	base.FormulasMap["direnv"] = brew.FormulaState{Installed: true, Version: "2.37.1"}
	fake := &headLatestErrFake{Fake: base, errName: "direnv", errMsg: "network flake"}
	useBrewer(t, fake)

	var runErr error
	out, errOut := captureOutput(t, func() {
		runErr = runApply(context.Background(), profile.KindHead, nil)
	})

	if runErr == nil || !strings.Contains(runErr.Error(), "installed but not as HEAD") {
		t.Fatalf("expected stable HEAD entry error, got %v", runErr)
	}
	if !strings.Contains(errOut, "✗ direnv: installed but not as HEAD") {
		t.Errorf("expected error icon for stable install:\n%s", errOut)
	}
	if !strings.Contains(out, "Summary: 1 failed") {
		t.Errorf("expected failure summary:\n%s", out)
	}
	for _, c := range fake.Calls {
		if c.Op == brew.OpHeadReinstall {
			t.Errorf("HeadReinstall should NOT be called for a stable install; got %+v", c)
		}
	}
}

func TestRunApply_Head_InstalledStableWithoutHeadSourceErrorsNoReinstall(t *testing.T) {
	resetFlags()
	defer resetFlags()

	dir := fixtureRepo(t, map[string]string{
		"Headfile.common": `direnv  # no source upstream` + "\n",
	})
	flags.configPath = filepath.Join(dir, "brewkit.toml")

	fake := brew.NewFake()
	fake.FormulasMap["direnv"] = brew.FormulaState{Installed: true, Version: "2.37.1"}
	useBrewer(t, fake)

	var runErr error
	_, errOut := captureOutput(t, func() {
		runErr = runApply(context.Background(), profile.KindHead, nil)
	})

	if runErr == nil || !strings.Contains(runErr.Error(), "installed but not as HEAD") {
		t.Fatalf("expected stable HEAD entry error, got %v", runErr)
	}
	if !strings.Contains(errOut, "✗ direnv: installed but not as HEAD") {
		t.Errorf("expected error icon for stable install:\n%s", errOut)
	}
	for _, c := range fake.Calls {
		if c.Op == brew.OpHeadReinstall {
			t.Errorf("HeadReinstall should NOT be called for a stable install; got %+v", c)
		}
	}
}

func TestRunApply_Head_NoSourceErrorsNoInstall(t *testing.T) {
	resetFlags()
	defer resetFlags()

	dir := fixtureRepo(t, map[string]string{
		"Headfile.common": `lonely  # no head source upstream` + "\n",
	})
	flags.configPath = filepath.Join(dir, "brewkit.toml")
	flags.hideUnchanged = true

	fake := brew.NewFake()
	// HeadHasURL["lonely"] is false (zero value); HeadLatest also empty.
	// Fake.HeadLatestSHA returns hasHead=false in this case.
	useBrewer(t, fake)

	var runErr error
	out, errOut := captureOutput(t, func() {
		runErr = runApply(context.Background(), profile.KindHead, nil)
	})

	if runErr == nil || !strings.Contains(runErr.Error(), "no HEAD source") {
		t.Fatalf("expected no HEAD source error, got %v", runErr)
	}
	if !strings.Contains(errOut, "✗ lonely: no HEAD source") {
		t.Errorf("expected error icon for no HEAD source:\n%s", errOut)
	}
	if !strings.Contains(out, "Summary: 1 failed") {
		t.Errorf("expected failure summary:\n%s", out)
	}
	for _, c := range fake.Calls {
		if c.Op == brew.OpHeadInstall {
			t.Errorf("HeadInstall should NOT be called when formula has no HEAD source; got call %+v", c)
		}
	}
}

func TestRunApply_DryRun_LayeredProfilesProjectsState(t *testing.T) {
	resetFlags()
	defer resetFlags()

	dir := t.TempDir()
	tomlPath := filepath.Join(dir, "brewkit.toml")
	toml := `dir = "` + dir + `"` + "\n" +
		`profiles = ["a", "b"]` + "\n" +
		`profiles_env = ""` + "\n"
	if err := os.WriteFile(tomlPath, []byte(toml), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Brewfile.a"),
		[]byte(`brew "ripgrep"  # search`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Brewfile.b"),
		[]byte(`brew "ripgrep"  # search`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	flags.configPath = tomlPath
	flags.dryRun = true

	fake := brew.NewFake()
	useBrewer(t, fake)

	out := captureStdout(t, func() {
		if err := runApply(context.Background(), profile.KindBrew, nil); err != nil {
			t.Errorf("runApply err: %v", err)
		}
	})

	if !strings.Contains(out, "+ ripgrep") {
		t.Errorf("expected first 'ripgrep' to be added:\n%s", out)
	}
	if !strings.Contains(out, "Summary: 1 added, 1 up-to-date") {
		t.Errorf("layered dry-run should project install — second occurrence should be up-to-date:\n%s", out)
	}
}

// sentinelFailFake wraps brew.Fake and returns caller-supplied sentinel
// errors from BrewInstall so tests can verify errors.Is walks the
// aggregated errors.Join tree.
//
// brew.Fake.BrewInstall records into Calls *before* checking shouldFail
// (see internal/brew/fake.go), so a normal Fake failure still appears
// in the ledger. Because this wrapper short-circuits *before* delegating
// to f.Fake.BrewInstall, it must mirror that recording itself — keep
// the FakeCall shape in sync with brew.Fake.record if that ever grows
// new fields.
type sentinelFailFake struct {
	*brew.Fake
	errs map[string]error
}

func (f *sentinelFailFake) BrewInstall(ctx context.Context, name string) (brew.Result, error) {
	if err, ok := f.errs[name]; ok {
		f.Calls = append(f.Calls, brew.FakeCall{Op: brew.OpBrewInstall, Name: name})
		return brew.Result{}, err
	}
	return f.Fake.BrewInstall(ctx, name)
}

func TestRunApply_AggregatedErrors_UnwrapWalk(t *testing.T) {
	resetFlags()
	defer resetFlags()

	dir := t.TempDir()
	tomlPath := filepath.Join(dir, "brewkit.toml")
	toml := `dir = "` + dir + `"` + "\n" +
		`profiles = ["common"]` + "\n" +
		`profiles_env = ""` + "\n" +
		`fail_fast = false` + "\n"
	if err := os.WriteFile(tomlPath, []byte(toml), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Brewfile.common"),
		[]byte(`brew "alpha"  # a`+"\n"+`brew "bravo"  # b`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	flags.configPath = tomlPath

	errAlpha := fmt.Errorf("alpha sentinel")
	errBravo := fmt.Errorf("bravo sentinel")
	fake := &sentinelFailFake{
		Fake: brew.NewFake(),
		errs: map[string]error{"alpha": errAlpha, "bravo": errBravo},
	}
	useBrewer(t, fake)

	var runErr error
	captureStdout(t, func() {
		runErr = runApply(context.Background(), profile.KindBrew, nil)
	})
	if runErr == nil {
		t.Fatal("expected aggregated error")
	}
	if !errors.Is(runErr, errAlpha) {
		t.Errorf("errors.Is(err, errAlpha) = false — errors.Join contract broken: %v", runErr)
	}
	if !errors.Is(runErr, errBravo) {
		t.Errorf("errors.Is(err, errBravo) = false — errors.Join contract broken: %v", runErr)
	}
	if !strings.Contains(runErr.Error(), "2 brew operation(s) failed") {
		t.Errorf("expected prefix '2 brew operation(s) failed' in: %v", runErr)
	}
}

func TestRunApply_Head_FailFastFalseContinuesAfterInvalidEntries(t *testing.T) {
	resetFlags()
	defer resetFlags()

	dir := t.TempDir()
	tomlPath := filepath.Join(dir, "brewkit.toml")
	toml := `dir = "` + dir + `"` + "\n" +
		`profiles = ["common"]` + "\n" +
		`profiles_env = ""` + "\n" +
		`fail_fast = false` + "\n"
	if err := os.WriteFile(tomlPath, []byte(toml), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Headfile.common"),
		[]byte(`lonely  # no source`+"\n"+`tmux  # stable`+"\n"+`ok  # valid`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	flags.configPath = tomlPath

	fake := brew.NewFake()
	fake.FormulasMap["tmux"] = brew.FormulaState{Installed: true, Version: "3.4"}
	fake.HeadLatest["tmux"] = "abc1234"
	fake.HeadHasURL["tmux"] = true
	fake.HeadLatest["ok"] = "def5678"
	fake.HeadHasURL["ok"] = true
	useBrewer(t, fake)

	var runErr error
	out, errOut := captureOutput(t, func() {
		runErr = runApply(context.Background(), profile.KindHead, nil)
	})

	if runErr == nil {
		t.Fatal("expected aggregated error")
	}
	for _, want := range []string{"2 head operation(s) failed", "lonely: no HEAD source", "tmux: installed but not as HEAD"} {
		if !strings.Contains(runErr.Error(), want) {
			t.Errorf("aggregated error missing %q: %v", want, runErr)
		}
	}
	if !strings.Contains(errOut, "✗ lonely: no HEAD source") || !strings.Contains(errOut, "✗ tmux: installed but not as HEAD") {
		t.Errorf("expected both invalid head entries on stderr:\n%s", errOut)
	}
	if !strings.Contains(out, "+ ok") {
		t.Errorf("fail_fast=false should continue to valid entry:\n%s", out)
	}
	if !fake.FormulasMap["ok"].Installed {
		t.Error("valid entry should be installed after earlier errors")
	}
	for _, c := range fake.Calls {
		if c.Op == brew.OpHeadReinstall && c.Name == "tmux" {
			t.Errorf("stable install should not be reinstalled: %+v", c)
		}
	}
}

// retryFailFake fails HeadInstall on the first call to a given name
// but lets subsequent calls succeed. Used to verify the headSeen
// retry-on-failure invariant in layered profiles. attempts counts
// every invocation, including the short-circuited failure.
//
// As with sentinelFailFake above, brew.Fake.HeadInstall records before
// checking shouldFail, so this wrapper mirrors that behavior on its
// short-circuit failure path to keep the call ledger consistent with
// what tests would observe from the unwrapped Fake.
type retryFailFake struct {
	*brew.Fake
	failOnce map[string]bool
	attempts map[string]int
}

func (f *retryFailFake) HeadInstall(ctx context.Context, name string) (brew.Result, error) {
	f.attempts[name]++
	if f.failOnce[name] {
		delete(f.failOnce, name)
		f.Calls = append(f.Calls, brew.FakeCall{Op: brew.OpHeadInstall, Name: name})
		return brew.Result{}, fmt.Errorf("transient failure")
	}
	return f.Fake.HeadInstall(ctx, name)
}

func TestRunApply_Head_RetryAfterFailure_LayeredProfiles(t *testing.T) {
	resetFlags()
	defer resetFlags()

	// Two layered profiles both list `tmux`. The first invocation
	// fails transiently, but because fail_fast=false and headSeen is
	// marked only on success, the second profile's entry gets a
	// second chance and succeeds.
	dir := t.TempDir()
	tomlPath := filepath.Join(dir, "brewkit.toml")
	toml := `dir = "` + dir + `"` + "\n" +
		`profiles = ["a", "b"]` + "\n" +
		`profiles_env = ""` + "\n" +
		`fail_fast = false` + "\n"
	if err := os.WriteFile(tomlPath, []byte(toml), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Headfile.a"),
		[]byte(`tmux  # mux`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Headfile.b"),
		[]byte(`tmux  # mux`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	flags.configPath = tomlPath

	base := brew.NewFake()
	base.HeadLatest["tmux"] = "abc1234"
	base.HeadHasURL["tmux"] = true
	fake := &retryFailFake{
		Fake:     base,
		failOnce: map[string]bool{"tmux": true},
		attempts: map[string]int{},
	}
	useBrewer(t, fake)

	var runErr error
	out := captureStdout(t, func() {
		runErr = runApply(context.Background(), profile.KindHead, nil)
	})

	if fake.attempts["tmux"] != 2 {
		t.Errorf("expected 2 HeadInstall attempts (1 failed + 1 retry), got %d\noutput:\n%s",
			fake.attempts["tmux"], out)
	}
	if !base.FormulasMap["tmux"].Installed {
		t.Errorf("tmux should be installed after retry succeeds:\n%s", out)
	}
	// runApply must surface the first failure via the aggregated error,
	// not silently swallow it just because the retry succeeded.
	if runErr == nil {
		t.Fatal("expected runApply to return an aggregated error after the first attempt failed")
	}
	if !strings.Contains(runErr.Error(), "transient failure") {
		t.Errorf("aggregated error should preserve the underlying 'transient failure' text: %v", runErr)
	}
	if strings.Contains(out, "already processed") {
		t.Errorf("second profile must retry, not short-circuit with 'already processed':\n%s", out)
	}
}

func TestRunApply_UnknownLineFailsRun(t *testing.T) {
	resetFlags()
	defer resetFlags()

	dir := fixtureRepo(t, map[string]string{
		"Brewfile.common": `brew "git"  # vcs` + "\n" +
			`brew "neovim", args: ["with-luajit"]  # broken bundle option` + "\n",
	})
	flags.configPath = filepath.Join(dir, "brewkit.toml")

	fake := brew.NewFake()
	useBrewer(t, fake)

	captureStdout(t, func() {
		err := runApply(context.Background(), profile.KindBrew, nil)
		if err == nil {
			t.Error("expected runApply to fail on LineUnknown content")
		}
	})
}

func TestRunApply_Head_HideUnchangedHidesUpToDateAndDuplicate(t *testing.T) {
	resetFlags()
	defer resetFlags()

	dir := t.TempDir()
	tomlPath := filepath.Join(dir, "brewkit.toml")
	toml := `dir = "` + dir + `"` + "\n" +
		`profiles = ["a", "b"]` + "\n" +
		`profiles_env = ""` + "\n"
	if err := os.WriteFile(tomlPath, []byte(toml), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Headfile.a"), []byte(`tmux  # mux`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Headfile.b"), []byte(`tmux  # mux`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	flags.configPath = tomlPath
	flags.hideUnchanged = true

	fake := brew.NewFake()
	fake.HeadInstalls["tmux"] = "abc1234"
	fake.HeadLatest["tmux"] = "abc1234"
	useBrewer(t, fake)

	out := captureStdout(t, func() {
		if err := runApply(context.Background(), profile.KindHead, nil); err != nil {
			t.Errorf("runApply err: %v", err)
		}
	})

	if strings.Contains(out, "tmux") || strings.Contains(out, "already processed") {
		t.Errorf("up-to-date and duplicate HEAD lines should be hidden with --hide-unchanged:\n%s", out)
	}
	if !strings.Contains(out, "Summary: 2 up-to-date") {
		t.Errorf("summary should count hidden HEAD up-to-date and duplicate entries:\n%s", out)
	}
}

func TestRunApply_Head_LayeredDedupeCanonicalizesNames(t *testing.T) {
	resetFlags()
	defer resetFlags()

	dir := t.TempDir()
	tomlPath := filepath.Join(dir, "brewkit.toml")
	toml := `dir = "` + dir + `"` + "\n" +
		`profiles = ["a", "b"]` + "\n" +
		`profiles_env = ""` + "\n"
	if err := os.WriteFile(tomlPath, []byte(toml), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Headfile.a"),
		[]byte(`homebrew/core/tmux  # qualified`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Headfile.b"),
		[]byte(`tmux  # short`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	flags.configPath = tomlPath
	flags.dryRun = true

	fake := brew.NewFake()
	fake.HeadInstalls["homebrew/core/tmux"] = "abc1234"
	fake.HeadLatest["homebrew/core/tmux"] = "abc1234"
	useBrewer(t, fake)

	out := captureStdout(t, func() {
		if err := runApply(context.Background(), profile.KindHead, nil); err != nil {
			t.Errorf("runApply err: %v", err)
		}
	})

	if !strings.Contains(out, "already processed") {
		t.Errorf("expected dedupe across qualified/short forms:\n%s", out)
	}
}

func TestRunApply_Head_LayeredProfilesDeduped(t *testing.T) {
	resetFlags()
	defer resetFlags()

	dir := t.TempDir()
	tomlPath := filepath.Join(dir, "brewkit.toml")
	toml := `dir = "` + dir + `"` + "\n" +
		`profiles = ["a", "b"]` + "\n" +
		`profiles_env = ""` + "\n"
	if err := os.WriteFile(tomlPath, []byte(toml), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Headfile.a"),
		[]byte(`tmux  # multiplexer`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Headfile.b"),
		[]byte(`tmux  # multiplexer`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	flags.configPath = tomlPath
	flags.dryRun = true

	fake := brew.NewFake()
	fake.HeadInstalls["tmux"] = "abc1234"
	fake.HeadLatest["tmux"] = "abc1234"
	useBrewer(t, fake)

	out := captureStdout(t, func() {
		if err := runApply(context.Background(), profile.KindHead, nil); err != nil {
			t.Errorf("runApply err: %v", err)
		}
	})

	// Both layers should print, but the brewer should only be probed once.
	headCalls := 0
	for _, c := range fake.Calls {
		if c.Op == brew.OpHeadInstall || c.Op == brew.OpHeadReinstall {
			headCalls++
		}
	}
	if headCalls != 0 {
		t.Errorf("dry-run should not invoke head install/reinstall; got %d calls", headCalls)
	}
	if !strings.Contains(out, "already processed") {
		t.Errorf("expected dedup notice on second occurrence:\n%s", out)
	}
}

func TestRunApply_Head_DoesNotFetchState(t *testing.T) {
	resetFlags()
	defer resetFlags()

	dir := fixtureRepo(t, map[string]string{
		"Headfile.common": `tmux  # multiplexer` + "\n",
	})
	flags.configPath = filepath.Join(dir, "brewkit.toml")
	flags.dryRun = true

	stateFetched := false
	fake := brew.NewFake()
	fake.HeadInstalls["tmux"] = "abc1234"
	fake.HeadLatest["tmux"] = "abc1234"
	wrapped := &stateProbe{Fake: fake, fetched: &stateFetched}
	useBrewer(t, wrapped)

	captureStdout(t, func() {
		if err := runApply(context.Background(), profile.KindHead, nil); err != nil {
			t.Errorf("runApply err: %v", err)
		}
	})
	if stateFetched {
		t.Error("brewer.State should NOT be called for `brewkit head`")
	}
}

func TestRunApply_StateNotFetched_WhenNoFiles(t *testing.T) {
	resetFlags()
	defer resetFlags()

	dir := fixtureRepo(t, map[string]string{}) // no profile files at all
	flags.configPath = filepath.Join(dir, "brewkit.toml")

	stateFetched := false
	fake := brew.NewFake()
	wrapped := &stateProbe{Fake: fake, fetched: &stateFetched}
	useBrewer(t, wrapped)

	captureStdout(t, func() {
		if err := runApply(context.Background(), profile.KindBrew, nil); err != nil {
			t.Errorf("runApply err: %v", err)
		}
	})
	if stateFetched {
		t.Error("brewer.State should not be called when no profile files exist")
	}
}

// stateProbe wraps brew.Fake to record whether State was called.
type stateProbe struct {
	*brew.Fake
	fetched *bool
}

func (p *stateProbe) State(ctx context.Context) (*brew.State, error) {
	*p.fetched = true
	return p.Fake.State(ctx)
}

func TestRunApply_PositionalFilter_NotFoundErrors(t *testing.T) {
	resetFlags()
	defer resetFlags()

	dir := fixtureRepo(t, map[string]string{
		"Brewfile.common": `brew "git"  # vcs` + "\n",
	})
	flags.configPath = filepath.Join(dir, "brewkit.toml")

	fake := brew.NewFake()
	useBrewer(t, fake)

	captureStdout(t, func() {
		err := runApply(context.Background(), profile.KindBrew, []string{"absent"})
		if err == nil {
			t.Error("expected not-found error")
		}
	})
}

func TestRunLint_Clean(t *testing.T) {
	resetFlags()
	defer resetFlags()

	dir := fixtureRepo(t, map[string]string{
		"Brewfile.common": strings.Join([]string{
			`brew "abc"  # a`,
			`brew "def"  # d`,
			``,
		}, "\n"),
	})
	flags.configPath = filepath.Join(dir, "brewkit.toml")

	out := captureStdout(t, func() {
		if err := runLint(context.Background()); err != nil {
			t.Errorf("runLint err: %v", err)
		}
	})
	if !strings.Contains(out, "no violations") {
		t.Errorf("expected 'no violations':\n%s", out)
	}
}

func TestRunLint_ReportsViolations(t *testing.T) {
	resetFlags()
	defer resetFlags()

	dir := fixtureRepo(t, map[string]string{
		"Brewfile.common": `brew "z"  # z` + "\n" + `brew "a"  # a` + "\n",
	})
	flags.configPath = filepath.Join(dir, "brewkit.toml")

	captureStdout(t, func() {
		err := runLint(context.Background())
		if err == nil {
			t.Error("expected lint to error")
		}
	})
}

func TestRunConfig_PrintsToml(t *testing.T) {
	resetFlags()
	defer resetFlags()

	dir := fixtureRepo(t, map[string]string{})
	flags.configPath = filepath.Join(dir, "brewkit.toml")

	out := captureStdout(t, func() {
		if err := runConfig(context.Background()); err != nil {
			t.Errorf("runConfig err: %v", err)
		}
	})

	if !strings.Contains(out, "profiles") || !strings.Contains(out, "common") {
		t.Errorf("expected toml output:\n%s", out)
	}
}

func TestRunConfig_FiltersAutoLocal(t *testing.T) {
	resetFlags()
	defer resetFlags()

	dir := fixtureRepo(t, map[string]string{
		"Brewfile.local": "",
	})
	flags.configPath = filepath.Join(dir, "brewkit.toml")

	out := captureStdout(t, func() {
		if err := runConfig(context.Background()); err != nil {
			t.Errorf("runConfig err: %v", err)
		}
	})

	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "profiles =") && strings.Contains(line, "local") {
			t.Errorf("profiles= line should not include reserved 'local': %q", line)
		}
	}
	if !strings.Contains(out, "auto-appended") {
		t.Errorf("output should include the auto-append note:\n%s", out)
	}
}

func TestRunDocs_PrintsManual(t *testing.T) {
	resetFlags()
	defer resetFlags()

	out := captureStdout(t, func() {
		if err := runDocs(context.Background()); err != nil {
			t.Errorf("runDocs err: %v", err)
		}
	})
	if len(out) == 0 {
		t.Error("expected non-empty docs output")
	}
}

func TestEntryMatches_QualifiedShortName(t *testing.T) {
	e := &parse.Entry{Name: "user/tap/cmdk"}
	if !entryMatches(e, "cmdk") {
		t.Error("short name should match qualified entry")
	}
	if !entryMatches(e, "user/tap/cmdk") {
		t.Error("full name should match")
	}
	if entryMatches(e, "other") {
		t.Error("unrelated should not match")
	}
}
