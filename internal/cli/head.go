package cli

import (
	"github.com/jmcampanini/brewkit/internal/profile"
	"github.com/spf13/cobra"
)

func newHeadCmd() *cobra.Command {
	return newApplyCmd(
		"head [FORMULA]",
		"Apply Headfile entries for active profiles (SHA-idempotent)",
		profile.KindHead,
	)
}
