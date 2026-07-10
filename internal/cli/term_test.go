package cli

import (
	"bytes"
	"image/color"
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
		{name: "negative", value: "15;-1", ok: false},
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

func TestIsDarkColor(t *testing.T) {
	tests := []struct {
		name  string
		color color.Color
		dark  bool
	}{
		{name: "missing color", color: nil, dark: true},
		{name: "black", color: color.RGBA{R: 0, G: 0, B: 0, A: 255}, dark: true},
		{name: "white", color: color.RGBA{R: 255, G: 255, B: 255, A: 255}, dark: false},
		{name: "Catppuccin Frappe base", color: color.RGBA{R: 48, G: 52, B: 70, A: 255}, dark: true},
		{name: "Catppuccin Latte base", color: color.RGBA{R: 239, G: 241, B: 245, A: 255}, dark: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isDarkColor(tt.color); got != tt.dark {
				t.Fatalf("isDarkColor(%v) = %v, want %v", tt.color, got, tt.dark)
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

func TestDetectDarkBackgroundPrecedence(t *testing.T) {
	queryCalled := false
	queryLight := func() (color.Color, bool) {
		queryCalled = true
		return color.White, true
	}
	if got := detectDarkBackground(colorprofile.ASCII, queryLight, "0;15"); !got || queryCalled {
		t.Fatalf("color-disabled detection = %v, query called = %v; want dark without a query", got, queryCalled)
	}

	if got := detectDarkBackground(colorprofile.TrueColor, queryLight, "15;0"); got {
		t.Fatal("a light terminal query should override a dark COLORFGBG hint")
	}
	queryDark := func() (color.Color, bool) { return color.Black, true }
	if got := detectDarkBackground(colorprofile.TrueColor, queryDark, "0;15"); !got {
		t.Fatal("a dark terminal query should override a light COLORFGBG hint")
	}
	queryFailed := func() (color.Color, bool) { return nil, false }
	if got := detectDarkBackground(colorprofile.TrueColor, queryFailed, "0;15"); got {
		t.Fatal("a failed query should fall back to a light COLORFGBG hint")
	}
	if got := detectDarkBackground(colorprofile.TrueColor, queryFailed, "invalid"); !got {
		t.Fatal("failed query and invalid COLORFGBG should fall back to dark")
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
