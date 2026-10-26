package outliner

import (
	"fmt"
	"testing"

	"charm.land/lipgloss/v2"
)

// TestSearchSuggestionsAndPeekDockShareOneFrame holds the suggestion pane and an
// open Peek dock to one budget. The dock is frame-sized and decided first, so
// the pane is what yields: with a status message in the strip and a dozen tag
// suggestions open, the view still fits the frame the shell sized it to, with
// the strip it reserves for that message beside it.
func TestSearchSuggestionsAndPeekDockShareOneFrame(t *testing.T) {
	for _, height := range []int{18, 20, 22, 24} {
		t.Run(fmt.Sprintf("h%d", height), func(t *testing.T) {
			outliner := searchACWideOutliner(14)
			outliner.SetSize(80, height)
			outliner.StatusMessage = "Saved the earlier change"
			outliner.PeekMode = true

			typeSearchAC(outliner, "#")
			if !outliner.Search.Autocomplete.Active() {
				t.Fatal("no suggestion pane to budget")
			}
			content := outliner.RenderOutlinerTreeContent(80)
			if outliner.PeekDockRows() == 0 {
				t.Fatal("the fixture needs the Peek dock drawn")
			}
			strip := outliner.StatusStripHeight(80)
			if got := lipgloss.Height(content) + strip; got > height {
				t.Fatalf("view %d rows + strip %d = %d, want at most the %d-row frame",
					lipgloss.Height(content), strip, got, height)
			}
			if outliner.LastViewport.VisibleRows < peekDockListFloorRows {
				t.Fatalf("list kept %d rows, want at least the floor", outliner.LastViewport.VisibleRows)
			}
		})
	}
}
