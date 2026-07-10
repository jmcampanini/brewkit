package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/charmbracelet/colorprofile"
)

func TestDarkBackgroundFromEnv(t *testing.T) {
	tests := []struct {
		name  string
		value string
		dark  bool
		ok    bool
	}{
		{name: "missing", value: "", ok: false},
		{name: "single field", value: "15", ok: false},
		{name: "black", value: "15;0", dark: true, ok: true},
		{name: "dark color", value: "15;6", dark: true, ok: true},
		{name: "light gray", value: "0;7", dark: false, ok: true},
		{name: "dark gray", value: "0;8", dark: true, ok: true},
		{name: "bright color", value: "0;9", dark: false, ok: true},
		{name: "white", value: "0;15", dark: false, ok: true},
		{name: "extra fields use last", value: "1;2;0", dark: true, ok: true},
		{name: "negative follows Grove classification", value: "15;-1", dark: true, ok: true},
		{name: "invalid", value: "15;white", ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dark, ok := darkBackgroundFromEnv(tt.value)
			if dark != tt.dark || ok != tt.ok {
				t.Fatalf("darkBackgroundFromEnv(%q) = (%v, %v), want (%v, %v)", tt.value, dark, ok, tt.dark, tt.ok)
			}
		})
	}
}

func TestTerminalColorProfileHonorsAnyNonemptyNoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "please")
	t.Setenv("CLICOLOR_FORCE", "1")
	t.Setenv("TERM", "xterm-256color")

	if got := terminalColorProfile(&bytes.Buffer{}); got != colorprofile.ASCII {
		t.Fatalf("terminalColorProfile with NO_COLOR = %v, want ASCII", got)
	}
}

func TestTerminalColorEnvironmentOmitsTmux(t *testing.T) {
	t.Setenv("TMUX", "/private/tmp/tmux/default,1,0")
	t.Setenv("BREWKIT_TEST_COLOR_ENV", "kept")

	foundKept := false
	for _, entry := range terminalColorEnvironment() {
		key, value, _ := strings.Cut(entry, "=")
		switch key {
		case "TMUX":
			t.Fatalf("terminal color environment should omit TMUX: %q", entry)
		case "BREWKIT_TEST_COLOR_ENV":
			foundKept = value == "kept"
		}
	}
	if !foundKept {
		t.Fatal("terminal color environment should preserve unrelated variables")
	}
}

func TestDetectDarkBackgroundPrecedence(t *testing.T) {
	queryCalled := false
	queryLight := func() bool {
		queryCalled = true
		return false
	}
	if got := detectDarkBackground(colorprofile.ASCII, queryLight, "0;15"); got || queryCalled {
		t.Fatalf("COLORFGBG light detection = %v, query called = %v; want light without a query", got, queryCalled)
	}

	if got := detectDarkBackground(colorprofile.TrueColor, queryLight, "15;0"); !got || queryCalled {
		t.Fatalf("COLORFGBG dark detection = %v, query called = %v; want dark without a query", got, queryCalled)
	}
	if got := detectDarkBackground(colorprofile.ASCII, queryLight, "invalid"); !got || queryCalled {
		t.Fatalf("color-disabled detection = %v, query called = %v; want dark without a query", got, queryCalled)
	}
	if got := detectDarkBackground(colorprofile.TrueColor, nil, "invalid"); !got || queryCalled {
		t.Fatalf("query-ineligible detection = %v, query called = %v; want dark without a query", got, queryCalled)
	}
	if got := detectDarkBackground(colorprofile.TrueColor, queryLight, "invalid"); got || !queryCalled {
		t.Fatalf("queried light detection = %v, query called = %v; want light from the query", got, queryCalled)
	}
}

func TestTerminalHasDarkBackgroundCanSkipQuery(t *testing.T) {
	t.Setenv("COLORFGBG", "0;15")
	if terminalHasDarkBackground(nil, colorprofile.TrueColor, false) {
		t.Fatal("query-disabled detection should still use the light COLORFGBG hint")
	}

	t.Setenv("COLORFGBG", "15;0")
	if !terminalHasDarkBackground(nil, colorprofile.TrueColor, false) {
		t.Fatal("query-disabled detection should still use the dark COLORFGBG hint")
	}
}
