package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

// TestFormFlagsPicker_TogglesPinnedAndDimmed verifies the surviving two-flag
// picker (Pinned/Dimmed). The retired Active flag is gone, so there is no
// longer any mutual-exclusion behavior to enforce.
func TestFormFlagsPicker_TogglesPinnedAndDimmed(t *testing.T) {
	t.Run("cursor 0 toggles pinned", func(t *testing.T) {
		p := newFormFlagsPicker(false, false)
		p.cursor = 0
		p.handleKey(tea.KeyPressMsg{Code: ' '}) //nolint:errcheck
		if !p.pinned {
			t.Error("pinned should be true after toggle")
		}
		if p.dimmed {
			t.Error("dimmed should remain false")
		}
	})

	t.Run("cursor 1 toggles dimmed", func(t *testing.T) {
		p := newFormFlagsPicker(false, false)
		p.cursor = 1
		p.handleKey(tea.KeyPressMsg{Code: ' '}) //nolint:errcheck
		if !p.dimmed {
			t.Error("dimmed should be true after toggle")
		}
		if p.pinned {
			t.Error("pinned should remain false")
		}
	})

	t.Run("cursor does not advance past last row", func(t *testing.T) {
		p := newFormFlagsPicker(false, false)
		p.cursor = 1
		p.handleKey(tea.KeyPressMsg{Code: 'j'}) //nolint:errcheck
		if p.cursor != 1 {
			t.Errorf("cursor = %d, want clamped at 1", p.cursor)
		}
	})
}
