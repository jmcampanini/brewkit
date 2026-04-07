package parse

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jmcampanini/brewkit/internal/profile"
)

func TestParseBrewfile_CommonFixture(t *testing.T) {
	f, err := ParseBrewfile(filepath.Join("testdata", "Brewfile.common"))
	if err != nil {
		t.Fatal(err)
	}
	entries := f.Entries()
	if len(entries) < 5 {
		t.Errorf("expected at least 5 entries from the synthetic fixture, got %d", len(entries))
	}
	for _, e := range entries {
		if e.Name == "" {
			t.Error("entry with empty name")
		}
		if !e.HasInlineComment {
			t.Errorf("entry %q missing inline description", e.Name)
		}
	}
	for _, l := range f.Lines {
		if l.Kind == LineUnknown {
			t.Errorf("line %d unexpectedly Unknown: %q", l.Number, l.Raw)
		}
	}
}

func TestParseBrewfile_PersonalFixture(t *testing.T) {
	f, err := ParseBrewfile(filepath.Join("testdata", "Brewfile.personal"))
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Entries()) == 0 {
		t.Error("expected at least one entry")
	}
}

func TestParseSimple_Caskfile(t *testing.T) {
	f, err := ParseSimple(filepath.Join("testdata", "Caskfile.common"), profile.KindCask)
	if err != nil {
		t.Fatal(err)
	}
	entries := f.Entries()
	if len(entries) < 5 {
		t.Errorf("expected at least 5 entries from the synthetic fixture, got %d", len(entries))
	}
	for _, e := range entries {
		if e.Extra != "" {
			t.Errorf("cask entry %q should not have Extra: %q", e.Name, e.Extra)
		}
	}
}

func TestParseSimple_Headfile(t *testing.T) {
	f, err := ParseSimple(filepath.Join("testdata", "Headfile.common"), profile.KindHead)
	if err != nil {
		t.Fatal(err)
	}
	wantNames := map[string]bool{
		"alpha":               true,
		"delta":               true,
		"example/tap/bravo":   true,
		"example/tap/charlie": true,
	}
	for _, e := range f.Entries() {
		if !wantNames[e.Name] {
			t.Errorf("unexpected entry %q", e.Name)
		}
		delete(wantNames, e.Name)
	}
	if len(wantNames) > 0 {
		t.Errorf("missing entries: %v", wantNames)
	}
}

func TestParseSimple_Tapfile(t *testing.T) {
	f, err := ParseSimple(filepath.Join("testdata", "Tapfile.common"), profile.KindTap)
	if err != nil {
		t.Fatal(err)
	}
	var withURL int
	for _, e := range f.Entries() {
		if e.Extra != "" {
			withURL++
		}
	}
	if withURL == 0 {
		t.Error("expected at least one tap with a URL")
	}
}

func TestParseSimple_RejectsBrewfile(t *testing.T) {
	if _, err := ParseSimple("testdata/Brewfile.common", profile.KindBrew); err == nil {
		t.Error("ParseSimple should reject KindBrew")
	}
}

func TestParseBrewfile_InlineFixtures(t *testing.T) {
	cases := []struct {
		name       string
		raw        string
		wantKind   LineKind
		wantName   string
		wantDesc   string
		wantInline bool
	}{
		{
			name:       "bare entry",
			raw:        `brew "git"`,
			wantKind:   LineEntry,
			wantName:   "git",
			wantInline: false,
		},
		{
			name:       "entry with description",
			raw:        `brew "git" # version control`,
			wantKind:   LineEntry,
			wantName:   "git",
			wantDesc:   "version control",
			wantInline: true,
		},
		{
			name:       "qualified name with aligned description",
			raw:        `brew "user/tap/foo"   # foo`,
			wantKind:   LineEntry,
			wantName:   "user/tap/foo",
			wantDesc:   "foo",
			wantInline: true,
		},
		{
			name:     "comment line",
			raw:      `# section`,
			wantKind: LineComment,
		},
		{
			name:     "blank",
			raw:      ``,
			wantKind: LineBlank,
		},
		{
			name:     "tap line is unknown for brewfile",
			raw:      `tap "user/repo"`,
			wantKind: LineUnknown,
		},
		{
			name:     "brew with args is unknown",
			raw:      `brew "git", args: ["foo"]`,
			wantKind: LineUnknown,
		},
		{
			name:     "brew with args plus inline comment is unknown",
			raw:      `brew "git", args: ["with-pcre2"]  # build with PCRE2`,
			wantKind: LineUnknown,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tmp := t.TempDir()
			path := filepath.Join(tmp, "Brewfile.test")
			if err := os.WriteFile(path, []byte(tc.raw+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			f, err := ParseBrewfile(path)
			if err != nil {
				t.Fatal(err)
			}
			if len(f.Lines) != 1 {
				t.Fatalf("expected 1 line, got %d", len(f.Lines))
			}
			line := f.Lines[0]
			if line.Kind != tc.wantKind {
				t.Errorf("Kind = %v, want %v", line.Kind, tc.wantKind)
			}
			if tc.wantKind == LineEntry {
				if line.Entry == nil {
					t.Fatal("Entry nil")
				}
				if line.Entry.Name != tc.wantName {
					t.Errorf("Name = %q, want %q", line.Entry.Name, tc.wantName)
				}
				if line.Entry.Description != tc.wantDesc {
					t.Errorf("Description = %q, want %q", line.Entry.Description, tc.wantDesc)
				}
				if line.Entry.HasInlineComment != tc.wantInline {
					t.Errorf("HasInlineComment = %v, want %v", line.Entry.HasInlineComment, tc.wantInline)
				}
			}
		})
	}
}

func TestParseSimple_InlineFixtures(t *testing.T) {
	cases := []struct {
		name      string
		raw       string
		kind      profile.Kind
		wantKind  LineKind
		wantName  string
		wantExtra string
		wantDesc  string
	}{
		{
			name:     "cask with description",
			raw:      `ghostty                # Terminal`,
			kind:     profile.KindCask,
			wantKind: LineEntry,
			wantName: "ghostty",
			wantDesc: "Terminal",
		},
		{
			name:     "head qualified",
			raw:      `example/tap/foo           # foo`,
			kind:     profile.KindHead,
			wantKind: LineEntry,
			wantName: "example/tap/foo",
			wantDesc: "foo",
		},
		{
			name:      "tap with url",
			raw:       `example/foo https://example.com/foo`,
			kind:      profile.KindTap,
			wantKind:  LineEntry,
			wantName:  "example/foo",
			wantExtra: "https://example.com/foo",
		},
		{
			name:     "tap without url",
			raw:      `example/bar`,
			kind:     profile.KindTap,
			wantKind: LineEntry,
			wantName: "example/bar",
		},
		{
			name:      "tap with url and description",
			raw:       `example/foo https://example.com/foo  # foo repo`,
			kind:      profile.KindTap,
			wantKind:  LineEntry,
			wantName:  "example/foo",
			wantExtra: "https://example.com/foo",
			wantDesc:  "foo repo",
		},
		{
			name:     "cask with extra token is unknown",
			raw:      `ghostty extra-token`,
			kind:     profile.KindCask,
			wantKind: LineUnknown,
		},
		{
			name:     "tap with three tokens is unknown",
			raw:      `a b c`,
			kind:     profile.KindTap,
			wantKind: LineUnknown,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tmp := t.TempDir()
			path := filepath.Join(tmp, "fixture")
			if err := os.WriteFile(path, []byte(tc.raw+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			f, err := ParseSimple(path, tc.kind)
			if err != nil {
				t.Fatal(err)
			}
			if len(f.Lines) != 1 {
				t.Fatalf("expected 1 line, got %d", len(f.Lines))
			}
			line := f.Lines[0]
			if line.Kind != tc.wantKind {
				t.Errorf("Kind = %v, want %v", line.Kind, tc.wantKind)
			}
			if tc.wantKind == LineEntry {
				if line.Entry.Name != tc.wantName {
					t.Errorf("Name = %q, want %q", line.Entry.Name, tc.wantName)
				}
				if line.Entry.Extra != tc.wantExtra {
					t.Errorf("Extra = %q, want %q", line.Entry.Extra, tc.wantExtra)
				}
				if line.Entry.Description != tc.wantDesc {
					t.Errorf("Description = %q, want %q", line.Entry.Description, tc.wantDesc)
				}
			}
		})
	}
}
