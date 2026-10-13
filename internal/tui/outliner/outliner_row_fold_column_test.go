package outliner

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
)

func foldColumnRows(t *testing.T) (*OutlinerModel, []OutlinerRow) {
	t.Helper()
	parent := renderFixtureTask(t, "parent", 1, "Parent work", models.StateBacklog, 0, models.StateBacklog)
	leaf := renderFixtureTask(t, "leaf", 2, "Leaf work", models.StateBacklog, 0, models.StateBacklog)
	project := renderFixtureProject(t, "fold-project", "fold", "Fold", []*models.Task{parent, leaf})
	outliner := NewOutlinerModel(project, nil, newTestConfig())
	return outliner, []OutlinerRow{
		{Task: parent, ProjectID: project.ID, HasVisibleChildren: true},
		{Task: leaf, ProjectID: project.ID},
	}
}

// TestFoldColumnIsReservedAcrossAProjectionHoldingAParent is the alignment the
// fold column exists for: one row that folds makes every task row keep the
// column, so the state glyph and the title start where the parent's do.
func TestFoldColumnIsReservedAcrossAProjectionHoldingAParent(t *testing.T) {
	outliner, rows := foldColumnRows(t)
	layout := outliner.ComputeOutlinerRowLayout(rows, 80)
	if layout == nil || !layout.HasFold {
		t.Fatalf("a projection holding a parent measured no fold column: %+v", layout)
	}
	parent := ansi.Strip(outliner.RenderOutlinerTaskRow(rows[0], false, 80, 0, layout))
	leaf := ansi.Strip(outliner.RenderOutlinerTaskRow(rows[1], false, 80, 1, layout))
	column := func(line, title string) int { return lipgloss.Width(line[:strings.Index(line, title)]) }
	if got, want := column(leaf, "Leaf work"), column(parent, "Parent work"); got != want {
		t.Fatalf("titles start in columns %d and %d:\n%q\n%q", want, got, parent, leaf)
	}

	parentSegments := outliner.buildTaskRowSegments(rows[0], 80, layout, cursorPaint{})
	leafSegments := outliner.buildTaskRowSegments(rows[1], 80, layout, cursorPaint{})
	if got, want := leafSegments.titleWidth(80), parentSegments.titleWidth(80); got != want {
		t.Fatalf("the inline editor's title budget differs: leaf %d, parent %d", got, want)
	}
}

// TestFoldColumnIsNotReservedWithoutAFoldOrALayout keeps the two columns the
// title's wherever nothing folds, and below the alignment floor.
func TestFoldColumnIsNotReservedWithoutAFoldOrALayout(t *testing.T) {
	outliner, rows := foldColumnRows(t)
	flat := []OutlinerRow{rows[1]}
	layout := outliner.ComputeOutlinerRowLayout(flat, 80)
	if layout == nil || layout.HasFold {
		t.Fatalf("a flat projection measured a fold column: %+v", layout)
	}
	if got := outliner.renderTaskFoldIndicator(rows[1], layout, nil); got != "" {
		t.Fatalf("a flat projection reserved %q", got)
	}
	if got := outliner.renderTaskFoldIndicator(rows[1], nil, nil); got != "" {
		t.Fatalf("a nil layout reserved %q", got)
	}
	if got := outliner.ComputeOutlinerRowLayout(rows, chrome.MinColumnAlignWidth-1); got != nil {
		t.Fatalf("a row under the alignment floor measured a layout: %+v", got)
	}
	if got := ansi.Strip(outliner.renderTaskFoldIndicator(rows[1], &chrome.ColumnLayout{HasFold: true}, nil)); got != "  " {
		t.Fatalf("a reserved fold column = %q, want two blanks", got)
	}
}

// TestRankedProjectLabelCapFollowsTheRow pins the one cap: a fifth of the row,
// held between twelve and twenty-four columns.
func TestRankedProjectLabelCapFollowsTheRow(t *testing.T) {
	for width, want := range map[int]int{40: 12, 60: 12, 80: 16, 100: 20, 120: 24, 200: 24} {
		if got := rankedProjectLabelMaxSlug(width); got != want {
			t.Errorf("rankedProjectLabelMaxSlug(%d) = %d, want %d", width, got, want)
		}
	}
}
