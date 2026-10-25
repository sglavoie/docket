package help

import (
	"fmt"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/tui/operation"
)

// helpKeyPress spells the two key shapes these tests need: a named arrow and a
// literal rune the search input would otherwise swallow.
func helpKeyPress(key string) tea.KeyPressMsg {
	switch key {
	case "tab":
		return tea.KeyPressMsg{Code: tea.KeyTab}
	case "left":
		return tea.KeyPressMsg{Code: tea.KeyLeft}
	case "right":
		return tea.KeyPressMsg{Code: tea.KeyRight}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "end":
		return tea.KeyPressMsg{Code: tea.KeyEnd}
	case "backspace":
		return tea.KeyPressMsg{Code: tea.KeyBackspace}
	default:
		return tea.KeyPressMsg{Text: key, Code: rune(key[0])}
	}
}

func newTabbedHelpModel(t *testing.T) *HelpViewModel {
	t.Helper()
	m := NewHelpViewModel([]operation.HelpItem{
		{Key: "j", Description: "Move down", Category: "Navigate"},
		{Key: "wd", Description: "Complete", Category: "Status"},
	}, "Outliner")
	m.SetSize(90, 24)
	if len(m.categories) < 2 {
		t.Fatalf("categories = %d, want at least 2 so a cycle is observable", len(m.categories))
	}
	return m
}

func TestHelpTabsCycleOnUnmodifiedArrowsAndHL(t *testing.T) {
	forward := []string{"right", "l"}
	backward := []string{"left", "h"}

	for _, key := range forward {
		m := newTabbedHelpModel(t)
		m.Update(helpKeyPress(key))
		if m.activeTab != 1 {
			t.Errorf("%s: activeTab = %d, want 1", key, m.activeTab)
		}
	}

	for _, key := range backward {
		m := newTabbedHelpModel(t)
		last := len(m.categories) // len(categories)+1 tabs, so the last index
		m.Update(helpKeyPress(key))
		if m.activeTab != last {
			t.Errorf("%s: activeTab = %d, want %d", key, m.activeTab, last)
		}
	}
}

func TestHelpSearchModeKeepsHLLiteralAndCyclesOnShiftArrows(t *testing.T) {
	m := newTabbedHelpModel(t)
	m.searchMode = true
	m.searchInput.Focus()

	m.Update(helpKeyPress("l"))
	if m.activeTab != 0 {
		t.Errorf("activeTab = %d after typing l in search, want 0", m.activeTab)
	}
	if m.searchQuery != "l" {
		t.Errorf("searchQuery = %q after typing l in search, want %q", m.searchQuery, "l")
	}

	m.Update(tea.KeyPressMsg{Code: tea.KeyRight, Mod: tea.ModShift})
	if m.activeTab != 1 {
		t.Errorf("activeTab = %d after Shift+Right in search, want 1", m.activeTab)
	}
}

func newScrollableHelpModel(t *testing.T) *HelpViewModel {
	t.Helper()
	items := make([]operation.HelpItem, 0, 60)
	for i := range 60 {
		items = append(items, operation.HelpItem{Key: fmt.Sprintf("k%d", i), Description: "Entry", Category: "Navigate"})
	}
	m := NewHelpViewModel(items, "Outliner")
	m.SetSize(90, 20)
	return m
}

func TestHelpScrollKeysMoveOnce(t *testing.T) {
	m := newScrollableHelpModel(t)
	m.Update(helpKeyPress("j"))
	if got := m.viewport.YOffset(); got != 1 {
		t.Fatalf("YOffset after j = %d, want 1", got)
	}
	m.Update(helpKeyPress("j"))
	m.Update(helpKeyPress("k"))
	if got := m.viewport.YOffset(); got != 1 {
		t.Fatalf("YOffset after j k = %d, want 1", got)
	}
	// The viewport's own bindings are retired, so d does not scroll.
	m.Update(helpKeyPress("d"))
	if got := m.viewport.YOffset(); got != 1 {
		t.Fatalf("YOffset after d = %d, want 1", got)
	}
}

func TestHelpGoToTopIsTheGGChord(t *testing.T) {
	m := newScrollableHelpModel(t)
	m.Update(helpKeyPress("G"))
	bottom := m.viewport.YOffset()
	if bottom == 0 {
		t.Fatal("G did not scroll")
	}
	m.Update(helpKeyPress("g"))
	if got := m.viewport.YOffset(); got != bottom {
		t.Fatalf("a lone g moved to %d, want %d", got, bottom)
	}
	m.Update(helpKeyPress("g"))
	if got := m.viewport.YOffset(); got != 0 {
		t.Fatalf("gg left YOffset %d, want 0", got)
	}
}
