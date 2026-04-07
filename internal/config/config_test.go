package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestDefaults(t *testing.T) {
	got := Defaults()
	want := Config{
		Dir:         ".",
		Profiles:    []string{"common"},
		ProfilesEnv: "BREWKIT_PROFILES",
		FailFast:    true,
		Source:      "defaults",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Defaults() = %+v, want %+v", got, want)
	}
}

func TestLoad_MissingImplicitFallsBackToDefaults(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	got, err := Load("")
	if err != nil {
		t.Fatalf("Load(\"\") returned err: %v", err)
	}
	if got.Source != "defaults" {
		t.Errorf("Source = %q, want %q", got.Source, "defaults")
	}
	if !reflect.DeepEqual(got.Profiles, []string{"common"}) {
		t.Errorf("Profiles = %v, want [common]", got.Profiles)
	}
}

func TestLoad_MissingExplicitErrors(t *testing.T) {
	if _, err := Load("/nonexistent/brewkit.toml"); err == nil {
		t.Fatal("Load with nonexistent explicit path should error")
	}
}

func TestLoad_ParsesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "brewkit.toml")
	contents := []byte(`
dir = "./homebrew"
profiles = ["common", "work"]
profiles_env = "DOTFILES_PROFILES"
fail_fast = false
`)
	if err := writeFile(t, path, contents); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned err: %v", err)
	}
	wantDir := filepath.Join(dir, "homebrew")
	if cfg.Dir != wantDir {
		t.Errorf("Dir = %q, want %q", cfg.Dir, wantDir)
	}
	if !reflect.DeepEqual(cfg.Profiles, []string{"common", "work"}) {
		t.Errorf("Profiles = %v, want [common work]", cfg.Profiles)
	}
	if cfg.ProfilesEnv != "DOTFILES_PROFILES" {
		t.Errorf("ProfilesEnv = %q, want DOTFILES_PROFILES", cfg.ProfilesEnv)
	}
	if cfg.FailFast {
		t.Errorf("FailFast = true, want false")
	}
	if cfg.Source != path {
		t.Errorf("Source = %q, want %q", cfg.Source, path)
	}
}

func TestLoad_PartialFileKeepsDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "brewkit.toml")
	if err := writeFile(t, path, []byte(`profiles = ["work"]`)); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned err: %v", err)
	}
	// Non-empty config file with omitted dir key now resolves
	// the implicit default "." against the config file's directory,
	// matching the common "run brewkit from anywhere" use case.
	if cfg.Dir != dir {
		t.Errorf("Dir = %q, want %q (non-empty config with omitted dir should resolve to config-file dir)", cfg.Dir, dir)
	}
	if cfg.ProfilesEnv != "BREWKIT_PROFILES" {
		t.Errorf("ProfilesEnv = %q, want default", cfg.ProfilesEnv)
	}
	if !cfg.FailFast {
		t.Errorf("FailFast = false, want default true")
	}
	if !reflect.DeepEqual(cfg.Profiles, []string{"work"}) {
		t.Errorf("Profiles = %v, want [work]", cfg.Profiles)
	}
}

func TestLoad_MalformedTomlErrors(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "brewkit.toml")
	if err := writeFile(t, path, []byte("dir = ")); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("Load with malformed TOML should error")
	}
}

func TestLoad_RelativeDirResolvedAgainstConfigPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "brewkit.toml")
	if err := writeFile(t, path, []byte(`dir = "."`)); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Dir != dir {
		t.Errorf("Dir = %q, want %q", cfg.Dir, dir)
	}
}

func TestLoad_EmptyFileKeepsDefaultDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "brewkit.toml")
	if err := writeFile(t, path, []byte("")); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	// Empty (0-byte) file short-circuits to pure Defaults so
	// `brewkit --config /dev/null config` still shows the
	// documented defaults without kicking in dir resolution.
	if cfg.Dir != "." {
		t.Errorf("Dir = %q, want default \".\" (an empty file should not trigger dir resolution)", cfg.Dir)
	}
	if cfg.ProfilesEnv != "BREWKIT_PROFILES" {
		t.Errorf("ProfilesEnv = %q, want default", cfg.ProfilesEnv)
	}
}

func TestLoad_WhitespaceOnlyFileResolvesAgainstConfigPath(t *testing.T) {
	// Pin the cliff: a comment-only or whitespace-only file is NOT
	// the same as a zero-byte file. The 0-byte case short-circuits to
	// pure Defaults; any other file (even if it parses to an entirely
	// empty fileConfig) goes through the normal resolution path so
	// `dir` lands next to the config file.
	dir := t.TempDir()
	path := filepath.Join(dir, "brewkit.toml")
	if err := writeFile(t, path, []byte("# disabled, no keys\n")); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Dir != dir {
		t.Errorf("Dir = %q, want %q (whitespace/comment-only file should still resolve dir to config-file dir)", cfg.Dir, dir)
	}
}

func TestLoad_EmptyFileSetsSource(t *testing.T) {
	// 0-byte short-circuit returns Defaults but the user explicitly
	// passed --config <path>, so Source should reflect where the
	// (empty) bytes came from for debugging purposes.
	dir := t.TempDir()
	path := filepath.Join(dir, "brewkit.toml")
	if err := writeFile(t, path, []byte("")); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Source != path {
		t.Errorf("Source = %q, want %q (0-byte file should still record its source path)", cfg.Source, path)
	}
}

func TestLoad_OmittedDirResolvesAgainstConfigPath(t *testing.T) {
	// Regression: a user with a real brewkit.toml in /foo that only
	// sets `profiles = ["work"]` (no `dir` key) runs brewkit from
	// /bar and passes --config /foo/brewkit.toml. Profile files must
	// be read from /foo/, not /bar/.
	dir := t.TempDir()
	path := filepath.Join(dir, "brewkit.toml")
	if err := writeFile(t, path, []byte(`profiles = ["work"]`)); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Dir != dir {
		t.Errorf("Dir = %q, want %q (omitted dir in a non-empty file must resolve to config-file dir)", cfg.Dir, dir)
	}
}

func TestLoad_AbsoluteDirPreserved(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "brewkit.toml")
	if err := writeFile(t, path, []byte(`dir = "/abs/path"`)); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Dir != "/abs/path" {
		t.Errorf("Dir = %q, want /abs/path", cfg.Dir)
	}
}

func writeFile(t *testing.T, path string, data []byte) error {
	t.Helper()
	return os.WriteFile(path, data, 0o644)
}
