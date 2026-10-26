package cli

import "github.com/spf13/cobra"

func newLintCmd(app *commandContext) *cobra.Command {
	return &cobra.Command{
		GroupID: commandGroupID,
		Use:     "lint",
		Short:   "Validate layer files (sort order + comment style)",
		Long: `Check the layer files for style violations and print each one on
stdout as '<path>:<line>: [<rule>] <message>'. With neither --layers nor
--layer, every Tapfile, Brewfile, Headfile, and Caskfile in dir whose
suffix is a valid layer name (lowercase letters, digits, and single
hyphens between groups), including the local files, is checked. With
--layers or --layer only the effective layers' files are checked: the
flag values plus the env_layers additions, after deduplication, and
never the local files. Missing files are skipped, nothing is written,
and no brew command runs. 'brewkit help layer-files' describes the
file format and the editing conventions these rules enforce.

` + configDiscoveryHelp + `

Rules: sort-order (entries within a section, delimited by blank lines,
are in byte order by name), trailing-description (Brewfile, Caskfile, and
Headfile entries carry a '# description'; Tapfile entries are exempt),
comment-space (every '#' is followed by exactly one space),
inline-comment-gap (a trailing comment is preceded by at least two
spaces), no-consecutive-blanks, no-trailing-whitespace, and
known-entry-shape (every non-blank, non-comment line is a valid entry:
brew "name" in a Brewfile, one name in a Caskfile or Headfile, a tap and
optional URL in a Tapfile).

With no violations 'no violations' is printed and the exit status is 0.
Otherwise the violations are followed by 'Summary: N violation(s)', the
exit status is 1, and 'brewkit: lint failed: N violation(s)' goes to
stderr. --verbose prints the offending line under each violation.
--quiet prints only the violation lines and nothing when clean.
--dry-run has no effect.

` + noTerminalHelp,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return app.runLint(cmd.Context())
		},
	}
}
