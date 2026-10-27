package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"
	"github.com/jmcampanini/brewkit/internal/brew"
	"github.com/spf13/cobra"
)

func TestExitCodesTopicListsEveryStatus(t *testing.T) {
	out, errOut, err := executeForTest(t, "exit-codes")
	if err != nil {
		t.Fatalf("exit-codes error = %v, stderr = %s", err, errOut)
	}

	for _, want := range []string{"\n  0  ", "\n  1  ", "\n  2  ", "'brewkit: <message>'", "brew"} {
		if !strings.Contains(out, want) {
			t.Errorf("exit-codes help missing %q:\n%s", want, out)
		}
	}
}

// TestEveryApplicationCommandHasHelp holds the help baseline: short and
// long help on every command, every rendered help line (flag usage
// included) within 80 columns, and a help topic that prints its help when
// invoked directly.
func TestEveryApplicationCommandHasHelp(t *testing.T) {
	resetFlags()
	defer resetFlags()
	root := newRootCmd(testApp)

	walkApplicationCommands(root, func(command *cobra.Command) {
		path := command.CommandPath()
		if strings.TrimSpace(command.Short) == "" {
			t.Errorf("%q has no short help", path)
		}
		if strings.TrimSpace(command.Long) == "" {
			t.Errorf("%q has no long help", path)
		}

		args := strings.Fields(path)[1:]
		rendered, errOut, err := executeForTest(t, append(args, "--help")...)
		if err != nil || errOut != "" {
			t.Fatalf("%q --help error = %v, stderr = %q", path, err, errOut)
		}
		for i, line := range strings.Split(rendered, "\n") {
			// pflagloader appends this fixed suffix to the singular flag's
			// usage; the repository cannot shorten it, so the width rule
			// exempts it.
			if strings.HasPrefix(strings.TrimSpace(line), "--layer ") {
				line = strings.TrimSuffix(line, " "+singularFlagHelpSuffix)
			}
			if width := utf8.RuneCountInString(line); width > 80 {
				t.Errorf("%q help line %d is %d columns, want at most 80: %q", path, i+1, width, line)
			}
		}

		if command.GroupID != helpTopicGroupID {
			return
		}
		direct, errOut, err := executeForTest(t, command.Name())
		if err != nil || errOut != "" {
			t.Fatalf("%q error = %v, stderr = %q", path, err, errOut)
		}
		if direct != rendered {
			t.Errorf("%q output differs from its --help:\n%s\n---\n%s", path, direct, rendered)
		}
		viaHelp, errOut, err := executeForTest(t, "help", command.Name())
		if err != nil || errOut != "" {
			t.Fatalf("help %q error = %v, stderr = %q", command.Name(), err, errOut)
		}
		if viaHelp != rendered {
			t.Errorf("help %q output differs from %q --help:\n%s\n---\n%s", command.Name(), path, viaHelp, rendered)
		}
	})
}

// singularFlagHelpSuffix is the text go-config-loader's pflagloader appends
// to the usage of a singular list flag such as --layer.
const singularFlagHelpSuffix = "Adds a single value to the array; empty values are not allowed."

// TestEveryApplicationCommandDeclaresGrammar holds the CLI command contract's
// presence invariant. The root has subcommands, so Cobra owns its grammar; a
// validator there would disable the built-in help topic. Every other command
// declares one, every command with subcommands has a RunE, because Cobra
// prints help for a non-runnable command before validating operands, and
// every help topic is runnable and rejects an operand.
func TestEveryApplicationCommandDeclaresGrammar(t *testing.T) {
	resetFlags()
	defer resetFlags()
	root := newRootCmd(testApp)

	walkApplicationCommands(root, func(command *cobra.Command) {
		if command != root && command.Args == nil {
			t.Errorf("%q has no Args validator", command.CommandPath())
		}
		if command.HasSubCommands() || command.GroupID == helpTopicGroupID {
			if !command.Runnable() {
				t.Errorf("%q is not runnable", command.CommandPath())
			}
		}
		if command.GroupID == helpTopicGroupID {
			if err := command.Args(command, nil); err != nil {
				t.Errorf("help topic %q rejects a bare invocation: %v", command.CommandPath(), err)
			}
			if command.Args(command, []string{"operand"}) == nil {
				t.Errorf("help topic %q accepts an operand", command.CommandPath())
			}
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

// TestExecuteReturnsStatusAndPrintsErrors covers the exit contract: 2 when
// the invocation is rejected before any command runs, 1 for every other
// failure, and one 'brewkit: <message>' line on stderr with no usage text.
func TestExecuteReturnsStatusAndPrintsErrors(t *testing.T) {
	originalArgs := os.Args
	t.Cleanup(func() { os.Args = originalArgs })
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.WriteFile(filepath.Join(dir, "Caskfile.local"), []byte("ghostty  # terminal\nalacritty  # terminal\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name        string
		args        []string
		status      int
		output      string
		errorOutput string
	}{
		{name: "success", args: []string{"--version"}, output: "brewkit version n/a\n"},
		{name: "unknown command", args: []string{"bogus"}, status: 2,
			errorOutput: "brewkit: unknown command \"bogus\" for \"brewkit\"\n"},
		{name: "unknown flag", args: []string{"lint", "--bogus"}, status: 2,
			errorOutput: "brewkit: unknown flag: --bogus\n"},
		{name: "flag value outside its set", args: []string{"--color", "sometimes", "lint"}, status: 2,
			errorOutput: "brewkit: invalid argument \"sometimes\" for \"--color\" flag: must be auto, always, or never\n"},
		{name: "extra operand", args: []string{"lint", "extra"}, status: 2,
			errorOutput: "brewkit: unknown command \"extra\" for \"brewkit lint\"\n"},
		{name: "operand past the grammar", args: []string{"brew", "one", "two"}, status: 2,
			errorOutput: "brewkit: accepts at most 1 arg(s), received 2\n"},
		{name: "operand on a help topic", args: []string{"exit-codes", "extra"}, status: 2,
			errorOutput: "brewkit: unknown command \"extra\" for \"brewkit exit-codes\"\n"},
		{name: "unknown help topic", args: []string{"help", "bogus"}, status: 2,
			errorOutput: "brewkit: unknown command \"bogus\" for \"brewkit\"\n"},
		{name: "extra operand after a help command", args: []string{"help", "lint", "extra"}, status: 2,
			errorOutput: "brewkit: unknown command \"extra\" for \"brewkit lint\"\n"},
		{name: "unknown completion shell", args: []string{"completion", "bogus"}, status: 2,
			errorOutput: "brewkit: unknown command \"bogus\" for \"brewkit completion\"\n"},
		{name: "quiet with verbose", args: []string{"--quiet", "--verbose", "lint"}, status: 2,
			errorOutput: "brewkit: if any flags in the group [quiet verbose] are set none of the others can be; [quiet verbose] were all set\n"},
		{name: "configuration failure", args: []string{"--output-prefix", "prefix: ", "--layer", "local", "brew"}, status: 1,
			errorOutput: "prefix: brewkit: layer \"local\" is reserved and cannot be selected; remove it from the layers\n"},
		{name: "command failure", args: []string{"lint"}, status: 1,
			output:      "Caskfile.local:2: [sort-order] \"alacritty\" should sort before \"ghostty\"\nSummary: 1 violation(s)\n",
			errorOutput: "brewkit: lint failed: 1 violation(s)\n"},
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
	for _, name := range []string{"config", "layers", "layer", "color", "output-prefix"} {
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

	want := "  brewkit: first; second\n"
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

func TestForcedColorKeepsMachineReadableOutputPlain(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("BREWKIT_LAYERS", "")
	originalArgs := os.Args
	t.Cleanup(func() { os.Args = originalArgs })
	for _, command := range []string{"config", "lint"} {
		t.Run(command, func(t *testing.T) {
			os.Args = []string{"brewkit", "--color=always", command}
			var status int
			out, errOut := captureOutput(t, func() { status = Execute() })
			if status != 0 || out == "" || errOut != "" {
				t.Fatalf("Execute() = (%d, %q, %q), want success with stdout", status, out, errOut)
			}
			if strings.Contains(out, "\x1b") || strings.Contains(out, "\r") {
				t.Fatalf("%s output carries terminal escapes under --color=always: %q", command, out)
			}
		})
	}
}

func TestForcedColorStylesApplyOutput(t *testing.T) {
	dir := fixtureRepo(t, map[string]string{"Brewfile.common": "brew \"ripgrep\"  # search\n"})
	t.Chdir(dir)
	t.Setenv("BREWKIT_LAYERS", "")
	t.Setenv("NO_COLOR", "1")
	t.Setenv("COLORFGBG", "")
	useBrewer(t, brew.NewFake())
	originalArgs := os.Args
	t.Cleanup(func() { os.Args = originalArgs })
	os.Args = []string{"brewkit", "--color=always", "--layer=common", "--dry-run", "brew"}

	var status int
	out, errOut := captureOutput(t, func() { status = Execute() })

	if status != 0 || errOut != "" {
		t.Fatalf("Execute() = %d, stderr = %q, want success without progress", status, errOut)
	}
	want := "+ ripgrep (dry-run)\nSummary: 1 added\n"
	if ansi.Strip(out) != want || !strings.Contains(out, "\x1b[") {
		t.Fatalf("forced-color output = %q, want styled %q", out, want)
	}
}
