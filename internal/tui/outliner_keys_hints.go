package tui

import (
	"fmt"
	"strings"

	"github.com/sglavoie/tt/internal/models"
)

type outlinerEmptyCause uint8

const (
	outlinerEmptyNoTasks outlinerEmptyCause = iota
	outlinerEmptyNoProjects
	outlinerEmptyMode
	outlinerEmptyFiltered
	outlinerEmptyProjectFiltered
	outlinerEmptySearch
	outlinerEmptyZoom
)

func (m *OutlinerModel) HandledKeys() []string {
	if m.operationRegistry == nil {
		return nil
	}
	return m.operationRegistry.HandledKeysForInputStrategy(InputStrategySimplified)
}

func (m *OutlinerModel) GetHelpItems() []HelpItem {
	if m.operationRegistry == nil {
		return nil
	}
	return m.operationRegistry.HelpItemsForInputStrategy(
		InputStrategySimplified,
		m.operationCommandContext(),
	)
}

// ContextualGuidance selects one Outliner-specific profile before falling back
// to the ranked operation projection for an ordinary task row. Interaction and
// causal recovery controls are required so narrow terminals retain the action
// that exits the current state.
func (m *OutlinerModel) ContextualGuidance() (GuidanceSpec, bool) {
	if m == nil {
		return GuidanceSpec{}, false
	}
	switch {
	case m.confirm.IsActive():
		return GuidanceSpec{Hints: requiredGuidanceHints("Y confirm", "N/Esc cancel"), Rows: 2}, true
	case m.quickEdit.IsActive():
		hints := requiredGuidanceHints("Enter save", "Esc cancel")
		if !m.quickEdit.IsRenameOnly() {
			hints = append(hints, optionalGuidanceHints(10,
				"!0-3 priority", "#tag tags", "@date due", "$state state",
			)...)
		}
		return GuidanceSpec{Hints: hints, Rows: 2}, true
	case m.IsSearchMode():
		return GuidanceSpec{Hints: requiredGuidanceHints("Enter apply search", "Esc clear search"), Rows: 2}, true
	case len(m.rows) == 0:
		return m.emptyGuidance(), true
	case m.multiSelectMode:
		return m.multiSelectGuidance(), true
	case m.peekMode:
		return GuidanceSpec{
			Hints: append(requiredGuidanceHints("Enter close Peek", "Esc close Peek"),
				optionalGuidanceHints(10, "Space actions", "e title", "? help")...),
			Rows: 2,
		}, true
	case m.hasSearchProjection():
		return GuidanceSpec{
			Hints: append(requiredGuidanceHints("Esc clear search"),
				optionalGuidanceHints(10, "/ edit search", "a capture", "p projects", "? help")...),
			Rows: 1,
		}, true
	case m.cursorNamesHeader():
		return m.headerGuidance(), true
	case m.operationRegistry == nil:
		return GuidanceSpec{}, false
	}

	hints := m.operationRegistry.GuidanceHintsForInputStrategy(
		InputStrategySimplified,
		m.operationCommandContext(),
	)
	if len(hints) == 0 {
		return GuidanceSpec{}, false
	}
	return GuidanceSpec{Hints: hints, Rows: 1}, true
}

func (m *OutlinerModel) multiSelectGuidance() GuidanceSpec {
	rangeAction := "V range"
	if m.IsRangeSelectionActive() {
		rangeAction = "V freeze range"
	}
	hints := []GuidanceHint{
		newGuidanceHint("Space actions", 1, m.SelectedCount() > 0),
		newGuidanceHint("v toggle", 2, false),
		newGuidanceHint(rangeAction, 3, false),
		newGuidanceHint("Ctrl+A select visible", 4, false),
		newGuidanceHint("Esc clear selection", 0, true),
		newGuidanceHint("? help", 10, false),
	}
	return GuidanceSpec{Hints: hints, Rows: 1}
}

func (m *OutlinerModel) cursorNamesHeader() bool {
	if m.cursor < 0 || m.cursor >= len(m.rows) {
		return false
	}
	row := m.rows[m.cursor]
	return row.IsSectionHeader || row.IsProjectHeader
}

func (m *OutlinerModel) headerGuidance() GuidanceSpec {
	row := m.rows[m.cursor]
	hints := make([]GuidanceHint, 0, 6)
	if m.isSectionRowCollapsed(row) {
		hints = append(hints, newGuidanceHint("l/Right expand", 1, false))
	} else {
		hints = append(hints, newGuidanceHint("h/Left collapse", 1, false))
	}
	if row.IsProjectHeader {
		hints = append(hints, newGuidanceHint("# project tags", 2, false))
	} else {
		hints = append(hints, newGuidanceHint("S sections", 2, false))
	}
	hints = append(hints, optionalGuidanceHints(3, "Tab next mode", "p projects", "/ search", "? help")...)
	return GuidanceSpec{Hints: hints, Rows: 1}
}

func (m *OutlinerModel) emptyGuidance() GuidanceSpec {
	var required string
	switch m.classifyEmptyCause() {
	case outlinerEmptyNoProjects:
		required = "p projects"
	case outlinerEmptyMode:
		if m.primaryMode == ModeInbox {
			required = "a capture"
		} else {
			required = "Tab next mode"
		}
	case outlinerEmptyFiltered:
		required = "fx clear filters"
	case outlinerEmptyProjectFiltered:
		required = "Fx clear project filters"
	case outlinerEmptySearch:
		required = "Esc clear search"
	case outlinerEmptyZoom:
		required = "Esc zoom out"
	default:
		required = "a capture"
	}
	optional := []string{"p projects", "? help"}
	if strings.HasPrefix(required, "p ") {
		optional = []string{"? help"}
	}
	hints := append(requiredGuidanceHints(required), optionalGuidanceHints(10, optional...)...)
	return GuidanceSpec{Hints: hints, Rows: 1}
}

func (m *OutlinerModel) classifyEmptyCause() outlinerEmptyCause {
	if m.hasSearchProjection() && m.SearchQuery() != "" {
		return outlinerEmptySearch
	}
	if m.project == nil && len(m.loadedProjects) == 0 {
		if hasActiveProjectFilters(&m.projectFilter) {
			return outlinerEmptyProjectFiltered
		}
		return outlinerEmptyNoProjects
	}
	if hasActiveCommonFilters(&m.filter.CommonFilterState) {
		return outlinerEmptyFiltered
	}
	if len(m.zoomStack) > 0 || len(m.crossZoomStack) > 0 {
		return outlinerEmptyZoom
	}
	if m.primaryMode != ModeAll {
		return outlinerEmptyMode
	}
	return outlinerEmptyNoTasks
}

// decoratePendingChordHint adds effective filter state to the existing live
// registry entry. It never creates or reorders a key, so which-key and dispatch
// continue sharing the same metadata and applicability checks.
func (m *OutlinerModel) decoratePendingChordHint(path []string, hint ChordHint) ChordHint {
	if m == nil || len(path) != 2 {
		return hint
	}
	switch path[0] {
	case "f":
		filter := &m.filter.CommonFilterState
		switch path[1] {
		case "p":
			if filter.Priority > 0 {
				hint.Label = "priority:" + strings.ToLower(models.PriorityName(filter.Priority)) + "+"
			}
		case "s":
			if filter.hasStateFilter() {
				hint.Label = fmt.Sprintf("states:%d", len(filter.States.Values()))
			}
		case "c":
			if filter.hasCompletionAgeFilter() {
				hint.Label = "completed:all"
				if days := filter.CompletionAge.days(); days > 0 {
					hint.Label = fmt.Sprintf("completed:%dd+", days)
				}
			}
		case "d":
			if filter.DueDate != DueDateAll {
				hint.Label = "due:" + dueDateFilterLabel(filter.DueDate)
			}
		case "t":
			if len(filter.Tags) > 0 {
				hint.Label = fmt.Sprintf("tags:%d", len(filter.Tags))
			}
		case "r":
			if filter.Tags[reviewTaskTag] {
				hint.Label = "review:on"
			}
		case "z":
			if filter.Snoozed {
				hint.Label = "snoozed:on"
			}
		case "x":
			if count := commonFilterFacetCount(filter); count > 0 {
				hint.Label = fmt.Sprintf("clear:%d", count)
			}
		}
	case "F":
		return m.decorateProjectFilterChordHint(path[1], hint)
	}
	return hint
}

// decorateProjectFilterChordHint decorates one F continuation. Each population
// key only reports the mode it owns, so the other key of the same axis keeps
// its static affordance instead of both rows naming one state.
func (m *OutlinerModel) decorateProjectFilterChordHint(key string, hint ChordHint) ChordHint {
	filter := &m.projectFilter
	switch key {
	case "t":
		if selected := projectFilterSelection(filter); len(selected) > 0 {
			hint.Label = fmt.Sprintf("project tags:%d", len(selected))
		}
	case "p":
		if filter.ParkedMode == projectPopulationInclude {
			hint.Label = "parked:on"
		}
	case "P":
		if filter.ParkedMode == projectPopulationOnly {
			hint.Label = "parked:only"
		}
	case "a":
		if filter.ArchivedMode == projectPopulationInclude {
			hint.Label = "archived:on"
		}
	case "A":
		if filter.ArchivedMode == projectPopulationOnly {
			hint.Label = "archived:only"
		}
	case "x":
		if count := projectFilterFacetCount(filter); count > 0 {
			hint.Label = fmt.Sprintf("clear:%d", count)
		}
	}
	return hint
}
