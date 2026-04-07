package lint

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jmcampanini/brewkit/internal/parse"
	"github.com/jmcampanini/brewkit/internal/profile"
)

// TestPinnedFixtures_Clean enforces that the dotfiles snapshots committed
// under internal/parse/testdata/ remain free of lint violations. These
// fixtures are the canonical "what well-formed profile files look like"
// reference for both humans and the parser tests; if a real lint rule
// flags one of them, either the rule is wrong or the fixture needs to
// be updated. Either way, this test should fail loudly.
func TestPinnedFixtures_Clean(t *testing.T) {
	cases := []struct {
		path string
		kind profile.Kind
	}{
		{"../parse/testdata/Brewfile.common", profile.KindBrew},
		{"../parse/testdata/Brewfile.personal", profile.KindBrew},
		{"../parse/testdata/Caskfile.common", profile.KindCask},
		{"../parse/testdata/Caskfile.personal", profile.KindCask},
		{"../parse/testdata/Headfile.common", profile.KindHead},
		{"../parse/testdata/Tapfile.common", profile.KindTap},
	}
	for _, tc := range cases {
		t.Run(filepath.Base(tc.path), func(t *testing.T) {
			f, err := parse.Parse(tc.path, tc.kind)
			if err != nil {
				t.Fatal(err)
			}
			vs := Check(f)
			if len(vs) != 0 {
				for _, v := range vs {
					t.Errorf("%s:%d [%s] %s", tc.path, v.Line, v.RuleID, v.Message)
				}
			}
		})
	}
}

// writeFixture creates a file under t.TempDir and parses it with the
// appropriate parser for the given kind.
func writeFixture(t *testing.T, name string, kind profile.Kind, contents string) *parse.File {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := parse.Parse(path, kind)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

// findRule extracts violations matching a rule ID for assertions.
func findRule(vs []Violation, id string) []Violation {
	var out []Violation
	for _, v := range vs {
		if v.RuleID == id {
			out = append(out, v)
		}
	}
	return out
}

func TestClean_Brewfile(t *testing.T) {
	contents := strings.Join([]string{
		`# packages`,
		`brew "agent-browser"  # Browser automation`,
		`brew "bash"           # Bourne-Again SHell`,
		`brew "git"            # Distributed VCS`,
		``,
	}, "\n")
	f := writeFixture(t, "Brewfile.test", profile.KindBrew, contents)
	if v := Check(f); len(v) != 0 {
		t.Errorf("clean Brewfile produced violations: %+v", v)
	}
}

func TestClean_Caskfile(t *testing.T) {
	contents := strings.Join([]string{
		`# casks`,
		`antinote   # Notes`,
		`ghostty    # Terminal`,
		``,
	}, "\n")
	f := writeFixture(t, "Caskfile.test", profile.KindCask, contents)
	if v := Check(f); len(v) != 0 {
		t.Errorf("clean Caskfile produced violations: %+v", v)
	}
}

func TestClean_Tapfile(t *testing.T) {
	contents := strings.Join([]string{
		`example/bar`,
		`example/foo https://example.com/foo`,
		``,
	}, "\n")
	f := writeFixture(t, "Tapfile.test", profile.KindTap, contents)
	if v := Check(f); len(v) != 0 {
		t.Errorf("clean Tapfile produced violations: %+v", v)
	}
}

func TestRule_SortOrder(t *testing.T) {
	contents := strings.Join([]string{
		`brew "git"     # vcs`,
		`brew "abc"     # alphabet`,
		``,
	}, "\n")
	f := writeFixture(t, "Brewfile.test", profile.KindBrew, contents)
	got := findRule(Check(f), "sort-order")
	if len(got) != 1 {
		t.Fatalf("got %d sort-order violations, want 1", len(got))
	}
	if got[0].Line != 2 {
		t.Errorf("violation on line %d, want 2", got[0].Line)
	}
}

func TestRule_SortOrder_PerSection(t *testing.T) {
	contents := strings.Join([]string{
		`brew "abc"  # a`,
		`brew "ghi"  # g`,
		``,
		`brew "def"  # d`,
		`brew "xyz"  # x`,
		``,
	}, "\n")
	f := writeFixture(t, "Brewfile.test", profile.KindBrew, contents)
	got := findRule(Check(f), "sort-order")
	if len(got) != 0 {
		t.Errorf("expected 0 sort-order violations across sections, got %d: %+v", len(got), got)
	}
}

func TestRule_TrailingDescription(t *testing.T) {
	contents := `brew "git"` + "\n"
	f := writeFixture(t, "Brewfile.test", profile.KindBrew, contents)
	got := findRule(Check(f), "trailing-description")
	if len(got) != 1 {
		t.Errorf("got %d trailing-description violations, want 1", len(got))
	}
}

func TestRule_TrailingDescription_TapfileSkipped(t *testing.T) {
	contents := `charmbracelet/tap` + "\n"
	f := writeFixture(t, "Tapfile.test", profile.KindTap, contents)
	got := findRule(Check(f), "trailing-description")
	if len(got) != 0 {
		t.Errorf("Tapfile should not produce trailing-description violations, got %+v", got)
	}
}

func TestRule_CommentSpace_NoSpace(t *testing.T) {
	contents := `#packages` + "\n"
	f := writeFixture(t, "Brewfile.test", profile.KindBrew, contents)
	got := findRule(Check(f), "comment-space")
	if len(got) != 1 {
		t.Errorf("got %d comment-space violations, want 1", len(got))
	}
}

func TestRule_CommentSpace_TwoSpaces(t *testing.T) {
	contents := `#  packages` + "\n"
	f := writeFixture(t, "Brewfile.test", profile.KindBrew, contents)
	got := findRule(Check(f), "comment-space")
	if len(got) != 1 {
		t.Errorf("got %d comment-space violations, want 1", len(got))
	}
}

func TestRule_CommentSpace_InlineEntry(t *testing.T) {
	contents := `brew "git"  #vcs` + "\n"
	f := writeFixture(t, "Brewfile.test", profile.KindBrew, contents)
	got := findRule(Check(f), "comment-space")
	if len(got) != 1 {
		t.Errorf("got %d comment-space violations, want 1", len(got))
	}
}

func TestRule_InlineCommentGap(t *testing.T) {
	contents := `brew "git" # vcs` + "\n"
	f := writeFixture(t, "Brewfile.test", profile.KindBrew, contents)
	got := findRule(Check(f), "inline-comment-gap")
	if len(got) != 1 {
		t.Errorf("got %d inline-comment-gap violations, want 1", len(got))
	}
}

func TestRule_NoConsecutiveBlanks(t *testing.T) {
	contents := strings.Join([]string{
		`brew "abc"  # a`,
		``,
		``,
		`brew "def"  # d`,
		``,
	}, "\n")
	f := writeFixture(t, "Brewfile.test", profile.KindBrew, contents)
	got := findRule(Check(f), "no-consecutive-blanks")
	if len(got) != 1 {
		t.Errorf("got %d no-consecutive-blanks violations, want 1", len(got))
	}
}

func TestRule_NoTrailingWhitespace(t *testing.T) {
	contents := `brew "git"  # vcs   ` + "\n"
	f := writeFixture(t, "Brewfile.test", profile.KindBrew, contents)
	got := findRule(Check(f), "no-trailing-whitespace")
	if len(got) != 1 {
		t.Errorf("got %d no-trailing-whitespace violations, want 1", len(got))
	}
}

func TestRule_KnownEntryShape_Brewfile(t *testing.T) {
	contents := `tap "user/repo"` + "\n"
	f := writeFixture(t, "Brewfile.test", profile.KindBrew, contents)
	got := findRule(Check(f), "known-entry-shape")
	if len(got) != 1 {
		t.Errorf("got %d known-entry-shape violations, want 1", len(got))
	}
}

func TestRule_KnownEntryShape_Caskfile(t *testing.T) {
	contents := `ghostty extra-token` + "\n"
	f := writeFixture(t, "Caskfile.test", profile.KindCask, contents)
	got := findRule(Check(f), "known-entry-shape")
	if len(got) != 1 {
		t.Errorf("got %d known-entry-shape violations, want 1", len(got))
	}
}

func TestCheck_StableOrder(t *testing.T) {
	contents := strings.Join([]string{
		`brew "git" #vcs`, // line 1: comment-space + inline-comment-gap
	}, "\n") + "\n"
	f := writeFixture(t, "Brewfile.test", profile.KindBrew, contents)
	vs := Check(f)
	if len(vs) < 2 {
		t.Fatalf("expected at least 2 violations, got %d", len(vs))
	}
	for i := 1; i < len(vs); i++ {
		if vs[i-1].Line > vs[i].Line {
			t.Errorf("violations not sorted by line: %+v", vs)
		}
	}
}
