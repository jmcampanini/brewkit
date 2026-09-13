package main

import (
	"bytes"
	"errors"
	"io/fs"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestApplyCommandsRejectSecondOperandBeforeAnyWork runs the built binary,
// the only place the real brew lookup on PATH and the process exit code
// exist. PATH holds nothing but shims for brew and git that log every call,
// and the fixture is a valid config with one profile of every file kind, so
// a rejected operand that reached command work would leave a log line or a
// changed file.
func TestApplyCommandsRejectSecondOperandBeforeAnyWork(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "brewkit")
	if out, err := exec.Command("go", "build", "-o", binary, ".").CombinedOutput(); err != nil {
		t.Fatalf("build brewkit: %v\n%s", err, out)
	}

	shimDir := t.TempDir()
	shimLog := filepath.Join(shimDir, "calls.log")
	shim := "#!/bin/sh\nprintf '%s %s\\n' \"$(basename \"$0\")\" \"$*\" >> " + shimLog + "\n"
	for _, program := range []string{"brew", "git"} {
		if err := os.WriteFile(filepath.Join(shimDir, program), []byte(shim), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	fixture := t.TempDir()
	for name, contents := range map[string]string{
		"brewkit.toml":    "profiles = [\"common\"]\n",
		"Tapfile.common":  "jmcampanini/overlay https://github.com/jmcampanini/overlay\n",
		"Brewfile.common": "brew \"git\"  # version control\n",
		"Headfile.common": "direnv  # shell environment loader\n",
		"Caskfile.common": "ghostty  # terminal emulator\n",
	} {
		if err := os.WriteFile(filepath.Join(fixture, name), []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	before := snapshotDir(t, fixture)

	for _, command := range []string{"tap", "brew", "head", "cask"} {
		t.Run(command, func(t *testing.T) {
			cmd := exec.Command(binary, "--config", filepath.Join(fixture, "brewkit.toml"), command, "one", "two")
			cmd.Dir = fixture
			cmd.Env = []string{"PATH=" + shimDir}
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr

			err := cmd.Run()

			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ExitCode() != 1 {
				t.Errorf("brewkit %s one two: err = %v, want exit status 1", command, err)
			}
			if want := "accepts at most 1 arg(s), received 2"; !strings.Contains(stderr.String(), want) {
				t.Errorf("brewkit %s one two: stderr = %q, want it to contain %q", command, stderr.String(), want)
			}
			if stdout.Len() != 0 {
				t.Errorf("brewkit %s one two: stdout = %q, want empty", command, stdout.String())
			}
			if calls, err := os.ReadFile(shimLog); !errors.Is(err, fs.ErrNotExist) {
				t.Errorf("brewkit %s one two: shim log = %q, %v; want no log file", command, calls, err)
			}
			if after := snapshotDir(t, fixture); !maps.Equal(before, after) {
				t.Errorf("brewkit %s one two: fixture changed:\nbefore: %q\nafter:  %q", command, before, after)
			}
		})
	}
}

// snapshotDir maps every file under dir, by relative path, to its contents.
func snapshotDir(t *testing.T, dir string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		contents, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		files[rel] = string(contents)
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot %s: %v", dir, err)
	}
	return files
}
