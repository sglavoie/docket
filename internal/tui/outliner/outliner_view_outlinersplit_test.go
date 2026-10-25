package outliner

import (
	"fmt"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/taskref"
)

func TestBuildTreePrefix(t *testing.T) {
	tests := []struct {
		name           string
		ancestorIsLast []bool
		isLast         bool
		expected       string
	}{
		{"root non-last", nil, false, "├── "},
		{"root last", nil, true, "└── "},
		{"depth 1, parent not last, non-last", []bool{false}, false, "│   ├── "},
		{"depth 1, parent not last, last", []bool{false}, true, "│   └── "},
		{"depth 1, parent last, non-last", []bool{true}, false, "    ├── "},
		{"depth 1, parent last, last", []bool{true}, true, "    └── "},
		{"depth 3 mixed", []bool{false, true, false}, false, "│       │   ├── "},
		{"depth 3 mixed last", []bool{false, true, false}, true, "│       │   └── "},
		{"depth 5 all not last", []bool{false, false, false, false, false}, false, "│   │   │   │   │   ├── "},
		{"depth 5 all last", []bool{true, true, true, true, true}, true, "                    └── "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := BuildTreePrefix(tt.ancestorIsLast, tt.isLast); got != tt.expected {
				t.Errorf("buildTreePrefix(%v, %v)\ngot:  %q\nwant: %q", tt.ancestorIsLast, tt.isLast, got, tt.expected)
			}
		})
	}
}

func TestRenderTaskPrefixKeepsLeafConnectorsCompact(t *testing.T) {
	prefix, fold := renderTaskPrefix("├── ", "", nil, lipgloss.NewStyle())
	if got, want := ansi.Strip(prefix), "├── "; got != want {
		t.Fatalf("leaf connector = %q, want compact %q", got, want)
	}
	if fold != "" {
		t.Fatalf("leaf connector should not add a filler segment, got %q", ansi.Strip(fold))
	}
}

func TestOutlinerPersistentScrollKeepsFirstRowMovingUpWithinViewport(t *testing.T) {
	for _, tt := range []struct {
		name    string
		project *models.Project
	}{
		{name: "single project", project: &models.Project{Name: "Single"}},
		{name: "cross project", project: nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			rows := make([]OutlinerRow, 10)
			for i := range rows {
				rows[i] = OutlinerRow{Task: &models.Task{
					ID: fmt.Sprintf("task-%02d", i), ShortID: i + 1, Title: fmt.Sprintf("Task %02d", i),
				}}
			}
			m := &OutlinerModel{
				Project: tt.project, Rows: rows,
				Folds: foldState{CollapsedSections: make(map[string]bool)},
			}
			m.SetSize(100, 6)

			m.Cursor = 5
			viewport := m.ComputeOutlinerViewport(100, 0)
			start := viewport.startIdx
			if start == 0 {
				t.Fatal("expected initial viewport to scroll down")
			}
			firstTitle := m.Rows[start].Task.Title

			m.Cursor = 4
			viewport = m.ComputeOutlinerViewport(100, 0)
			if viewport.startIdx != start {
				t.Fatalf("moving up inside viewport changed first rendered row: start=%d want %d", viewport.startIdx, start)
			}
			if got := m.Rows[viewport.startIdx].Task.Title; got != firstTitle {
				t.Fatalf("first rendered row = %q, want %q", got, firstTitle)
			}
		})
	}
}

// TestViewportReservesTheWholePeekDockWhateverTheRowHolds is the measurement-path
// contract, restated now that the dock is a fixed block below the list. A title
// too long for its row costs document rows, not reserved ones, and the row the
// cursor rests on costs nothing at all: the reservation, the render, and the
// list window are the same numbers over a long task and a short one, which is
// what keeps moving between them from shifting the frame.
func TestViewportReservesTheWholePeekDockWhateverTheRowHolds(t *testing.T) {
	const (
		width  = 60
		height = 20
	)
	long := &models.Task{
		ID: "long", ShortID: 1, WorkflowState: models.StateBacklog,
		Title: "Investigate intermittent timeout when syncing very large project " +
			"archives over slow connections and report which retry policy the " +
			"transport eventually settled on before the deadline",
	}
	short := &models.Task{ID: "short", ShortID: 2, Title: "Short", WorkflowState: models.StateBacklog}
	project := renderFixtureProject(t, "project", "project", "Project", []*models.Task{long, short})
	outliner := NewOutlinerModel(project, nil, newTestConfig())
	outliner.SetSize(width, height)
	outliner.RebuildRows()
	outliner.MoveCursorToIdentity(taskref.TaskIdentity{ProjectID: project.ID, TaskID: long.ID})

	outliner.PeekMode = true
	reserved := outliner.PeekDockRows()
	if reserved < peekDockMinRows {
		t.Fatalf("reserved %d dock rows, want at least the %d-row minimum", reserved, peekDockMinRows)
	}
	if drawn := len(outliner.PeekDockLines()); drawn != reserved {
		t.Fatalf("render drew %d dock rows against %d reserved", drawn, reserved)
	}
	longWindow := outliner.ComputeOutlinerViewport(width, 0).VisibleRows

	// The short task's detail is a fraction of the long one's, and neither the
	// dock nor the list window notices.
	outliner.MoveCursorToIdentity(taskref.TaskIdentity{ProjectID: project.ID, TaskID: short.ID})
	if drawn := len(outliner.PeekDockLines()); drawn != reserved {
		t.Fatalf("the short task's dock drew %d rows, want %d", drawn, reserved)
	}
	if got := outliner.ComputeOutlinerViewport(width, 0).VisibleRows; got != longWindow {
		t.Fatalf("the short task left %d list rows and the long one %d", got, longWindow)
	}
	outliner.MoveCursorToIdentity(taskref.TaskIdentity{ProjectID: project.ID, TaskID: long.ID})

	// The title is in the document rather than above it, so it is the long task's
	// document that grew, not its reservation.
	whole := ansi.Strip(strings.Join(
		inlinePeekDocumentFor(inlinePeekOptions{Task: long, Indent: peekDockIndent, Width: width, Window: testPeekWindow()}).Body, " "))
	if want := strings.Join(strings.Fields(long.Title), " "); !strings.Contains(strings.Join(strings.Fields(whole), " "), want) {
		t.Fatalf("the long title is not in its Peek document:\n%s", whole)
	}

	for lineNumber, line := range strings.Split(outliner.View().Content, "\n") {
		if got := lipgloss.Width(line); got > width {
			t.Fatalf("line %d overflowed to %d columns", lineNumber, got)
		}
	}
	if got := lipgloss.Height(outliner.View().Content); got > height {
		t.Fatalf("view height = %d, want <= %d", got, height)
	}
}
