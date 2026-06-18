package tui

import (
	"fmt"

	"github.com/sglavoie/tt/internal/models"
)

func (m *AppModel) appPersister() Persister {
	if m.persister != nil {
		return m.persister
	}
	return NewProjectService()
}

func (m *AppModel) applyBulkTaskRefsWithWorkflow(
	refs []TaskProjectRef,
	undoAction string,
	mutate func(project *models.Project, taskID string) bool,
) (int, []TaskProjectGroup, error) {
	groups := groupTaskProjectRefsByProject(refs)
	total := 0
	var changedGroups []changedTaskProjectGroup

	for _, group := range groups {
		var changedIDs []string
		for _, ref := range group.Items {
			if ref.Task == nil || ref.Task.ID == "" {
				continue
			}
			if mutate(group.Project, ref.Task.ID) {
				total++
				changedIDs = append(changedIDs, ref.Task.ID)
			}
		}
		if len(changedIDs) == 0 {
			continue
		}
		changedGroups = append(changedGroups, changedTaskProjectGroup{group: group, taskIDs: changedIDs})
	}

	groupID, groupProjectIDs := workflowGroupMetadata(taskProjectGroupsFromChanged(changedGroups))
	for _, changed := range changedGroups {
		opts := BuildWorkflowOpts(
			m.appPersister(),
			changed.group.Project,
			m.config,
			m.configManager,
			changed.prevStates,
			undoAction,
			fmt.Sprintf("%d tasks", len(changed.taskIDs)),
			changed.taskIDs,
		)
		opts.UndoGroupID = groupID
		opts.UndoGroupProjectIDs = groupProjectIDs
		if err := persistWithWorkflow(opts); err != nil {
			return total, groups, err
		}
	}

	return total, groups, nil
}

func (m *AppModel) clearBulkTaskSelectionsForRefs(refs []TaskProjectRef) {
	selectedIDs := taskIDSetFromRefs(refs)
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
		ViewFocusBoard,
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
	case *FocusBoardModel:
		clearMultiSelectStateForTaskIDs(&v.MultiSelectState, selectedIDs)
	case *KanbanModel:
		clearMultiSelectStateForTaskIDs(&v.MultiSelectState, selectedIDs)
	case *OutlinerModel:
		if multiSelectStateHasTaskID(&v.MultiSelectState, selectedIDs) {
			v.clearMultiSelect()
		}
	}
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
