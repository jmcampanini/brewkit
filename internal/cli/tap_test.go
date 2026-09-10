package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/jmcampanini/brewkit/internal/brew"
	"github.com/jmcampanini/brewkit/internal/profile"
)

func TestRunApply_TapTrust(t *testing.T) {
	for _, tt := range []struct {
		name          string
		installed     bool
		trusted       bool
		dryRun        bool
		hideUnchanged bool
		quiet         bool
		url           string
		wantLine      string
		wantSummary   string
		wantOps       []brew.FakeOp
	}{
		{name: "new custom remote", url: "https://example.com/tools.git", wantLine: "+ user/tools registered and trusted", wantSummary: "1 added", wantOps: []brew.FakeOp{brew.OpTrustTap, brew.OpTap}},
		{name: "new default remote", wantLine: "+ user/tools registered and trusted", wantSummary: "1 added", wantOps: []brew.FakeOp{brew.OpTrustTap, brew.OpTap}},
		{name: "installed untrusted keeps remote", installed: true, url: "https://example.com/ignored.git", wantLine: "+ user/tools trusted", wantSummary: "1 trusted", wantOps: []brew.FakeOp{brew.OpTrustTap}},
		{name: "installed trusted", installed: true, trusted: true, wantLine: "✓ user/tools", wantSummary: "1 up-to-date"},
		{name: "preview new", dryRun: true, wantLine: "+ user/tools registered and trusted (dry-run)", wantSummary: "1 added"},
		{name: "preview trust", installed: true, dryRun: true, wantLine: "+ user/tools trusted (dry-run)", wantSummary: "1 trusted"},
		{name: "preview satisfied", installed: true, trusted: true, dryRun: true, wantLine: "✓ user/tools", wantSummary: "1 up-to-date"},
		{name: "hide satisfied", installed: true, trusted: true, hideUnchanged: true, wantSummary: "1 up-to-date"},
		{name: "hide keeps trust change", installed: true, hideUnchanged: true, wantLine: "+ user/tools trusted", wantSummary: "1 trusted", wantOps: []brew.FakeOp{brew.OpTrustTap}},
		{name: "quiet trust change", installed: true, quiet: true, wantOps: []brew.FakeOp{brew.OpTrustTap}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			resetFlags()
			defer resetFlags()
			dir := fixtureRepo(t, map[string]string{"Tapfile.common": "user/tools " + tt.url + "\n"})
			flags.configPath = filepath.Join(dir, "brewkit.toml")
			flags.dryRun, flags.hideUnchanged, flags.quiet = tt.dryRun, tt.hideUnchanged, tt.quiet
			fake := brew.NewFake()
			fake.TapsSet["user/tools"] = tt.installed
			if tt.installed {
				fake.TapRemotes["user/tools"] = "https://example.com/existing.git"
				fake.TrustedTaps["https://example.com/existing.git"] = tt.trusted
			}
			probe := &tapProbe{Fake: fake}
			useBrewer(t, probe)

			out, errOut := captureOutput(t, func() {
				if err := runApply(context.Background(), profile.KindTap, nil); err != nil {
					t.Errorf("runApply: %v", err)
				}
			})

			want := ""
			if tt.wantLine != "" {
				want += tt.wantLine + "\n"
			}
			if tt.wantSummary != "" {
				want += "Summary: " + tt.wantSummary + "\n"
			}
			if out != want || errOut != "" {
				t.Errorf("output = (%q, %q), want (%q, empty)", out, errOut, want)
			}
			var ops []brew.FakeOp
			for _, call := range fake.Calls {
				ops = append(ops, call.Op)
				if call.Op == brew.OpTap && call.Arg != tt.url {
					t.Errorf("tap remote = %q", call.Arg)
				}
				wantTarget := "user/tools"
				if !tt.installed && tt.url != "" {
					wantTarget = tt.url
				}
				if call.Op == brew.OpTrustTap && call.Name != wantTarget {
					t.Errorf("trust target = %q, want %q", call.Name, wantTarget)
				}
			}
			if !reflect.DeepEqual(ops, tt.wantOps) {
				t.Errorf("operations = %v, want %v", ops, tt.wantOps)
			}
			if probe.packageQueries != 0 || probe.tapQueries != 1 {
				t.Errorf("queries = (%d package, %d tap), want (0, 1)", probe.packageQueries, probe.tapQueries)
			}
			state, err := fake.TapState(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if tt.dryRun {
				if fake.TapsSet["user/tools"] != tt.installed || state["user/tools"] != tt.trusted {
					t.Error("dry-run changed registration or trust")
				}
			} else if !fake.TapsSet["user/tools"] || !state["user/tools"] {
				t.Error("successful tap entry is not registered and trusted")
			}
		})
	}
}

func TestRunApply_TapTrustFilteringAndDuplicates(t *testing.T) {
	for _, dryRun := range []bool{false, true} {
		for _, filter := range []string{"tools", "user/tools"} {
			t.Run(fmt.Sprintf("%s/dryRun=%v", filter, dryRun), func(t *testing.T) {
				resetFlags()
				defer resetFlags()
				dir := fixtureRepo(t, map[string]string{
					"Tapfile.common":   "user/tools\nother/ignored\nuser/tools\n",
					"Tapfile.local":    "user/tools\n",
					"Tapfile.inactive": "inactive/tools\n",
				})
				flags.configPath = filepath.Join(dir, "brewkit.toml")
				flags.dryRun = dryRun
				probe := &tapProbe{Fake: brew.NewFake()}
				probe.TapsSet["unlisted/keep"] = true
				probe.TrustedTaps["unlisted/keep"] = true
				useBrewer(t, probe)

				out := captureStdout(t, func() {
					if err := runApply(context.Background(), profile.KindTap, []string{filter}); err != nil {
						t.Errorf("runApply: %v", err)
					}
				})

				if !strings.Contains(out, "Summary: 1 added, 2 up-to-date") || strings.Contains(out, "ignored") || strings.Contains(out, "inactive") {
					t.Errorf("filtered output = %q", out)
				}
				wantCalls := 2
				if dryRun {
					wantCalls = 0
				}
				if len(probe.Calls) != wantCalls || probe.tapQueries != 1 || probe.packageQueries != 0 {
					t.Errorf("calls = %v, tap queries = %d, package queries = %d", probe.Calls, probe.tapQueries, probe.packageQueries)
				}
				if !probe.TapsSet["unlisted/keep"] || !probe.TrustedTaps["unlisted/keep"] {
					t.Error("unlisted tap changed")
				}
			})
		}
	}
}

func TestRunApply_TapTrustFailures(t *testing.T) {
	for _, failFast := range []bool{false, true} {
		for _, op := range []brew.FakeOp{brew.OpTap, brew.OpTrustTap} {
			t.Run(fmt.Sprintf("%s/failFast=%v", op, failFast), func(t *testing.T) {
				resetFlags()
				defer resetFlags()
				dir := fixtureRepo(t, map[string]string{"Tapfile.common": "broken/tools\nbroken/tools\nother/tools\n"})
				flags.configPath = filepath.Join(dir, "brewkit.toml")
				flags.quiet = true
				configText := fmt.Sprintf("dir = %q\nprofiles = [\"common\"]\nenv_profiles = \"\"\nfail_fast = %v\n", dir, failFast)
				if err := os.WriteFile(flags.configPath, []byte(configText), 0o644); err != nil {
					t.Fatal(err)
				}
				probe := &tapProbe{Fake: brew.NewFake()}
				probe.FailOps[op] = map[string]bool{"broken/tools": true}
				useBrewer(t, probe)
				var runErr error

				out, errOut := captureOutput(t, func() {
					runErr = runApply(context.Background(), profile.KindTap, nil)
				})

				if runErr == nil || out != "" || !strings.Contains(errOut, "broken/tools") {
					t.Errorf("runApply = %v, output = (%q, %q)", runErr, out, errOut)
				}
				if !failFast && (runErr == nil || !strings.Contains(runErr.Error(), "2 tap operation(s) failed")) {
					t.Errorf("runApply = %v, want both duplicate failures retained", runErr)
				}
				if probe.TapsSet["other/tools"] == failFast || probe.TrustedTaps["other/tools"] == failFast {
					t.Errorf("later tap state does not match fail_fast=%v", failFast)
				}
				if op == brew.OpTrustTap {
					if probe.TapsSet["broken/tools"] || probe.TrustedTaps["broken/tools"] {
						t.Error("trust failure changed registration or trust")
					}
					for _, call := range probe.Calls {
						if call.Op == brew.OpTap && call.Name == "broken/tools" {
							t.Error("registered tap after trust failed")
						}
					}
				} else if probe.TapsSet["broken/tools"] || !probe.TrustedTaps["broken/tools"] || !strings.Contains(errOut, "trust retained") {
					t.Errorf("registration failure lost trust or marked registration: %q", errOut)
				}
			})
		}
	}
}

func TestRunApply_TapQueryFailure(t *testing.T) {
	resetFlags()
	defer resetFlags()
	dir := fixtureRepo(t, map[string]string{"Tapfile.common": "user/tools\n"})
	flags.configPath = filepath.Join(dir, "brewkit.toml")
	probe := &tapProbe{Fake: brew.NewFake(), queryErr: errors.New("trust data unavailable")}
	useBrewer(t, probe)
	var runErr error

	out, errOut := captureOutput(t, func() {
		runErr = runApply(context.Background(), profile.KindTap, nil)
	})

	if !errors.Is(runErr, probe.queryErr) || !strings.Contains(out, "1 failed") || !strings.Contains(errOut, "trust data unavailable") {
		t.Errorf("runApply = %v, output = (%q, %q)", runErr, out, errOut)
	}
	if len(probe.Calls) != 0 {
		t.Errorf("query failure caused mutations: %v", probe.Calls)
	}
}

func TestRunApply_TapWithoutSelectedEntriesDoesNotQuery(t *testing.T) {
	for _, tt := range []struct {
		name      string
		files     map[string]string
		args      []string
		wantError bool
	}{
		{name: "no files"},
		{name: "empty file", files: map[string]string{"Tapfile.common": "# empty\n"}},
		{name: "no match", files: map[string]string{"Tapfile.common": "user/tools\n"}, args: []string{"missing"}, wantError: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			resetFlags()
			defer resetFlags()
			dir := fixtureRepo(t, tt.files)
			flags.configPath = filepath.Join(dir, "brewkit.toml")
			probe := &tapProbe{Fake: brew.NewFake()}
			useBrewer(t, probe)

			captureStdout(t, func() {
				err := runApply(context.Background(), profile.KindTap, tt.args)
				if (err != nil) != tt.wantError {
					t.Errorf("runApply = %v, want error %v", err, tt.wantError)
				}
			})

			if probe.tapQueries != 0 || probe.packageQueries != 0 || len(probe.Calls) != 0 {
				t.Errorf("no selected entries caused Homebrew operations: %+v", probe)
			}
		})
	}
}

func TestRunApply_TapSubprocessOutput(t *testing.T) {
	for _, failure := range []string{"none", "trust", "tap"} {
		t.Run(failure, func(t *testing.T) {
			resetFlags()
			defer resetFlags()
			dir := fixtureRepo(t, map[string]string{"Tapfile.common": "user/tools https://example.com/tools.git\n"})
			flags.configPath = filepath.Join(dir, "brewkit.toml")
			flags.verbose = failure == "none"
			flags.quiet = failure != "none"
			t.Setenv("BREWKIT_TEST_FAILURE", failure)
			trustMarker := filepath.Join(dir, "trusted")
			t.Setenv("BREWKIT_TEST_TRUST_MARKER", trustMarker)
			bin := filepath.Join(dir, "brew")
			script := `#!/bin/sh
set -eu
case "$*" in
  'tap-info --installed --json=v1')
    echo '[]'
    ;;
  'tap -- user/tools https://example.com/tools.git')
    test -f "$BREWKIT_TEST_TRUST_MARKER"
    echo 'registration stdout'
    echo 'registration stderr' >&2
    if [ "$BREWKIT_TEST_FAILURE" = tap ]; then exit 7; fi
    ;;
  'trust --tap -- https://example.com/tools.git')
    echo 'trust stdout'
    echo 'trust stderr' >&2
    if [ "$BREWKIT_TEST_FAILURE" = trust ]; then exit 6; fi
    touch "$BREWKIT_TEST_TRUST_MARKER"
    ;;
  *) echo "unexpected brew command: $*" >&2; exit 9 ;;
esac
`
			if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
				t.Fatal(err)
			}
			useBrewer(t, &brew.Exec{Bin: bin})
			var runErr error

			out, errOut := captureOutput(t, func() {
				runErr = runApply(context.Background(), profile.KindTap, nil)
			})

			if (runErr != nil) != (failure != "none") {
				t.Errorf("runApply = %v, want failure %q", runErr, failure)
			}
			details := out
			if failure != "none" {
				details = errOut
				if out != "" || !strings.Contains(errOut, failure+" failed") {
					t.Errorf("quiet failure output = (%q, %q)", out, errOut)
				}
			} else if errOut != "" || !strings.Contains(out, "Summary: 1 added") {
				t.Errorf("verbose success output = (%q, %q)", out, errOut)
			}
			lines := []string{"trust stdout", "trust stderr"}
			if failure != "trust" {
				lines = append(lines, "registration stdout", "registration stderr")
			}
			for _, line := range lines {
				if !strings.Contains(details, line) {
					t.Errorf("subprocess output missing %q: %q", line, details)
				}
			}
			_, statErr := os.Stat(trustMarker)
			if failure == "trust" {
				if !errors.Is(statErr, os.ErrNotExist) || strings.Contains(details, "registration stdout") {
					t.Error("registration ran after failed trust")
				}
			} else if statErr != nil {
				t.Errorf("trust was not retained: %v", statErr)
			}
		})
	}
}

type tapProbe struct {
	*brew.Fake
	packageQueries int
	tapQueries     int
	queryErr       error
}

func (p *tapProbe) State(context.Context) (*brew.State, error) {
	p.packageQueries++
	return nil, errors.New("package inventory must not be queried for taps")
}

func (p *tapProbe) TapState(ctx context.Context) (map[string]bool, error) {
	p.tapQueries++
	if p.queryErr != nil {
		return nil, p.queryErr
	}
	return p.Fake.TapState(ctx)
}
