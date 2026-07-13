package tui

func (m *AppModel) appPersister() Persister {
	if m.persister != nil {
		return m.persister
	}
	return NewProjectService()
}

func (m *AppModel) clearBulkTaskSelectionsForRefs(refs []TaskProjectRef) {
	m.clearBulkTaskSelectionsForTaskIDSet(taskIDSetFromRefs(refs))
}

// clearBulkTaskSelectionsForTaskIDs is the ID-list entry point the async batch
// result path uses: the shell knows only the committed task IDs, not the live
// refs, so it clears every view's multiselect keyed by ID after a successful
// commit.
func (m *AppModel) clearBulkTaskSelectionsForTaskIDs(ids []string) {
	m.clearBulkTaskSelectionsForTaskIDSet(taskIDSliceToSet(ids))
}

func (m *AppModel) clearBulkTaskSelectionsForTaskIDSet(selectedIDs map[string]bool) {
	if len(selectedIDs) == 0 {
		return
	}

	if m.projectTree != nil && clearMultiSelectStateForTaskIDs(&m.projectTree.MultiSelectState, selectedIDs) {
		m.projectTree.buildTree()
		m.projectTree.clampCursor()
	}
	if m.allTasksView != nil {
		clearMultiSelectStateForTaskIDs(&m.allTasksView.MultiSelectState, selectedIDs)
	}
	if m.outliner != nil && multiSelectStateHasTaskID(&m.outliner.MultiSelectState, selectedIDs) {
		m.outliner.clearMultiSelect()
	}
	if m.crossOutliner != nil && multiSelectStateHasTaskID(&m.crossOutliner.MultiSelectState, selectedIDs) {
		m.crossOutliner.clearMultiSelect()
	}

	for _, vt := range []ViewType{
		ViewAllTasks,
		ViewFocusView,
		ViewDailyReview,
		ViewWeeklyReview,
		ViewPriorityView,
		ViewDueDateOverview,
		ViewKanban,
		ViewCrossProjectKanban,
		ViewOutliner,
		ViewTemporaryOutliner,
		ViewCrossProjectOutliner,
	} {
		m.clearViewBulkTaskSelectionForRefs(vt, selectedIDs)
	}
}

func (m *AppModel) clearViewBulkTaskSelectionForRefs(vt ViewType, selectedIDs map[string]bool) {
	if m.views == nil {
		return
	}
	view := m.views.Get(vt)
	switch v := view.(type) {
	case *AllTasksViewModel:
		clearMultiSelectStateForTaskIDs(&v.MultiSelectState, selectedIDs)
	case *FocusViewModel:
		clearMultiSelectStateForTaskIDs(&v.MultiSelectState, selectedIDs)
	case *DailyReviewModel:
		clearMultiSelectStateForTaskIDs(&v.MultiSelectState, selectedIDs)
	case *WeeklyReviewModel:
		clearMultiSelectStateForTaskIDs(&v.MultiSelectState, selectedIDs)
	case *PriorityViewModel:
		clearMultiSelectStateForTaskIDs(&v.MultiSelectState, selectedIDs)
	case *DueDateOverviewModel:
		clearMultiSelectStateForTaskIDs(&v.MultiSelectState, selectedIDs)
	case *KanbanModel:
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

func taskIDSetFromRefs(refs []TaskProjectRef) map[string]bool {
	ids := make(map[string]bool, len(refs))
	for _, ref := range refs {
		if ref.Task == nil || ref.Task.ID == "" {
			continue
		}
		ids[ref.Task.ID] = true
	}
	return ids
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
