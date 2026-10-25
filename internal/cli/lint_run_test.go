package cli

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

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
	testApp.flags.configPath = filepath.Join(dir, "brewkit.toml")

	out := captureStdout(t, func() {
		if err := testApp.runLint(context.Background()); err != nil {
			t.Errorf("runLint err: %v", err)
		}
	})
	if !strings.Contains(out, "no violations") {
		t.Errorf("expected 'no violations':\n%s", out)
	}
}

func TestRunLint_ProfileFlagNarrowsScan(t *testing.T) {
	resetFlags()
	defer resetFlags()

	dir := fixtureRepo(t, map[string]string{
		"Brewfile.common": strings.Join([]string{
			`brew "abc"  # a`,
			`brew "def"  # d`,
			``,
		}, "\n"),
		"Brewfile.other": strings.Join([]string{
			`brew "z"  # z`,
			`brew "a"  # a`,
			``,
		}, "\n"),
	})

	root := newRootCmd(testApp)
	root.SetArgs([]string{"--config", filepath.Join(dir, "brewkit.toml"), "--profile", "common", "lint"})

	out := captureStdout(t, func() {
		if err := root.Execute(); err != nil {
			t.Errorf("root.Execute err: %v", err)
		}
	})
	if !strings.Contains(out, "no violations") {
		t.Errorf("expected narrowed lint to ignore other profile violations:\n%s", out)
	}
}

func TestRunLint_OutputPrefix(t *testing.T) {
	resetFlags()
	defer resetFlags()

	dir := fixtureRepo(t, map[string]string{
		"Brewfile.common": strings.Join([]string{
			`brew "abc"  # a`,
			`brew "def"  # d`,
			``,
		}, "\n"),
	})
	testApp.flags.configPath = filepath.Join(dir, "brewkit.toml")
	testApp.flags.outputPrefix = "  "

	out := captureStdout(t, func() {
		if err := testApp.runLint(context.Background()); err != nil {
			t.Errorf("runLint err: %v", err)
		}
	})
	if out != "  no violations\n" {
		t.Errorf("unexpected prefixed lint output: %q", out)
	}
}

func TestRunLint_QuietCleanSilent(t *testing.T) {
	resetFlags()
	defer resetFlags()

	dir := fixtureRepo(t, map[string]string{
		"Brewfile.common": strings.Join([]string{
			`brew "abc"  # a`,
			`brew "def"  # d`,
			``,
		}, "\n"),
	})
	testApp.flags.configPath = filepath.Join(dir, "brewkit.toml")
	testApp.flags.quiet = true

	out := captureStdout(t, func() {
		if err := testApp.runLint(context.Background()); err != nil {
			t.Errorf("runLint err: %v", err)
		}
	})
	if out != "" {
		t.Errorf("quiet clean lint should be silent, got %q", out)
	}
}

func TestRunLint_ReportsViolations(t *testing.T) {
	resetFlags()
	defer resetFlags()

	dir := fixtureRepo(t, map[string]string{
		"Brewfile.common": `brew "z"  # z` + "\n" + `brew "a"  # a` + "\n",
	})
	testApp.flags.configPath = filepath.Join(dir, "brewkit.toml")

	captureStdout(t, func() {
		err := testApp.runLint(context.Background())
		if err == nil {
			t.Error("expected lint to error")
		}
	})
}

func TestRunLint_QuietReportsViolationsNoSummary(t *testing.T) {
	resetFlags()
	defer resetFlags()

	dir := fixtureRepo(t, map[string]string{
		"Brewfile.common": `brew "z"  # z` + "\n" + `brew "a"  # a` + "\n",
	})
	testApp.flags.configPath = filepath.Join(dir, "brewkit.toml")
	testApp.flags.quiet = true

	out := captureStdout(t, func() {
		err := testApp.runLint(context.Background())
		if err == nil {
			t.Error("expected lint to error")
		}
	})

	if !strings.Contains(out, "[sort-order]") {
		t.Errorf("quiet lint should print individual violations:\n%s", out)
	}
	if strings.Contains(out, "Summary:") {
		t.Errorf("quiet lint should not print summary:\n%s", out)
	}
}
