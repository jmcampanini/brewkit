package cli

import "github.com/spf13/cobra"

func newDocsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "docs",
		Short: "Print the embedded brewkit manual as plain text",
		Long: `Print the embedded manual on stdout as plain text. It covers the file
formats, the lint rules, the HEAD and cask strategies, and the
configuration keys at more length than command help does; command help
(--help) is the canonical contract and the manual supplements it. The
text has no terminal escapes, no configuration is read, and no brew
command runs. --quiet does not suppress the manual, and --output-prefix
prefixes each of its lines.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runDocs(cmd.Context())
		},
	}
}
