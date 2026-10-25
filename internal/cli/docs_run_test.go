package cli

import (
	"context"
	"testing"
)

func TestRunDocs_PrintsManual(t *testing.T) {
	resetFlags()
	defer resetFlags()

	out := captureStdout(t, func() {
		if err := testApp.runDocs(context.Background()); err != nil {
			t.Errorf("runDocs err: %v", err)
		}
	})
	if len(out) == 0 {
		t.Error("expected non-empty docs output")
	}
}

func TestRunDocs_QuietStillPrintsManual(t *testing.T) {
	resetFlags()
	defer resetFlags()

	testApp.flags.quiet = true
	out := captureStdout(t, func() {
		if err := testApp.runDocs(context.Background()); err != nil {
			t.Errorf("runDocs err: %v", err)
		}
	})
	if len(out) == 0 {
		t.Error("quiet docs should still print manual output")
	}
}
