package conflict

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/docket/internal/tui/route"
)

// popupRowOf is the popup row the browser's last render drew text on.
func popupRowOf(t *testing.T, rendered, text string) int {
	t.Helper()
	for row, line := range strings.Split(ansi.Strip(rendered), "\n") {
		if strings.Contains(line, text) {
			return row
		}
	}
	t.Fatalf("browser does not draw %q:\n%s", text, ansi.Strip(rendered))
	return -1
}

// TestConflictBrowserPointerSeatsAndInspects reads a click against the popup's
// own cells: it seats the cursor on the conflict drawn on that row, a
// double-click there presses Enter, and the inspected detail answers only the
// wheel.
func TestConflictBrowserPointerSeatsAndInspects(t *testing.T) {
	browser := newTestConflictBrowser(t,
		conflictBrowserFixture("aaaa1111-first", false),
		conflictBrowserFixture("bbbb2222-second", false),
	)
	rendered := browser.RenderWithin(100, 30)
	row := popupRowOf(t, rendered, "bbbb2222")

	if cmd := browser.HandlePointer(route.PointerEvent{X: 6, Y: row, Button: route.PointerButtonLeft, Clicks: 1}); cmd != nil {
		t.Fatal("a single click pressed a key")
	}
	if item, ok := browser.surface.CurrentItem(); !ok || item.ID != conflictBrowserItemID("bbbb2222-second") {
		t.Fatalf("cursor = %+v, want the second conflict", item)
	}
	if cmd := browser.HandlePointer(route.PointerEvent{X: 6, Y: popupRowOf(t, rendered, "Merge Conflicts"), Button: route.PointerButtonLeft, Clicks: 2}); cmd != nil {
		t.Fatal("a double-click on the title pressed a key")
	}

	cmd := browser.HandlePointer(route.PointerEvent{X: 6, Y: row, Button: route.PointerButtonLeft, Clicks: 2})
	if press, ok := cmd().(tea.KeyPressMsg); !ok || press.String() != "enter" {
		t.Fatalf("a double-click pressed %v, want Enter", press)
	}
	if _, _, msg := browser.HandleKey(tea.KeyPressMsg{Code: tea.KeyEnter}); msg != nil {
		t.Fatalf("inspecting raised %T", msg)
	}
	browser.RenderWithin(100, 30)
	if !browser.inspecting() {
		t.Fatal("Enter did not inspect the seated conflict")
	}
	if cmd := browser.HandlePointer(route.PointerEvent{X: 6, Y: row, Button: route.PointerButtonLeft, Clicks: 2}); cmd != nil {
		t.Fatal("a click on the detail pane pressed a key")
	}
	if press, ok := browser.HandlePointer(route.PointerEvent{X: 6, Y: row, WheelDelta: 1})().(tea.KeyPressMsg); !ok || press.String() != "down" {
		t.Fatal("the wheel over the detail pane did not step it")
	}
}
