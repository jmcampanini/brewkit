package cli

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jmcampanini/brewkit/internal/brew"
	"github.com/spf13/cobra"
)

func TestExitCodesTopicPrintsSameHelpFromBothEntryPoints(t *testing.T) {
	direct, errOut, err := executeForTest(t, "exit-codes")
	if err != nil {
		t.Fatalf("exit-codes error = %v, stderr = %s", err, errOut)
	}
	viaHelp, errOut, err := executeForTest(t, "help", "exit-codes")
	if err != nil {
		t.Fatalf("help exit-codes error = %v, stderr = %s", err, errOut)
	}

	if direct != viaHelp {
		t.Fatalf("exit-codes output differs between entry points:\n%s\n---\n%s", direct, viaHelp)
	}
	for _, want := range []string{"\n  0  ", "\n  1  ", "brew"} {
		if !strings.Contains(direct, want) {
			t.Fatalf("exit-codes help missing %q:\n%s", want, direct)
		}
	}
}

func TestEveryApplicationCommandHasWrappedLongHelp(t *testing.T) {
	resetFlags()
	defer resetFlags()
	root := newRootCmd()

	var visit func(*cobra.Command)
	visit = func(command *cobra.Command) {
		if command.Name() == "help" || command.Name() == "completion" {
			return
		}
		if strings.TrimSpace(command.Long) == "" {
			t.Errorf("%q has no long help", command.CommandPath())
		}
		for field, text := range map[string]string{"Long": command.Long, "Example": command.Example} {
			for i, line := range strings.Split(text, "\n") {
				if len(line) > 80 {
					t.Errorf("%q %s line %d is %d columns, want at most 80: %q", command.CommandPath(), field, i+1, len(line), line)
				}
			}
		}
		for _, child := range command.Commands() {
			visit(child)
		}
	}
	visit(root)
}

// TestEveryApplicationCommandGrammar runs each command through a fresh root
// with every legitimate operand count and the first rejected one. --config
// names a file that does not exist and the brewer is a fake, so a rejected
// operand that reached the runner would surface as the missing-config error
// or a recorded brew call instead of the Cobra grammar error.
func TestEveryApplicationCommandGrammar(t *testing.T) {
	missingConfig := filepath.Join(t.TempDir(), "missing.toml")
	grammar := []struct {
		path     string
		accepted [][]string
		rejected []string
		wantErr  string
	}{
		{path: "brewkit", accepted: [][]string{{}}, rejected: []string{"extra"}, wantErr: `unknown command "extra" for "brewkit"`},
		{path: "brewkit tap", accepted: [][]string{{}, {"one"}}, rejected: []string{"one", "two"}, wantErr: "accepts at most 1 arg(s), received 2"},
		{path: "brewkit brew", accepted: [][]string{{}, {"one"}}, rejected: []string{"one", "two"}, wantErr: "accepts at most 1 arg(s), received 2"},
		{path: "brewkit head", accepted: [][]string{{}, {"one"}}, rejected: []string{"one", "two"}, wantErr: "accepts at most 1 arg(s), received 2"},
		{path: "brewkit cask", accepted: [][]string{{}, {"one"}}, rejected: []string{"one", "two"}, wantErr: "accepts at most 1 arg(s), received 2"},
		{path: "brewkit lint", accepted: [][]string{{}}, rejected: []string{"extra"}, wantErr: `unknown command "extra" for "brewkit lint"`},
		{path: "brewkit config", accepted: [][]string{{}}, rejected: []string{"extra"}, wantErr: `unknown command "extra" for "brewkit config"`},
		{path: "brewkit docs", accepted: [][]string{{}}, rejected: []string{"extra"}, wantErr: `unknown command "extra" for "brewkit docs"`},
		{path: "brewkit exit-codes", accepted: [][]string{{}}, rejected: []string{"extra"}, wantErr: `unknown command "extra" for "brewkit exit-codes"`},
	}

	resetFlags()
	defer resetFlags()
	covered := map[string]bool{}
	for _, tt := range grammar {
		covered[tt.path] = true
	}
	var visit func(*cobra.Command)
	visit = func(command *cobra.Command) {
		if command.Name() == "help" || command.Name() == "completion" {
			return
		}
		if !covered[command.CommandPath()] {
			t.Errorf("%q has no grammar case; add it to the table", command.CommandPath())
		}
		for _, child := range command.Commands() {
			visit(child)
		}
	}
	visit(newRootCmd())

	for _, tt := range grammar {
		t.Run(tt.path, func(t *testing.T) {
			words := strings.Fields(tt.path)[1:]
			invoke := func(operands []string) (stdout string, fake *brew.Fake, err error) {
				fake = brew.NewFake()
				useBrewer(t, fake)
				args := append([]string{"--config", missingConfig}, words...)
				args = append(args, operands...)
				var cobraOut string
				runnerOut := captureStdout(t, func() {
					cobraOut, _, err = executeForTest(t, args...)
				})
				return cobraOut + runnerOut, fake, err
			}

			// An accepted arity either succeeds or reaches the runner, which
			// fails on the missing config file; any other error is a grammar
			// rejection of a legitimate arity.
			for _, operands := range tt.accepted {
				_, _, err := invoke(operands)
				if err != nil && !strings.Contains(err.Error(), missingConfig) {
					t.Errorf("%q with operands %q = %v, want nil or missing-config error", tt.path, operands, err)
				}
			}

			stdout, fake, err := invoke(tt.rejected)
			if err == nil || err.Error() != tt.wantErr {
				t.Errorf("%q with operands %q = %v, want %q", tt.path, tt.rejected, err, tt.wantErr)
			}
			if stdout != "" {
				t.Errorf("%q with operands %q wrote stdout %q, want empty", tt.path, tt.rejected, stdout)
			}
			if len(fake.Calls) != 0 {
				t.Errorf("%q with operands %q called brew %d time(s), want 0: %+v", tt.path, tt.rejected, len(fake.Calls), fake.Calls)
			}
		})
	}
}

// TestFrameworkFlowsSucceedBeforeConfigDiscovery proves the help, version,
// and completion short-circuits still win over positional validation and
// never reach configuration discovery.
func TestFrameworkFlowsSucceedBeforeConfigDiscovery(t *testing.T) {
	missingConfig := filepath.Join(t.TempDir(), "missing.toml")
	for _, flow := range [][]string{{"--help"}, {"--version"}, {"help", "tap"}, {"completion", "zsh"}} {
		t.Run(strings.Join(flow, " "), func(t *testing.T) {
			stdout, _, err := executeForTest(t, append([]string{"--config", missingConfig}, flow...)...)
			if err != nil {
				t.Fatalf("brewkit %s = %v, want success", strings.Join(flow, " "), err)
			}
			if stdout == "" {
				t.Errorf("brewkit %s wrote no stdout, want help, version, or completion output", strings.Join(flow, " "))
			}
		})
	}
}

func executeForTest(t *testing.T, args ...string) (stdout string, stderr string, err error) {
	t.Helper()
	resetFlags()
	t.Cleanup(resetFlags)

	var out bytes.Buffer
	var errOut bytes.Buffer
	root := newRootCmd()
	root.SetArgs(args)
	root.SetOut(&out)
	root.SetErr(&errOut)
	err = root.Execute()
	return out.String(), errOut.String(), err
}
