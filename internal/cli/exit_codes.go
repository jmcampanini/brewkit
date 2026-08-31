package cli

import "github.com/spf13/cobra"

func newExitCodesTopic() *cobra.Command {
	return &cobra.Command{
		Use:   "exit-codes",
		Short: "Exit codes and error categories",
		Long: `brewkit exits 0 or 1; the exit status of brew is never passed through:

  0  Success. Reported outcomes that are not failures also exit 0: an
     apply run where every entry was already satisfied or every profile
     lacked the file, a --dry-run preview whose checks all passed, 'lint'
     finding no violations, and --help, --version, a bare 'brewkit', or
     'brewkit help <command>' printing help. 'brewkit help' with an
     unknown topic prints 'Unknown help topic' and the usage on stderr
     and still exits 0, and 'lint', 'config', and 'docs' ignore extra
     operands.
  1  Any failure. Usage errors (an unknown command or flag, a second
     operand on an apply command, an operand on 'exit-codes', or --quiet
     together with --verbose), a --config file that does not exist, an
     unknown key or an empty dir in brewkit.toml, an invalid profile
     list (local listed explicitly, an empty name, or no active profile
     for an apply command), an operand that matches no entry, a profile
     file that cannot be read, an invalid line in a profile file, and
     'lint' with at least one violation end with 'brewkit: <message>' on
     stderr. A brew install, upgrade, tap, uninstall, or fetch that
     exits nonzero, and a HEAD check that fails, print '✗ <name>:
     <reason>' and brew's captured output on stderr; the run then stops
     (fail_fast = true) or continues and lists every failure at the end
     (fail_fast = false). A failing state query, including brew missing
     from PATH, ends with 'brewkit: brew state: <reason>'. When brew
     ran, the final message carries its own exit status ('exit status
     N'); brewkit itself exits 1.

--quiet and --verbose change only the output, never the exit status.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
}
