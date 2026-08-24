package tui

import (
	"fmt"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
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
			if got := buildTreePrefix(tt.ancestorIsLast, tt.isLast); got != tt.expected {
				t.Errorf("buildTreePrefix(%v, %v)\ngot:  %q\nwant: %q", tt.ancestorIsLast, tt.isLast, got, tt.expected)
			}
		})
	}
}

func TestRenderTaskPrefixKeepsLeafConnectorsCompact(t *testing.T) {
	prefix, fold := renderTaskPrefix("├── ", "", nil)
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
			rows := make([]outlinerRow, 10)
			for i := range rows {
				rows[i] = outlinerRow{task: &models.Task{
					ID: fmt.Sprintf("task-%02d", i), ShortID: i + 1, Title: fmt.Sprintf("Task %02d", i),
				}}
			}
			m := &OutlinerModel{
				project: tt.project, rows: rows,
				collapsedSections: make(map[string]bool),
			}
			m.SetSize(100, 6)

			m.cursor = 5
			viewport := m.computeOutlinerViewport(100, 0)
			start := viewport.startIdx
			if start == 0 {
				t.Fatal("expected initial viewport to scroll down")
			}
			firstTitle := m.rows[start].task.Title

			m.cursor = 4
			viewport = m.computeOutlinerViewport(100, 0)
			if viewport.startIdx != start {
				t.Fatalf("moving up inside viewport changed first rendered row: start=%d want %d", viewport.startIdx, start)
			}
			if got := m.rows[viewport.startIdx].task.Title; got != firstTitle {
				t.Fatalf("first rendered row = %q, want %q", got, firstTitle)
			}
		})
	}
}

// TestViewportReservesTheTitleContinuationRows is the measurement-path contract:
// the continuation sits outside the additive cap, so computeOutlinerViewport
// must reserve those rows too or the list overflows the bottom of the terminal.
func TestViewportReservesTheTitleContinuationRows(t *testing.T) {
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
	outliner.rebuildRows()
	outliner.moveCursorToIdentity(TaskIdentity{ProjectID: project.ID, TaskID: long.ID})

	outliner.peekMode = true
	row := outliner.rows[outliner.cursor]
	indent := strings.Repeat(" ", lipgloss.Width(row.prefix)+4)
	reserved := outliner.inlinePeekLineCount(row, width-1)
	bare := len(inlinePeekLines(inlinePeekOptions{Task: long, Indent: indent, Width: width - 1}))
	if reserved <= bare {
		t.Fatalf("reserved %d Peek rows, want more than the %d the block holds without a continuation", reserved, bare)
	}
	if drawn := len(outliner.inlinePeekLinesForRow(row, indent, width, nil)); drawn > reserved {
		t.Fatalf("render drew %d Peek rows, more than the %d reserved", drawn, reserved)
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

// screenJumpApp seats a scrolled window and returns the app, its outliner, and
// the half-open window the render drew.
func screenJumpApp(t *testing.T) (*AppModel, *OutlinerModel, int, int) {
	t.Helper()
	app := newUITestApp()
	out := app.crossOutliner
	out.rows = make([]outlinerRow, 40)
	for i := range out.rows {
		out.rows[i] = outlinerRow{task: &models.Task{
			ID: fmt.Sprintf("task-%02d", i), ShortID: i + 1, Title: fmt.Sprintf("Task %02d", i),
		}}
	}
	out.SetSize(100, 12)
	out.cursor = 25
	out.computeOutlinerViewport(100, 0)
	start, end := out.visibleRowWindow()
	if start == 0 || end-start < 3 {
		t.Fatalf("window = [%d,%d), want a scrolled window holding three distinct landings", start, end)
	}
	return app, out, start, end
}

func TestScreenRelativeJumpsLandInTheDrawnWindowWithoutScrolling(t *testing.T) {
	app, out, start, end := screenJumpApp(t)
	offset := out.viewScrollOffset

	for _, tc := range []struct {
		key  string
		want int
	}{
		{key: "H", want: start},
		{key: "M", want: start + (end-start-1)/2},
		{key: "L", want: end - 1},
	} {
		out.cursor = start + 1
		result := executeDirectPath(t, app, app.operationCommandContext(), []string{tc.key})
		if result.Err != nil {
			t.Fatalf("%s: %v", tc.key, result.Err)
		}
		if out.cursor != tc.want {
			t.Errorf("%s cursor = %d, want %d", tc.key, out.cursor, tc.want)
		}
		if out.viewScrollOffset != offset {
			t.Errorf("%s scroll offset = %d, want %d unchanged", tc.key, out.viewScrollOffset, offset)
		}
	}
}

func TestScreenRelativeJumpsSkipSeparatorsInsideTheWindow(t *testing.T) {
	app, out, start, end := screenJumpApp(t)
	out.rows[start] = outlinerRow{IsSeparator: true}
	out.rows[end-1] = outlinerRow{IsSeparator: true}

	for _, tc := range []struct {
		key  string
		want int
	}{
		{key: "H", want: start + 1},
		{key: "L", want: end - 2},
	} {
		out.cursor = start + 2
		if result := executeDirectPath(t, app, app.operationCommandContext(), []string{tc.key}); result.Err != nil {
			t.Fatalf("%s: %v", tc.key, result.Err)
		}
		if out.cursor != tc.want {
			t.Errorf("%s cursor = %d, want the nearest navigable row inside the window %d", tc.key, out.cursor, tc.want)
		}
	}
}

func TestScrollAnchorsMoveTheWindowAndLeaveTheCursor(t *testing.T) {
	app, out, start, end := screenJumpApp(t)
	height := end - start
	cursor := out.cursor

	for _, tc := range []struct {
		key  string
		want int
	}{
		{key: "t", want: cursor},
		{key: "z", want: cursor - windowMiddleIndex(height)},
		{key: "b", want: cursor - height + 1},
	} {
		out.cursor = cursor
		out.viewScrollOffset = start
		out.computeOutlinerViewport(100, 0)

		result := executeDirectPath(t, app, app.operationCommandContext(), []string{"z", tc.key})
		if result.Err != nil {
			t.Fatalf("z%s: %v", tc.key, result.Err)
		}
		if result.Status != "" {
			t.Errorf("z%s status = %q, want silence", tc.key, result.Status)
		}
		if out.cursor != cursor {
			t.Errorf("z%s cursor = %d, want %d unchanged", tc.key, out.cursor, cursor)
		}
		if out.viewScrollOffset != tc.want {
			t.Errorf("z%s scroll offset = %d, want %d", tc.key, out.viewScrollOffset, tc.want)
		}
		// The written offset still contains the cursor, so the next render's
		// clamp leaves it exactly where the key put it.
		out.computeOutlinerViewport(100, 0)
		if out.viewScrollOffset != tc.want {
			t.Errorf("z%s scroll offset after render = %d, want %d", tc.key, out.viewScrollOffset, tc.want)
		}
	}
}

func TestScrollAnchorTopClampsAtTheLastRow(t *testing.T) {
	app, out, start, end := screenJumpApp(t)
	height := end - start
	out.cursor = len(out.rows) - 1
	out.computeOutlinerViewport(100, 0)

	if result := executeDirectPath(t, app, app.operationCommandContext(), []string{"z", "t"}); result.Err != nil {
		t.Fatalf("zt: %v", result.Err)
	}
	want := len(out.rows) - height
	if out.viewScrollOffset != want {
		t.Fatalf("zt scroll offset = %d, want %d with the last row still visible", out.viewScrollOffset, want)
	}
	if _, last := out.visibleRowWindow(); last != len(out.rows) {
		t.Fatalf("window end = %d, want %d", last, len(out.rows))
	}
}

func TestScrollAnchorMiddleAndScreenMiddleAgree(t *testing.T) {
	app, out, _, _ := screenJumpApp(t)
	ctx := app.operationCommandContext()

	// zz then M: the window moved under the cursor, so M lands back on it.
	if result := executeDirectPath(t, app, ctx, []string{"z", "z"}); result.Err != nil {
		t.Fatalf("zz: %v", result.Err)
	}
	out.computeOutlinerViewport(100, 0)
	cursor, offset := out.cursor, out.viewScrollOffset
	if result := executeDirectPath(t, app, ctx, []string{"M"}); result.Err != nil {
		t.Fatalf("M: %v", result.Err)
	}
	if out.cursor != cursor {
		t.Errorf("M after zz moved the cursor to %d, want %d", out.cursor, cursor)
	}

	// M then zz: the cursor already sits mid-window, so zz leaves the offset.
	if result := executeDirectPath(t, app, ctx, []string{"z", "z"}); result.Err != nil {
		t.Fatalf("zz: %v", result.Err)
	}
	if out.viewScrollOffset != offset {
		t.Errorf("zz after M moved the offset to %d, want %d", out.viewScrollOffset, offset)
	}
}

func TestScrollAnchorsLeaveAnEmptyWindowAlone(t *testing.T) {
	app, out, _, _ := screenJumpApp(t)
	out.rows = nil
	out.cursor = 0
	out.viewScrollOffset = 0

	result := executeDirectPath(t, app, app.operationCommandContext(), []string{"z", "b"})
	if result.Err != nil {
		t.Fatalf("zb: %v", result.Err)
	}
	if result.Status != "" {
		t.Errorf("zb status = %q, want silence", result.Status)
	}
	if out.cursor != 0 || out.viewScrollOffset != 0 {
		t.Fatalf("cursor = %d offset = %d, want both left at 0", out.cursor, out.viewScrollOffset)
	}
}

func TestScreenRelativeJumpsReportAWindowWithNoNavigableRow(t *testing.T) {
	app, out, start, end := screenJumpApp(t)
	for i := start; i < end; i++ {
		out.rows[i] = outlinerRow{IsSeparator: true}
	}
	out.cursor = start

	result := executeDirectPath(t, app, app.operationCommandContext(), []string{"M"})
	if result.Err != nil {
		t.Fatalf("M: %v", result.Err)
	}
	if result.Status != "No row to jump to on screen" {
		t.Fatalf("status = %q, want the empty-window refusal", result.Status)
	}
	if out.cursor != start {
		t.Fatalf("cursor = %d, want %d left where it was", out.cursor, start)
	}
}
