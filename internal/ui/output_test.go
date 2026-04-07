package ui

import (
	"bytes"
	"strings"
	"testing"
)

func newTestPrinter(level Level) (*Printer, *bytes.Buffer, *bytes.Buffer) {
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	return New(out, errOut, level, false, false), out, errOut
}

func TestPrinter_NormalStream(t *testing.T) {
	p, out, _ := newTestPrinter(LevelNormal)
	p.Group("brew")
	p.Item(SymUpToDate, "git", "(2.45.0)")
	p.Item(SymUpgraded, "neovim", "0.10.0 → 0.10.2")
	p.Item(SymAdded, "ripgrep", "")
	p.Footer()

	got := out.String()
	wantContains := []string{
		"brew",
		"✓ git",
		"↑ neovim",
		"+ ripgrep",
		"Summary: 1 added, 1 upgraded, 1 up-to-date",
	}
	for _, s := range wantContains {
		if !strings.Contains(got, s) {
			t.Errorf("output missing %q\nfull output:\n%s", s, got)
		}
	}
}

func TestPrinter_QuietSuppressesStream(t *testing.T) {
	p, out, _ := newTestPrinter(LevelQuiet)
	p.Group("brew")
	p.Item(SymUpToDate, "git", "(2.45.0)")
	p.Item(SymAdded, "ripgrep", "")
	p.Footer()

	got := out.String()
	if strings.Contains(got, "brew\n") || strings.Contains(got, "ripgrep") {
		t.Errorf("quiet output should not contain stream lines:\n%s", got)
	}
	if !strings.Contains(got, "Summary:") {
		t.Errorf("quiet output should contain summary:\n%s", got)
	}
}

func TestPrinter_QuietStillShowsErrors(t *testing.T) {
	p, _, errOut := newTestPrinter(LevelQuiet)
	p.Error("git", "install failed", "==> brew error\nblah")
	p.Footer()

	got := errOut.String()
	if !strings.Contains(got, "git: install failed") {
		t.Errorf("error not printed: %s", got)
	}
	if !strings.Contains(got, "brew error") {
		t.Errorf("error detail not printed: %s", got)
	}
}

func TestPrinter_VerboseAddsRawOutput(t *testing.T) {
	p, out, _ := newTestPrinter(LevelVerbose)
	p.Group("brew")
	p.Item(SymAdded, "ripgrep", "")
	p.Verbose("==> Downloading...\n==> Pouring ripgrep.bottle...")
	p.Footer()

	got := out.String()
	if !strings.Contains(got, "Downloading") || !strings.Contains(got, "Pouring") {
		t.Errorf("verbose did not include raw output:\n%s", got)
	}
}

func TestPrinter_NormalSkipsRawOutput(t *testing.T) {
	p, out, _ := newTestPrinter(LevelNormal)
	p.Item(SymAdded, "ripgrep", "")
	p.Verbose("==> Downloading...")
	p.Footer()

	if strings.Contains(out.String(), "Downloading") {
		t.Errorf("normal level should not print verbose output:\n%s", out.String())
	}
}

func TestPrinter_Notice(t *testing.T) {
	p, out, _ := newTestPrinter(LevelNormal)
	p.Notice("work: no Headfile, skipping")
	p.Footer()

	got := out.String()
	if !strings.Contains(got, "work: no Headfile") {
		t.Errorf("notice not rendered:\n%s", got)
	}
	if !strings.Contains(got, "1 skipped") {
		t.Errorf("summary should reflect skip:\n%s", got)
	}
}

func TestPrinter_DryRunMarker(t *testing.T) {
	out := &bytes.Buffer{}
	p := New(out, out, LevelNormal, false, true)
	p.Item(SymAdded, "ripgrep", "")
	if !strings.Contains(out.String(), "(dry-run)") {
		t.Errorf("expected (dry-run) marker:\n%s", out.String())
	}
}

func TestPrinter_RestartAppsNotice(t *testing.T) {
	p, out, _ := newTestPrinter(LevelNormal)
	p.RestartAppsNotice([]string{"chatgpt", "claude"})

	got := out.String()
	if !strings.Contains(got, "Restart these apps") {
		t.Errorf("notice missing:\n%s", got)
	}
	if !strings.Contains(got, "chatgpt") || !strings.Contains(got, "claude") {
		t.Errorf("apps missing:\n%s", got)
	}
}

func TestPrinter_FooterNothingToDo(t *testing.T) {
	p, out, _ := newTestPrinter(LevelNormal)
	p.Footer()
	if !strings.Contains(out.String(), "nothing to do") {
		t.Errorf("expected 'nothing to do':\n%s", out.String())
	}
}
