package tui

import "github.com/sglavoie/tt/internal/app"

// clearBulkTaskSelectionsForTaskIDs is the compatibility entry point for async
// results that lack project-qualified locators. It clears identity-keyed state
// only when each matching task ID belongs to one selected project; ambiguous
// duplicate-ID selections are left intact.
func (m *AppModel) clearBulkTaskSelectionsForTaskIDs(ids []string) {
	m.clearBulkTaskSelectionsForTaskIDSet(taskIDSliceToSet(ids))
}

func (m *AppModel) clearBulkTaskSelectionsForLocators(locators []app.TaskLocator) {
	if len(locators) == 0 {
		return
	}
	set := make(map[TaskIdentity]bool, len(locators))
	for _, locator := range locators {
		identity := taskIdentityFromLocator(locator)
		if identity.Valid() {
			set[identity] = true
		}
	}
	clearOutliner := func(outliner *OutlinerModel) {
		if outliner == nil {
			return
		}
		for identity := range set {
			delete(outliner.selectedTaskIdentities, identity)
		}
		if outliner.SelectedCount() == 0 {
			outliner.clearMultiSelect()
		}
	}
	clearOutliner(m.outliner)
	clearOutliner(m.crossOutliner)
	if m.views != nil {
		for _, vt := range []PresentationID{ViewOutliner, ViewCrossProjectOutliner} {
			if view := m.views.Get(vt); view != nil {
				if outliner, ok := view.(*OutlinerModel); ok {
					clearOutliner(outliner)
				}
			}
		}
	}
	m.clearViewBulkTaskSelectionForIdentities(ViewWeeklyReview, set)
}

func (m *AppModel) clearViewBulkTaskSelectionForIdentities(vt PresentationID, selected map[TaskIdentity]bool) {
	if m.views == nil {
		return
	}
	clearSelected := func(state *MultiSelectState) {
		for identity := range selected {
			state.DeselectTaskIdentity(identity)
		}
		if state.SelectedCount() == 0 {
			state.ExitMultiSelect()
		}
	}
	switch view := m.views.Get(vt).(type) {
	case *WeeklyReviewModel:
		clearSelected(&view.MultiSelectState)
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
		ViewWeeklyReview,
		ViewOutliner,
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
	if state == nil {
		return false
	}
	matchedProjects := make(map[string]string, len(selectedIDs))
	for identity := range state.selectedTaskIdentities {
		if !selectedIDs[identity.TaskID] {
			continue
		}
		if projectID, matched := matchedProjects[identity.TaskID]; matched && projectID != identity.ProjectID {
			// An ID-only result cannot identify which duplicate selection committed.
			// Leave the selection intact instead of clearing both projects.
			return false
		}
		matchedProjects[identity.TaskID] = identity.ProjectID
	}
	return len(matchedProjects) > 0
}

func clearMultiSelectStateForTaskIDs(state *MultiSelectState, selectedIDs map[string]bool) bool {
	if !multiSelectStateHasTaskID(state, selectedIDs) {
		return false
	}
	state.ExitMultiSelect()
	return true
}
