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
	"github.com/jmcampanini/brewkit/internal/config"
	"github.com/jmcampanini/brewkit/internal/layer"
	"github.com/spf13/pflag"
)

var testApp = &commandContext{}

// resetFlags gives each test fresh command state with the config-backed
// flags registered but unset.
func resetFlags() {
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	if err := config.RegisterFlags(flags); err != nil {
		panic(err)
	}
	testApp = &commandContext{configFlagSet: flags}
	_ = os.Unsetenv("BREWKIT_LAYERS")
}

// useConfigFile points --config at path for the current testApp.
func useConfigFile(t *testing.T, path string) {
	t.Helper()
	if err := testApp.configFlagSet.Set("config", path); err != nil {
		t.Fatal(err)
	}
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

// fixtureRepo writes a brewkit.toml selecting the common layer plus the
// requested layer files in a temp directory and returns its path.
func fixtureRepo(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	tomlPath := filepath.Join(dir, "brewkit.toml")
	toml := `dir = "` + dir + `"` + "\n" +
		`layers = ["common"]` + "\n" +
		`env_layers = []` + "\n"
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

	root := newRootCmd(testApp)
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

func TestRunApply_ConfigOmittedDir_ResolvesAgainstProcessCWD(t *testing.T) {
	resetFlags()
	defer resetFlags()

	// A brewkit.toml that omits `dir` leaves the raw default `.` in place.
	// Runtime file lookups therefore resolve relative to the process CWD,
	// not relative to the config file's directory.
	configDir := t.TempDir()
	tomlPath := filepath.Join(configDir, "brewkit.toml")
	if err := os.WriteFile(tomlPath,
		[]byte(`layers = ["common"]`+"\n"+`env_layers = []`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cwd := t.TempDir()
	if err := os.WriteFile(filepath.Join(cwd, "Brewfile.common"),
		[]byte(`brew "git"  # vcs`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(cwd)

	useConfigFile(t, tomlPath)
	testApp.flags.dryRun = true

	fake := brew.NewFake()
	useBrewer(t, fake)

	out := captureStdout(t, func() {
		if err := runApply(context.Background(), layer.KindBrew, nil); err != nil {
			t.Errorf("runApply err: %v", err)
		}
	})

	if !strings.Contains(out, "+ git") {
		t.Errorf("expected '+ git' in output - layer files must be discovered under process CWD:\n%s", out)
	}
}

func TestRunApply_Tap_Adds(t *testing.T) {
	resetFlags()
	defer resetFlags()

	dir := fixtureRepo(t, map[string]string{
		"Tapfile.common": `charmbracelet/tap` + "\n",
	})
	useConfigFile(t, filepath.Join(dir, "brewkit.toml"))

	fake := brew.NewFake()
	useBrewer(t, fake)

	out := captureStdout(t, func() {
		if err := runApply(context.Background(), layer.KindTap, nil); err != nil {
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
	useConfigFile(t, filepath.Join(dir, "brewkit.toml"))
	testApp.flags.dryRun = true

	fake := brew.NewFake()
	useBrewer(t, fake)

	out := captureStdout(t, func() {
		if err := runApply(context.Background(), layer.KindBrew, nil); err != nil {
			t.Errorf("runApply err: %v", err)
		}
	})

	want := "+ ripgrep (dry-run)\nSummary: 1 added\n"
	if out != want {
		t.Errorf("unexpected dry-run output:\nwant: %q\ngot:  %q", want, out)
	}
	if len(fake.Calls) != 0 {
		t.Errorf("dry-run should not call brewer; got %d calls", len(fake.Calls))
	}
}

func TestRunApply_Brew_OutputPrefix(t *testing.T) {
	resetFlags()
	defer resetFlags()

	dir := fixtureRepo(t, map[string]string{
		"Brewfile.common": `brew "ripgrep"  # search` + "\n",
	})
	useConfigFile(t, filepath.Join(dir, "brewkit.toml"))
	testApp.flags.dryRun = true
	testApp.flags.outputPrefix = "  "

	fake := brew.NewFake()
	useBrewer(t, fake)

	out := captureStdout(t, func() {
		if err := runApply(context.Background(), layer.KindBrew, nil); err != nil {
			t.Errorf("runApply err: %v", err)
		}
	})

	want := "  + ripgrep (dry-run)\n  Summary: 1 added\n"
	if out != want {
		t.Errorf("unexpected prefixed dry-run output:\nwant: %q\ngot:  %q", want, out)
	}
}

func TestRunApply_Brew_QuietSuccessSilent(t *testing.T) {
	resetFlags()
	defer resetFlags()

	dir := fixtureRepo(t, map[string]string{
		"Brewfile.common": `brew "ripgrep"  # search` + "\n",
	})
	useConfigFile(t, filepath.Join(dir, "brewkit.toml"))
	testApp.flags.quiet = true

	fake := brew.NewFake()
	useBrewer(t, fake)

	out, errOut := captureOutput(t, func() {
		if err := runApply(context.Background(), layer.KindBrew, nil); err != nil {
			t.Errorf("runApply err: %v", err)
		}
	})

	if out != "" || errOut != "" {
		t.Errorf("quiet successful run should be silent; stdout=%q stderr=%q", out, errOut)
	}
}

type outputFailFake struct {
	*brew.Fake
	output string
}

func (f *outputFailFake) BrewInstall(_ context.Context, name string) (brew.Result, error) {
	f.Calls = append(f.Calls, brew.FakeCall{Op: brew.OpBrewInstall, Name: name})
	return brew.Result{Output: f.output}, fmt.Errorf("fake: brew install %s failed", name)
}

func TestRunApply_Brew_QuietFailureShowsErrorOutputNoSummary(t *testing.T) {
	resetFlags()
	defer resetFlags()

	dir := fixtureRepo(t, map[string]string{
		"Brewfile.common": `brew "borked"  # broken` + "\n",
	})
	useConfigFile(t, filepath.Join(dir, "brewkit.toml"))
	testApp.flags.quiet = true

	fake := &outputFailFake{Fake: brew.NewFake(), output: "==> Downloading\nError: nope\n"}
	useBrewer(t, fake)

	var runErr error
	out, errOut := captureOutput(t, func() {
		runErr = runApply(context.Background(), layer.KindBrew, nil)
	})

	if runErr == nil {
		t.Fatal("expected runApply to error")
	}
	if out != "" {
		t.Errorf("quiet failure should not print stdout summary, got %q", out)
	}
	for _, want := range []string{"✗ borked: install failed", "==> Downloading", "Error: nope"} {
		if !strings.Contains(errOut, want) {
			t.Errorf("quiet failure stderr missing %q:\n%s", want, errOut)
		}
	}
	if strings.Contains(errOut, "Summary:") {
		t.Errorf("quiet failure should not print summary:\n%s", errOut)
	}
}

func TestRunApply_Brew_UpgradesAndUpToDate(t *testing.T) {
	resetFlags()
	defer resetFlags()

	dir := fixtureRepo(t, map[string]string{
		"Brewfile.common": `brew "neovim"  # editor` + "\n" + `brew "git"     # vcs` + "\n",
	})
	useConfigFile(t, filepath.Join(dir, "brewkit.toml"))

	fake := brew.NewFake()
	fake.FormulasMap["neovim"] = brew.FormulaState{
		Installed: true, Version: "0.10.0", Outdated: true, OutdatedTo: "0.10.2",
	}
	fake.FormulasMap["git"] = brew.FormulaState{Installed: true, Version: "2.45.0"}
	useBrewer(t, fake)

	out := captureStdout(t, func() {
		if err := runApply(context.Background(), layer.KindBrew, nil); err != nil {
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
	useConfigFile(t, filepath.Join(dir, "brewkit.toml"))
	testApp.flags.hideUnchanged = true

	fake := brew.NewFake()
	fake.FormulasMap["neovim"] = brew.FormulaState{
		Installed: true, Version: "0.10.0", Outdated: true, OutdatedTo: "0.10.2",
	}
	fake.FormulasMap["git"] = brew.FormulaState{Installed: true, Version: "2.45.0"}
	useBrewer(t, fake)

	out := captureStdout(t, func() {
		if err := runApply(context.Background(), layer.KindBrew, nil); err != nil {
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
	useConfigFile(t, filepath.Join(dir, "brewkit.toml"))
	testApp.flags.hideUnchanged = true

	fake := brew.NewFake()
	fake.TapsSet["homebrew/core"] = true
	fake.TrustedTaps["homebrew/core"] = true
	useBrewer(t, fake)

	out := captureStdout(t, func() {
		if err := runApply(context.Background(), layer.KindTap, nil); err != nil {
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
	useConfigFile(t, filepath.Join(dir, "brewkit.toml"))
	testApp.flags.hideUnchanged = true

	fake := brew.NewFake()
	fake.CasksMap["iterm2"] = brew.CaskState{Installed: true, Version: "3.5.0"}
	useBrewer(t, fake)

	out := captureStdout(t, func() {
		if err := runApply(context.Background(), layer.KindCask, nil); err != nil {
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
	useConfigFile(t, filepath.Join(dir, "brewkit.toml"))

	fake := brew.NewFake()
	fake.CasksMap["claude"] = brew.CaskState{
		Installed: true, Version: "1.0.0", Outdated: true, OutdatedTo: "1.1.0",
	}
	useBrewer(t, fake)

	out, errOut := captureOutput(t, func() {
		if err := runApply(context.Background(), layer.KindCask, nil); err != nil {
			t.Errorf("runApply err: %v", err)
		}
	})

	wantOut := "↑ claude 1.0.0 → 1.1.0\nSummary: 1 upgraded\n"
	if out != wantOut {
		t.Errorf("cask stdout = %q, want %q", out, wantOut)
	}
	wantErr := "\n⚠ Restart these apps to apply upgrades\n  claude\n"
	if errOut != wantErr {
		t.Errorf("cask stderr = %q, want %q", errOut, wantErr)
	}
}

func TestRunApply_MissingFileNotice(t *testing.T) {
	for _, tt := range []struct {
		name  string
		files map[string]string
		want  string
	}{
		{name: "selected layer", want: "⊘ common: no Brewfile, skipping\nSummary: 1 skipped\n"},
		{name: "local with another kind", files: map[string]string{"Tapfile.local": ""},
			want: "⊘ common: no Brewfile, skipping\n⊘ local: no Brewfile, skipping\nSummary: 2 skipped\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			resetFlags()
			defer resetFlags()
			dir := fixtureRepo(t, tt.files)
			useConfigFile(t, filepath.Join(dir, "brewkit.toml"))
			fake := brew.NewFake()
			useBrewer(t, fake)

			out := captureStdout(t, func() {
				if err := runApply(context.Background(), layer.KindBrew, nil); err != nil {
					t.Errorf("runApply err: %v", err)
				}
			})

			if out != tt.want {
				t.Errorf("unexpected missing-file notice:\nwant: %q\ngot:  %q", tt.want, out)
			}
		})
	}
}

func TestRunApply_MissingFileQuietSilent(t *testing.T) {
	resetFlags()
	defer resetFlags()

	dir := fixtureRepo(t, map[string]string{}) // no layer files
	useConfigFile(t, filepath.Join(dir, "brewkit.toml"))
	testApp.flags.quiet = true

	fake := brew.NewFake()
	useBrewer(t, fake)

	out, errOut := captureOutput(t, func() {
		if err := runApply(context.Background(), layer.KindBrew, nil); err != nil {
			t.Errorf("runApply err: %v", err)
		}
	})

	if out != "" || errOut != "" {
		t.Errorf("quiet run with missing layer files should be silent; stdout=%q stderr=%q", out, errOut)
	}
}

func TestRunApply_LayerOrder(t *testing.T) {
	for _, tt := range []struct {
		name   string
		layers string
		files  map[string]string
		want   string
	}{
		{name: "nothing available", layers: `[]`, want: "Summary: nothing to do\n"},
		{name: "empty selection applies local only", layers: `[]`, files: map[string]string{
			"Brewfile":        `brew "git"  # unsuffixed, never read` + "\n",
			"Brewfile.local":  `brew "jq"  # json` + "\n",
			"Brewfile.common": `brew "ripgrep"  # not selected` + "\n",
		}, want: "+ jq (dry-run)\nSummary: 1 added\n"},
		{name: "selected layers precede local", layers: `["work", "laptop", "work"]`, files: map[string]string{
			"Brewfile.laptop": `brew "laptop-entry"  # laptop` + "\n",
			"Brewfile.work":   `brew "work-entry"    # work` + "\n",
			"Brewfile.local":  `brew "local-entry"   # local` + "\n",
		}, want: "+ laptop-entry (dry-run)\n+ work-entry (dry-run)\n+ local-entry (dry-run)\nSummary: 3 added\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			resetFlags()
			defer resetFlags()
			dir := t.TempDir()
			tomlPath := filepath.Join(dir, "brewkit.toml")
			if err := os.WriteFile(tomlPath, []byte(`dir = "`+dir+`"`+"\n"+`layers = `+tt.layers+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			for name, contents := range tt.files {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			useConfigFile(t, tomlPath)
			testApp.flags.dryRun = true
			useBrewer(t, brew.NewFake())

			out := captureStdout(t, func() {
				if err := runApply(context.Background(), layer.KindBrew, nil); err != nil {
					t.Errorf("runApply err: %v", err)
				}
			})

			if out != tt.want {
				t.Errorf("unexpected output:\nwant: %q\ngot:  %q", tt.want, out)
			}
		})
	}
}

func TestRunApply_PositionalFilter(t *testing.T) {
	resetFlags()
	defer resetFlags()

	dir := fixtureRepo(t, map[string]string{
		"Brewfile.common": `brew "git"     # vcs` + "\n" + `brew "neovim"  # editor` + "\n",
	})
	useConfigFile(t, filepath.Join(dir, "brewkit.toml"))
	testApp.flags.dryRun = true

	fake := brew.NewFake()
	useBrewer(t, fake)

	out := captureStdout(t, func() {
		if err := runApply(context.Background(), layer.KindBrew, []string{"git"}); err != nil {
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
	useConfigFile(t, filepath.Join(dir, "brewkit.toml"))

	fake := brew.NewFake()
	fake.FailOps[brew.OpTap] = map[string]bool{"broken/tap": true}
	useBrewer(t, fake)

	captureStdout(t, func() {
		err := runApply(context.Background(), layer.KindTap, nil)
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
	useConfigFile(t, filepath.Join(dir, "brewkit.toml"))

	fake := brew.NewFake()
	fake.FormulasMap["neovim"] = brew.FormulaState{
		Installed: true, Version: "0.10.0", Outdated: true, OutdatedTo: "0.10.2",
	}
	fake.FailOps[brew.OpBrewUpgrade] = map[string]bool{"neovim": true}
	useBrewer(t, fake)

	captureStdout(t, func() {
		if err := runApply(context.Background(), layer.KindBrew, nil); err == nil {
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
	useConfigFile(t, filepath.Join(dir, "brewkit.toml"))

	fake := brew.NewFake()
	fake.CasksMap["claude"] = brew.CaskState{
		Installed: true, Version: "1.0.0", Outdated: true, OutdatedTo: "1.1.0",
	}
	fake.FailOps[brew.OpCaskUpgrade] = map[string]bool{"claude": true}
	useBrewer(t, fake)

	out, errOut := captureOutput(t, func() {
		if err := runApply(context.Background(), layer.KindCask, nil); err == nil {
			t.Error("expected runApply to error on cask upgrade failure")
		}
	})
	if strings.Contains(out+errOut, "Restart these apps") {
		t.Errorf("failed upgrade must not produce restart notice; stdout=%q stderr=%q", out, errOut)
	}
}

func TestRunApply_Cask_InstallFailure(t *testing.T) {
	resetFlags()
	defer resetFlags()

	dir := fixtureRepo(t, map[string]string{
		"Caskfile.common": `antinote  # notes` + "\n",
	})
	useConfigFile(t, filepath.Join(dir, "brewkit.toml"))

	fake := brew.NewFake()
	fake.FailOps[brew.OpCaskInstall] = map[string]bool{"antinote": true}
	useBrewer(t, fake)

	captureStdout(t, func() {
		if err := runApply(context.Background(), layer.KindCask, nil); err == nil {
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
	useConfigFile(t, filepath.Join(dir, "brewkit.toml"))

	fake := brew.NewFake()
	fake.FailOps[brew.OpBrewInstall] = map[string]bool{"borked": true}
	useBrewer(t, fake)

	captureStdout(t, func() {
		if err := runApply(context.Background(), layer.KindBrew, nil); err == nil {
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
		`layers = ["common"]` + "\n" +
		`env_layers = []` + "\n" +
		`fail_fast = false` + "\n"
	if err := os.WriteFile(tomlPath, []byte(toml), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Brewfile.common"),
		[]byte(`brew "borked"   # broken`+"\n"+`brew "alright"  # ok`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	useConfigFile(t, tomlPath)

	fake := brew.NewFake()
	fake.FailOps[brew.OpBrewInstall] = map[string]bool{"borked": true}
	useBrewer(t, fake)

	captureStdout(t, func() {
		err := runApply(context.Background(), layer.KindBrew, nil)
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
	useConfigFile(t, filepath.Join(dir, "brewkit.toml"))

	fake := brew.NewFake()
	useBrewer(t, fake)

	captureStdout(t, func() {
		if err := runApply(context.Background(), layer.KindBrew, nil); err != nil {
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
	useConfigFile(t, filepath.Join(dir, "brewkit.toml"))

	fake := &headInstalledErrFake{Fake: brew.NewFake(), errName: "tmux"}
	useBrewer(t, fake)

	captureStdout(t, func() {
		if err := runApply(context.Background(), layer.KindHead, nil); err == nil {
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
	useConfigFile(t, filepath.Join(dir, "brewkit.toml"))

	base := brew.NewFake()
	base.HeadInstalls["tmux"] = "abc1234"
	fake := &headLatestErrFake{Fake: base, errName: "tmux", errMsg: "network flake"}
	useBrewer(t, fake)

	captureStdout(t, func() {
		if err := runApply(context.Background(), layer.KindHead, nil); err == nil {
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
	useConfigFile(t, filepath.Join(dir, "brewkit.toml"))

	fake := brew.NewFake()
	fake.HeadLatest["tmux"] = "abc1234"
	fake.HeadHasURL["tmux"] = true
	fake.FailOps[brew.OpHeadInstall] = map[string]bool{"tmux": true}
	useBrewer(t, fake)

	captureStdout(t, func() {
		if err := runApply(context.Background(), layer.KindHead, nil); err == nil {
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
	useConfigFile(t, filepath.Join(dir, "brewkit.toml"))
	testApp.flags.dryRun = true

	fake := brew.NewFake()
	fake.HeadInstalls["tmux"] = "oldsha0"
	fake.HeadLatest["tmux"] = "newsha1"
	useBrewer(t, fake)

	out := captureStdout(t, func() {
		if err := runApply(context.Background(), layer.KindHead, nil); err != nil {
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
	useConfigFile(t, filepath.Join(dir, "brewkit.toml"))

	fake := brew.NewFake()
	fake.HeadInstalls["tmux"] = "oldsha0"
	fake.HeadLatest["tmux"] = "newsha1"
	useBrewer(t, fake)

	out := captureStdout(t, func() {
		if err := runApply(context.Background(), layer.KindHead, nil); err != nil {
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

func (p *stateFailProbe) State(_ context.Context) (*brew.State, error) {
	return nil, fmt.Errorf("brew unavailable")
}

func TestRunApply_EnsureStateFailure(t *testing.T) {
	resetFlags()
	defer resetFlags()

	dir := fixtureRepo(t, map[string]string{
		"Brewfile.common": `brew "git"  # vcs` + "\n",
	})
	useConfigFile(t, filepath.Join(dir, "brewkit.toml"))

	fake := &stateFailProbe{Fake: brew.NewFake()}
	useBrewer(t, fake)

	captureStdout(t, func() {
		err := runApply(context.Background(), layer.KindBrew, nil)
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
	useConfigFile(t, filepath.Join(dir, "brewkit.toml"))

	base := brew.NewFake()
	base.FormulasMap["direnv"] = brew.FormulaState{Installed: true, Version: "2.37.1"}
	fake := &headLatestErrFake{Fake: base, errName: "direnv", errMsg: "network flake"}
	useBrewer(t, fake)

	var runErr error
	out, errOut := captureOutput(t, func() {
		runErr = runApply(context.Background(), layer.KindHead, nil)
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
	useConfigFile(t, filepath.Join(dir, "brewkit.toml"))

	fake := brew.NewFake()
	fake.FormulasMap["direnv"] = brew.FormulaState{Installed: true, Version: "2.37.1"}
	useBrewer(t, fake)

	var runErr error
	_, errOut := captureOutput(t, func() {
		runErr = runApply(context.Background(), layer.KindHead, nil)
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
	useConfigFile(t, filepath.Join(dir, "brewkit.toml"))
	testApp.flags.hideUnchanged = true

	fake := brew.NewFake()
	// HeadHasURL["lonely"] is false (zero value); HeadLatest also empty.
	// Fake.HeadLatestSHA returns hasHead=false in this case.
	useBrewer(t, fake)

	var runErr error
	out, errOut := captureOutput(t, func() {
		runErr = runApply(context.Background(), layer.KindHead, nil)
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

func TestRunApply_DryRun_LayersProjectState(t *testing.T) {
	resetFlags()
	defer resetFlags()

	dir := t.TempDir()
	tomlPath := filepath.Join(dir, "brewkit.toml")
	toml := `dir = "` + dir + `"` + "\n" +
		`layers = ["a", "b"]` + "\n" +
		`env_layers = []` + "\n"
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
	useConfigFile(t, tomlPath)
	testApp.flags.dryRun = true

	fake := brew.NewFake()
	useBrewer(t, fake)

	out := captureStdout(t, func() {
		if err := runApply(context.Background(), layer.KindBrew, nil); err != nil {
			t.Errorf("runApply err: %v", err)
		}
	})

	if !strings.Contains(out, "+ ripgrep") {
		t.Errorf("expected first 'ripgrep' to be added:\n%s", out)
	}
	if !strings.Contains(out, "Summary: 1 added, 1 up-to-date") {
		t.Errorf("layered dry-run should project install - second occurrence should be up-to-date:\n%s", out)
	}
}

// sentinelFailFake wraps brew.Fake and returns caller-supplied sentinel
// errors from BrewInstall so tests can verify errors.Is walks the
// aggregated errors.Join tree.
//
// brew.Fake.BrewInstall records into Calls *before* checking shouldFail
// (see internal/brew/fake.go), so a normal Fake failure still appears
// in the ledger. Because this wrapper short-circuits *before* delegating
// to f.Fake.BrewInstall, it must mirror that recording itself - keep
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
		`layers = ["common"]` + "\n" +
		`env_layers = []` + "\n" +
		`fail_fast = false` + "\n"
	if err := os.WriteFile(tomlPath, []byte(toml), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Brewfile.common"),
		[]byte(`brew "alpha"  # a`+"\n"+`brew "bravo"  # b`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	useConfigFile(t, tomlPath)

	errAlpha := fmt.Errorf("alpha sentinel")
	errBravo := fmt.Errorf("bravo sentinel")
	fake := &sentinelFailFake{
		Fake: brew.NewFake(),
		errs: map[string]error{"alpha": errAlpha, "bravo": errBravo},
	}
	useBrewer(t, fake)

	var runErr error
	captureStdout(t, func() {
		runErr = runApply(context.Background(), layer.KindBrew, nil)
	})
	if runErr == nil {
		t.Fatal("expected aggregated error")
	}
	if !errors.Is(runErr, errAlpha) {
		t.Errorf("errors.Is(err, errAlpha) = false - errors.Join contract broken: %v", runErr)
	}
	if !errors.Is(runErr, errBravo) {
		t.Errorf("errors.Is(err, errBravo) = false - errors.Join contract broken: %v", runErr)
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
		`layers = ["common"]` + "\n" +
		`env_layers = []` + "\n" +
		`fail_fast = false` + "\n"
	if err := os.WriteFile(tomlPath, []byte(toml), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Headfile.common"),
		[]byte(`lonely  # no source`+"\n"+`tmux  # stable`+"\n"+`ok  # valid`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	useConfigFile(t, tomlPath)

	fake := brew.NewFake()
	fake.FormulasMap["tmux"] = brew.FormulaState{Installed: true, Version: "3.4"}
	fake.HeadLatest["tmux"] = "abc1234"
	fake.HeadHasURL["tmux"] = true
	fake.HeadLatest["ok"] = "def5678"
	fake.HeadHasURL["ok"] = true
	useBrewer(t, fake)

	var runErr error
	out, errOut := captureOutput(t, func() {
		runErr = runApply(context.Background(), layer.KindHead, nil)
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
// retry-on-failure invariant across layers. attempts counts
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

func TestRunApply_Head_RetryAfterFailure_AcrossLayers(t *testing.T) {
	resetFlags()
	defer resetFlags()

	// Two layers both list `tmux`. The first invocation
	// fails transiently, but because fail_fast=false and headSeen is
	// marked only on success, the second layer's entry gets a
	// second chance and succeeds.
	dir := t.TempDir()
	tomlPath := filepath.Join(dir, "brewkit.toml")
	toml := `dir = "` + dir + `"` + "\n" +
		`layers = ["a", "b"]` + "\n" +
		`env_layers = []` + "\n" +
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
	useConfigFile(t, tomlPath)

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
		runErr = runApply(context.Background(), layer.KindHead, nil)
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
		t.Errorf("second layer must retry, not short-circuit with 'already processed':\n%s", out)
	}
}

func TestRunApply_UnknownLineFailsRun(t *testing.T) {
	resetFlags()
	defer resetFlags()

	dir := fixtureRepo(t, map[string]string{
		"Brewfile.common": `brew "git"  # vcs` + "\n" +
			`brew "neovim", args: ["with-luajit"]  # broken bundle option` + "\n",
	})
	useConfigFile(t, filepath.Join(dir, "brewkit.toml"))

	fake := brew.NewFake()
	useBrewer(t, fake)

	captureStdout(t, func() {
		err := runApply(context.Background(), layer.KindBrew, nil)
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
		`layers = ["a", "b"]` + "\n" +
		`env_layers = []` + "\n"
	if err := os.WriteFile(tomlPath, []byte(toml), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Headfile.a"), []byte(`tmux  # mux`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Headfile.b"), []byte(`tmux  # mux`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	useConfigFile(t, tomlPath)
	testApp.flags.hideUnchanged = true

	fake := brew.NewFake()
	fake.HeadInstalls["tmux"] = "abc1234"
	fake.HeadLatest["tmux"] = "abc1234"
	useBrewer(t, fake)

	out := captureStdout(t, func() {
		if err := runApply(context.Background(), layer.KindHead, nil); err != nil {
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
		`layers = ["a", "b"]` + "\n" +
		`env_layers = []` + "\n"
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
	useConfigFile(t, tomlPath)
	testApp.flags.dryRun = true

	fake := brew.NewFake()
	fake.HeadInstalls["homebrew/core/tmux"] = "abc1234"
	fake.HeadLatest["homebrew/core/tmux"] = "abc1234"
	useBrewer(t, fake)

	out := captureStdout(t, func() {
		if err := runApply(context.Background(), layer.KindHead, nil); err != nil {
			t.Errorf("runApply err: %v", err)
		}
	})

	if !strings.Contains(out, "already processed") {
		t.Errorf("expected dedupe across qualified/short forms:\n%s", out)
	}
}

func TestRunApply_Head_LayersDeduped(t *testing.T) {
	resetFlags()
	defer resetFlags()

	dir := t.TempDir()
	tomlPath := filepath.Join(dir, "brewkit.toml")
	toml := `dir = "` + dir + `"` + "\n" +
		`layers = ["a", "b"]` + "\n" +
		`env_layers = []` + "\n"
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
	useConfigFile(t, tomlPath)
	testApp.flags.dryRun = true

	fake := brew.NewFake()
	fake.HeadInstalls["tmux"] = "abc1234"
	fake.HeadLatest["tmux"] = "abc1234"
	useBrewer(t, fake)

	out := captureStdout(t, func() {
		if err := runApply(context.Background(), layer.KindHead, nil); err != nil {
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
	useConfigFile(t, filepath.Join(dir, "brewkit.toml"))
	testApp.flags.dryRun = true

	stateFetched := false
	fake := brew.NewFake()
	fake.HeadInstalls["tmux"] = "abc1234"
	fake.HeadLatest["tmux"] = "abc1234"
	wrapped := &stateProbe{Fake: fake, fetched: &stateFetched}
	useBrewer(t, wrapped)

	captureStdout(t, func() {
		if err := runApply(context.Background(), layer.KindHead, nil); err != nil {
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

	dir := fixtureRepo(t, map[string]string{}) // no layer files at all
	useConfigFile(t, filepath.Join(dir, "brewkit.toml"))

	stateFetched := false
	fake := brew.NewFake()
	wrapped := &stateProbe{Fake: fake, fetched: &stateFetched}
	useBrewer(t, wrapped)

	captureStdout(t, func() {
		if err := runApply(context.Background(), layer.KindBrew, nil); err != nil {
			t.Errorf("runApply err: %v", err)
		}
	})
	if stateFetched {
		t.Error("brewer.State should not be called when no layer files exist")
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
	useConfigFile(t, filepath.Join(dir, "brewkit.toml"))

	fake := brew.NewFake()
	useBrewer(t, fake)

	captureStdout(t, func() {
		err := runApply(context.Background(), layer.KindBrew, []string{"absent"})
		if err == nil {
			t.Error("expected not-found error")
		}
	})
}

func runApply(ctx context.Context, kind layer.Kind, args []string) error {
	cmd := newApplyCmd(testApp, applyCommandSpec{kind: kind})
	cmd.SetContext(ctx)
	return cmd.RunE(cmd, args)
}
