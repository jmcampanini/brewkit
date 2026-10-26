package cli

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/jmcampanini/brewkit/internal/docs"
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
	root := newRootCmd(testApp)

	walkApplicationCommands(root, func(command *cobra.Command) {
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
	})
}

// TestEveryApplicationCommandDeclaresGrammar holds the CLI command contract's
// presence invariant. The root has subcommands, so Cobra owns its grammar; a
// validator there would disable the built-in help topic. Every other command
// declares one, and every command with subcommands has a RunE, because Cobra
// prints help for a non-runnable command before validating operands.
func TestEveryApplicationCommandDeclaresGrammar(t *testing.T) {
	resetFlags()
	defer resetFlags()
	root := newRootCmd(testApp)

	walkApplicationCommands(root, func(command *cobra.Command) {
		if command != root && command.Args == nil {
			t.Errorf("%q has no Args validator", command.CommandPath())
		}
		if command.HasSubCommands() && command.RunE == nil {
			t.Errorf("%q has subcommands but no RunE", command.CommandPath())
		}
	})
}

// walkApplicationCommands calls visit on command and every descendant,
// skipping Cobra's own help and completion commands.
func walkApplicationCommands(command *cobra.Command, visit func(*cobra.Command)) {
	if command.Name() == "help" || command.Name() == "completion" {
		return
	}
	visit(command)
	for _, child := range command.Commands() {
		walkApplicationCommands(child, visit)
	}
}

func executeForTest(t *testing.T, args ...string) (stdout string, stderr string, err error) {
	t.Helper()
	resetFlags()
	t.Cleanup(resetFlags)

	var out bytes.Buffer
	var errOut bytes.Buffer
	root := newRootCmd(testApp)
	root.SetArgs(args)
	root.SetOut(&out)
	root.SetErr(&errOut)
	err = root.Execute()
	return out.String(), errOut.String(), err
}

func TestExecuteReturnsStatusAndPrintsErrors(t *testing.T) {
	originalArgs := os.Args
	t.Cleanup(func() { os.Args = originalArgs })
	t.Chdir(t.TempDir())
	for _, tt := range []struct {
		name        string
		args        []string
		status      int
		output      string
		errorOutput string
	}{
		{name: "success", args: []string{"docs"}, output: docs.Manual()},
		{name: "failure", args: []string{"--output-prefix", "prefix: ", "--layer", "local", "brew"}, status: 1,
			errorOutput: "prefix: brewkit: layer \"local\" is automatic and applies whenever its files exist; remove it from the layers\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("BREWKIT_LAYERS", "")
			os.Args = append([]string{"brewkit"}, tt.args...)
			var status int
			out, errOut := captureOutput(t, func() { status = Execute() })
			if status != tt.status || out != tt.output || errOut != tt.errorOutput {
				t.Fatalf("Execute() = (%d, %q, %q), want (%d, %q, %q)", status, out, errOut, tt.status, tt.output, tt.errorOutput)
			}
		})
	}
}

func TestRootPersistentFlagsRegistered(t *testing.T) {
	resetFlags()
	defer resetFlags()

	root := newRootCmd(testApp)
	for _, name := range []string{"config", "layers", "layer", "output-prefix"} {
		if root.PersistentFlags().Lookup(name) == nil {
			t.Errorf("root should register --%s", name)
		}
	}
}

func TestOutputPrefixDoesNotPrefixHelpOrVersion(t *testing.T) {
	const outputPrefix = "prefix: "

	for _, tt := range []struct {
		name string
		args []string
	}{
		{name: "help", args: []string{"--output-prefix", outputPrefix, "--help"}},
		{name: "version", args: []string{"--output-prefix", outputPrefix, "--version"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			resetFlags()
			defer resetFlags()

			root := newRootCmd(testApp)
			var out, errOut bytes.Buffer
			root.SetArgs(tt.args)
			root.SetOut(&out)
			root.SetErr(&errOut)

			if err := root.Execute(); err != nil {
				t.Fatal(err)
			}
			gotOut := out.String()
			if gotOut == "" {
				t.Fatal("expected command output")
			}
			if strings.Contains(gotOut, outputPrefix) {
				t.Fatalf("help/version output should not be prefixed; got %q", gotOut)
			}
			if errOut.Len() != 0 {
				t.Fatalf("expected no stderr output, got %q", errOut.String())
			}
		})
	}
}

func TestPrintError_OutputPrefix(t *testing.T) {
	resetFlags()
	defer resetFlags()

	testApp.flags.outputPrefix = "  "
	_, errOut := captureOutput(t, func() {
		printError(errors.New("first\nsecond"), testApp.flags.outputPrefix)
	})

	want := "  brewkit: first\n  second\n"
	if errOut != want {
		t.Errorf("unexpected prefixed top-level error:\nwant: %q\ngot:  %q", want, errOut)
	}
}

func TestRootRejectsQuietAndVerboseTogether(t *testing.T) {
	resetFlags()
	defer resetFlags()

	root := newRootCmd(testApp)
	root.SetArgs([]string{"--quiet", "--verbose"})
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected quiet/verbose mutual exclusion error")
	}
	for _, want := range []string{"quiet", "verbose", "none of the others can be"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("expected native mutual exclusion error to contain %q, got: %v", want, err)
		}
	}
}
