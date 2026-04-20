package tui

import "strings"

// renderKanbanTab renders the Kanban settings tab content.
// Focus indices 94–104:
//
//	94  Default Mode
//	95  Default Start Column
//	96  Show Empty Columns
//	97  Detail Pane Visible
//	98  Detail Pane Height
//	99  Enabled: Section (GTD)
//	100 Enabled: Status
//	101 Enabled: Priority
//	102 Enabled: Stages
//	103 Enabled: Custom Sections
//	104 Enabled: Score Buckets
func (m *SettingsModel) renderKanbanTab() string {
	var b strings.Builder
	const labelWidth = 22
	const indent = 24
	base := tabFieldStart[tabKanban]

	// Default Mode
	b.WriteString(renderFormLabelWithWidth("Default Mode: ", labelWidth, m.focusIndex == base+0))
	b.WriteString(renderEnumSelector(kanbanModeOptions, m.kanbanDefaultModeCursor, m.focusIndex == base+0))
	b.WriteString("\n")
	b.WriteString(renderFormFieldDesc("Column axis used when opening kanban", indent, m.focusIndex == base+0))
	b.WriteString("\n")

	// Default Start Column
	b.WriteString(renderFormLabelWithWidth("Start Column: ", labelWidth, m.focusIndex == base+1))
	b.WriteString(m.kanbanStartColInput.View())
	b.WriteString("\n")
	b.WriteString(renderFormFieldDesc("Column ID to focus on open (e.g. inbox, next, todo)", indent, m.focusIndex == base+1))
	b.WriteString("\n")

	// Show Empty Columns
	b.WriteString(renderFormLabelWithWidth("Empty Columns: ", labelWidth, m.focusIndex == base+2))
	b.WriteString(renderFormToggle("Show", "Hide", m.kanbanShowEmptyColumns, m.focusIndex == base+2))
	b.WriteString("\n")
	b.WriteString(renderFormFieldDesc("Show columns with no tasks", indent, m.focusIndex == base+2))
	b.WriteString("\n")

	// Detail Pane Visible
	b.WriteString(renderFormLabelWithWidth("Detail Pane: ", labelWidth, m.focusIndex == base+3))
	b.WriteString(renderFormToggle("Visible", "Hidden", m.kanbanDetailPaneVisible, m.focusIndex == base+3))
	b.WriteString("\n")
	b.WriteString(renderFormFieldDesc("Show bottom detail pane by default (Tab toggles)", indent, m.focusIndex == base+3))
	b.WriteString("\n")

	// Detail Pane Height
	b.WriteString(renderFormLabelWithWidth("Detail Pane Height: ", labelWidth, m.focusIndex == base+4))
	b.WriteString(renderEnumSelector(kanbanDetailHeightOptions, m.kanbanDetailHeightCursor, m.focusIndex == base+4))
	b.WriteString("\n")
	b.WriteString(renderFormFieldDesc("Number of lines for the detail pane (1–5)", indent, m.focusIndex == base+4))
	b.WriteString("\n")

	// Enabled Modes (offsets 5–10)
	b.WriteString(MutedStyle.Render("Enabled Axis Modes"))
	b.WriteString("\n")
	for i, label := range kanbanModeLabels {
		focusIdx := base + 5 + i
		b.WriteString(renderFormLabelWithWidth(label+": ", labelWidth, m.focusIndex == focusIdx))
		b.WriteString(renderFormToggle("Enabled", "Disabled", m.kanbanEnabledModes[i], m.focusIndex == focusIdx))
		b.WriteString("\n")
	}
	return b.String()
}
