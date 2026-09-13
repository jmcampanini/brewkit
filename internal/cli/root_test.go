package cli

import (
	"bytes"
	"strings"
	"testing"

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
// validator there would disable suggestions and the built-in help topic. Every
// other command declares one, and every command with subcommands has a RunE,
// because Cobra prints help for a non-runnable command before validating
// operands.
func TestEveryApplicationCommandDeclaresGrammar(t *testing.T) {
	resetFlags()
	defer resetFlags()
	root := newRootCmd()

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
	root := newRootCmd()
	root.SetArgs(args)
	root.SetOut(&out)
	root.SetErr(&errOut)
	err = root.Execute()
	return out.String(), errOut.String(), err
}
