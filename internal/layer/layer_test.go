package layer

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestValidName(t *testing.T) {
	for _, tt := range []struct {
		name string
		want bool
	}{
		{name: "work", want: true},
		{name: "work-laptop", want: true},
		{name: "2026", want: true},
		{name: "a", want: true},
		{name: strings.Repeat("a", MaxNameLength), want: true},
		{name: "base", want: true},
		{name: "local", want: true},
		{name: ""},
		{name: strings.Repeat("a", MaxNameLength+1)},
		{name: "Work"},
		{name: "work_laptop"},
		{name: "-work"},
		{name: "work-"},
		{name: "work--laptop"},
		{name: "work laptop"},
		{name: "wörk"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := ValidName(tt.name); got != tt.want {
				t.Errorf("ValidName(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestValidateSelected(t *testing.T) {
	for _, tt := range []struct {
		name    string
		wantErr string
	}{
		{name: "work"},
		{name: "Work", wantErr: `invalid layer name "Work"`},
		{name: "", wantErr: `invalid layer name ""`},
		{name: "base", wantErr: `layer "base" is automatic`},
		{name: "local", wantErr: `layer "local" is automatic`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSelected(tt.name)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateSelected(%q) = %v, want nil", tt.name, err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ValidateSelected(%q) = %v, want error containing %q", tt.name, err, tt.wantErr)
			}
		})
	}
}

func TestHasFiles(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "Caskfile"), "")
	mustWrite(t, filepath.Join(dir, "Headfile.local"), "")
	if err := os.Mkdir(filepath.Join(dir, "Brewfile.work"), 0o755); err != nil {
		t.Fatal(err)
	}

	for _, tt := range []struct {
		name string
		want bool
	}{
		{name: BaseName, want: true},
		{name: LocalName, want: true},
		{name: "work", want: true},
		{name: "absent"},
	} {
		got, err := HasFiles(dir, tt.name)
		if err != nil {
			t.Fatalf("HasFiles(%q) err: %v", tt.name, err)
		}
		if got != tt.want {
			t.Errorf("HasFiles(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestDiscover(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{
		"Brewfile",
		"Brewfile.common",
		"Caskfile.common",
		"Headfile.work-laptop",
		"Tapfile.work",
		"Brewfile.local",
		"Brewfile.base",
		"Brewfile.Work",
		"Brewfile.work_laptop",
		"Brewfile.common.bak",
		"Brewfile.common.swp",
		"Brewfile.foo~",
		"Brewfile.",
		".Brewfile.swp",
		"unrelated.txt",
		"README.md",
	} {
		mustWrite(t, filepath.Join(dir, name), "")
	}
	if err := os.Mkdir(filepath.Join(dir, "Tapfile.dir"), 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := Discover(dir)
	if err != nil {
		t.Fatalf("Discover err: %v", err)
	}
	want := []string{"base", "common", "local", "work", "work-laptop"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Discover() = %v, want %v", got, want)
	}
}

func TestFilenameFor(t *testing.T) {
	for _, tt := range []struct {
		k     Kind
		layer string
		want  string
	}{
		{KindBrew, "work", "Brewfile.work"},
		{KindCask, "work", "Caskfile.work"},
		{KindHead, "work", "Headfile.work"},
		{KindTap, "work", "Tapfile.work"},
		{KindBrew, LocalName, "Brewfile.local"},
		{KindBrew, BaseName, "Brewfile"},
		{KindTap, BaseName, "Tapfile"},
	} {
		if got := FilenameFor(tt.k, tt.layer); got != tt.want {
			t.Errorf("FilenameFor(%s, %s) = %q, want %q", tt.k, tt.layer, got, tt.want)
		}
	}
}

func mustWrite(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}
