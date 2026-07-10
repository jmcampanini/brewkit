package ui

import (
	"bytes"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/charmbracelet/colorprofile"
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

func TestPrinter_OutputPrefixPrefixesDurableLines(t *testing.T) {
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	p := New(out, errOut, PrinterOptions{Level: LevelNormal, OutputPrefix: "  "})
	p.Item(SymAdded, "ripgrep", "")
	p.Error("git", "install failed", "brew error")
	p.Footer()

	wantOut := "  + ripgrep\n" +
		"  \n" +
		"  Summary: 1 added, 1 failed\n"
	if got := out.String(); got != wantOut {
		t.Errorf("unexpected prefixed stdout:\nwant: %q\ngot:  %q", wantOut, got)
	}
	wantErr := "  ✗ git: install failed\n" +
		"      brew error\n"
	if got := errOut.String(); got != wantErr {
		t.Errorf("unexpected prefixed stderr:\nwant: %q\ngot:  %q", wantErr, got)
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

	err := p.WithSpinner("Checking Homebrew state...", func() error {
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

	if err := p.WithSpinner("Installing ripgrep...", func() error {
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
	if !strings.Contains(got, "Installing ripgrep...") || !strings.HasSuffix(got, spinnerClearSequence) {
		t.Fatalf("spinner should render then clear stderr; got %q", got)
	}
}

func TestPrinter_OutputPrefixPrefixesSpinner(t *testing.T) {
	out := &bytes.Buffer{}
	errOut := newSignalWriter()
	p := New(out, errOut, PrinterOptions{
		Level:        LevelNormal,
		OutputPrefix: "  ",
		Spinner:      true,
	})

	if err := p.WithSpinner("Installing ripgrep...", func() error {
		select {
		case <-errOut.wrote:
			return nil
		case <-time.After(time.Second):
			return errors.New("spinner did not render")
		}
	}); err != nil {
		t.Fatal(err)
	}

	got := errOut.String()
	if !strings.Contains(got, "  ⠋ Installing ripgrep...") {
		t.Fatalf("spinner should include output prefix; got %q", got)
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

func TestPrinter_CatppuccinThemes(t *testing.T) {
	tests := []struct {
		name       string
		theme      Theme
		okRGB      string
		addedRGB   string
		accentRGB  string
		errorRGB   string
		detailRGB  string
		warningRGB string
	}{
		{
			name:       "Latte on a light background",
			theme:      LightTheme(),
			okRGB:      "64;160;43",
			addedRGB:   "30;102;245",
			accentRGB:  "136;57;239",
			errorRGB:   "210;15;57",
			detailRGB:  "92;95;119",
			warningRGB: "223;142;29",
		},
		{
			name:       "Frappe on a dark background",
			theme:      DarkTheme(),
			okRGB:      "166;209;137",
			addedRGB:   "140;170;238",
			accentRGB:  "202;158;230",
			errorRGB:   "231;130;132",
			detailRGB:  "181;191;226",
			warningRGB: "229;200;144",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := &bytes.Buffer{}
			errOut := &bytes.Buffer{}
			p := New(out, errOut, PrinterOptions{
				Level:         LevelNormal,
				OutputProfile: colorprofile.TrueColor,
				ErrorProfile:  colorprofile.TrueColor,
				Theme:         tt.theme,
			})
			p.Item(SymUpgraded, "neovim", "0.10.0 → 0.10.2")
			p.Item(SymUpToDate, "git", "(2.45.0)")
			p.Item(SymAdded, "ripgrep", "")
			p.Notice("work: no Headfile, skipping")
			p.RestartAppsNotice([]string{"chatgpt"})
			p.Error("broken", "install failed", "")

			upgrade := trueColor(tt.accentRGB, "↑") + " neovim " +
				trueColor(tt.detailRGB, "0.10.0 ") +
				trueColor(tt.accentRGB, "→") +
				trueColor(tt.detailRGB, " 0.10.2") + "\n"
			if !strings.HasPrefix(out.String(), upgrade) {
				t.Fatalf("unexpected themed upgrade:\nwant prefix: %q\ngot:         %q", upgrade, out.String())
			}

			combined := out.String() + errOut.String()
			for role, rgb := range map[string]string{
				"up to date": tt.okRGB,
				"added":      tt.addedRGB,
				"upgraded":   tt.accentRGB,
				"error":      tt.errorRGB,
				"detail":     tt.detailRGB,
				"warning":    tt.warningRGB,
			} {
				if !strings.Contains(combined, "\x1b[38;2;"+rgb+"m") {
					t.Errorf("%s color %s missing from %q", role, rgb, combined)
				}
			}
			if strings.Contains(combined, "\x1b[2m") {
				t.Fatalf("secondary text should use a palette color, not terminal faint: %q", combined)
			}
			if !strings.Contains(out.String(), trueColor(tt.warningRGB, "⚠")+" Restart these apps") {
				t.Fatalf("only the warning glyph should carry the low-contrast yellow: %q", out.String())
			}
		})
	}
}

func TestThemeForBackground(t *testing.T) {
	if got := ThemeForBackground(false).name; got != "Catppuccin Latte" {
		t.Fatalf("light background theme = %q, want Catppuccin Latte", got)
	}
	if got := ThemeForBackground(true).name; got != "Catppuccin Frappe" {
		t.Fatalf("dark background theme = %q, want Catppuccin Frappe", got)
	}
}

func TestPrinter_UsesIndependentOutputProfiles(t *testing.T) {
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	p := New(out, errOut, PrinterOptions{
		Level:         LevelNormal,
		OutputProfile: colorprofile.ASCII,
		ErrorProfile:  colorprofile.TrueColor,
		Theme:         DarkTheme(),
	})
	p.Item(SymAdded, "ripgrep", "")
	p.Error("git", "install failed", "")

	if strings.Contains(out.String(), "\x1b[") {
		t.Fatalf("ASCII stdout profile should be plain: %q", out.String())
	}
	if !strings.Contains(errOut.String(), "\x1b[38;2;231;130;132m✗\x1b[m") {
		t.Fatalf("truecolor stderr profile should remain colored: %q", errOut.String())
	}
}

func TestPrinter_DownsamplesThemeForLimitedProfiles(t *testing.T) {
	tests := []struct {
		name        string
		profile     colorprofile.Profile
		wantANSI256 bool
	}{
		{name: "ANSI", profile: colorprofile.ANSI},
		{name: "ANSI256", profile: colorprofile.ANSI256, wantANSI256: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := &bytes.Buffer{}
			p := New(out, &bytes.Buffer{}, PrinterOptions{
				Level:         LevelNormal,
				OutputProfile: tt.profile,
				Theme:         DarkTheme(),
			})
			p.Item(SymUpgraded, "neovim", "1 → 2")

			if !strings.Contains(out.String(), "\x1b[") {
				t.Fatalf("%s profile should retain color: %q", tt.name, out.String())
			}
			if strings.Contains(out.String(), "38;2;") {
				t.Fatalf("%s profile should downsample truecolor: %q", tt.name, out.String())
			}
			if got := strings.Contains(out.String(), "38;5;"); got != tt.wantANSI256 {
				t.Fatalf("%s ANSI256 sequence present = %v, want %v: %q", tt.name, got, tt.wantANSI256, out.String())
			}
		})
	}
}

func TestPrinter_SpinnerNeedsRoomForThreeDotTail(t *testing.T) {
	tooNarrow := New(&bytes.Buffer{}, &bytes.Buffer{}, PrinterOptions{
		Level:        LevelNormal,
		Spinner:      true,
		SpinnerWidth: minSpinnerWidth - 1,
	})
	if tooNarrow.spinner {
		t.Fatal("spinner should be disabled when only the three-dot tail would fit")
	}

	usable := New(&bytes.Buffer{}, &bytes.Buffer{}, PrinterOptions{
		Level:        LevelNormal,
		Spinner:      true,
		SpinnerWidth: minSpinnerWidth,
	})
	if !usable.spinner {
		t.Fatal("spinner should be enabled when one content cell and the three-dot tail fit")
	}
	if got := usable.truncateSpinnerLine("⠋ Installing ripgrep..."); got != "⠋..." {
		t.Fatalf("narrow spinner = %q, want %q", got, "⠋...")
	}
}

func trueColor(rgb, s string) string {
	return "\x1b[38;2;" + rgb + "m" + s + "\x1b[m"
}

func TestLinePrefixWriterPrefixesEachLine(t *testing.T) {
	out := &bytes.Buffer{}
	w := NewLinePrefixWriter(out, "  ")
	if _, err := w.Write([]byte("a\nb\n\nc")); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("d\n")); err != nil {
		t.Fatal(err)
	}

	want := "  a\n  b\n  \n  cd\n"
	if got := out.String(); got != want {
		t.Errorf("unexpected prefixed output:\nwant: %q\ngot:  %q", want, got)
	}
}
