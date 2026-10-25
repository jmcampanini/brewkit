package apply

import (
	"testing"

	"github.com/jmcampanini/brewkit/internal/parse"
)

func TestEntryMatches_QualifiedShortName(t *testing.T) {
	e := &parse.Entry{Name: "user/tap/cmdk"}
	if !entryMatches(e, "cmdk") {
		t.Error("short name should match qualified entry")
	}
	if !entryMatches(e, "user/tap/cmdk") {
		t.Error("full name should match")
	}
	if entryMatches(e, "other") {
		t.Error("unrelated should not match")
	}
}
