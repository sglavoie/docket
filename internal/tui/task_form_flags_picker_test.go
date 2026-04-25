package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestFormFlagsPicker_ActiveDimmedMutualExclusion(t *testing.T) {
	t.Run("setting active clears dimmed", func(t *testing.T) {
		p := newFormFlagsPicker(false, false, true)
		if !p.dimmed {
			t.Fatal("precondition: dimmed should be true")
		}
		p.cursor = 1
		p.handleKey(tea.KeyPressMsg{Code: ' '}) //nolint:errcheck
		if !p.active {
			t.Error("active should be true after toggle")
		}
		if p.dimmed {
			t.Error("dimmed should be cleared when active is set")
		}
	})

	t.Run("setting dimmed clears active", func(t *testing.T) {
		p := newFormFlagsPicker(false, true, false)
		if !p.active {
			t.Fatal("precondition: active should be true")
		}
		p.cursor = 2
		p.handleKey(tea.KeyPressMsg{Code: ' '}) //nolint:errcheck
		if !p.dimmed {
			t.Error("dimmed should be true after toggle")
		}
		if p.active {
			t.Error("active should be cleared when dimmed is set")
		}
	})

	t.Run("unsetting active does not affect dimmed", func(t *testing.T) {
		p := newFormFlagsPicker(false, true, false)
		p.cursor = 1
		p.handleKey(tea.KeyPressMsg{Code: ' '}) //nolint:errcheck
		if p.active {
			t.Error("active should be false after toggle off")
		}
		if p.dimmed {
			t.Error("dimmed should remain false")
		}
	})

	t.Run("unsetting dimmed does not affect active", func(t *testing.T) {
		p := newFormFlagsPicker(false, false, true)
		p.cursor = 2
		p.handleKey(tea.KeyPressMsg{Code: ' '}) //nolint:errcheck
		if p.dimmed {
			t.Error("dimmed should be false after toggle off")
		}
		if p.active {
			t.Error("active should remain false")
		}
	})
}
