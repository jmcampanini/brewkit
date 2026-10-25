package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/jmcampanini/brewkit/internal/docs"
	"github.com/jmcampanini/brewkit/internal/ui"
)

func (app *commandContext) runDocs(_ context.Context) error {
	_, err := fmt.Fprint(ui.NewLinePrefixWriter(os.Stdout, app.flags.outputPrefix), docs.Manual())
	return err
}
