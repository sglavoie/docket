package listsurface

import (
	"fmt"
	"slices"

	"github.com/sglavoie/tt/internal/tui/operation"
)

// SelectCurrent stores the enabled cursor item. Single selection replaces the
// prior value; multi selection adds to it.
func (model *ListSurfaceModel) SelectCurrent() error {
	item, ok := model.CurrentItem()
	if !ok {
		return ErrListItemNotFound
	}
	if !item.Enabled() {
		return fmt.Errorf("%w: %s", ErrListItemDisabled, item.DisabledReason)
	}
	if !model.config.SelectionMode.Many() {
		clear(model.selected)
	}
	model.selected[item.ID] = struct{}{}
	return nil
}

// ToggleCurrentSelection toggles in multi-select mode and selects in
// single-select mode. It returns the resulting selected state.
func (model *ListSurfaceModel) ToggleCurrentSelection() (bool, error) {
	item, ok := model.CurrentItem()
	if !ok {
		return false, ErrListItemNotFound
	}
	if !item.Enabled() {
		return false, fmt.Errorf("%w: %s", ErrListItemDisabled, item.DisabledReason)
	}
	if !model.config.SelectionMode.Many() {
		clear(model.selected)
		model.selected[item.ID] = struct{}{}
		return true, nil
	}
	if _, selected := model.selected[item.ID]; selected {
		delete(model.selected, item.ID)
		return false, nil
	}
	model.selected[item.ID] = struct{}{}
	return true, nil
}

// SetSelectedIDs atomically restores explicit stable-ID selections.
func (model *ListSurfaceModel) SetSelectedIDs(ids []ListItemID) error {
	if model == nil {
		return nil
	}
	normalized := make([]ListItemID, 0, len(ids))
	seen := make(map[ListItemID]struct{}, len(ids))
	for _, id := range ids {
		if _, duplicate := seen[id]; duplicate {
			continue
		}
		item, ok := model.sourceItem(id)
		if !ok {
			return fmt.Errorf("%w: %s", ErrListItemNotFound, id)
		}
		if !item.Enabled() {
			return fmt.Errorf("%w: %s", ErrListItemDisabled, item.DisabledReason)
		}
		seen[id] = struct{}{}
		normalized = append(normalized, id)
	}
	if !model.config.SelectionMode.Many() && len(normalized) > 1 {
		return ErrListSelectionCardinality
	}

	next := make(map[ListItemID]struct{}, len(normalized))
	for _, id := range normalized {
		next[id] = struct{}{}
	}
	model.selected = next
	return nil
}

// ClearSelection removes every explicit selection.
func (model *ListSurfaceModel) ClearSelection() {
	if model != nil {
		clear(model.selected)
	}
}

// IsSelected reports whether a stable ID is explicitly selected.
func (model *ListSurfaceModel) IsSelected(id ListItemID) bool {
	if model == nil {
		return false
	}
	_, ok := model.selected[id]
	return ok
}

// SelectedIDs returns IDs in source order rather than map iteration order.
func (model *ListSurfaceModel) SelectedIDs() []ListItemID {
	if model == nil {
		return []ListItemID{}
	}
	ids := make([]ListItemID, 0, len(model.selected))
	for _, item := range model.items {
		if model.IsSelected(item.ID) {
			ids = append(ids, item.ID)
		}
	}
	return ids
}

// SelectedItems returns selected items in source order.
func (model *ListSurfaceModel) SelectedItems() []ListItem {
	if model == nil {
		return []ListItem{}
	}
	items := make([]ListItem, 0, len(model.selected))
	for _, item := range model.items {
		if model.IsSelected(item.ID) {
			items = append(items, cloneListItem(item))
		}
	}
	return items
}

// CurrentPreview returns preview content for the cursor item.
func (model *ListSurfaceModel) CurrentPreview() (ListPreview, bool) {
	item, ok := model.CurrentItem()
	if !ok || item.Preview.Empty() {
		return ListPreview{}, false
	}
	return cloneListPreview(item.Preview), true
}

// ActionState returns operations common to the active item set.
func (model *ListSurfaceModel) ActionState() ListActionState {
	if model == nil {
		return ListActionState{}
	}
	items := model.actionItems()
	if len(items) == 0 {
		return ListActionState{}
	}

	state := ListActionState{
		ItemIDs: make([]ListItemID, 0, len(items)),
	}
	for _, item := range items {
		state.ItemIDs = append(state.ItemIDs, item.ID)
		if !item.Enabled() {
			state.DisabledReason = item.DisabledReason
			state.OperationIDs = nil
			return state
		}
	}

	state.OperationIDs = append([]operation.OperationID(nil), items[0].OperationIDs...)
	for _, item := range items[1:] {
		state.OperationIDs = intersectOperationIDs(state.OperationIDs, item.OperationIDs)
	}
	return state
}

// ExecuteAction enforces item-declared action availability, then delegates all
// execution policy to the centralized OperationExecutor.
func (model *ListSurfaceModel) ExecuteAction(
	executor *operation.OperationExecutor,
	request operation.OperationExecutionRequest,
) operation.OperationExecutionResult {
	if executor == nil {
		return operation.OperationExecutionResult{
			Err: fmt.Errorf("%w: operation executor is nil", ErrListActionUnavailable),
		}
	}
	state := model.ActionState()
	if !state.Enabled() || !slices.Contains(state.OperationIDs, request.ID) {
		detail := string(request.ID)
		if state.DisabledReason != "" {
			detail = state.DisabledReason
		}
		return operation.OperationExecutionResult{
			Err: fmt.Errorf("%w: %s", ErrListActionUnavailable, detail),
		}
	}
	return executor.Execute(request)
}
