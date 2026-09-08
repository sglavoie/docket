package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/external"
)

// withFakeClipboard swaps external.CopyText for the duration of the test,
// capturing the copied text (or forcing an error) so clipboard-dependent code
// paths are exercised without touching the real system clipboard.
func withFakeClipboard(t *testing.T, err error, captured *string) {
	t.Helper()
	orig := external.CopyText
	t.Cleanup(func() { external.CopyText = orig })
	external.CopyText = func(text string) error {
		if captured != nil {
			*captured = text
		}
		return err
	}
}

// captureClipboard points external.CopyText at the driver for the duration of a
// scenario and returns the restore. The seam is what lets a scenario press y at
// all: external.CopyText is the product's one clipboard write and it shells out
// to pbcopy/xclip, so without this a committed scenario would either fail on a
// machine with no clipboard tool or quietly overwrite the author's own
// clipboard. Installing it per run rather than per driver keeps every other
// caller — the interactive ApplyAction path included — on the real seam.
func (d *UITestDriver) captureClipboard() func() {
	orig := external.CopyText
	external.CopyText = func(text string) error {
		d.clipboard = text
		return nil
	}
	return func() { external.CopyText = orig }
}
