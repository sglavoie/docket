package tui

import (
	"fmt"
	"strings"

	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/keymap"
)

// Cursor returns the selectable match index, or -1 when no items match.
func (model *ListSurfaceModel) Cursor() int {
	if model == nil {
		return -1
	}
	return model.cursor
}

// CurrentItem returns the item under the cursor.
func (model *ListSurfaceModel) CurrentItem() (ListItem, bool) {
	if model == nil || model.cursor < 0 || model.cursor >= len(model.matches) {
		return ListItem{}, false
	}
	return cloneListItem(model.matches[model.cursor].item), true
}

// SetCursorToID moves to a visible item and updates the preservation anchor.
func (model *ListSurfaceModel) SetCursorToID(id ListItemID) bool {
	if model == nil {
		return false
	}
	for index, match := range model.matches {
		if match.item.ID != id {
			continue
		}
		model.cursor = index
		model.cursorAnchor = id
		model.ensureCursorVisible()
		return true
	}
	return false
}

// MoveCursor moves without wrapping and skips category headers by indexing only
// matches. It returns whether the cursor changed.
func (model *ListSurfaceModel) MoveCursor(delta int) bool {
	if model == nil || len(model.matches) == 0 || delta == 0 {
		return false
	}
	next := min(max(model.cursor+delta, 0), len(model.matches)-1)
	if next == model.cursor {
		return false
	}
	model.cursor = next
	model.cursorAnchor = model.matches[next].item.ID
	model.ensureCursorVisible()
	return true
}

// MoveCursorToStart selects the first matching item.
func (model *ListSurfaceModel) MoveCursorToStart() bool {
	if model == nil || len(model.matches) == 0 {
		return false
	}
	return model.setCursorIndex(0)
}

// MoveCursorToEnd selects the last matching item.
func (model *ListSurfaceModel) MoveCursorToEnd() bool {
	if model == nil || len(model.matches) == 0 {
		return false
	}
	return model.setCursorIndex(len(model.matches) - 1)
}

// PageCursor moves by the selectable capacity of each page.
func (model *ListSurfaceModel) PageCursor(pages int) bool {
	if model == nil || pages == 0 {
		return false
	}
	pageSize := max(model.height-1, 1)
	return model.MoveCursor(pages * pageSize)
}

// SetSize updates the headless viewport and keeps its offset in bounds.
func (model *ListSurfaceModel) SetSize(width, height int) {
	if model == nil {
		return
	}
	model.width = max(width, 0)
	model.height = max(height, 0)
	model.ensureCursorVisible()
}

// Viewport returns current scroll boundaries.
func (model *ListSurfaceModel) Viewport() ListViewport {
	if model == nil {
		return ListViewport{}
	}
	total := len(model.buildRows())
	maxOffset := max(total-model.height, 0)
	offset := min(max(model.scrollOffset, 0), maxOffset)
	return ListViewport{
		Width:         model.width,
		Height:        model.height,
		Offset:        offset,
		TotalRows:     total,
		CanScrollUp:   offset > 0,
		CanScrollDown: offset < maxOffset,
	}
}

// Rows returns every category and item row.
func (model *ListSurfaceModel) Rows() []ListSurfaceRow {
	if model == nil {
		return []ListSurfaceRow{}
	}
	return model.buildRows()
}

// VisibleRows returns a bounded copy of the current viewport.
func (model *ListSurfaceModel) VisibleRows() []ListSurfaceRow {
	if model == nil || model.height <= 0 {
		return []ListSurfaceRow{}
	}
	rows := model.buildRows()
	if len(rows) == 0 {
		return []ListSurfaceRow{}
	}
	start := min(max(model.scrollOffset, 0), max(len(rows)-model.height, 0))
	end := min(start+model.height, len(rows))
	return append([]ListSurfaceRow(nil), rows[start:end]...)
}

// EmptyContext returns contextual copy only while the result set is empty.
func (model *ListSurfaceModel) EmptyContext() (ListEmptyContext, bool) {
	if model == nil || len(model.matches) > 0 {
		return ListEmptyContext{}, false
	}
	if len(model.items) == 0 {
		return ListEmptyContext{
			Kind:  ListEmptySource,
			State: model.config.EmptyState,
		}, true
	}
	return ListEmptyContext{
		Kind:  ListEmptyNoMatches,
		State: model.config.NoMatchState,
	}, true
}

// GuidanceState selects one shared profile from live list state. Active search
// editing wins because it owns input; causal empty results then take precedence
// over hidden selections, followed by disabled and selected item states.
func (model *ListSurfaceModel) GuidanceState(searchEditing bool) ListGuidanceState {
	state := ListGuidanceState{}
	if model == nil {
		state.Profile = ListGuidanceEmpty
		return state
	}
	state.Query = model.query.Normalized
	state.SelectionCount = len(model.selected)
	if searchEditing {
		state.Profile = ListGuidanceSearchEditing
		return state
	}
	if empty, ok := model.EmptyContext(); ok {
		state.Profile = ListGuidanceEmpty
		state.Empty = empty
		return state
	}
	action := model.ActionState()
	if action.DisabledReason != "" {
		state.Profile = ListGuidanceDisabled
		state.DisabledReason = action.DisabledReason
		return state
	}
	if state.SelectionCount > 0 {
		state.Profile = ListGuidanceSelection
		return state
	}
	if state.Query != "" {
		state.Profile = ListGuidanceSearchApplied
		return state
	}
	state.Profile = ListGuidanceBrowse
	return state
}

// listSurfaceGuidanceSpec combines live gated navigation bindings with already
// applicable operation guidance. The list profile owns only row budgeting and
// supplemental state; keys remain authoritative in the keymap or registry that
// produced each hint.
func listSurfaceGuidanceSpec(
	state ListGuidanceState,
	km *keymap.KeyMap,
	actionHints []chrome.GuidanceHint,
) (chrome.GuidanceSpec, bool) {
	rows := 1
	if state.Profile == ListGuidanceSearchEditing {
		rows = 2
	}
	spec, _ := chrome.GuidanceSpecFromKeyMap(km, rows)
	spec.Rows = rows
	spec.Hints = append(spec.Hints, actionHints...)
	switch state.Profile {
	case ListGuidanceBrowse, ListGuidanceSearchEditing, ListGuidanceEmpty:
		// These profiles need no supplemental context.
	case ListGuidanceSearchApplied:
		if state.Query != "" {
			spec.Supplemental = "search: " + state.Query
		}
	case ListGuidanceSelection:
		spec.Supplemental = fmt.Sprintf("%d selected", state.SelectionCount)
	case ListGuidanceDisabled:
		spec.Supplemental = state.DisabledReason
	}
	return spec, len(spec.Hints) > 0 || strings.TrimSpace(spec.Supplemental) != ""
}
