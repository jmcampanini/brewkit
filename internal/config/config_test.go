package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	configloader "github.com/jmcampanini/go-config-loader"
	"github.com/jmcampanini/go-config-loader/pflagloader"
	"github.com/spf13/pflag"
)

func TestDefaults(t *testing.T) {
	got := Defaults()
	want := Config{
		Dir:         ".",
		Profiles:    []string{},
		EnvProfiles: "",
		FailFast:    true,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Defaults() = %+v, want %+v", got, want)
	}
}

func TestLoad_MissingImplicitFallsBackToDefaults(t *testing.T) {
	unsetEnv(t, "BREWKIT_PROFILES")
	dir := t.TempDir()
	t.Chdir(dir)

	got, report, err := LoadWithReport("", nil)
	if err != nil {
		t.Fatalf("LoadWithReport(\"\") returned err: %v", err)
	}
	if len(report.LoadedFiles) != 0 {
		t.Errorf("LoadedFiles = %v, want none", report.LoadedFiles)
	}
	if !reflect.DeepEqual(got, Defaults()) {
		t.Errorf("config = %+v, want defaults %+v", got, Defaults())
	}
	if report.Updates["profiles"] != configloader.SourceDefault {
		t.Errorf("profiles source = %q, want default", report.Updates["profiles"])
	}
}

func TestLoad_ImplicitConfigDirectoryErrors(t *testing.T) {
	unsetEnv(t, "BREWKIT_PROFILES")
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.Mkdir("brewkit.toml", 0o755); err != nil {
		t.Fatal(err)
	}

	_, _, err := LoadWithReport("", nil)
	if err == nil {
		t.Fatal("LoadWithReport should error when implicit brewkit.toml is a directory")
	}
	if !strings.Contains(err.Error(), "directory") {
		t.Fatalf("error should mention directory, got: %v", err)
	}
}

func TestLoad_MissingExplicitErrors(t *testing.T) {
	unsetEnv(t, "BREWKIT_PROFILES")
	if _, _, err := LoadWithReport("/nonexistent/brewkit.toml", nil); err == nil {
		t.Fatal("LoadWithReport with nonexistent explicit path should error")
	}
}

func TestLoad_ExplicitConfigDirectoryErrors(t *testing.T) {
	unsetEnv(t, "BREWKIT_PROFILES")
	dir := t.TempDir()
	path := filepath.Join(dir, "brewkit.toml")
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatal(err)
	}

	_, _, err := LoadWithReport(path, nil)
	if err == nil {
		t.Fatal("LoadWithReport should error when explicit config is a directory")
	}
	if !strings.Contains(err.Error(), "directory") {
		t.Fatalf("error should mention directory, got: %v", err)
	}
}

func TestLoad_ParsesFile(t *testing.T) {
	unsetEnv(t, "BREWKIT_PROFILES")
	dir := t.TempDir()
	path := filepath.Join(dir, "brewkit.toml")
	contents := []byte(`
dir = "./homebrew"
profiles = ["common", "work"]
env_profiles = "DOTFILES_PROFILES"
fail_fast = false
`)
	if err := writeFile(t, path, contents); err != nil {
		t.Fatal(err)
	}

	cfg, report, err := LoadWithReport(path, nil)
	if err != nil {
		t.Fatalf("LoadWithReport returned err: %v", err)
	}
	if cfg.Dir != "./homebrew" {
		t.Errorf("Dir = %q, want raw ./homebrew", cfg.Dir)
	}
	if !reflect.DeepEqual(cfg.Profiles, []string{"common", "work"}) {
		t.Errorf("Profiles = %v, want [common work]", cfg.Profiles)
	}
	if cfg.EnvProfiles != "DOTFILES_PROFILES" {
		t.Errorf("EnvProfiles = %q, want DOTFILES_PROFILES", cfg.EnvProfiles)
	}
	if cfg.FailFast {
		t.Errorf("FailFast = true, want false")
	}
	if !reflect.DeepEqual(report.LoadedFiles, []string{path}) {
		t.Errorf("LoadedFiles = %v, want [%s]", report.LoadedFiles, path)
	}
	if report.Updates["profiles"] != path {
		t.Errorf("profiles source = %q, want file path", report.Updates["profiles"])
	}
}

func TestLoad_EmptyDirErrors(t *testing.T) {
	unsetEnv(t, "BREWKIT_PROFILES")
	dir := t.TempDir()
	path := filepath.Join(dir, "brewkit.toml")
	if err := writeFile(t, path, []byte(`dir = ""`)); err != nil {
		t.Fatal(err)
	}

	_, _, err := LoadWithReport(path, nil)
	if err == nil {
		t.Fatal("LoadWithReport should reject empty dir")
	}
	if !strings.Contains(err.Error(), "dir must not be empty") {
		t.Fatalf("error should mention empty dir, got: %v", err)
	}
}

func TestLoad_PartialFileKeepsDefaults(t *testing.T) {
	unsetEnv(t, "BREWKIT_PROFILES")
	dir := t.TempDir()
	path := filepath.Join(dir, "brewkit.toml")
	if err := writeFile(t, path, []byte(`profiles = ["work"]`)); err != nil {
		t.Fatal(err)
	}

	cfg, _, err := LoadWithReport(path, nil)
	if err != nil {
		t.Fatalf("LoadWithReport returned err: %v", err)
	}
	if cfg.Dir != "." {
		t.Errorf("Dir = %q, want default . (dir is not anchored to config path)", cfg.Dir)
	}
	if cfg.EnvProfiles != "" {
		t.Errorf("EnvProfiles = %q, want default empty", cfg.EnvProfiles)
	}
	if !cfg.FailFast {
		t.Errorf("FailFast = false, want default true")
	}
	if !reflect.DeepEqual(cfg.Profiles, []string{"work"}) {
		t.Errorf("Profiles = %v, want [work]", cfg.Profiles)
	}
}

func TestLoad_MalformedTomlErrors(t *testing.T) {
	unsetEnv(t, "BREWKIT_PROFILES")
	dir := t.TempDir()
	path := filepath.Join(dir, "brewkit.toml")
	if err := writeFile(t, path, []byte("dir = ")); err != nil {
		t.Fatal(err)
	}
	if _, _, err := LoadWithReport(path, nil); err == nil {
		t.Fatal("LoadWithReport with malformed TOML should error")
	}
}

func TestLoad_UnknownTomlKeysError(t *testing.T) {
	unsetEnv(t, "BREWKIT_PROFILES")
	dir := t.TempDir()
	path := filepath.Join(dir, "brewkit.toml")
	if err := writeFile(t, path, []byte("profiles_env = \"OLD\"\n")); err != nil {
		t.Fatal(err)
	}
	_, _, err := LoadWithReport(path, nil)
	if err == nil {
		t.Fatal("LoadWithReport should reject unknown TOML keys")
	}
	if !strings.Contains(err.Error(), "unknown") || !strings.Contains(err.Error(), "profiles_env") {
		t.Fatalf("error should mention unknown profiles_env, got: %v", err)
	}
}

func TestLoad_RelativeDirNotResolvedAgainstConfigPath(t *testing.T) {
	unsetEnv(t, "BREWKIT_PROFILES")
	configDir := t.TempDir()
	path := filepath.Join(configDir, "brewkit.toml")
	if err := writeFile(t, path, []byte(`dir = "homebrew"`)); err != nil {
		t.Fatal(err)
	}
	cwd := t.TempDir()
	t.Chdir(cwd)

	cfg, _, err := LoadWithReport(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Dir != "homebrew" {
		t.Errorf("Dir = %q, want raw relative homebrew", cfg.Dir)
	}
}

func TestLoad_EmptyFileLoadsDefaults(t *testing.T) {
	unsetEnv(t, "BREWKIT_PROFILES")
	dir := t.TempDir()
	path := filepath.Join(dir, "brewkit.toml")
	if err := writeFile(t, path, []byte("")); err != nil {
		t.Fatal(err)
	}
	cfg, report, err := LoadWithReport(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(cfg, Defaults()) {
		t.Errorf("config = %+v, want defaults %+v", cfg, Defaults())
	}
	if !reflect.DeepEqual(report.LoadedFiles, []string{path}) {
		t.Errorf("LoadedFiles = %v, want [%s]", report.LoadedFiles, path)
	}
}

func TestLoad_WhitespaceOnlyFileLoadsDefaults(t *testing.T) {
	unsetEnv(t, "BREWKIT_PROFILES")
	dir := t.TempDir()
	path := filepath.Join(dir, "brewkit.toml")
	if err := writeFile(t, path, []byte("# disabled, no keys\n")); err != nil {
		t.Fatal(err)
	}
	cfg, _, err := LoadWithReport(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(cfg, Defaults()) {
		t.Errorf("config = %+v, want defaults %+v", cfg, Defaults())
	}
}

func TestLoad_AbsoluteDirPreserved(t *testing.T) {
	unsetEnv(t, "BREWKIT_PROFILES")
	dir := t.TempDir()
	path := filepath.Join(dir, "brewkit.toml")
	if err := writeFile(t, path, []byte(`dir = "/abs/path"`)); err != nil {
		t.Fatal(err)
	}
	cfg, _, err := LoadWithReport(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Dir != "/abs/path" {
		t.Errorf("Dir = %q, want /abs/path", cfg.Dir)
	}
}

func TestLoad_BrewkitProfilesEnvOverridesRawProfiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "brewkit.toml")
	if err := writeFile(t, path, []byte(`profiles = ["file"]`)); err != nil {
		t.Fatal(err)
	}
	t.Setenv("BREWKIT_PROFILES", "work, personal")

	cfg, report, err := LoadWithReport(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(cfg.Profiles, []string{"work", "personal"}) {
		t.Errorf("Profiles = %v, want env override [work personal]", cfg.Profiles)
	}
	if report.Updates["profiles"] != configloader.SourceEnv {
		t.Errorf("profiles source = %q, want env", report.Updates["profiles"])
	}
}

func TestLoad_ProfilesFlagOverridesFileAndEnv(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "brewkit.toml")
	if err := writeFile(t, path, []byte(`profiles = ["file"]`)); err != nil {
		t.Fatal(err)
	}
	t.Setenv("BREWKIT_PROFILES", "env")
	flags := newTestFlagSet(t, "--profiles", "work,personal")

	cfg, report, err := LoadWithReport(path, flags)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(cfg.Profiles, []string{"work", "personal"}) {
		t.Errorf("Profiles = %v, want flag override [work personal]", cfg.Profiles)
	}
	if report.Updates["profiles"] != pflagloader.SourcePFlag {
		t.Errorf("profiles source = %q, want pflag", report.Updates["profiles"])
	}
}

func newTestFlagSet(t *testing.T, args ...string) *pflag.FlagSet {
	t.Helper()
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	if err := RegisterFlags(flags); err != nil {
		t.Fatalf("RegisterFlags: %v", err)
	}
	if err := flags.Parse(args); err != nil {
		t.Fatalf("Parse(%v): %v", args, err)
	}
	return flags
}

func unsetEnv(t *testing.T, key string) {
	t.Helper()
	old, had := os.LookupEnv(key)
	if err := os.Unsetenv(key); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if had {
			_ = os.Setenv(key, old)
		} else {
			_ = os.Unsetenv(key)
		}
	})
}

func writeFile(t *testing.T, path string, data []byte) error {
	t.Helper()
	return os.WriteFile(path, data, 0o644)
}
