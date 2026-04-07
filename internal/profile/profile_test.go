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
		Profiles:    []string{"common"},
		ProfilesEnv: "BREWKIT_PROFILES",
		FailFast:    true,
	}
}

func TestResolve_DefaultsOnly(t *testing.T) {
	cfg := baseCfg(t.TempDir())
	t.Setenv("BREWKIT_PROFILES", "")
	_ = os.Unsetenv("BREWKIT_PROFILES")

	got, err := Resolve(cfg, nil)
	if err != nil {
		t.Fatalf("Resolve err: %v", err)
	}
	if !reflect.DeepEqual(got, []string{"common"}) {
		t.Errorf("got %v, want [common]", got)
	}
}

func TestResolve_EnvReplacesConfig(t *testing.T) {
	cfg := baseCfg(t.TempDir())
	t.Setenv("BREWKIT_PROFILES", "work,personal")

	got, err := Resolve(cfg, nil)
	if err != nil {
		t.Fatalf("Resolve err: %v", err)
	}
	if !reflect.DeepEqual(got, []string{"work", "personal"}) {
		t.Errorf("got %v, want [work personal]", got)
	}
}

func TestResolve_EnvWithSpacesAndEmpties(t *testing.T) {
	cfg := baseCfg(t.TempDir())
	t.Setenv("BREWKIT_PROFILES", "  work , , personal  ")

	got, err := Resolve(cfg, nil)
	if err != nil {
		t.Fatalf("Resolve err: %v", err)
	}
	if !reflect.DeepEqual(got, []string{"work", "personal"}) {
		t.Errorf("got %v, want [work personal]", got)
	}
}

func TestResolve_FlagOverridesEnv(t *testing.T) {
	cfg := baseCfg(t.TempDir())
	t.Setenv("BREWKIT_PROFILES", "ignored")

	got, err := Resolve(cfg, []string{"flag-only"})
	if err != nil {
		t.Fatalf("Resolve err: %v", err)
	}
	if !reflect.DeepEqual(got, []string{"flag-only"}) {
		t.Errorf("got %v, want [flag-only]", got)
	}
}

func TestResolve_CustomEnvVarName(t *testing.T) {
	cfg := baseCfg(t.TempDir())
	cfg.ProfilesEnv = "DOTFILES_PROFILES"
	_ = os.Unsetenv("BREWKIT_PROFILES")
	t.Setenv("DOTFILES_PROFILES", "common,work")

	got, err := Resolve(cfg, nil)
	if err != nil {
		t.Fatalf("Resolve err: %v", err)
	}
	if !reflect.DeepEqual(got, []string{"common", "work"}) {
		t.Errorf("got %v, want [common work]", got)
	}
}

func TestResolve_EmptyProfilesEnvDisables(t *testing.T) {
	cfg := baseCfg(t.TempDir())
	cfg.ProfilesEnv = ""
	t.Setenv("BREWKIT_PROFILES", "should-be-ignored")

	got, err := Resolve(cfg, nil)
	if err != nil {
		t.Fatalf("Resolve err: %v", err)
	}
	if !reflect.DeepEqual(got, []string{"common"}) {
		t.Errorf("got %v, want [common]", got)
	}
}

func TestResolve_UnsetEnvFallsThrough(t *testing.T) {
	cfg := baseCfg(t.TempDir())
	_ = os.Unsetenv("BREWKIT_PROFILES")

	got, err := Resolve(cfg, nil)
	if err != nil {
		t.Fatalf("Resolve err: %v", err)
	}
	if !reflect.DeepEqual(got, []string{"common"}) {
		t.Errorf("got %v, want [common]", got)
	}
}

func TestResolve_FlagEmptyListIsValid(t *testing.T) {
	cfg := baseCfg(t.TempDir())
	got, err := Resolve(cfg, []string{})
	if err != nil {
		t.Fatalf("Resolve err: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got %v, want []", got)
	}
}

func TestResolve_LocalAutoAppendedWhenFilesPresent(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "Brewfile.local"), "")
	cfg := baseCfg(dir)

	got, err := Resolve(cfg, nil)
	if err != nil {
		t.Fatalf("Resolve err: %v", err)
	}
	if !reflect.DeepEqual(got, []string{"common", "local"}) {
		t.Errorf("got %v, want [common local]", got)
	}
}

func TestResolve_LocalNotAppendedWhenAbsent(t *testing.T) {
	cfg := baseCfg(t.TempDir())
	got, err := Resolve(cfg, nil)
	if err != nil {
		t.Fatalf("Resolve err: %v", err)
	}
	for _, p := range got {
		if p == LocalName {
			t.Errorf("local appended when no *file.local exists: %v", got)
		}
	}
}

func TestResolve_LocalRejectedFromExplicit(t *testing.T) {
	cfg := baseCfg(t.TempDir())
	if _, err := Resolve(cfg, []string{"local"}); err == nil {
		t.Error("Resolve should reject explicit 'local' profile")
	}

	cfg.Profiles = []string{"local"}
	if _, err := Resolve(cfg, nil); err == nil {
		t.Error("Resolve should reject 'local' from config")
	}

	t.Setenv("BREWKIT_PROFILES", "local")
	cfg.Profiles = []string{"common"}
	if _, err := Resolve(cfg, nil); err == nil {
		t.Error("Resolve should reject 'local' from env")
	}
}

func TestResolve_EmptyProfileNameRejected(t *testing.T) {
	cfg := baseCfg(t.TempDir())
	if _, err := Resolve(cfg, []string{""}); err == nil {
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
