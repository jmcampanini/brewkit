// Package docs holds the embedded brewkit manual and the help fragments
// that command long descriptions share.
package docs

import _ "embed"

//go:embed manual.txt
var manual string

// Manual returns the embedded manual text.
func Manual() string { return manual }
