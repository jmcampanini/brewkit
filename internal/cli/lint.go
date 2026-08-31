package cli

import "github.com/spf13/cobra"

func newLintCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "lint",
		Short: "Validate profile files (sort order + comment style)",
		Long: `Check the profile files for style violations and print each one on
stdout as '<path>:<line>: [<rule>] <message>'. With neither --profiles
nor --profile, every Tapfile, Brewfile, Headfile, and Caskfile in dir
whose suffix is a profile name (letters, digits, '_', and '-'), including
local, is checked. With --profiles or --profile only the named profiles'
files are checked; profiles in brewkit.toml, BREWKIT_PROFILES,
env_profiles, and the automatic local profile never narrow the scan.
Missing files are skipped and no brew command runs.

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
--dry-run has no effect.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runLint(cmd.Context())
		},
	}
}
