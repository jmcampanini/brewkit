package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/jmcampanini/brewkit/internal/profile"
)

func runConfig(_ context.Context) error {
	cfg, profiles, err := loadEffectiveConfig()
	if err != nil {
		return err
	}

	// Strip the reserved local profile from the printed list — it is
	// auto-appended at runtime when *file.local exists, and brewkit
	// rejects an explicit "local" entry. Surfacing the fact via a
	// trailing TOML comment keeps the output round-trippable.
	visible := make([]string, 0, len(profiles))
	localActive := false
	for _, p := range profiles {
		if p == profile.LocalName {
			localActive = true
			continue
		}
		visible = append(visible, p)
	}
	cfg.Profiles = visible

	out, err := cfg.MarshalTOML()
	if err != nil {
		return err
	}
	if _, err := fmt.Fprint(os.Stdout, string(out)); err != nil {
		return err
	}
	if localActive {
		if _, err := fmt.Fprintln(os.Stdout, "# note: 'local' profile auto-appended (*file.local present)"); err != nil {
			return err
		}
	}
	return nil
}
