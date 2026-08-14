package tui

import (
	"fmt"
	"testing"

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
	prefix, fold := renderTaskPrefix("├── ", "")
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
