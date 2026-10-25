package cli

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunConfig_PrintsToml(t *testing.T) {
	resetFlags()
	defer resetFlags()

	dir := fixtureRepo(t, map[string]string{})
	testApp.flags.configPath = filepath.Join(dir, "brewkit.toml")

	out := captureStdout(t, func() {
		if err := testApp.runConfig(context.Background()); err != nil {
			t.Errorf("runConfig err: %v", err)
		}
	})

	for _, want := range []string{
		`profiles = ["common"]`,
		"# provenance:",
		"# effective:",
		`# effective_profiles = ["common"]`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("config output missing %q:\n%s", want, out)
		}
	}
}

func TestRunConfig_QuietStillPrintsToml(t *testing.T) {
	resetFlags()
	defer resetFlags()

	dir := fixtureRepo(t, map[string]string{})
	testApp.flags.configPath = filepath.Join(dir, "brewkit.toml")
	testApp.flags.quiet = true

	out := captureStdout(t, func() {
		if err := testApp.runConfig(context.Background()); err != nil {
			t.Errorf("runConfig err: %v", err)
		}
	})
	if !strings.Contains(out, `profiles = ["common"]`) {
		t.Errorf("quiet config should still print config payload:\n%s", out)
	}
}

func TestRunConfig_EffectiveDirAbsError(t *testing.T) {
	resetFlags()
	defer resetFlags()

	dir := fixtureRepo(t, map[string]string{})
	testApp.flags.configPath = filepath.Join(dir, "brewkit.toml")

	oldAbs := filepathAbs
	filepathAbs = func(string) (string, error) { return "", errors.New("cwd unavailable") }
	defer func() { filepathAbs = oldAbs }()

	captureStdout(t, func() {
		err := testApp.runConfig(context.Background())
		if err == nil {
			t.Fatal("expected effective dir resolution error")
		}
		if !strings.Contains(err.Error(), "resolve effective dir") || !strings.Contains(err.Error(), "cwd unavailable") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestRunConfig_FiltersAutoLocal(t *testing.T) {
	resetFlags()
	defer resetFlags()

	dir := fixtureRepo(t, map[string]string{
		"Brewfile.local": "",
	})
	testApp.flags.configPath = filepath.Join(dir, "brewkit.toml")

	out := captureStdout(t, func() {
		if err := testApp.runConfig(context.Background()); err != nil {
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
