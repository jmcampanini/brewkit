package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/jmcampanini/brewkit/internal/docs"
)

func runDocs(_ context.Context) error {
	_, err := fmt.Fprint(os.Stdout, docs.Manual())
	return err
}
