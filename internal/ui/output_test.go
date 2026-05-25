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
	p.Item(SymUpToDate, "git", "(2.45.0)")
	p.Item(SymUpgraded, "neovim", "0.10.0 → 0.10.2")
	p.Item(SymAdded, "ripgrep", "")
	p.Footer()

	got := out.String()
	want := "✓ git (2.45.0)\n" +
		"↑ neovim 0.10.0 → 0.10.2\n" +
		"+ ripgrep\n" +
		"\n" +
		"Summary: 1 added, 1 upgraded, 1 up-to-date\n"
	if got != want {
		t.Errorf("unexpected output:\nwant:\n%q\ngot:\n%q", want, got)
	}
}

func TestPrinter_QuietSuppressesStream(t *testing.T) {
	p, out, _ := newTestPrinter(LevelQuiet)
	p.Item(SymUpToDate, "git", "(2.45.0)")
	p.Item(SymAdded, "ripgrep", "")
	p.Footer()

	got := out.String()
	want := "Summary: 1 added, 1 up-to-date\n"
	if got != want {
		t.Errorf("unexpected quiet output:\nwant: %q\ngot:  %q", want, got)
	}
}

func TestPrinter_QuietStillShowsErrors(t *testing.T) {
	p, _, errOut := newTestPrinter(LevelQuiet)
	p.Error("git", "install failed", "==> brew error\nblah")
	p.Footer()

	got := errOut.String()
	want := "✗ git: install failed\n" +
		"    ==> brew error\n" +
		"    blah\n"
	if got != want {
		t.Errorf("unexpected error output:\nwant: %q\ngot:  %q", want, got)
	}
}

func TestPrinter_VerboseAddsRawOutput(t *testing.T) {
	p, out, _ := newTestPrinter(LevelVerbose)
	p.Item(SymAdded, "ripgrep", "")
	p.Verbose("==> Downloading...\n==> Pouring ripgrep.bottle...")
	p.Footer()

	got := out.String()
	want := "+ ripgrep\n" +
		"    ==> Downloading...\n" +
		"    ==> Pouring ripgrep.bottle...\n" +
		"\n" +
		"Summary: 1 added\n"
	if got != want {
		t.Errorf("unexpected verbose output:\nwant: %q\ngot:  %q", want, got)
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
	want := "∘ work: no Headfile, skipping\n\nSummary: 1 skipped\n"
	if got != want {
		t.Errorf("unexpected notice output:\nwant: %q\ngot:  %q", want, got)
	}
}

func TestPrinter_DryRunMarker(t *testing.T) {
	out := &bytes.Buffer{}
	p := New(out, out, LevelNormal, false, true)
	p.Item(SymAdded, "ripgrep", "")
	want := "+ ripgrep (dry-run)\n"
	if out.String() != want {
		t.Errorf("unexpected dry-run output:\nwant: %q\ngot:  %q", want, out.String())
	}
}

func TestPrinter_RestartAppsNotice(t *testing.T) {
	p, out, _ := newTestPrinter(LevelNormal)
	p.RestartAppsNotice([]string{"chatgpt", "claude"})

	got := out.String()
	want := "⚠ Restart these apps to apply upgrades\n" +
		"  chatgpt\n" +
		"  claude\n"
	if got != want {
		t.Errorf("unexpected restart notice:\nwant: %q\ngot:  %q", want, got)
	}
}

func TestPrinter_FooterNothingToDo(t *testing.T) {
	p, out, _ := newTestPrinter(LevelNormal)
	p.Footer()
	want := "Summary: nothing to do\n"
	if out.String() != want {
		t.Errorf("unexpected footer output:\nwant: %q\ngot:  %q", want, out.String())
	}
}
