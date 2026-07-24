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
func (m *AppModel) applyAttrFilterToView(source PresentationID, prefix, key string) {
	applyAttr := func(f *CommonFilterState, refresh func()) {
		if prefix == "Fa" {
			handleFilterAttrKey(key, f)
		} else {
			handleFilterAttrSubKey(prefix, key, f)
		}
		refresh()
	}
	if outliner, _, ok := m.outlinerPrimary(); ok {
		applyAttr(&outliner.filter.CommonFilterState, outliner.rebuildRows)
		return
	}

	// Views implementing tagFilterApplicable (have getFilter() + refreshAndClamp()).
	standardViews := []PresentationID{
		ViewFocusBoard,
		ViewDailyReview, ViewWeeklyReview, ViewRecentActivity,
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
	case ViewOutliner:
		if target := m.outliner; target != nil {
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
