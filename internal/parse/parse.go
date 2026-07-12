// Package parse parses brewkit profile files (Brewfile, Caskfile, Headfile,
// Tapfile) into a uniform line-oriented structure suitable for both runtime
// application and lint checks.
package parse

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/jmcampanini/brewkit/internal/profile"
)

// LineKind classifies a single line in a profile file.
type LineKind int

const (
	// LineBlank is an empty or whitespace-only line.
	LineBlank LineKind = iota
	// LineComment is a comment-only line (first non-whitespace char is '#').
	LineComment
	// LineEntry is a parsed entry (a package, cask, formula, or tap).
	LineEntry
	// LineUnknown is a non-blank, non-comment line that did not parse as
	// a valid entry for the file's kind.
	LineUnknown
)

// Entry is a single parsed entry from a profile file.
type Entry struct {
	// Name is the canonical key for sort/identity. For Brewfile this is
	// the contents of the quoted string; for the simple file types it
	// is the first whitespace-delimited token.
	Name string

	// Extra holds the optional second token (used by Tapfile for the
	// repository URL). Empty for the other kinds.
	Extra string

	// Description is the trailing inline comment text with the leading
	// '#' and surrounding whitespace stripped. Empty if no inline comment.
	Description string

	// HasInlineComment is true when the entry's source line had a
	// trailing '# ...' segment.
	HasInlineComment bool

	// CommentColumn is the byte offset of the '#' that began the inline
	// comment within the raw line, or -1 if no inline comment is present.
	// Used by the inline-comment-gap lint rule.
	CommentColumn int
}

// Line is one parsed line from a profile file.
type Line struct {
	Number int
	Raw    string
	Kind   LineKind
	Entry  *Entry
}

// File is a parsed profile file.
type File struct {
	Path  string
	Kind  profile.Kind
	Lines []Line
}

// Entries returns just the LineEntry lines as a slice of *Entry pointers.
func (f *File) Entries() []*Entry {
	var out []*Entry
	for i := range f.Lines {
		if f.Lines[i].Kind == LineEntry && f.Lines[i].Entry != nil {
			out = append(out, f.Lines[i].Entry)
		}
	}
	return out
}

// Parse dispatches to the appropriate parser for the given file kind.
func Parse(path string, kind profile.Kind) (*File, error) {
	if kind == profile.KindBrew {
		return Brewfile(path)
	}
	return Simple(path, kind)
}

var brewfileEntryRe = regexp.MustCompile(`^\s*brew\s+"([^"]+)"(.*)$`)

// Brewfile parses a Brewfile (Homebrew Bundle subset) file. Only
// `brew "name"` and `brew "name" # description` lines are recognized as
// entries; anything else non-comment is reported as LineUnknown for lint.
func Brewfile(path string) (*File, error) {
	return parseFile(path, profile.KindBrew, parseBrewfileLine)
}

// Simple parses a Tapfile, Headfile, or Caskfile (one entry per line,
// optional `# description`). Tapfile may carry an optional URL as a second
// token.
func Simple(path string, kind profile.Kind) (*File, error) {
	if kind == profile.KindBrew {
		return nil, fmt.Errorf("parse.Simple cannot parse %s; use parse.Brewfile", kind)
	}
	allowExtra := kind == profile.KindTap
	return parseFile(path, kind, func(raw string) (LineKind, *Entry) {
		return parseSimpleLine(raw, allowExtra)
	})
}

type lineParser func(raw string) (LineKind, *Entry)

func parseFile(path string, kind profile.Kind, parser lineParser) (*File, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	out := &File{Path: path, Kind: kind}
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	lineNo := 0
	for scanner.Scan() {
		lineNo++
		raw := scanner.Text()
		kind, entry := classifyLine(raw, parser)
		out.Lines = append(out.Lines, Line{
			Number: lineNo,
			Raw:    raw,
			Kind:   kind,
			Entry:  entry,
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan %s: %w", path, err)
	}
	return out, nil
}

// classifyLine returns LineBlank/LineComment without invoking the
// kind-specific parser; otherwise it delegates.
func classifyLine(raw string, parser lineParser) (LineKind, *Entry) {
	trimmed := strings.TrimSpace(raw)
	switch {
	case trimmed == "":
		return LineBlank, nil
	case strings.HasPrefix(trimmed, "#"):
		return LineComment, nil
	}
	return parser(raw)
}

func parseBrewfileLine(raw string) (LineKind, *Entry) {
	m := brewfileEntryRe.FindStringSubmatch(raw)
	if m == nil {
		return LineUnknown, nil
	}
	name := m[1]
	rest := m[2]

	hashIdx := strings.Index(rest, "#")
	hasComment := hashIdx >= 0

	// Anything between the closing quote and the comment (or EOL) must
	// be whitespace; otherwise the line carries Bundle options that
	// brewkit does not support, and we must not silently drop them.
	preComment := rest
	if hasComment {
		preComment = rest[:hashIdx]
	}
	if strings.TrimSpace(preComment) != "" {
		return LineUnknown, nil
	}

	var desc string
	col := -1
	if hasComment {
		desc = strings.TrimSpace(strings.TrimPrefix(rest[hashIdx:], "#"))
		col = len(raw) - len(rest) + hashIdx
	}
	return LineEntry, &Entry{
		Name:             name,
		Description:      desc,
		HasInlineComment: hasComment,
		CommentColumn:    col,
	}
}

func parseSimpleLine(raw string, allowExtra bool) (LineKind, *Entry) {
	// Simple file formats have no quoting, so any '#' starts a comment.
	commentCol := strings.Index(raw, "#")
	body := raw
	hasComment := commentCol >= 0
	var desc string
	if hasComment {
		body = raw[:commentCol]
		desc = strings.TrimSpace(strings.TrimPrefix(raw[commentCol:], "#"))
	}
	fields := strings.Fields(body)
	if len(fields) == 0 {
		return LineBlank, nil
	}
	maxFields := 1
	if allowExtra {
		maxFields = 2
	}
	if len(fields) > maxFields {
		return LineUnknown, nil
	}
	entry := &Entry{
		Name:             fields[0],
		Description:      desc,
		HasInlineComment: hasComment,
		CommentColumn:    commentCol,
	}
	if allowExtra && len(fields) == 2 {
		entry.Extra = fields[1]
	}
	return LineEntry, entry
}
