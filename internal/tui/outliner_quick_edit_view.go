package tui

import "strings"

// clearActiveSavedFilter clears the active saved filter indicator and any
// criteria filter. Called when the user manually changes filter/sort state.
func (m *OutlinerModel) clearActiveSavedFilter() {
	m.activeSavedFilter = ""
	m.activeCriteriaFilter = nil
}

// ActiveFilterCount returns the number of active filters for the status bar pill.
func (m *OutlinerModel) ActiveFilterCount() int {
	return len(m.activeFilterParts())
}

// activeFilterParts returns the list of active filter description strings.
func (m *OutlinerModel) activeFilterParts() []string {
	var parts []string

	// Show saved filter badge if active
	if m.activeSavedFilter != "" {
		parts = append(parts, SavedFilterBadgeStyle.Render("View: "+m.activeSavedFilter))
	}

	// Add common filter parts (type, status, priority, tags, due date, attrs, score)
	parts = append(parts, commonFilterParts(&m.filter.CommonFilterState)...)

	// Add view-specific filter parts
	if len(m.filter.Sections) > 0 {
		var secNames []string
		for sec := range m.filter.Sections {
			secNames = append(secNames, "@"+sec)
		}
		parts = append(parts, strings.Join(secNames, " "))
	}
	if m.filter.Actionable {
		parts = append(parts, "ready")
	}

	return parts
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

	helpHint := MutedStyle.Render("Enter=save  Esc=cancel")
	if !m.quickEdit.IsRenameOnly() {
		helpHint = MutedStyle.Render("!1-5=priority #tag @date /section $status · Enter=save Esc=cancel")
	}
	lines = append(lines, helpHint)

	return strings.Join(lines, "\n")
}
