package outliner

import (
	"slices"
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
)

// TestDefaultOrderIsPriorityThenManual holds the default tree order and the J/K
// window it implies: priority leads, stored order breaks ties, and a reorder
// only trades places inside one priority band, since a swap across bands would
// rewrite stored order without moving a row.
func TestDefaultOrderIsPriorityThenManual(t *testing.T) {
	first := primaryRowsTask("first-high", models.StateBacklog)
	first.Priority = models.PriorityHigh
	low := primaryRowsTask("low", models.StateBacklog)
	low.Priority = models.PriorityLow
	second := primaryRowsTask("second-high", models.StateBacklog)
	second.Priority = models.PriorityHigh
	unset := primaryRowsTask("unset", models.StateBacklog)
	project := primaryRowsProject("alpha", first, low, second, unset)

	outliner, _ := primaryRowsOutliner(project)
	outliner.SelectOutlinerMode(chrome.ModeAll)
	if got, want := visibleTaskIDs(outliner), []string{first.ID, second.ID, low.ID, unset.ID}; !slices.Equal(got, want) {
		t.Fatalf("rows = %v, want %v", got, want)
	}

	outliner.MoveCursorToTask(second.ID)
	siblings := outliner.visibleSiblings(outliner.Cursor)
	ids := make([]string, 0, len(siblings))
	for _, task := range siblings {
		ids = append(ids, task.ID)
	}
	if want := []string{first.ID, second.ID}; !slices.Equal(ids, want) {
		t.Fatalf("reorder window = %v, want the priority-1 band %v", ids, want)
	}

	outliner.ReorderTask(1)
	if got, want := visibleTaskIDs(outliner), []string{first.ID, second.ID, low.ID, unset.ID}; !slices.Equal(got, want) {
		t.Fatalf("J across a band boundary moved rows to %v, want %v", got, want)
	}
	outliner.ReorderTask(-1)
	if got, want := visibleTaskIDs(outliner), []string{second.ID, first.ID, low.ID, unset.ID}; !slices.Equal(got, want) {
		t.Fatalf("K inside the band gave rows %v, want %v", got, want)
	}
}
