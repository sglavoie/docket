package tui

import (
	"testing"
	"time"
)

// TestMessageLogScrollShowsRecentAtBottomOnOpen verifies that opening the
// message log scrolls so the most recent entries are visible at the bottom
// of the viewport — not just the very last entry alone with all preceding
// history off-screen. Previously NewMessageLogViewModel set scroll to
// len(entries)-1, which only showed the final entry instead of filling the
// viewport with the trailing entries.
func TestMessageLogScrollShowsRecentAtBottomOnOpen(t *testing.T) {
	// Build 20 entries.
	entries := make([]MessageLogEntry, 20)
	for i := range entries {
		entries[i] = MessageLogEntry{
			Timestamp: time.Now(),
			Message:   "entry",
			Level:     MessageLogLevelInfo,
		}
	}

	m := NewMessageLogViewModel(entries)
	// Simulate the typical overlay size set by showView (height-2).
	// A height of 30 yields visibleLines() = max(30-10, 1) = 20 — exactly
	// enough for all entries, so scroll should snap to 0.
	m.SetSize(80, 30)
	if m.scroll != 0 {
		t.Errorf("with viewport >= entries, scroll should be 0, got %d", m.scroll)
	}

	// A smaller viewport that fits only 5 entries: scroll should land on
	// len(entries) - visible = 20 - 5 = 15, so the bottom 5 entries fill
	// the viewport (regression: previously scroll = len(entries)-1 = 19,
	// pushing all but the final entry off-screen).
	m2 := NewMessageLogViewModel(entries)
	m2.SetSize(80, 15) // visibleLines() = 5
	if got, want := m2.scroll, 15; got != want {
		t.Errorf("with viewport < entries, scroll should be %d (entries - visible), got %d", want, got)
	}
}

// TestMessageLogScrollEmpty verifies SetSize handles the empty-entries case
// without panicking and leaves scroll at 0.
func TestMessageLogScrollEmpty(t *testing.T) {
	m := NewMessageLogViewModel(nil)
	m.SetSize(80, 24)
	if m.scroll != 0 {
		t.Errorf("empty log should have scroll = 0, got %d", m.scroll)
	}
}
