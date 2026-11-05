package outliner

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

// Editing the query under the ranked list seats the cursor on the first
// result: the list is ordered by relevance, so a new query asks for its best
// answer rather than for whatever row the old cursor index lands on.
func TestRankedSearchQueryEditSeatsTheTopResult(t *testing.T) {
	outliner := matchCountOutliner()
	outliner.SetSize(120, 40)
	_ = outliner.ActivateSearch()
	outliner.HandleKeyMsg(tea.KeyPressMsg{Code: 'e', Text: "e"})
	if !outliner.ShowsRankedList() || len(outliner.Rows) < 3 {
		t.Fatalf("typing opened %d ranked rows, want at least 3", len(outliner.Rows))
	}
	outliner.Cursor = 2
	outliner.HandleKeyMsg(tea.KeyPressMsg{Code: 'e', Text: "e"})
	if outliner.Cursor != 0 {
		t.Fatalf("cursor = %d after a query edit, want the top result", outliner.Cursor)
	}
	outliner.Cursor = 1
	outliner.HandleKeyMsg(tea.KeyPressMsg{Code: tea.KeyBackspace})
	if outliner.Cursor != 0 {
		t.Fatalf("cursor = %d after deleting a character, want the top result", outliner.Cursor)
	}
}
