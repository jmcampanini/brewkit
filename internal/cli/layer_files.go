package cli

import "github.com/spf13/cobra"

func newLayerFilesTopic() *cobra.Command {
	return newHelpTopic("layer-files", "The layer file kinds and their line shapes",
		`Each layer has up to four files in the layer directory, named for the
kind and the layer: Tapfile.<layer>, Brewfile.<layer>,
Headfile.<layer>, and Caskfile.<layer>. The *file.local files belong to
the automatic local layer ('brewkit help layers'). Every file is plain
text with one entry per line, and every kind accepts blank lines,
'# comment' lines, and a trailing '  # description' after an entry.
Entries are read in file order, whatever their sort order.

Brewfile.<layer> is the brew subset of the Homebrew Bundle format:

  brew "name"
  brew "name"            # description
  brew "user/tap/name"   # a fully qualified formula

Only brew "..." lines are entries; the other Bundle directives (tap,
cask, mas, vscode) are not recognized, so list taps in the Tapfile and
casks in the Caskfile instead.

Caskfile.<layer> and Headfile.<layer> hold one name per line, and a
Headfile name may be fully qualified as user/tap/formula:

  name
  name                   # description

Tapfile.<layer> holds one tap per line with an optional URL, which is
used when the tap is trusted and registered:

  user/tap
  user/tap https://github.com/user/homebrew-tap
  user/tap               # description

A line that fits none of these shapes is an invalid entry: an apply
command reports it as a failure, and 'brewkit lint' reports it as a
known-entry-shape violation.

The files are meant to be edited by people and by language models.
Within a section, a run of entries delimited by blank lines, keep the
entries sorted by name; comment-only lines belong to the surrounding
section and do not delimit it. Give every Brewfile, Caskfile, and
Headfile entry a trailing description, leave at least two spaces
before its '#', follow every '#' with exactly one space, and avoid
trailing whitespace and consecutive blank lines. 'brewkit lint --help'
lists the rules that enforce these conventions and how violations are
reported.`)
}
