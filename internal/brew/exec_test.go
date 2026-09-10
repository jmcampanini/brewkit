package brew

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestExecTapState(t *testing.T) {
	for _, tt := range []struct {
		name      string
		json      string
		want      map[string]bool
		wantError bool
	}{
		{
			name: "effective trust includes official and custom remote taps",
			json: `[
				{"name":"homebrew/core","installed":true,"trusted":true,"official":true},
				{"name":"user/custom","installed":true,"trusted":true,"remote":"ssh://git@example.com/tools.git","custom_remote":true},
				{"name":"user/partial","installed":true,"trusted":false},
				{"name":"user/missing","installed":false,"trusted":true}
			]`,
			want: map[string]bool{"homebrew/core": true, "user/custom": true, "user/partial": false},
		},
		{name: "no installed taps", json: `[]`, want: map[string]bool{}},
		{name: "missing trust", json: `[{"name":"user/tools","installed":true}]`, wantError: true},
		{name: "null trust", json: `[{"name":"user/tools","installed":true,"trusted":null}]`, wantError: true},
		{name: "invalid trust", json: `[{"name":"user/tools","installed":true,"trusted":"true"}]`, wantError: true},
		{name: "missing installed", json: `[{"name":"user/tools","trusted":true}]`, wantError: true},
		{name: "missing name", json: `[{"installed":true,"trusted":true}]`, wantError: true},
		{name: "null response", json: `null`, wantError: true},
		{name: "wrong shape", json: `{}`, wantError: true},
		{name: "malformed", json: `[`, wantError: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("BREWKIT_TEST_TAP_JSON", tt.json)
			e := &Exec{Bin: brewFixture(t, `
if [ "$#" -ne 3 ] || [ "$1" != tap-info ] || [ "$2" != --installed ] || [ "$3" != --json=v1 ]; then
  echo "unexpected probe: $*" >&2
  exit 9
fi
printf '%s\n' "$BREWKIT_TEST_TAP_JSON"
`)}

			got, err := e.TapState(context.Background())

			if tt.wantError {
				if err == nil || !strings.Contains(err.Error(), "brew update") || got != nil {
					t.Errorf("TapState = (%v, %v), want no state and actionable schema error", got, err)
				}
				return
			}
			if err != nil || !reflect.DeepEqual(got, tt.want) {
				t.Errorf("TapState = (%v, %v), want (%v, nil)", got, err, tt.want)
			}
		})
	}
}

func TestExecTapStateFailure(t *testing.T) {
	e := &Exec{Bin: brewFixture(t, `echo 'cannot read tap metadata' >&2; exit 7`)}

	state, err := e.TapState(context.Background())

	var exitErr *exec.ExitError
	if state != nil || !errors.As(err, &exitErr) || exitErr.ExitCode() != 7 || !strings.Contains(err.Error(), "cannot read tap metadata") {
		t.Errorf("TapState = (%v, %v), want exit 7 with stderr", state, err)
	}
}

func TestExecTapArgumentsAndOutput(t *testing.T) {
	e := &Exec{Bin: brewFixture(t, `
printf '<%s>\n' "$@"
echo 'brew diagnostic' >&2
`)}
	for _, tt := range []struct {
		name string
		url  string
		want string
	}{
		{name: "user/tools", want: "<tap>\n<-->\n<user/tools>\nbrew diagnostic\n"},
		{name: "user/tools", url: "ssh://git@example.com/tools.git", want: "<tap>\n<-->\n<user/tools>\n<ssh://git@example.com/tools.git>\nbrew diagnostic\n"},
		{name: "--repair", url: "--custom-remote", want: "<tap>\n<-->\n<--repair>\n<--custom-remote>\nbrew diagnostic\n"},
	} {
		t.Run(tt.name+tt.url, func(t *testing.T) {
			res, err := e.Tap(context.Background(), tt.name, tt.url)

			if err != nil || res.Output != tt.want {
				t.Errorf("Tap = (%q, %v), want (%q, nil)", res.Output, err, tt.want)
			}
		})
	}

	res, err := e.TrustTap(context.Background(), "--all")

	want := "<trust>\n<--tap>\n<-->\n<--all>\nbrew diagnostic\n"
	if err != nil || res.Output != want {
		t.Errorf("TrustTap = (%q, %v), want (%q, nil)", res.Output, err, want)
	}
}

func TestExecTrustTapFailure(t *testing.T) {
	e := &Exec{Bin: brewFixture(t, `echo 'trust stdout'; echo 'trust stderr' >&2; exit 6`)}

	res, err := e.TrustTap(context.Background(), "user/tools")

	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 6 || res.Output != "trust stdout\ntrust stderr\n" {
		t.Errorf("TrustTap = (%q, %v), want captured stdout/stderr and exit 6", res.Output, err)
	}
}

func brewFixture(t *testing.T, script string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "brew")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nset -eu\n"+script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}
