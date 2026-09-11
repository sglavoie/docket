package tui

import (
	"fmt"
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/outliner"
)

// scrollAnchorApp seats a scrolled window and returns the app, its outliner, and
// the half-open window the render drew.
func scrollAnchorApp(t *testing.T) (*AppModel, *outliner.OutlinerModel, int, int) {
	t.Helper()
	app := newUITestApp()
	out := app.crossOutliner
	out.Rows = make([]outliner.OutlinerRow, 40)
	for i := range out.Rows {
		out.Rows[i] = outliner.OutlinerRow{Task: &models.Task{
			ID: fmt.Sprintf("task-%02d", i), ShortID: i + 1, Title: fmt.Sprintf("Task %02d", i),
		}}
	}
	out.SetSize(100, 12)
	out.Cursor = 25
	out.ComputeOutlinerViewport(100, 0)
	start, end := out.VisibleRowWindow()
	if start == 0 || end-start < 3 {
		t.Fatalf("window = [%d,%d), want a scrolled window holding three distinct anchors", start, end)
	}
	return app, out, start, end
}

func TestScrollAnchorsMoveTheWindowAndLeaveTheCursor(t *testing.T) {
	app, out, start, end := scrollAnchorApp(t)
	height := end - start
	cursor := out.Cursor

	for _, tc := range []struct {
		key  string
		want int
	}{
		{key: "t", want: cursor},
		{key: "z", want: cursor - (height-1)/2},
		{key: "b", want: cursor - height + 1},
	} {
		out.Cursor = cursor
		out.ViewScrollOffset = start
		out.ComputeOutlinerViewport(100, 0)

		result := executeDirectPath(t, app, app.operationCommandContext(), []string{"z", tc.key})
		if result.Err != nil {
			t.Fatalf("z%s: %v", tc.key, result.Err)
		}
		if result.Status != "" {
			t.Errorf("z%s status = %q, want silence", tc.key, result.Status)
		}
		if out.Cursor != cursor {
			t.Errorf("z%s cursor = %d, want %d unchanged", tc.key, out.Cursor, cursor)
		}
		if out.ViewScrollOffset != tc.want {
			t.Errorf("z%s scroll offset = %d, want %d", tc.key, out.ViewScrollOffset, tc.want)
		}
		// The written offset still contains the cursor, so the next render's
		// clamp leaves it exactly where the key put it.
		out.ComputeOutlinerViewport(100, 0)
		if out.ViewScrollOffset != tc.want {
			t.Errorf("z%s scroll offset after render = %d, want %d", tc.key, out.ViewScrollOffset, tc.want)
		}
	}
}

func TestScrollAnchorTopClampsAtTheLastRow(t *testing.T) {
	app, out, start, end := scrollAnchorApp(t)
	height := end - start
	out.Cursor = len(out.Rows) - 1
	out.ComputeOutlinerViewport(100, 0)

	if result := executeDirectPath(t, app, app.operationCommandContext(), []string{"z", "t"}); result.Err != nil {
		t.Fatalf("zt: %v", result.Err)
	}
	want := len(out.Rows) - height
	if out.ViewScrollOffset != want {
		t.Fatalf("zt scroll offset = %d, want %d with the last row still visible", out.ViewScrollOffset, want)
	}
	if _, last := out.VisibleRowWindow(); last != len(out.Rows) {
		t.Fatalf("window end = %d, want %d", last, len(out.Rows))
	}
}

func TestScrollAnchorMiddleSettlesOnASecondPress(t *testing.T) {
	app, out, _, _ := scrollAnchorApp(t)
	ctx := app.operationCommandContext()

	if result := executeDirectPath(t, app, ctx, []string{"z", "z"}); result.Err != nil {
		t.Fatalf("zz: %v", result.Err)
	}
	out.ComputeOutlinerViewport(100, 0)
	offset := out.ViewScrollOffset

	// The cursor already sits mid-window, so a second zz has nothing to move.
	if result := executeDirectPath(t, app, ctx, []string{"z", "z"}); result.Err != nil {
		t.Fatalf("zz: %v", result.Err)
	}
	if out.ViewScrollOffset != offset {
		t.Errorf("second zz moved the offset to %d, want %d", out.ViewScrollOffset, offset)
	}
}

func TestScrollAnchorsLeaveAnEmptyWindowAlone(t *testing.T) {
	app, out, _, _ := scrollAnchorApp(t)
	out.Rows = nil
	out.Cursor = 0
	out.ViewScrollOffset = 0

	result := executeDirectPath(t, app, app.operationCommandContext(), []string{"z", "b"})
	if result.Err != nil {
		t.Fatalf("zb: %v", result.Err)
	}
	if result.Status != "" {
		t.Errorf("zb status = %q, want silence", result.Status)
	}
	if out.Cursor != 0 || out.ViewScrollOffset != 0 {
		t.Fatalf("cursor = %d offset = %d, want both left at 0", out.Cursor, out.ViewScrollOffset)
	}
}
