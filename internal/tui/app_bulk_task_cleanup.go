package tui

import (
	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

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
	set := make(map[taskref.TaskIdentity]bool, len(locators))
	for _, locator := range locators {
		identity := taskref.FromLocator(locator)
		if identity.Valid() {
			set[identity] = true
		}
	}
	clearOutliner := func(outlinerView *outliner.OutlinerModel) {
		if outlinerView == nil {
			return
		}
		for identity := range set {
			delete(outlinerView.Selected, identity)
		}
		if outlinerView.SelectedCount() == 0 {
			outlinerView.ClearMultiSelect()
		}
	}
	clearOutliner(m.crossOutliner)
	if m.views != nil {
		if view := m.views.Get(presentation.ViewCrossProjectOutliner); view != nil {
			if outlinerView, ok := view.(*outliner.OutlinerModel); ok {
				clearOutliner(outlinerView)
			}
		}
	}
}

func (m *AppModel) clearBulkTaskSelectionsForTaskIDSet(selectedIDs map[string]bool) {
	if len(selectedIDs) == 0 {
		return
	}

	if m.crossOutliner != nil && multiSelectStateHasTaskID(&m.crossOutliner.MultiSelectState, selectedIDs) {
		m.crossOutliner.ClearMultiSelect()
	}

	m.clearViewBulkTaskSelectionForRefs(presentation.ViewCrossProjectOutliner, selectedIDs)
}

func (m *AppModel) clearViewBulkTaskSelectionForRefs(vt presentation.PresentationID, selectedIDs map[string]bool) {
	if m.views == nil {
		return
	}
	view := m.views.Get(vt)
	switch v := view.(type) {
	case *outliner.OutlinerModel:
		if multiSelectStateHasTaskID(&v.MultiSelectState, selectedIDs) {
			v.ClearMultiSelect()
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

func multiSelectStateHasTaskID(state *listsurface.MultiSelectState, selectedIDs map[string]bool) bool {
	if state == nil {
		return false
	}
	matchedProjects := make(map[string]string, len(selectedIDs))
	for identity := range state.Selected {
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
