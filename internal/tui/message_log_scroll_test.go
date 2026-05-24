package tui

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

// msgLogKey builds a tea.KeyPressMsg whose String() method returns the given
// keystroke. Supports plain runes (e.g. "j", "G") and the "ctrl+<rune>" form.
func msgLogKey(s string) tea.KeyPressMsg {
	if len(s) > 5 && s[:5] == "ctrl+" {
		r := rune(s[5])
		return tea.KeyPressMsg{Code: r, Mod: tea.ModCtrl}
	}
	r := rune(s[0])
	return tea.KeyPressMsg{Code: r, Text: s}
}

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

// TestMessageLogScrollBottomKeepsViewportFilled verifies that scroll-to-bottom
// (G), page-down (ctrl+d), and j keep the window-top offset at most
// len(entries) - visible, so the viewport stays filled with trailing entries
// instead of scrolling them out of view. Regression: clamping to len-1 left
// only the final entry visible because that is a cursor-index bound, not a
// window-top bound.
func TestMessageLogScrollBottomKeepsViewportFilled(t *testing.T) {
	entries := make([]MessageLogEntry, 20)
	for i := range entries {
		entries[i] = MessageLogEntry{
			Timestamp: time.Now(),
			Message:   "entry",
			Level:     MessageLogLevelInfo,
		}
	}

	// visibleLines() = max(15-10, 1) = 5; max valid scroll = 20 - 5 = 15.
	const wantMax = 15

	cases := []struct {
		name string
		key  string
	}{
		{"G", "G"},
		{"ctrl+d", "ctrl+d"},
		{"j", "j"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := NewMessageLogViewModel(entries)
			m.SetSize(80, 15)
			// Hammer the key enough times to saturate any per-keypress clamp.
			for range 30 {
				m.Update(msgLogKey(tc.key))
			}
			if m.scroll > wantMax {
				t.Errorf("%s: scroll = %d, want <= %d (entries - visible)", tc.name, m.scroll, wantMax)
			}
		})
	}
}
