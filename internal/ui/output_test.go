package ui

import (
	"bytes"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
)

func newTestPrinter(level Level) (*Printer, *bytes.Buffer, *bytes.Buffer) {
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	return New(out, errOut, PrinterOptions{Level: level}), out, errOut
}

type signalWriter struct {
	mu    sync.Mutex
	buf   bytes.Buffer
	wrote chan struct{}
	once  sync.Once
}

func newSignalWriter() *signalWriter {
	return &signalWriter{wrote: make(chan struct{})}
}

func (w *signalWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	n, err := w.buf.Write(p)
	w.mu.Unlock()
	w.once.Do(func() { close(w.wrote) })
	return n, err
}

func (w *signalWriter) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.String()
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

func TestPrinter_HideUnchangedSuppressesOnlyUpToDate(t *testing.T) {
	out := &bytes.Buffer{}
	p := New(out, out, PrinterOptions{Level: LevelNormal, HideUnchanged: true})
	p.Item(SymUpToDate, "git", "(2.45.0)")
	p.Item(SymAdded, "ripgrep", "")
	p.Footer()

	got := out.String()
	want := "+ ripgrep\n\nSummary: 1 added, 1 up-to-date\n"
	if got != want {
		t.Errorf("unexpected hide-unchanged output:\nwant: %q\ngot:  %q", want, got)
	}
}

func TestPrinter_QuietSuppressesStreamAndFooter(t *testing.T) {
	p, out, _ := newTestPrinter(LevelQuiet)
	p.Item(SymUpToDate, "git", "(2.45.0)")
	p.Item(SymAdded, "ripgrep", "")
	p.Footer()

	if got := out.String(); got != "" {
		t.Errorf("quiet success output should be empty, got %q", got)
	}
}

func TestPrinter_QuietStillShowsErrors(t *testing.T) {
	p, out, errOut := newTestPrinter(LevelQuiet)
	p.Error("git", "install failed", "==> brew error\nblah")
	p.Footer()

	if got := out.String(); got != "" {
		t.Errorf("quiet error footer should be suppressed, got stdout %q", got)
	}
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

func TestPrinter_SpinnerDisabledIsSilentAndPreservesError(t *testing.T) {
	p, out, errOut := newTestPrinter(LevelNormal)
	wantErr := errors.New("boom")
	called := false

	err := p.WithSpinner("Checking Homebrew state…", func() error {
		called = true
		return wantErr
	})

	if !called {
		t.Fatal("spinner callback was not called")
	}
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected spinner to preserve callback error, got %v", err)
	}
	if out.Len() != 0 || errOut.Len() != 0 {
		t.Fatalf("disabled spinner should be silent; stdout=%q stderr=%q", out.String(), errOut.String())
	}
}

func TestPrinter_SpinnerActiveWritesOnlyTransientStderr(t *testing.T) {
	out := &bytes.Buffer{}
	errOut := newSignalWriter()
	p := New(out, errOut, PrinterOptions{Level: LevelNormal, Spinner: true})

	if err := p.WithSpinner("Installing ripgrep…", func() error {
		select {
		case <-errOut.wrote:
			return nil
		case <-time.After(time.Second):
			return errors.New("spinner did not render")
		}
	}); err != nil {
		t.Fatal(err)
	}

	if out.Len() != 0 {
		t.Fatalf("spinner should not write durable stdout output: %q", out.String())
	}
	got := errOut.String()
	if !strings.Contains(got, "Installing ripgrep…") || !strings.HasSuffix(got, spinnerClearSequence) {
		t.Fatalf("spinner should render then clear stderr; got %q", got)
	}
}

func TestPrinter_SpinnerTruncatesToTerminalWidth(t *testing.T) {
	p := New(&bytes.Buffer{}, &bytes.Buffer{}, PrinterOptions{
		Level:        LevelNormal,
		Spinner:      true,
		SpinnerWidth: 16,
	})
	line := p.truncateSpinnerLine("⠋ Installing " + strings.Repeat("x", 100))

	if width := ansi.StringWidth(line); width > 15 {
		t.Fatalf("spinner line width = %d, want <= 15; line=%q", width, line)
	}
	if !strings.HasSuffix(line, spinnerTruncateTail) {
		t.Fatalf("long spinner line should be visibly truncated; got %q", line)
	}
}

func TestPrinter_Notice(t *testing.T) {
	p, out, _ := newTestPrinter(LevelNormal)
	p.Notice("work: no Headfile, skipping")
	p.Footer()

	got := out.String()
	want := "⊘ work: no Headfile, skipping\n\nSummary: 1 skipped\n"
	if got != want {
		t.Errorf("unexpected notice output:\nwant: %q\ngot:  %q", want, got)
	}
}

func TestPrinter_DryRunMarker(t *testing.T) {
	out := &bytes.Buffer{}
	p := New(out, out, PrinterOptions{Level: LevelNormal, DryRun: true})
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
