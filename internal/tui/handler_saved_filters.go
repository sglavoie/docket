package tui

import (
	"maps"

	tea "charm.land/bubbletea/v2"
	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// handleSavedFilterMessages handles saved filter messages.
// Returns true if the message was handled.
func (m *AppModel) handleSavedFilterMessages(msg tea.Msg) (bool, tea.Cmd) { //nolint:gocyclo
	switch msg := msg.(type) {
	case ShowSaveFilterDialogMsg:
		return m.showViewInit(ViewSaveFilterInput, NewSaveFilterInputModel(m.buildCurrentViewSummary()))

	case SaveFilterMsg:
		sf := m.captureCurrentFilter(msg.Name)
		// Saved filters are planning-domain state; commit only the planning
		// domain under a checked commit rather than saving the whole config.
		if err := m.commitPlanningMutation(func(p *config.PlanningState) {
			p.SaveFilter(sf)
		}); err != nil {
			return true, errorCmd("failed to save filter: %w", err)
		}
		// Pop the input dialog
		m.popView()
		// Set active saved filter indicator on the appropriate view
		if m.baseProjectView() == ViewCrossProjectOutliner && m.crossOutliner != nil {
			m.crossOutliner.activeSavedFilter = msg.Name
		} else if m.baseProjectView() == ViewOutliner && m.outliner != nil {
			m.outliner.activeSavedFilter = msg.Name
		}
		// Refresh the command palette if it's still showing
		m.refreshCommandPaletteItems()
		return true, nil

	case LoadSavedFilterMsg:
		if outliner, _, ok := m.outlinerPrimary(); ok {
			if outliner.project == nil {
				m.applySavedFilterToCrossOutliner(msg.Filter)
			} else {
				m.applySavedFilterToOutliner(msg.Filter)
			}
			return true, nil
		}
		if m.baseProjectView() == ViewCrossProjectOutliner && m.crossOutliner != nil {
			m.applySavedFilterToCrossOutliner(msg.Filter)
		} else if m.baseProjectView() == ViewOutliner && m.outliner != nil {
			m.applySavedFilterToOutliner(msg.Filter)
		}
		return true, nil

	case DeleteSavedFilterMsg:
		if err := m.commitPlanningMutation(func(p *config.PlanningState) {
			p.DeleteSavedFilter(msg.ID)
		}); err != nil {
			return true, errorCmd("failed to delete saved filter: %w", err)
		}
		// Clear active indicator (filter already deleted, so just clear any indicator)
		if m.baseProjectView() == ViewCrossProjectOutliner && m.crossOutliner != nil {
			m.crossOutliner.activeSavedFilter = ""
			m.crossOutliner.activeCriteriaFilter = nil
		} else if m.baseProjectView() == ViewOutliner && m.outliner != nil {
			m.outliner.activeSavedFilter = ""
			m.outliner.activeCriteriaFilter = nil
		}
		// Refresh the command palette if it's showing
		m.refreshCommandPaletteItems()
		return true, nil
	}
	return false, nil
}

// refreshCommandPaletteItems reloads the command palette's domain-native items
// when it is the current view.
func (m *AppModel) refreshCommandPaletteItems() {
	if m.currentView() != ViewCommandPalette {
		return
	}
	if palette, ok := m.views.Get(ViewCommandPalette).(*CommandPaletteModel); ok {
		palette.allItems = palette.loadAllItems()
		palette.filterItems()
	}
}

// captureCurrentFilter creates a SavedFilter from the current view's state.
// Captures from outliner when it's the active base view, otherwise from project tree.
func (m *AppModel) captureCurrentFilter(name string) config.SavedFilter {
	// Check if a filter with this name already exists (for overwrite/upsert).
	// Saved filters are planning-domain state, read from the cached planning
	// snapshot rather than the config.Config facade.
	existing := m.runtime.Planning().GetSavedFilterByName(name)
	id := uuid.New().String()
	if existing != nil {
		id = existing.ID
	}
	if outliner, _, ok := m.outlinerPrimary(); ok {
		return m.captureOutlinerFilterFrom(name, id, outliner)
	}

	switch m.baseProjectView() { //nolint:exhaustive // saved filters are captured from the outliner base views.
	case ViewCrossProjectOutliner:
		if m.crossOutliner != nil {
			return m.captureOutlinerFilterFrom(name, id, m.crossOutliner)
		}
	}
	return m.captureOutlinerFilter(name, id)
}

// captureOutlinerFilter creates a SavedFilter from the outliner state.
func (m *AppModel) captureOutlinerFilter(name, id string) config.SavedFilter {
	return m.captureOutlinerFilterFrom(name, id, m.outliner)
}

func (m *AppModel) captureOutlinerFilterFrom(name, id string, ol *OutlinerModel) config.SavedFilter {
	sf := config.SavedFilter{
		ID:   id,
		Name: name,
		Mode: "snapshot",
	}
	if ol == nil {
		return sf
	}

	sf.FilterType = string(ol.filter.Type)
	sf.SetFilterStates(ol.filter.States, ol.filter.hasStateFilter())
	sf.FilterPriority = ol.filter.Priority
	sf.FilterDueDate = int(ol.filter.DueDate)
	sf.FilterActionable = ol.filter.Actionable
	sf.FilterHideDimmed = ol.filter.HideDimmed
	sf.FilterGitHubIssue = ol.filter.GitHubIssue
	sf.FilterParked = ol.filter.Parked
	sf.FilterScoreMin = ol.filter.ScoreMin

	if len(ol.filter.Tags) > 0 {
		for tag := range ol.filter.Tags {
			sf.FilterTags = append(sf.FilterTags, tag)
		}
	}
	if len(ol.filter.Attrs) > 0 {
		sf.FilterAttrs = make(map[string]string)
		maps.Copy(sf.FilterAttrs, ol.filter.Attrs)
	}

	return sf
}

// applySavedFilterToOutliner applies a saved filter's state to the outliner.
func (m *AppModel) applySavedFilterToOutliner(sf config.SavedFilter) {
	m.applySavedFilterToOutlinerModel(m.outliner, sf)
}

func (m *AppModel) applySavedFilterToCrossOutliner(sf config.SavedFilter) {
	m.applySavedFilterToOutlinerModel(m.crossOutliner, sf)
}

func (m *AppModel) applySavedFilterToOutlinerModel(ol *OutlinerModel, sf config.SavedFilter) {
	if ol == nil {
		return
	}

	clearCommonFilters(&ol.filter.CommonFilterState)
	if sf.Mode == "criteria" {
		// Criteria-mode: apply per-task criteria matching, clear snapshot filters
		ol.activeCriteriaFilter = &sf
	} else {
		// Snapshot-mode: restore all flat filter/sort state
		ol.activeCriteriaFilter = nil
		ol.filter.Type = models.TaskType(sf.FilterType)
		ol.filter.setStateFilter(sf.FilterStates, sf.HasStateFilter())
		ol.filter.Priority = sf.FilterPriority
		ol.filter.DueDate = DueDateFilter(sf.FilterDueDate)
		ol.filter.Actionable = sf.FilterActionable
		ol.filter.HideDimmed = sf.FilterHideDimmed
		ol.filter.GitHubIssue = sf.FilterGitHubIssue
		ol.filter.Parked = sf.FilterParked
		ol.filter.ScoreMin = sf.FilterScoreMin

		// Restore tags — validate against the active outliner scope.
		validTags := CollectTagNamesForProject(ol.project)
		if ol.project == nil {
			validTags = CollectTagNamesForProjects(ol.loadedProjects)
		}
		ol.filter.Tags = validateSavedStringSet(sf.FilterTags, validTags)

		// Restore attributes
		ol.filter.Attrs = make(map[string]string)
		maps.Copy(ol.filter.Attrs, sf.FilterAttrs)

	}

	// Set active indicator
	ol.activeSavedFilter = sf.Name

	// Rebuild with new sort/filter
	ol.rebuildRows()
}

// buildCurrentViewSummary returns a human-readable summary of the current filter state.
func (m *AppModel) buildCurrentViewSummary() string {
	switch m.baseProjectView() {
	case ViewCrossProjectOutliner:
		return m.buildOutlinerSummaryFrom(m.crossOutliner)
	case ViewOutliner:
		return m.buildOutlinerViewSummary()
	default:
		return m.buildOutlinerViewSummary()
	}
}

// buildOutlinerViewSummary returns a summary of the outliner's filter state.
func (m *AppModel) buildOutlinerViewSummary() string {
	return m.buildOutlinerSummaryFrom(m.outliner)
}

func (m *AppModel) buildOutlinerSummaryFrom(ol *OutlinerModel) string {
	if ol == nil {
		return ""
	}

	var tags []string
	for tag := range ol.filter.Tags {
		tags = append(tags, tag)
	}
	return describeSnapshotFilters(
		string(ol.filter.Type),
		ol.filter.States,
		ol.filter.hasStateFilter(),
		ol.filter.Priority,
		tags,
		int(ol.filter.DueDate),
		ol.filter.Actionable,
		ol.filter.HideDimmed,
		ol.filter.GitHubIssue,
		ol.filter.Parked,
		ol.filter.Attrs,
		ol.filter.ScoreMin,
		int(ol.sort.Field),
		ol.activePerspective,
	)
}
