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
