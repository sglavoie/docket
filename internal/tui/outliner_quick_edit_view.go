package tui

import (
	"strings"
)

// ActiveFilterCount returns the number of active filters for the status bar pill.
func (m *OutlinerModel) ActiveFilterCount() int {
	return commonFilterFacetCount(&m.filter.CommonFilterState) +
		projectFilterFacetCount(&m.projectFilter)
}

// activeFilterParts returns the list of active filter description strings
// (state, priority, tags, due date, completion age, snoozed, and pinned).
func (m *OutlinerModel) activeFilterParts() []string {
	return commonFilterParts(&m.filter.CommonFilterState)
}

// renderQuickEditBar renders the quick edit input bar at the bottom of the view.
func (m *OutlinerModel) renderQuickEditBar() string {
	if !m.quickEdit.IsActive() {
		return ""
	}

	var lines []string

	mode := "Quick Edit"
	if m.quickEdit.IsRenameOnly() {
		mode = "Rename"
	}
	modeLabel := TitleStyle.Render(mode + ":")

	inputLine := modeLabel + " " + m.quickEdit.GetInput().View()
	lines = append(lines, inputLine)

	// Preview line (only for full quick edit mode)
	if !m.quickEdit.IsRenameOnly() {
		preview := m.quickEdit.GetPreview()
		original := m.quickEdit.GetOriginal()
		if preview != nil && original != nil {
			previewStr := renderQuickEditPreview(preview, original)
			if previewStr != "" {
				lines = append(lines, MutedStyle.Render(previewStr))
			}
		}
	}

	return strings.Join(lines, "\n")
}
