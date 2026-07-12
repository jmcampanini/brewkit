package brew

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// Exec is the production Brewer that shells out to the `brew` binary.
//
// Mutating action methods (Tap / BrewInstall / BrewUpgrade /
// HeadInstall / HeadReinstall / CaskInstall / CaskUpgrade) capture
// combined stdout+stderr in Result.Output so the caller can render
// full failure context regardless of verbosity. Read-only state probes
// go through runQuiet, which splits stderr into the error only.
type Exec struct {
	Bin string // path to brew, or "brew" to resolve via PATH
}

// NewExec returns an Exec brewer that uses `brew` from PATH.
func NewExec() *Exec { return &Exec{Bin: "brew"} }

func (e *Exec) bin() string {
	if e.Bin == "" {
		return "brew"
	}
	return e.Bin
}

// run invokes brew and captures combined output.
func (e *Exec) run(ctx context.Context, env []string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, e.bin(), args...)
	if env != nil {
		cmd.Env = append(cmd.Environ(), env...)
	}
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	return buf.String(), err
}

// runQuiet returns brew stdout on success and a wrapped error containing
// stderr on failure. Used for state queries where the user should not
// see brew's chatter unless something went wrong.
func (e *Exec) runQuiet(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, e.bin(), args...)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("brew %s: %w: %s", strings.Join(args, " "), err, stderr.String())
	}
	return buf.String(), nil
}

// State issues five separate `brew` subprocess probes (tap list,
// formula list, cask list, outdated formulas, outdated casks) and is
// the heavyweight call that runContext.ensureState gates behind a lazy
// check so kinds with no matching files don't shell out at all.
func (e *Exec) State(ctx context.Context) (*State, error) {
	state := EmptyState()

	// Tapped repositories.
	tapsOut, err := e.runQuiet(ctx, "tap")
	if err != nil {
		return nil, fmt.Errorf("brew tap: %w", err)
	}
	for _, line := range strings.Split(tapsOut, "\n") {
		t := strings.TrimSpace(line)
		if t != "" {
			state.Taps[t] = true
		}
	}

	// Installed formulas with versions.
	if err := e.fillInstalled(ctx, state); err != nil {
		return nil, err
	}
	// Outdated formulas.
	if err := e.fillOutdatedFormulas(ctx, state); err != nil {
		return nil, err
	}
	// Outdated casks.
	if err := e.fillOutdatedCasks(ctx, state); err != nil {
		return nil, err
	}

	return state, nil
}

func (e *Exec) fillInstalled(ctx context.Context, state *State) error {
	// `brew list --versions` prints "name v1 v2 ..." per line.
	formulasOut, err := e.runQuiet(ctx, "list", "--formula", "--versions")
	if err != nil {
		return fmt.Errorf("brew list --formula: %w", err)
	}
	for _, line := range strings.Split(formulasOut, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		name := fields[0]
		version := fields[len(fields)-1]
		state.Formulas[name] = FormulaState{
			Installed: true,
			Version:   version,
			IsHead:    strings.HasPrefix(version, "HEAD-"),
		}
	}

	casksOut, err := e.runQuiet(ctx, "list", "--cask", "--versions")
	if err != nil {
		return fmt.Errorf("brew list --cask: %w", err)
	}
	for _, line := range strings.Split(casksOut, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		name := fields[0]
		version := fields[len(fields)-1]
		state.Casks[name] = CaskState{Installed: true, Version: version}
	}
	return nil
}

type outdatedJSON struct {
	Formulae []struct {
		Name              string   `json:"name"`
		InstalledVersions []string `json:"installed_versions"`
		CurrentVersion    string   `json:"current_version"`
	} `json:"formulae"`
	Casks []struct {
		Name              string   `json:"name"`
		InstalledVersions []string `json:"installed_versions"`
		CurrentVersion    string   `json:"current_version"`
	} `json:"casks"`
}

func (e *Exec) fillOutdatedFormulas(ctx context.Context, state *State) error {
	out, err := e.runQuiet(ctx, "outdated", "--formula", "--json=v2")
	if err != nil {
		return fmt.Errorf("brew outdated --formula: %w", err)
	}
	var data outdatedJSON
	if err := json.Unmarshal([]byte(out), &data); err != nil {
		return fmt.Errorf("parse outdated formula json: %w", err)
	}
	for _, f := range data.Formulae {
		fs := state.Formulas[f.Name]
		fs.Outdated = true
		fs.OutdatedTo = f.CurrentVersion
		if len(f.InstalledVersions) > 0 && fs.Version == "" {
			fs.Version = f.InstalledVersions[len(f.InstalledVersions)-1]
		}
		state.Formulas[f.Name] = fs
	}
	return nil
}

func (e *Exec) fillOutdatedCasks(ctx context.Context, state *State) error {
	// --greedy is the detection-side companion to CaskUpgrade's --greedy:
	// it opts into casks that brew would otherwise ignore for outdated
	// checks — those with `auto_updates true` or `version :latest` in
	// the cask DSL. Without it those casks would silently never upgrade.
	out, err := e.runQuiet(ctx, "outdated", "--cask", "--greedy", "--json=v2")
	if err != nil {
		return fmt.Errorf("brew outdated --cask: %w", err)
	}
	var data outdatedJSON
	if err := json.Unmarshal([]byte(out), &data); err != nil {
		return fmt.Errorf("parse outdated cask json: %w", err)
	}
	for _, c := range data.Casks {
		cs := state.Casks[c.Name]
		cs.Outdated = true
		cs.OutdatedTo = c.CurrentVersion
		if cs.Version == "" && len(c.InstalledVersions) > 0 {
			cs.Version = c.InstalledVersions[len(c.InstalledVersions)-1]
		}
		state.Casks[c.Name] = cs
	}
	return nil
}

// Tap registers a tap. brew tap is idempotent at the brew layer.
func (e *Exec) Tap(ctx context.Context, name, url string) (Result, error) {
	args := []string{"tap", name}
	if url != "" {
		args = append(args, url)
	}
	out, err := e.run(ctx, nil, args...)
	return Result{Output: out, To: name}, err
}

// BrewInstall installs a formula via `brew install --formula`.
func (e *Exec) BrewInstall(ctx context.Context, name string) (Result, error) {
	out, err := e.run(ctx, nil, "install", "--formula", name)
	return Result{Output: out, To: name}, err
}

// BrewUpgrade upgrades a formula via `brew upgrade --formula`.
func (e *Exec) BrewUpgrade(ctx context.Context, name string) (Result, error) {
	out, err := e.run(ctx, nil, "upgrade", "--formula", name)
	return Result{Output: out, To: name}, err
}

// headEnv keeps HEAD operations from triggering Homebrew's auto-update
// (slow, surprising, mutates global state) and from hitting the GitHub
// API rate limit during HEAD source resolution.
var headEnv = []string{
	"HOMEBREW_NO_GITHUB_API=1",
	"HOMEBREW_NO_AUTO_UPDATE=1",
}

// HeadInstall installs a formula from its HEAD source.
func (e *Exec) HeadInstall(ctx context.Context, name string) (Result, error) {
	out, err := e.run(ctx, headEnv, "install", "--head", "--formula", name)
	return Result{Output: out, To: name}, err
}

// HeadReinstall is deliberately uninstall-then-install rather than
// `brew reinstall --head` because `brew reinstall` does not always pick
// up a moved HEAD source SHA — it keeps the cached keg when the version
// string matches. The explicit uninstall forces a fresh source tree.
//
// If the uninstall succeeds and the install fails, the caller is left
// with the formula MISSING from the system entirely; surface that loud
// and clear so the user knows to re-run.
func (e *Exec) HeadReinstall(ctx context.Context, name string) (Result, error) {
	uninstallOut, err := e.run(ctx, headEnv, "uninstall", "--formula", name)
	if err != nil {
		return Result{Output: uninstallOut}, fmt.Errorf("brew uninstall %s: %w", name, err)
	}
	installOut, err := e.run(ctx, headEnv, "install", "--head", "--formula", name)
	if err != nil {
		return Result{Output: uninstallOut + installOut},
			fmt.Errorf("brew install --head %s after successful uninstall (formula is now MISSING, re-run to recover): %w", name, err)
	}
	return Result{Output: uninstallOut + installOut, To: name}, nil
}

// brewInfoFormula is the subset of `brew info --json=v2` we consume.
type brewInfoFormula struct {
	LinkedKeg string `json:"linked_keg"`
	Installed []struct {
		Version string `json:"version"`
		Time    int64  `json:"time"`
	} `json:"installed"`
	URLs struct {
		Head struct {
			URL    string `json:"url"`
			Branch string `json:"branch"`
		} `json:"head"`
	} `json:"urls"`
}

// brewInfoFirst runs `brew info --json=v2 --formula <name>` and returns
// the first formula entry from the parsed output.
func (e *Exec) brewInfoFirst(ctx context.Context, name string) (brewInfoFormula, error) {
	out, err := e.runQuiet(ctx, "info", "--json=v2", "--formula", name)
	if err != nil {
		return brewInfoFormula{}, fmt.Errorf("brew info: %w", err)
	}
	var wrapper struct {
		Formulae []brewInfoFormula `json:"formulae"`
	}
	if err := json.Unmarshal([]byte(out), &wrapper); err != nil {
		return brewInfoFormula{}, fmt.Errorf("parse brew info json: %w", err)
	}
	if len(wrapper.Formulae) == 0 {
		return brewInfoFormula{}, errors.New("brew info returned no formulae")
	}
	return wrapper.Formulae[0], nil
}

// HeadInstalledSHA reads the installed HEAD SHA from `brew info`,
// preferring the linked keg and falling back to the newest HEAD keg.
func (e *Exec) HeadInstalledSHA(ctx context.Context, name string) (string, bool, bool, error) {
	f, err := e.brewInfoFirst(ctx, name)
	if err != nil {
		return "", false, false, err
	}
	if f.LinkedKeg != "" {
		if strings.HasPrefix(f.LinkedKeg, "HEAD-") {
			return strings.TrimPrefix(f.LinkedKeg, "HEAD-"), true, true, nil
		}
		return "", false, true, nil
	}
	if len(f.Installed) == 0 {
		return "", false, false, nil
	}
	var latest string
	var latestTime int64
	for _, inst := range f.Installed {
		if !strings.HasPrefix(inst.Version, "HEAD-") {
			continue
		}
		if latest == "" || inst.Time >= latestTime {
			latest = inst.Version
			latestTime = inst.Time
		}
	}
	if latest != "" {
		return strings.TrimPrefix(latest, "HEAD-"), true, true, nil
	}
	return "", false, true, nil
}

// HeadLatestSHA resolves the newest upstream HEAD commit by refreshing
// Homebrew's HEAD source cache and reading the cached git repo.
func (e *Exec) HeadLatestSHA(ctx context.Context, name string) (string, bool, error) {
	f, err := e.brewInfoFirst(ctx, name)
	if err != nil {
		return "", false, err
	}
	if f.URLs.Head.URL == "" {
		return "", false, nil
	}

	// Refresh the HEAD source cache so the local repo has the latest commits.
	if _, err := e.run(ctx, headEnv, "fetch", "--HEAD", "--formula", name); err != nil {
		return "", true, fmt.Errorf("brew fetch --HEAD: %w", err)
	}

	cacheRepo, err := e.runQuiet(ctx, "--cache", "--HEAD", name)
	if err != nil {
		return "", true, fmt.Errorf("brew --cache --HEAD: %w", err)
	}
	repo := strings.TrimSpace(cacheRepo)
	if repo == "" {
		return "", true, errors.New("brew --cache --HEAD returned empty path")
	}
	// Fallback ladder for finding the latest HEAD SHA in the cache repo:
	//   1. `origin/<branch>` where <branch> is from `brew info .urls.head.branch`
	//   2. resolve refs/remotes/origin/HEAD via `git symbolic-ref` to
	//      find the remote default branch name (some formulas leave
	//      `.branch` unset and rely on whatever the remote defaults to)
	//   3. the literal ref `origin/HEAD` (last-resort, for cache repos
	//      where the symbolic-ref is also unset)
	// Every tap/formula combo in practice hits one of these.
	branch := f.URLs.Head.Branch
	if branch != "" {
		if sha := gitShortSHA(ctx, repo, "origin/"+branch+"^{commit}"); sha != "" {
			return sha, true, nil
		}
	}
	if remoteHead := gitSymbolicRef(ctx, repo, "refs/remotes/origin/HEAD"); remoteHead != "" {
		ref := strings.TrimPrefix(remoteHead, "origin/")
		if sha := gitShortSHA(ctx, repo, "origin/"+ref+"^{commit}"); sha != "" {
			return sha, true, nil
		}
	}
	if sha := gitShortSHA(ctx, repo, "origin/HEAD^{commit}"); sha != "" {
		return sha, true, nil
	}
	return "", true, errors.New("could not resolve latest HEAD commit")
}

func gitShortSHA(ctx context.Context, repo, ref string) string {
	cmd := exec.CommandContext(ctx, "git", "-C", repo, "rev-parse", "--short=7", ref)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	if err := cmd.Run(); err != nil {
		return ""
	}
	return strings.TrimSpace(buf.String())
}

func gitSymbolicRef(ctx context.Context, repo, ref string) string {
	cmd := exec.CommandContext(ctx, "git", "-C", repo, "symbolic-ref", "--short", ref)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	if err := cmd.Run(); err != nil {
		return ""
	}
	return strings.TrimSpace(buf.String())
}

// CaskInstall installs a cask via `brew install --cask`.
func (e *Exec) CaskInstall(ctx context.Context, name string) (Result, error) {
	out, err := e.run(ctx, nil, "install", "--cask", name)
	return Result{Output: out, To: name}, err
}

// CaskUpgrade upgrades a cask via `brew upgrade --cask --greedy`.
func (e *Exec) CaskUpgrade(ctx context.Context, name string) (Result, error) {
	out, err := e.run(ctx, nil, "upgrade", "--cask", "--greedy", name)
	return Result{Output: out, To: name}, err
}
