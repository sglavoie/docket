package help

import (
	"fmt"
	"strings"
	"testing"

	"github.com/sglavoie/docket/internal/tui/operation"
)

func TestHelpSessionRestoresEachSurfaceWithFreshItems(t *testing.T) {
	var session Session
	items := make([]operation.HelpItem, 60)
	for i := range items {
		items[i] = operation.HelpItem{Key: fmt.Sprintf("k%d", i), Description: "Entry", Category: "Navigation"}
	}
	m := session.Open(items, "Outliner", 90, 20)
	m.Update(helpKeyPress("right"))
	m.Update(helpKeyPress("/"))
	m.Update(helpKeyPress("Entry"))
	m.Update(helpKeyPress("enter"))
	m.Update(helpKeyPress("j"))
	m.Update(helpKeyPress("j"))
	wantOffset := m.viewport.YOffset()
	if wantOffset == 0 {
		t.Fatal("fixture did not scroll")
	}
	m.Update(helpKeyPress("g")) // An unfinished chord is not remembered.

	notes := session.Open([]operation.HelpItem{{Key: "e", Description: "Edit note", Category: "Notes"}}, "Notes", 90, 20)
	if notes.searchQuery != "" || notes.activeTab != 0 || notes.viewport.YOffset() != 0 {
		t.Fatal("Outliner state leaked into Notes")
	}
	notes.Update(helpKeyPress("right"))

	fresh := append([]operation.HelpItem{{Key: "f", Description: "Find", Category: "Filters"}}, items...)
	fresh[1].Description = "Entry refreshed"
	m = session.Open(fresh, "Outliner", 90, 20)
	if m.searchQuery != "Entry" || m.searchInput.Value() != "Entry" || m.categories[m.activeTab-1] != "Navigation" {
		t.Fatalf("lost query or category: query=%q tab=%d", m.searchQuery, m.activeTab)
	}
	if m.viewport.YOffset() != wantOffset || m.HasTextInput() || m.pendingG {
		t.Fatalf("incorrect restored navigation: offset=%d editing=%v chord=%v", m.viewport.YOffset(), m.HasTextInput(), m.pendingG)
	}
	if !strings.Contains(m.renderContent(), "refreshed") {
		t.Fatal("restored Help used a stale action catalog")
	}
	// Registering the viewer sizes it again; this must preserve the position.
	m.SetSize(90, 20)
	if m.viewport.YOffset() != wantOffset {
		t.Fatal("registering Help lost the restored scroll position")
	}
	notes = session.Open(nil, "Notes", 90, 20)
	if notes.activeTab != 0 || notes.viewport.YOffset() != 0 {
		t.Fatal("a removed category did not fall back to All")
	}
}

func TestHelpSessionClampsScrollAndStartsFreshNextSession(t *testing.T) {
	var session Session
	m := newScrollableHelpModel(t)
	session.current = m
	m.Update(helpKeyPress("G"))
	if m.viewport.YOffset() == 0 {
		t.Fatal("fixture did not scroll")
	}
	m = session.Open(m.items[:1], "Outliner", 60, 40)
	if m.viewport.YOffset() != 0 {
		t.Fatal("shorter content did not clamp the restored offset")
	}
	m.Update(helpKeyPress("/"))
	m.Update(helpKeyPress("missing"))
	m.Update(helpKeyPress("enter"))
	m = session.Open(m.items, "Outliner", 60, 20)
	if m.searchQuery != "missing" || len(m.filteredItems) != 0 {
		t.Fatal("a saved query with no matches was silently cleared")
	}
	m.Update(helpKeyPress("backspace"))
	if m.searchQuery != "" || m.activeTab != 0 || len(m.filteredItems) != 1 {
		t.Fatal("Backspace did not reset the restored reference")
	}
	m.Update(helpKeyPress("/"))
	m.Update(helpKeyPress("Entry"))
	m.Update(helpKeyPress("enter"))
	var next Session
	m = next.Open(m.items, "Outliner", 60, 20)
	if m.searchQuery != "" || m.activeTab != 0 || m.viewport.YOffset() != 0 {
		t.Fatal("new session inherited Help state")
	}
}
