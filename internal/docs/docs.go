// Package docs exposes the embedded brewkit manual as plain text.
package docs

import _ "embed"

//go:embed manual.txt
var manual string

// Manual returns the embedded manual text.
func Manual() string { return manual }
