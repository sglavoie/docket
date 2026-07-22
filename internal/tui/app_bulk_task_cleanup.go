package tui

import "github.com/sglavoie/tt/internal/app"

// clearBulkTaskSelectionsForTaskIDs is the ID-list entry point the async batch
// result path uses: the shell knows only the committed task IDs, not the live
// refs, so it clears every view's multiselect keyed by ID after a successful
// commit.
func (m *AppModel) clearBulkTaskSelectionsForTaskIDs(ids []string) {
	m.clearBulkTaskSelectionsForTaskIDSet(taskIDSliceToSet(ids))
}

func (m *AppModel) clearBulkTaskSelectionsForLocators(locators []app.TaskLocator) {
	if len(locators) == 0 {
		return
	}
	set := make(map[TaskIdentity]bool, len(locators))
	legacyIDs := make(map[string]bool, len(locators))
	for _, locator := range locators {
		identity := TaskIdentity{ProjectID: locator.ProjectID, TaskID: locator.TaskID}
		if identity.Valid() {
			set[identity] = true
			legacyIDs[identity.TaskID] = true
		}
	}
	clearOutliner := func(outliner *OutlinerModel) {
		if outliner == nil {
			return
		}
		for identity := range set {
			delete(outliner.selectedTaskIdentities, identity)
			if outliner.project == nil || outliner.project.ID == identity.ProjectID {
				delete(outliner.selectedTasks, identity.TaskID)
			}
		}
		if outliner.SelectedCount() == 0 {
			outliner.clearMultiSelect()
		}
	}
	clearOutliner(m.outliner)
	clearOutliner(m.crossOutliner)
	if m.views != nil {
		for _, vt := range []PresentationID{ViewOutliner, ViewTemporaryOutliner, ViewCrossProjectOutliner} {
			if view := m.views.Get(vt); view != nil {
				if outliner, ok := view.(*OutlinerModel); ok {
					clearOutliner(outliner)
				}
			}
		}
	}
	// Review views still use their single-ID selection state; their row models
	// already carry project identity into the request, and a successful batch
	// consumes the whole selection.
	for _, vt := range []PresentationID{ViewDailyReview, ViewWeeklyReview} {
		m.clearViewBulkTaskSelectionForRefs(vt, legacyIDs)
	}
}

func (m *AppModel) clearBulkTaskSelectionsForTaskIDSet(selectedIDs map[string]bool) {
	if len(selectedIDs) == 0 {
		return
	}

	if m.outliner != nil && clearMultiSelectStateForTaskIDs(&m.outliner.MultiSelectState, selectedIDs) {
		m.outliner.rebuildRows()
		m.outliner.clampCursor()
	}
	if m.outliner != nil && multiSelectStateHasTaskID(&m.outliner.MultiSelectState, selectedIDs) {
		m.outliner.clearMultiSelect()
	}
	if m.crossOutliner != nil && multiSelectStateHasTaskID(&m.crossOutliner.MultiSelectState, selectedIDs) {
		m.crossOutliner.clearMultiSelect()
	}

	for _, vt := range []PresentationID{
		ViewDailyReview,
		ViewWeeklyReview,
		ViewOutliner,
		ViewTemporaryOutliner,
		ViewCrossProjectOutliner,
	} {
		m.clearViewBulkTaskSelectionForRefs(vt, selectedIDs)
	}
}

func (m *AppModel) clearViewBulkTaskSelectionForRefs(vt PresentationID, selectedIDs map[string]bool) {
	if m.views == nil {
		return
	}
	view := m.views.Get(vt)
	switch v := view.(type) {
	case *DailyReviewModel:
		clearMultiSelectStateForTaskIDs(&v.MultiSelectState, selectedIDs)
	case *WeeklyReviewModel:
		clearMultiSelectStateForTaskIDs(&v.MultiSelectState, selectedIDs)
	case *OutlinerModel:
		if multiSelectStateHasTaskID(&v.MultiSelectState, selectedIDs) {
			v.clearMultiSelect()
		}
	}
}

func taskIDSliceToSet(ids []string) map[string]bool {
	set := make(map[string]bool, len(ids))
	for _, id := range ids {
		if id != "" {
			set[id] = true
		}
	}
	return set
}

func multiSelectStateHasTaskID(state *MultiSelectState, selectedIDs map[string]bool) bool {
	if state == nil || len(state.selectedTasks) == 0 {
		return false
	}
	for id := range state.selectedTasks {
		if selectedIDs[id] {
			return true
		}
	}
	return false
}

func clearMultiSelectStateForTaskIDs(state *MultiSelectState, selectedIDs map[string]bool) bool {
	if !multiSelectStateHasTaskID(state, selectedIDs) {
		return false
	}
	state.ExitMultiSelect()
	return true
}
