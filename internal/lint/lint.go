// Package lint validates brewkit profile files against the shared style
// rules: alphabetical sort, comment style, and entry shape.
package lint

import (
	"fmt"
	"sort"
	"strings"

	"github.com/jmcampanini/brewkit/internal/parse"
	"github.com/jmcampanini/brewkit/internal/profile"
)

// Violation is one lint finding: which file/line broke which rule.
type Violation struct {
	Path    string
	Line    int
	RuleID  string
	Message string
	Raw     string
}

// Rule pairs a stable rule ID with its check function.
type Rule struct {
	ID    string
	Check func(*parse.File) []Violation
}

// AllRules is the canonical ordered list. Rule IDs are stable - they
// are surfaced in `brewkit lint` output and in test assertions.
var AllRules = []Rule{
	{ID: "sort-order", Check: ruleSortOrder},
	{ID: "trailing-description", Check: ruleTrailingDescription},
	{ID: "comment-space", Check: ruleCommentSpace},
	{ID: "inline-comment-gap", Check: ruleInlineCommentGap},
	{ID: "no-consecutive-blanks", Check: ruleNoConsecutiveBlanks},
	{ID: "no-trailing-whitespace", Check: ruleNoTrailingWhitespace},
	{ID: "known-entry-shape", Check: ruleKnownEntryShape},
}

// Check runs every rule and returns violations sorted by (line, rule ID)
// so output is stable across runs.
func Check(f *parse.File) []Violation {
	var out []Violation
	for _, r := range AllRules {
		out = append(out, r.Check(f)...)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Line != out[j].Line {
			return out[i].Line < out[j].Line
		}
		return out[i].RuleID < out[j].RuleID
	})
	return out
}

func ruleSortOrder(f *parse.File) []Violation {
	var out []Violation
	var section []*parse.Line

	flush := func() {
		defer func() { section = nil }()
		if len(section) < 2 {
			return
		}
		for i := 1; i < len(section); i++ {
			prev := section[i-1].Entry.Name
			cur := section[i].Entry.Name
			if cur < prev {
				out = append(out, Violation{
					Path:    f.Path,
					Line:    section[i].Number,
					RuleID:  "sort-order",
					Message: fmt.Sprintf("%q should sort before %q", cur, prev),
					Raw:     section[i].Raw,
				})
			}
		}
	}

	for i := range f.Lines {
		l := &f.Lines[i]
		switch l.Kind {
		case parse.LineBlank:
			flush()
		case parse.LineEntry:
			section = append(section, l)
		}
	}
	flush()
	return out
}

func ruleTrailingDescription(f *parse.File) []Violation {
	// Tapfile entries are exempt: the name already carries owner/repo,
	// and real-world Tapfiles don't conventionally carry descriptions.
	if f.Kind == profile.KindTap {
		return nil
	}
	var out []Violation
	for i := range f.Lines {
		l := &f.Lines[i]
		if l.Kind != parse.LineEntry {
			continue
		}
		if !l.Entry.HasInlineComment || l.Entry.Description == "" {
			out = append(out, Violation{
				Path:    f.Path,
				Line:    l.Number,
				RuleID:  "trailing-description",
				Message: fmt.Sprintf("entry %q should have a trailing # description", l.Entry.Name),
				Raw:     l.Raw,
			})
		}
	}
	return out
}

func ruleCommentSpace(f *parse.File) []Violation {
	var out []Violation
	for i := range f.Lines {
		l := &f.Lines[i]
		pos := -1
		switch l.Kind {
		case parse.LineComment:
			pos = strings.Index(l.Raw, "#")
		case parse.LineEntry:
			if l.Entry.HasInlineComment {
				pos = l.Entry.CommentColumn
			}
		}
		if pos < 0 {
			continue
		}
		rest := l.Raw[pos+1:]
		if rest == "" {
			continue
		}
		if rest[0] != ' ' {
			out = append(out, Violation{
				Path:    f.Path,
				Line:    l.Number,
				RuleID:  "comment-space",
				Message: "# must be followed by a single space",
				Raw:     l.Raw,
			})
			continue
		}
		if len(rest) >= 2 && rest[1] == ' ' {
			out = append(out, Violation{
				Path:    f.Path,
				Line:    l.Number,
				RuleID:  "comment-space",
				Message: "# must be followed by exactly one space, not more",
				Raw:     l.Raw,
			})
		}
	}
	return out
}

func ruleInlineCommentGap(f *parse.File) []Violation {
	var out []Violation
	for i := range f.Lines {
		l := &f.Lines[i]
		if l.Kind != parse.LineEntry || !l.Entry.HasInlineComment {
			continue
		}
		col := l.Entry.CommentColumn
		if col < 2 || l.Raw[col-1] != ' ' || l.Raw[col-2] != ' ' {
			out = append(out, Violation{
				Path:    f.Path,
				Line:    l.Number,
				RuleID:  "inline-comment-gap",
				Message: "trailing inline comment should be preceded by at least two spaces",
				Raw:     l.Raw,
			})
		}
	}
	return out
}

func ruleNoConsecutiveBlanks(f *parse.File) []Violation {
	var out []Violation
	for i := 1; i < len(f.Lines); i++ {
		if f.Lines[i].Kind == parse.LineBlank && f.Lines[i-1].Kind == parse.LineBlank {
			out = append(out, Violation{
				Path:    f.Path,
				Line:    f.Lines[i].Number,
				RuleID:  "no-consecutive-blanks",
				Message: "consecutive blank lines",
			})
		}
	}
	return out
}

func ruleNoTrailingWhitespace(f *parse.File) []Violation {
	var out []Violation
	for i := range f.Lines {
		l := &f.Lines[i]
		if l.Raw == "" {
			continue
		}
		if strings.TrimRight(l.Raw, " \t") != l.Raw {
			out = append(out, Violation{
				Path:    f.Path,
				Line:    l.Number,
				RuleID:  "no-trailing-whitespace",
				Message: "trailing whitespace",
				Raw:     l.Raw,
			})
		}
	}
	return out
}

func ruleKnownEntryShape(f *parse.File) []Violation {
	var out []Violation
	for i := range f.Lines {
		l := &f.Lines[i]
		if l.Kind != parse.LineUnknown {
			continue
		}
		out = append(out, Violation{
			Path:    f.Path,
			Line:    l.Number,
			RuleID:  "known-entry-shape",
			Message: fmt.Sprintf("line does not match expected %s entry shape", f.Kind),
			Raw:     l.Raw,
		})
	}
	return out
}
