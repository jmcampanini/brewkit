package cli

import (
	"context"
	"fmt"

	"github.com/jmcampanini/brewkit/internal/docs"
)

func runDocs(_ context.Context) error {
	_, err := fmt.Fprint(stdoutWriter(), docs.Manual())
	return err
}
