package tui

import (
	tea "charm.land/bubbletea/v2"
)

// handleTagMessages handles tag-related messages.
// Returns true if the message was handled.
func (m *AppModel) handleTagMessages(msg tea.Msg) (bool, tea.Cmd) { //nolint:gocyclo
	switch msg := msg.(type) {
	case ShowTagSelectorMsg:
		selector := NewTagSelectorModel(msg.Task, msg.Project, msg.CurrentTags)
		m.showView(ViewTagSelector, selector)
		return true, nil

	case ShowBulkTagSelectorMsg:
		if refs := validTaskProjectRefs(msg.TaskRefs); len(refs) > 0 {
			selector := NewBulkTagSelectorModel(refs)
			m.showView(ViewBulkTagSelector, selector)
		}
		return true, nil

	case TagSelectorDoneMsg:
		m.popView()
		if msg.Canceled {
			return true, nil
		}
		// Check if we came from project tree, outliner, or cross-project outliner - apply tags directly
		if m.currentView() == ViewCrossProjectOutliner && msg.Task != nil {
			return true, m.applyTagsToTask(msg.Task, msg.SelectedTags)
		}
		// Otherwise forward to the current view.
		cmd := m.views.Update(m.currentView(), msg)
		return true, cmd

	case BulkTagSelectorDoneMsg:
		m.popView()
		if msg.Canceled {
			return true, nil
		}
		return true, m.applyTagsToTaskRefs(msg.TaskRefs, msg.SelectedTags)

	// Unified tag filter selector: single Show handler for all views
	case ShowTagFilterMsg:
		m.showView(ViewTagFilterSelector, NewTagFilterSelectorModel(msg))
		return true, nil

	// Unified tag filter done: dispatch by Source to the originating view
	case TagFilterDoneMsg:
		m.popView()
		if msg.Canceled {
			return true, nil
		}
		tags := make(map[string]bool)
		for _, tag := range msg.SelectedTags {
			tags[tag] = true
		}
		m.applyTagFilterToView(msg.Source, tags)
		return true, nil

	}
	return false, nil
}

// applyTagFilterToView dispatches a tag filter selection to the originating view.
func (m *AppModel) applyTagFilterToView(source PresentationID, tags map[string]bool) {
	switch source {
	case ViewCrossProjectOutliner:
		if m.crossOutliner != nil {
			m.crossOutliner.filter.Tags = tags
			m.crossOutliner.crossForceRebuild = true
			m.crossOutliner.rebuildRows()
			m.crossOutliner.clampCursor()
		}
	default:
	}
}
