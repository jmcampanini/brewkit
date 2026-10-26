package ui

import (
	"bytes"
	"slices"
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

func TestStreamColorProfile(t *testing.T) {
	tests := []struct {
		name     string
		mode     ColorMode
		terminal bool
		env      []string
		want     colorprofile.Profile
	}{
		{name: "never on a color terminal", mode: ColorNever, terminal: true, env: []string{"TERM=xterm-256color", "COLORTERM=truecolor"}, want: colorprofile.ASCII},
		{name: "always on a non-terminal", mode: ColorAlways, terminal: false, env: []string{"TERM=xterm-256color"}, want: colorprofile.ANSI256},
		{name: "always overrides NO_COLOR", mode: ColorAlways, terminal: true, env: []string{"TERM=xterm-256color", "NO_COLOR=1"}, want: colorprofile.ANSI256},
		{name: "always without a terminal description", mode: ColorAlways, terminal: false, env: []string{"TERM=dumb"}, want: colorprofile.ANSI},
		{name: "auto honors any nonempty NO_COLOR", mode: ColorAuto, terminal: true, env: []string{"TERM=xterm-256color", "CLICOLOR_FORCE=1", "NO_COLOR=please"}, want: colorprofile.ASCII},
		{name: "auto disables color for TERM=dumb", mode: ColorAuto, terminal: true, env: []string{"TERM=dumb", "CLICOLOR_FORCE=1"}, want: colorprofile.ASCII},
		{name: "auto disables color on a non-terminal", mode: ColorAuto, terminal: false, env: []string{"TERM=xterm-256color", "CLICOLOR_FORCE=1"}, want: colorprofile.ASCII},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := streamColorProfile(tt.mode, &bytes.Buffer{}, tt.terminal, tt.env)
			if got != tt.want {
				t.Errorf("streamColorProfile(%v, terminal=%v, %q) = %v, want %v", tt.mode, tt.terminal, tt.env, got, tt.want)
			}
		})
	}
}

func TestColorModeSet(t *testing.T) {
	for _, value := range []string{"auto", "always", "never"} {
		var mode ColorMode
		if err := mode.Set(value); err != nil || mode.String() != value {
			t.Errorf("ColorMode.Set(%q) = %v with mode %q, want the mode named %q", value, err, mode, value)
		}
	}

	mode := ColorAlways
	if err := mode.Set("sometimes"); err == nil || mode != ColorAlways {
		t.Errorf("ColorMode.Set(\"sometimes\") = %v with mode %q, want a rejection that keeps always", err, mode)
	}
	if got := (ColorMode("")).String(); got != "auto" {
		t.Errorf("zero ColorMode.String() = %q, want auto", got)
	}
}

func TestTerminalActsOnEscapes(t *testing.T) {
	tests := []struct {
		name string
		env  []string
		want bool
	}{
		{name: "unset TERM", env: []string{"NO_COLOR=1"}, want: false},
		{name: "empty TERM", env: []string{"TERM="}, want: false},
		{name: "dumb", env: []string{"TERM=dumb"}, want: false},
		{name: "xterm", env: []string{"TERM=xterm-256color", "NO_COLOR=1"}, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := terminalActsOnEscapes(tt.env); got != tt.want {
				t.Errorf("terminalActsOnEscapes(%q) = %v, want %v", tt.env, got, tt.want)
			}
		})
	}
}

func TestColorDetectionEnvironmentOmitsTmuxAndDroppedKeys(t *testing.T) {
	env := []string{"TMUX=/private/tmp/tmux/default,1,0", "NO_COLOR=1", "TERM=xterm"}

	got := colorDetectionEnvironment(env, "NO_COLOR")

	if want := []string{"TERM=xterm"}; !slices.Equal(got, want) {
		t.Errorf("colorDetectionEnvironment(%q, NO_COLOR) = %q, want %q", env, got, want)
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
