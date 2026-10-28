package listsurface

import "github.com/sglavoie/docket/internal/tui/route"

// The pointer answer every list surface shares. A list is short and windows
// itself, so the wheel moves its cursor rather than its window: one row per
// step, through the arrow key that would move it. A click seats the cursor on
// the entry it landed on, exactly where j/k would have brought it; a
// double-click is Enter on that entry and a right-click is the surface's Space,
// where it has one. Only the seat is done here, since no key names a row; every
// act after it is a key the owner presses through route.PressKey, so the
// operation it reaches and every refusal it gives are the keyboard's own.

// ListPointerFrame is what one list drew, in the rows of the pane it was drawn
// into: each drawn entry's index and the rows it spans. The owner records it at
// render and reads it back when a click lands, so a hit is always the entry the
// user saw on that row and never one a recomputed window would put there.
// Continuation lines, category headers, and padding record nothing, so a click
// on them seats nothing.
type ListPointerFrame struct {
	spans []listPointerSpan
	width int
}

type listPointerSpan struct {
	index, top, rows int
}

// Reset forgets the last frame, and with it every hit, until the next render
// records one. width bounds the columns a hit may land in; zero bounds none.
func (frame *ListPointerFrame) Reset(width int) {
	frame.spans = frame.spans[:0]
	frame.width = max(width, 0)
}

// Record notes that entry index was drawn on rows [top, top+rows) of the pane.
func (frame *ListPointerFrame) Record(index, top, rows int) {
	if rows <= 0 {
		return
	}
	frame.spans = append(frame.spans, listPointerSpan{index: index, top: top, rows: rows})
}

// IndexAt reports the entry drawn at pane cell (x, y) in the recorded frame.
func (frame ListPointerFrame) IndexAt(x, y int) (int, bool) {
	if x < 0 || (frame.width > 0 && x >= frame.width) {
		return 0, false
	}
	for _, span := range frame.spans {
		if y >= span.top && y < span.top+span.rows {
			return span.index, true
		}
	}
	return 0, false
}

// ListPointerKey is the key one pointer event presses once its owner has
// answered the seat: the wheel's arrow whatever it was over, Enter for a
// double-click that seated, and Space for a right-click that seated on a
// surface whose menu that key is. Anything else presses nothing.
func ListPointerKey(event route.PointerEvent, seated, menu bool) string {
	if event.IsWheel() {
		if event.WheelDelta > 0 {
			return "down"
		}
		return "up"
	}
	if !seated {
		return ""
	}
	switch event.Button {
	case route.PointerButtonLeft:
		if event.IsDoubleClick() {
			return "enter"
		}
	case route.PointerButtonRight:
		if menu {
			return "space"
		}
	case route.PointerButtonNone, route.PointerButtonMiddle:
	}
	return ""
}

// AnswerPointer is the list surface's half of the shared answer, for an event
// already in the coordinates of the pane renderListSurfacePane last drew: a
// click on an entry row seats the cursor there, and the key the event then
// presses is returned for the owner to send. menu says whether Space is this
// surface's menu.
func (model *ListSurfaceModel) AnswerPointer(event route.PointerEvent, menu bool) string {
	if model == nil {
		return ""
	}
	seated := false
	if !event.IsWheel() {
		if index, ok := model.pointerFrame.IndexAt(event.X, event.Y); ok && index < len(model.matches) {
			model.setCursorIndex(index)
			seated = true
		}
	}
	return ListPointerKey(event, seated, menu)
}

// PointerItemAt reports the item the last render drew at pane cell (x, y),
// without seating the cursor there: for an owner whose single click is itself
// the act, as a menu entry's is.
func (model *ListSurfaceModel) PointerItemAt(x, y int) (ListItemID, bool) {
	if model == nil {
		return "", false
	}
	index, ok := model.pointerFrame.IndexAt(x, y)
	if !ok || index >= len(model.matches) {
		return "", false
	}
	return model.matches[index].item.ID, true
}

// ListSurfacePreviewLeft is the first column of the preview pane
// JoinListSurfacePanes draws beside a list pane listWidth columns wide.
func ListSurfacePreviewLeft(listWidth int) int {
	return listWidth + listSurfacePaneGapWidth
}

// PointerOverPreview reports whether pane column x is inside the preview pane
// the last render drew beside the list, and false when it drew none. The
// separator between the panes belongs to neither.
func (model *ListSurfaceModel) PointerOverPreview(x int) bool {
	return model != nil && model.previewLeft > 0 && x >= model.previewLeft
}

// recordPointerFrame notes which match each visible row was, below lead rows of
// continuation, so a click can name the match it landed on.
func (model *ListSurfaceModel) recordPointerFrame(rows []ListSurfaceRow, lead, width int) {
	model.pointerFrame.Reset(width)
	for offset, row := range rows {
		if row.Kind != ListRowItem {
			continue
		}
		if index, ok := model.matchIndex(row.Match.Item.ID); ok {
			model.pointerFrame.Record(index, lead+offset, 1)
		}
	}
}

func (model *ListSurfaceModel) matchIndex(id ListItemID) (int, bool) {
	for index, match := range model.matches {
		if match.item.ID == id {
			return index, true
		}
	}
	return 0, false
}
