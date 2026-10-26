package outliner

import (
	"testing"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/chrome"
)

// The empty-state sentence and the footer's Esc entry are one answer read
// twice. A Focus taken inside a zoomed section is the step the cascade leaves
// first, so the sentence may not tell the user Esc unzooms; and an open Peek
// over the empty view is closed before either, so neither sentence may name a
// layer the press does not reach.
func TestEmptySentenceNamesTheStepTheEscCascadeRuns(t *testing.T) {
	parent := primaryRowsTask("parent", models.StateBacklog)
	child := primaryRowsTask("child", models.StateBacklog)
	parentID := parent.ID
	child.ParentID = &parentID
	project := primaryRowsProject("alpha", parent, child)
	outliner, _ := primaryRowsOutliner(project)
	outliner.SelectOutlinerMode(chrome.ModeAll)

	zoomIntoSection(t, outliner, models.StateGroupBacklog.Label())
	if got := outliner.OutlinerEmptyMessage(); got != "" && got != "No tasks are left in this section. Press Esc to unzoom." {
		t.Fatalf("zoom-only message = %q", got)
	}
	if got := outliner.emptyEscSentence(escActionUnzoom); got != "Press Esc to unzoom." {
		t.Fatalf("zoom-only Esc sentence = %q, want the unzoom act", got)
	}

	cursor := -1
	for index, row := range outliner.Rows {
		if row.IsTaskRow() && row.Task.ID == parent.ID {
			cursor = index
		}
	}
	if cursor < 0 {
		t.Fatal("the zoomed projection lost the parent row")
	}
	outliner.Cursor = cursor
	if !outliner.FocusSubtreeAtCursor() {
		t.Fatal("the parent refused Focus")
	}
	if !outliner.Focus.Zoom.Active() || !outliner.Focus.Active() {
		t.Fatal("the fixture needs a Focus standing inside the zoom")
	}

	if got := outliner.ClassifyEmptyCause(); got != outlinerEmptyFocus {
		t.Fatalf("empty cause = %d, want the Focus cause the cascade unwinds first", got)
	}
	const wantFocus = "No tasks are visible in this Focus. Press Esc to leave Focus."
	if got := outliner.OutlinerEmptyMessage(); got != wantFocus {
		t.Fatalf("focus-in-zoom message = %q, want %q", got, wantFocus)
	}
	if hint := outliner.EscHint(); hint != "Esc leave Focus" {
		t.Fatalf("EscHint = %q, want the footer to agree with the sentence", hint)
	}

	outliner.PeekMode = true
	const wantPeek = "No tasks are visible in this Focus. Press Esc to close Peek."
	if got := outliner.OutlinerEmptyMessage(); got != wantPeek {
		t.Fatalf("message under Peek = %q, want %q", got, wantPeek)
	}
}
