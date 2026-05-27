package cli

import (
	"context"
	"fmt"

	"github.com/jmcampanini/brewkit/internal/brew"
	"github.com/jmcampanini/brewkit/internal/ui"
)

// progressBrewer decorates Homebrew operations with high-level transient
// progress messages. The wrapped ui.Printer decides whether a spinner should
// actually be rendered for the current mode/terminal.
type progressBrewer struct {
	next    brew.Brewer
	printer *ui.Printer
}

func newProgressBrewer(next brew.Brewer, printer *ui.Printer) brew.Brewer {
	return &progressBrewer{next: next, printer: printer}
}

func (b *progressBrewer) withSpinner(message string, fn func() error) error {
	if b.printer == nil {
		return fn()
	}
	return b.printer.WithSpinner(message, fn)
}

func (b *progressBrewer) withResult(message string, fn func() (brew.Result, error)) (brew.Result, error) {
	var res brew.Result
	err := b.withSpinner(message, func() error {
		var err error
		res, err = fn()
		return err
	})
	return res, err
}

func (b *progressBrewer) State(ctx context.Context) (*brew.State, error) {
	var state *brew.State
	err := b.withSpinner("Checking Homebrew state…", func() error {
		var err error
		state, err = b.next.State(ctx)
		return err
	})
	return state, err
}

func (b *progressBrewer) Tap(ctx context.Context, name, url string) (brew.Result, error) {
	return b.withResult(fmt.Sprintf("Tapping %s…", name), func() (brew.Result, error) {
		return b.next.Tap(ctx, name, url)
	})
}

func (b *progressBrewer) BrewInstall(ctx context.Context, name string) (brew.Result, error) {
	return b.withResult(fmt.Sprintf("Installing %s…", name), func() (brew.Result, error) {
		return b.next.BrewInstall(ctx, name)
	})
}

func (b *progressBrewer) BrewUpgrade(ctx context.Context, name string) (brew.Result, error) {
	return b.withResult(fmt.Sprintf("Upgrading %s…", name), func() (brew.Result, error) {
		return b.next.BrewUpgrade(ctx, name)
	})
}

func (b *progressBrewer) HeadInstall(ctx context.Context, name string) (brew.Result, error) {
	return b.withResult(fmt.Sprintf("Installing HEAD %s…", name), func() (brew.Result, error) {
		return b.next.HeadInstall(ctx, name)
	})
}

func (b *progressBrewer) HeadReinstall(ctx context.Context, name string) (brew.Result, error) {
	return b.withResult(fmt.Sprintf("Reinstalling HEAD %s…", name), func() (brew.Result, error) {
		return b.next.HeadReinstall(ctx, name)
	})
}

func (b *progressBrewer) HeadInstalledSHA(ctx context.Context, name string) (string, bool, bool, error) {
	var sha string
	var installedAsHead bool
	var installed bool
	err := b.withSpinner(fmt.Sprintf("Checking HEAD %s…", name), func() error {
		var err error
		sha, installedAsHead, installed, err = b.next.HeadInstalledSHA(ctx, name)
		return err
	})
	return sha, installedAsHead, installed, err
}

func (b *progressBrewer) HeadLatestSHA(ctx context.Context, name string) (string, bool, error) {
	var sha string
	var hasHead bool
	err := b.withSpinner(fmt.Sprintf("Checking latest HEAD %s…", name), func() error {
		var err error
		sha, hasHead, err = b.next.HeadLatestSHA(ctx, name)
		return err
	})
	return sha, hasHead, err
}

func (b *progressBrewer) CaskInstall(ctx context.Context, name string) (brew.Result, error) {
	return b.withResult(fmt.Sprintf("Installing %s…", name), func() (brew.Result, error) {
		return b.next.CaskInstall(ctx, name)
	})
}

func (b *progressBrewer) CaskUpgrade(ctx context.Context, name string) (brew.Result, error) {
	return b.withResult(fmt.Sprintf("Upgrading %s…", name), func() (brew.Result, error) {
		return b.next.CaskUpgrade(ctx, name)
	})
}

var _ brew.Brewer = (*progressBrewer)(nil)
