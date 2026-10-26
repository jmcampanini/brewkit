package cli

import "github.com/spf13/cobra"

// helpTopicGroupID is the cobra group the help topics belong to. The root
// help lists them under the group's title, and the help tests find them
// through it.
const helpTopicGroupID = "topics"

// helpTopicContract is the contract every help topic shares; it closes each
// topic's long description.
const helpTopicContract = `This topic is plain text on stdout. It reads no file, runs no program,
never prompts, and needs no terminal.`

// newHelpTopic builds a help topic: a command that does no work and prints
// long whether invoked directly or through 'brewkit help <use>'.
func newHelpTopic(use, short, long string) *cobra.Command {
	return &cobra.Command{
		Use:     use,
		Short:   short,
		Long:    long + "\n\n" + helpTopicContract,
		GroupID: helpTopicGroupID,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
}
