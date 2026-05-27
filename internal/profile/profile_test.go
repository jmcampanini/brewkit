package profile

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/jmcampanini/brewkit/internal/config"
)

func baseCfg(dir string) config.Config {
	return config.Config{
		Dir:         dir,
		Profiles:    []string{},
		EnvProfiles: "",
		FailFast:    true,
	}
}

func TestResolve_DefaultsOnly(t *testing.T) {
	cfg := baseCfg(t.TempDir())

	got, err := Resolve(cfg)
	if err != nil {
		t.Fatalf("Resolve err: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got %v, want []", got)
	}
}

func TestResolve_RawProfiles(t *testing.T) {
	cfg := baseCfg(t.TempDir())
	cfg.Profiles = []string{"work", "personal"}

	got, err := Resolve(cfg)
	if err != nil {
		t.Fatalf("Resolve err: %v", err)
	}
	if !reflect.DeepEqual(got, []string{"work", "personal"}) {
		t.Errorf("got %v, want [work personal]", got)
	}
}

func TestResolve_EnvProfilesAppends(t *testing.T) {
	cfg := baseCfg(t.TempDir())
	cfg.Profiles = []string{"work"}
	cfg.EnvProfiles = "DOTFILES_PROFILES"
	t.Setenv("DOTFILES_PROFILES", "personal, laptop")

	got, err := Resolve(cfg)
	if err != nil {
		t.Fatalf("Resolve err: %v", err)
	}
	if !reflect.DeepEqual(got, []string{"work", "personal", "laptop"}) {
		t.Errorf("got %v, want [work personal laptop]", got)
	}
}

func TestResolve_EnvProfilesUnsetDoesNothing(t *testing.T) {
	cfg := baseCfg(t.TempDir())
	cfg.Profiles = []string{"work"}
	cfg.EnvProfiles = "DOTFILES_PROFILES"
	_ = os.Unsetenv("DOTFILES_PROFILES")

	got, err := Resolve(cfg)
	if err != nil {
		t.Fatalf("Resolve err: %v", err)
	}
	if !reflect.DeepEqual(got, []string{"work"}) {
		t.Errorf("got %v, want [work]", got)
	}
}

func TestResolve_EnvProfilesWithSpacesAndEmpties(t *testing.T) {
	cfg := baseCfg(t.TempDir())
	cfg.EnvProfiles = "DOTFILES_PROFILES"
	t.Setenv("DOTFILES_PROFILES", "  work , , personal  ")

	got, err := Resolve(cfg)
	if err != nil {
		t.Fatalf("Resolve err: %v", err)
	}
	if !reflect.DeepEqual(got, []string{"work", "personal"}) {
		t.Errorf("got %v, want [work personal]", got)
	}
}

func TestResolve_DedupesPreservingOrder(t *testing.T) {
	cfg := baseCfg(t.TempDir())
	cfg.Profiles = []string{"work", "personal", "work"}
	cfg.EnvProfiles = "DOTFILES_PROFILES"
	t.Setenv("DOTFILES_PROFILES", "personal,laptop,work")

	got, err := Resolve(cfg)
	if err != nil {
		t.Fatalf("Resolve err: %v", err)
	}
	if !reflect.DeepEqual(got, []string{"work", "personal", "laptop"}) {
		t.Errorf("got %v, want [work personal laptop]", got)
	}
}

func TestResolve_LocalAutoAppendedWhenFilesPresent(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "Brewfile.local"), "")
	cfg := baseCfg(dir)
	cfg.Profiles = []string{"work"}

	got, err := Resolve(cfg)
	if err != nil {
		t.Fatalf("Resolve err: %v", err)
	}
	if !reflect.DeepEqual(got, []string{"work", "local"}) {
		t.Errorf("got %v, want [work local]", got)
	}
}

func TestResolve_LocalCanBeOnlyAutoProfile(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "Brewfile.local"), "")
	cfg := baseCfg(dir)

	got, err := Resolve(cfg)
	if err != nil {
		t.Fatalf("Resolve err: %v", err)
	}
	if !reflect.DeepEqual(got, []string{"local"}) {
		t.Errorf("got %v, want [local]", got)
	}
}

func TestResolve_LocalNotAppendedWhenAbsent(t *testing.T) {
	cfg := baseCfg(t.TempDir())
	cfg.Profiles = []string{"work"}
	got, err := Resolve(cfg)
	if err != nil {
		t.Fatalf("Resolve err: %v", err)
	}
	for _, p := range got {
		if p == LocalName {
			t.Errorf("local appended when no *file.local exists: %v", got)
		}
	}
}

func TestResolve_LocalRejectedFromExplicitSources(t *testing.T) {
	cfg := baseCfg(t.TempDir())
	cfg.Profiles = []string{"local"}
	if _, err := Resolve(cfg); err == nil {
		t.Error("Resolve should reject 'local' from raw profiles")
	}

	cfg = baseCfg(t.TempDir())
	cfg.EnvProfiles = "DOTFILES_PROFILES"
	t.Setenv("DOTFILES_PROFILES", "local")
	if _, err := Resolve(cfg); err == nil {
		t.Error("Resolve should reject 'local' from env_profiles")
	}
}

func TestResolve_EmptyProfileNameRejected(t *testing.T) {
	cfg := baseCfg(t.TempDir())
	cfg.Profiles = []string{""}
	if _, err := Resolve(cfg); err == nil {
		t.Error("Resolve should reject empty profile name")
	}
}

func TestDiscover(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{
		"Brewfile.common",
		"Caskfile.common",
		"Headfile.work",
		"Tapfile.work",
		"Brewfile.local",
		"unrelated.txt",
		"README.md",
	} {
		mustWrite(t, filepath.Join(dir, name), "")
	}

	got, err := Discover(dir)
	if err != nil {
		t.Fatalf("Discover err: %v", err)
	}
	want := []string{"common", "local", "work"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestDiscover_RejectsBackupAndSwapFiles(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{
		"Brewfile.common",
		"Brewfile.common.bak",
		"Brewfile.common.swp",
		"Brewfile.foo~",
		"Brewfile.",
		".Brewfile.swp",
	} {
		mustWrite(t, filepath.Join(dir, name), "")
	}

	got, err := Discover(dir)
	if err != nil {
		t.Fatalf("Discover err: %v", err)
	}
	if !reflect.DeepEqual(got, []string{"common"}) {
		t.Errorf("got %v, want [common] only — backup/swap/empty suffixes should be rejected", got)
	}
}

func TestFilenameFor(t *testing.T) {
	cases := []struct {
		k    Kind
		want string
	}{
		{KindBrew, "Brewfile.work"},
		{KindCask, "Caskfile.work"},
		{KindHead, "Headfile.work"},
		{KindTap, "Tapfile.work"},
	}
	for _, tc := range cases {
		if got := FilenameFor(tc.k, "work"); got != tc.want {
			t.Errorf("FilenameFor(%s, work) = %q, want %q", tc.k, got, tc.want)
		}
	}
}

func mustWrite(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}
