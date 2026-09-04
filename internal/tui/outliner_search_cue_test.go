package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/outliner"
)

// TestSearchCueStaysOutOfPeekAndProjectPreview keeps the annotation on the row
// that earned it: Peek restates fields in full, and the Hub's preview paints a
// tree no search ever attributed.
func TestSearchCueStaysOutOfPeekAndProjectPreview(t *testing.T) {
	task := cueTask()
	task.Description = "zeppelin hangar notes"
	outlinerView, project := cursorHighlightOutliner(t, task)
	attributed := outliner.OutlinerRow{Task: task, ProjectID: project.ID, MatchSource: outliner.MatchSourceDescription}
	bare := attributed
	bare.MatchSource = outliner.MatchSourceNone

	indent := "    "
	withCue := outlinerView.InlinePeekLinesForRow(attributed, indent, 120, nil)
	without := outlinerView.InlinePeekLinesForRow(bare, indent, 120, nil)
	if len(withCue) != len(without) {
		t.Fatalf("Peek changed length under attribution: %d vs %d", len(withCue), len(without))
	}
	for index := range withCue {
		if withCue[index] != without[index] {
			t.Fatalf("Peek line %d changed under attribution:\n%q\n%q", index, withCue[index], without[index])
		}
	}

	for _, previewRow := range projectPreviewTaskRows(project.ID, []*models.Task{task}) {
		if previewRow.MatchSource != outliner.MatchSourceNone {
			t.Fatalf("preview row carried source %d, want none", previewRow.MatchSource)
		}
		if plain := ansi.Strip(outlinerView.RenderOutlinerTaskRow(previewRow, false, 120, 120, 0, nil)); strings.Contains(plain, "·") {
			t.Fatalf("project preview row wore a cue: %q", plain)
		}
	}
}

// cueTask is a bland task: nothing in it renders a `·` of its own, so any middle
// dot in the row came from the match cue.
func cueTask() *models.Task {
	return &models.Task{
		ID:            "cue",
		ShortID:       1,
		Title:         "Untitled row",
		WorkflowState: models.StateBacklog,
	}
}
