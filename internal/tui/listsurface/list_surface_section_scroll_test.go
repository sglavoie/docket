package listsurface

import (
	"fmt"
	"testing"
)

// Scrolling up onto a section's first item must keep that section's heading in
// the window: an item drawn without the name it is filed under, under an
// "above" cue pointing at nothing selectable, is the regression this pins.
func TestListSurfaceScrollUpKeepsSectionHeadingAboveFirstItem(t *testing.T) {
	t.Parallel()

	categories := []ListCategory{
		{ID: "a", Label: "A"},
		{ID: "b", Label: "B"},
	}
	items := make([]ListItem, 0, 10)
	for index := range 10 {
		category := ListCategoryID("a")
		if index >= 5 {
			category = "b"
		}
		items = append(items, ListItem{
			ID:          ListItemID(fmt.Sprintf("item-%d", index)),
			CategoryID:  category,
			PrimaryText: fmt.Sprintf("Item %d", index),
		})
	}
	model := mustListSurface(t, items, categories, ListSurfaceConfig{})
	model.SetSize(80, 4)

	if !model.MoveCursorToEnd() {
		t.Fatal("failed to move cursor to end")
	}
	if !model.MoveCursorToStart() {
		t.Fatal("failed to move cursor to start")
	}
	if viewport := model.Viewport(); viewport.Offset != 0 || viewport.CanScrollUp {
		t.Fatalf("viewport at start = %+v, want offset 0 with the A heading drawn", viewport)
	}

	// Walk up from the end onto B's first item: B's heading is row 6.
	if !model.MoveCursorToEnd() {
		t.Fatal("failed to move cursor to end")
	}
	if !model.SetCursorToID("item-5") {
		t.Fatal("failed to move cursor to item-5")
	}
	visible := model.VisibleRows()
	if len(visible) < 2 || visible[0].Kind != ListRowCategory || !visible[1].Cursor {
		t.Fatalf("first visible rows = %+v, want B heading then the cursor item", visible)
	}

	// A one-row window has room for the item alone.
	model.SetSize(80, 1)
	if !model.MoveCursorToEnd() || !model.MoveCursorToStart() {
		t.Fatal("failed to move cursor")
	}
	if visible := model.VisibleRows(); len(visible) != 1 || !visible[0].Cursor {
		t.Fatalf("one-row window = %+v, want the cursor item", visible)
	}
}
