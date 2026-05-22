package tui

import (
	tea "charm.land/bubbletea/v2"
)

// handleAttrFilterMessages handles attribute filter picker messages.
func (m *AppModel) handleAttrFilterMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case ShowAttrFilterPickerMsg:
		m.showView(ViewAttrFilterPicker, NewAttrFilterPickerModel(msg))
		return true, nil

	case AttrFilterPickerDoneMsg:
		m.popView()
		if msg.Canceled {
			return true, nil
		}
		m.applyAttrFilterToView(msg.Source, msg.Prefix, msg.Key)
		return true, nil
	}
	return false, nil
}

// applyAttrFilterToView applies an attribute filter selection to the originating view.
func (m *AppModel) applyAttrFilterToView(source ViewType, prefix, key string) {
	applyAttr := func(f *CommonFilterState, refresh func()) {
		if prefix == "Fa" {
			handleFilterAttrKey(key, f)
		} else {
			handleFilterAttrSubKey(prefix, key, f)
		}
		refresh()
	}

	// Views implementing tagFilterApplicable (have getFilter() + refreshAndClamp()).
	standardViews := []ViewType{
		ViewFocusView, ViewFocusBoard, ViewDueDateOverview,
		ViewDailyReview, ViewWeeklyReview, ViewRecentActivity, ViewPriorityView,
		ViewAllTasks, ViewKanban, ViewCrossProjectKanban,
	}
	for _, vt := range standardViews {
		if source != vt {
			continue
		}
		if v := m.views.Get(vt); v != nil {
			if cfv, ok := v.(tagFilterApplicable); ok {
				applyAttr(cfv.getFilter(), cfv.refreshAndClamp)
			}
		}
		return
	}

	switch source {
	case ViewProjectTree:
		if m.projectTree != nil {
			applyAttr(&m.projectTree.filter.CommonFilterState, func() {
				m.projectTree.updateVisibility()
				m.projectTree.clampCursor()
			})
		}
	case ViewProjectFocusView:
		if v := m.views.Get(ViewProjectFocusView); v != nil {
			if pv, ok := v.(*ProjectFocusViewModel); ok {
				applyAttr(&pv.filter, func() {
					pv.loadTasks()
					pv.clampCursor()
				})
			}
		}
	case ViewOutliner:
		target := m.outliner
		if target == nil {
			if v := m.views.Get(ViewTemporaryOutliner); v != nil {
				if o, ok := v.(*OutlinerModel); ok {
					target = o
				}
			}
		}
		if target != nil {
			applyAttr(&target.filter.CommonFilterState, func() {
				target.rebuildRows()
				target.clampCursor()
			})
		}
	case ViewCrossProjectOutliner:
		if m.crossOutliner != nil {
			applyAttr(&m.crossOutliner.filter.CommonFilterState, func() {
				m.crossOutliner.rebuildRows()
				m.crossOutliner.clampCursor()
			})
		}
	default:
	}
}
