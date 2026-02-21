package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// handleSavedViewMessages handles saved view messages.
// Returns true if the message was handled.
func (m *AppModel) handleSavedViewMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case ShowSaveViewDialogMsg:
		summary := m.buildCurrentViewSummary()
		dialog := NewSaveViewInputModel(summary)
		m.showView(ViewSaveViewInput, dialog)
		return true, dialog.Init()

	case SaveViewMsg:
		view := m.captureCurrentView(msg.Name)
		m.config.SaveView(view)
		if err := m.configManager.Save(m.config); err != nil {
			return true, errorCmd("failed to save view: %w", err)
		}
		// Pop the input dialog
		m.popView()
		// Set active saved view indicator
		if m.projectTree != nil {
			m.projectTree.activeSavedView = msg.Name
		}
		// Refresh the command palette if it's still showing
		if m.currentView() == ViewCommandPalette {
			if palette, ok := m.views.Get(ViewCommandPalette).(*CommandPaletteModel); ok {
				palette.allItems = palette.loadAllItems()
				palette.filterItems()
			}
		}
		return true, nil

	case LoadSavedViewMsg:
		if m.projectTree != nil {
			m.applySavedView(msg.View)
		}
		return true, nil

	case DeleteSavedViewMsg:
		m.config.DeleteSavedView(msg.Name)
		if err := m.configManager.Save(m.config); err != nil {
			return true, errorCmd("failed to delete saved view: %w", err)
		}
		// Clear active indicator if this was the active view
		if m.projectTree != nil && m.projectTree.activeSavedView == msg.Name {
			m.projectTree.activeSavedView = ""
		}
		// Refresh the command palette if it's showing
		if m.currentView() == ViewCommandPalette {
			if palette, ok := m.views.Get(ViewCommandPalette).(*CommandPaletteModel); ok {
				palette.allItems = palette.loadAllItems()
				palette.filterItems()
			}
		}
		return true, nil
	}
	return false, nil
}

// captureCurrentView creates a SavedView from the current project tree state.
func (m *AppModel) captureCurrentView(name string) config.SavedView {
	view := config.SavedView{Name: name}
	if m.projectTree == nil {
		return view
	}

	pt := m.projectTree
	view.ViewMode = int(pt.viewMode)
	view.FilterType = string(pt.filter.Type)
	view.FilterStatus = string(pt.filter.Status)
	view.FilterPriority = pt.filter.Priority
	view.FilterDueDate = int(pt.filter.DueDate)
	view.FilterActionable = pt.filter.Actionable
	view.FilterScoreMin = pt.filter.ScoreMin
	view.SortField = int(pt.sortField)
	view.SortAscending = pt.sortAscending

	if len(pt.filter.Tags) > 0 {
		for tag := range pt.filter.Tags {
			view.FilterTags = append(view.FilterTags, tag)
		}
	}
	if len(pt.filter.Sections) > 0 {
		for sec := range pt.filter.Sections {
			view.FilterSections = append(view.FilterSections, sec)
		}
	}
	if len(pt.filter.Attrs) > 0 {
		view.FilterAttrs = make(map[string]string)
		for k, v := range pt.filter.Attrs {
			view.FilterAttrs[k] = v
		}
	}

	// Capture custom section tab name
	if tabs := pt.customSectionTabs(); tabs != nil && pt.customSectionTab < len(tabs) {
		view.CustomSectionTab = tabs[pt.customSectionTab]
	}

	return view
}

// applySavedView applies a saved view's filter/sort state to the project tree.
func (m *AppModel) applySavedView(view config.SavedView) {
	pt := m.projectTree
	if pt == nil {
		return
	}

	pt.viewMode = ViewMode(view.ViewMode)
	pt.filter.Type = models.TaskType(view.FilterType)
	pt.filter.Status = models.TaskStatus(view.FilterStatus)
	pt.filter.Priority = view.FilterPriority
	pt.filter.DueDate = DueDateFilter(view.FilterDueDate)
	pt.filter.Actionable = view.FilterActionable
	pt.filter.ScoreMin = view.FilterScoreMin
	pt.sortField = SortField(view.SortField)
	pt.sortAscending = view.SortAscending

	// Restore tags — validate against current project
	pt.filter.Tags = validateSavedStringSet(view.FilterTags, pt.getAllTags())

	// Restore sections — validate against current project
	pt.filter.Sections = make(map[string]bool)
	if pt.project != nil {
		existingSections := pt.project.GetSections()
		sectionSet := make(map[string]bool)
		for _, s := range existingSections {
			sectionSet[s] = true
		}
		for _, section := range view.FilterSections {
			if sectionSet[section] {
				pt.filter.Sections[section] = true
			}
		}
	}

	// Restore attributes
	pt.filter.Attrs = make(map[string]string)
	for k, v := range view.FilterAttrs {
		pt.filter.Attrs[k] = v
	}

	// Restore custom section tab
	if view.CustomSectionTab != "" {
		if tabs := pt.customSectionTabs(); tabs != nil {
			for i, tab := range tabs {
				if tab == view.CustomSectionTab {
					pt.customSectionTab = i
					break
				}
			}
		}
	}

	// Set active indicator
	pt.activeSavedView = view.Name

	// Rebuild tree with new sort/filter
	pt.sortNodes()
	pt.updateVisibility()
	pt.pruneInvisibleSelections()
	pt.clampCursor()
}

// buildCurrentViewSummary returns a human-readable summary of the current filter state.
func (m *AppModel) buildCurrentViewSummary() string {
	if m.projectTree == nil {
		return ""
	}
	pt := m.projectTree

	var tags []string
	for tag := range pt.filter.Tags {
		tags = append(tags, tag)
	}
	var sections []string
	for sec := range pt.filter.Sections {
		sections = append(sections, sec)
	}

	return describeSavedViewFilters(
		string(pt.filter.Type),
		string(pt.filter.Status),
		pt.filter.Priority,
		tags,
		sections,
		int(pt.filter.DueDate),
		pt.filter.Actionable,
		pt.filter.Attrs,
		pt.filter.ScoreMin,
		int(pt.sortField),
		int(pt.viewMode),
	)
}
