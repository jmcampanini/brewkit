package cli

import "github.com/spf13/cobra"

func newExitCodesTopic() *cobra.Command {
	return newHelpTopic("exit-codes", "Exit statuses and their meaning",
		`brewkit exits 0, 1, or 2; the exit status of brew is never passed
through. Every failure ends with one line on stderr in the form
'brewkit: <message>', with no usage text.

  0  Success. Reported outcomes that are not failures also exit 0: an
     apply run where every entry was already satisfied, every layer
     lacked the file, or no layer applied, a --dry-run preview whose
     checks all passed, 'lint' finding no violations, and --help,
     --version, a bare 'brewkit', a help topic, or 'brewkit help
     <command>' printing help. 'brewkit help' with an unknown topic
     prints 'Unknown help topic' and the usage on stderr and still
     exits 0, and 'brewkit completion' with an unknown shell prints its
     help; both commands belong to the command library.
  1  Any failure other than a usage error. A --config path that is
     empty or does not exist, an unknown key or an empty dir in
     brewkit.toml, a blank or whitespace-padded env_layers entry, a
     layer name that fails the grammar or names base or local, an
     operand that matches no entry, a layer file that cannot be read,
     an invalid line in a layer file, and 'lint' with at least one
     violation end with the 'brewkit: <message>' line. A brew install,
     upgrade, tap, trust, or uninstall that exits nonzero prints
     '✗ <name>: <reason>' and brew's captured output on stderr, and a
     HEAD check that fails prints '✗ <name>: <reason>' with the error
     text; the run then stops (fail_fast = true) or continues and lists
     every failure at the end (fail_fast = false). A failing state
     query, including brew missing from PATH, ends with a 'brewkit:'
     line naming the query ('brew state: <reason>' or 'query tap state
     for "<name>": <reason>'). When brew ran, the final message carries
     its own exit status ('exit status N'); brewkit itself exits 1.
  2  A usage error: an unknown command or flag, a flag without its
     value or with a value its type rejects (--color with an unknown
     mode, an empty --layer, a non-boolean --dry-run), an operand on
     'lint', 'config', or a help topic, a second operand on an apply
     command, or --quiet together with --verbose. brewkit fails before
     any file is read or any program runs.

--quiet and --verbose change only the output, never the exit status.`)
}
