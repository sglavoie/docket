package outliner

import (
	"math"
	"slices"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/tui/help"
	"github.com/sglavoie/docket/internal/tui/keymap"
)

// The full-screen reader borrows the presentation without moving the outline's
// cursor or viewport. Closing restores even the compact dock's reading position.
type fullscreenPeekReturn struct {
	open bool
	peek peekOffsetState
}

func (m *OutlinerModel) ToggleFullscreenPeek() {
	if m == nil || m.MultiSelectMode {
		return
	}
	if m.PeekFullscreen {
		m.closeFullscreenPeek()
		return
	}
	if !m.PeekMode && !m.cursorRowSupportsPeek() {
		return
	}
	m.fullscreenReturn = fullscreenPeekReturn{open: m.PeekMode, peek: m.peek}
	m.PeekMode, m.PeekFullscreen = true, true
	m.peek = peekOffsetState{}
}

func (m *OutlinerModel) closeFullscreenPeek() {
	m.PeekFullscreen = false
	m.PeekMode, m.peek = m.fullscreenReturn.open, m.fullscreenReturn.peek
	m.fullscreenReturn = fullscreenPeekReturn{}
}

func (m *OutlinerModel) fullscreenPeekTitle() string {
	if m.Cursor >= 0 && m.Cursor < len(m.Rows) {
		row := m.Rows[m.Cursor]
		if row.IsProjectHeader {
			return "Project Details"
		}
		if row.IsChecklistItem {
			return "Checklist Item Details"
		}
	}
	return "Task Details"
}

// This is a gated reader keymap, like the expanded query readout. The registry
// still owns B's entry operation; inside the reader, keys scroll its document
// and never dispatch mutations or navigate an invisible list.
func (m *OutlinerModel) fullscreenPeekKeyMap() *keymap.KeyMap {
	return &keymap.KeyMap{Entries: []keymap.Entry{
		keymap.Bind("esc", "Return to list", "Navigation", keymap.WithFooter(1, "return to list", true), keymap.WithFooterKey("Esc/B"), keymap.WithHelpKey("Esc/B/b")),
		keymap.Bind("B", "Return to list", "Navigation", keymap.AsHidden()),
		keymap.Bind("b", "Return to list", "Navigation", keymap.AsHidden()),
		keymap.Bind("j", "Scroll down", "Navigation", keymap.WithFooter(2, "scroll", false), keymap.WithFooterKey("j/k"), keymap.WithHelpKey("j/k/↑/↓")),
		keymap.Bind("k", "Scroll up", "Navigation", keymap.AsHidden()),
		keymap.Bind("down", "Scroll down", "Navigation", keymap.AsHidden()),
		keymap.Bind("up", "Scroll up", "Navigation", keymap.AsHidden()),
		keymap.Bind("ctrl+d", "Scroll half a page down", "Navigation", keymap.WithHelpKey("Ctrl+D/Ctrl+U")),
		keymap.Bind("ctrl+u", "Scroll half a page up", "Navigation", keymap.AsHidden()),
		keymap.Bind("ctrl+f", "Scroll a page down", "Navigation", keymap.WithFooter(3, "page", false), keymap.WithFooterKey("PgUp/PgDn"), keymap.WithHelpKey("PgDn/PgUp/Ctrl+F/Ctrl+B")),
		keymap.Bind("ctrl+b", "Scroll a page up", "Navigation", keymap.AsHidden()),
		keymap.Bind("pgdown", "Scroll a page down", "Navigation", keymap.AsHidden()),
		keymap.Bind("pgup", "Scroll a page up", "Navigation", keymap.AsHidden()),
		keymap.Bind("home", "Jump to top", "Navigation", keymap.WithHelpKey("Home")),
		keymap.Bind("end", "Jump to bottom", "Navigation", keymap.WithHelpKey("End/G")),
		keymap.Bind("G", "Jump to bottom", "Navigation", keymap.AsHidden()),
		keymap.Bind("?", "Help", "System", keymap.WithFooter(4, "help", true)),
	}}
}

func (m *OutlinerModel) handleFullscreenPeekKey(key string) tea.Cmd {
	km := m.fullscreenPeekKeyMap()
	if !slices.Contains(km.HandledKeys(), key) {
		return nil
	}
	switch key {
	case "esc", "B", "b":
		m.closeFullscreenPeek()
	case "j", "down":
		m.scrollPeek(1)
	case "k", "up":
		m.scrollPeek(-1)
	case "ctrl+d":
		m.HalfPagePeek(1)
	case "ctrl+u":
		m.HalfPagePeek(-1)
	case "ctrl+f", "pgdown":
		m.PagePeek(1)
	case "ctrl+b", "pgup":
		m.PagePeek(-1)
	case "home":
		m.scrollPeek(-math.MaxInt32)
	case "end", "G":
		m.scrollPeek(math.MaxInt32)
	case "?":
		return help.HelpCmd(m.fullscreenPeekTitle(), help.HelpItemsFromKeyMap(km))
	}
	return nil
}
