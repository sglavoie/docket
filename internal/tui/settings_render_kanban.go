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

	// Default Mode (94)
	b.WriteString(renderFormLabelWithWidth("Default Mode: ", labelWidth, m.focusIndex == 94))
	b.WriteString(renderEnumSelector(kanbanModeOptions, m.kanbanDefaultModeCursor, m.focusIndex == 94))
	b.WriteString("\n")
	b.WriteString(renderFormFieldDesc("Column axis used when opening kanban", indent, m.focusIndex == 94))
	b.WriteString("\n")

	// Default Start Column (95)
	b.WriteString(renderFormLabelWithWidth("Start Column: ", labelWidth, m.focusIndex == 95))
	b.WriteString(m.kanbanStartColInput.View())
	b.WriteString("\n")
	b.WriteString(renderFormFieldDesc("Column ID to focus on open (e.g. inbox, next, todo)", indent, m.focusIndex == 95))
	b.WriteString("\n")

	// Show Empty Columns (96)
	b.WriteString(renderFormLabelWithWidth("Empty Columns: ", labelWidth, m.focusIndex == 96))
	b.WriteString(renderFormToggle("Show", "Hide", m.kanbanShowEmptyColumns, m.focusIndex == 96))
	b.WriteString("\n")
	b.WriteString(renderFormFieldDesc("Show columns with no tasks", indent, m.focusIndex == 96))
	b.WriteString("\n")

	// Detail Pane Visible (97)
	b.WriteString(renderFormLabelWithWidth("Detail Pane: ", labelWidth, m.focusIndex == 97))
	b.WriteString(renderFormToggle("Visible", "Hidden", m.kanbanDetailPaneVisible, m.focusIndex == 97))
	b.WriteString("\n")
	b.WriteString(renderFormFieldDesc("Show bottom detail pane by default (Tab toggles)", indent, m.focusIndex == 97))
	b.WriteString("\n")

	// Detail Pane Height (98)
	b.WriteString(renderFormLabelWithWidth("Detail Pane Height: ", labelWidth, m.focusIndex == 98))
	b.WriteString(renderEnumSelector(kanbanDetailHeightOptions, m.kanbanDetailHeightCursor, m.focusIndex == 98))
	b.WriteString("\n")
	b.WriteString(renderFormFieldDesc("Number of lines for the detail pane (1–5)", indent, m.focusIndex == 98))
	b.WriteString("\n")

	// Enabled Modes (99–104)
	b.WriteString(MutedStyle.Render("Enabled Axis Modes"))
	b.WriteString("\n")
	for i, label := range kanbanModeLabels {
		focusIdx := 99 + i
		b.WriteString(renderFormLabelWithWidth(label+": ", labelWidth, m.focusIndex == focusIdx))
		b.WriteString(renderFormToggle("Enabled", "Disabled", m.kanbanEnabledModes[i], m.focusIndex == focusIdx))
		b.WriteString("\n")
	}
	b.WriteString(renderFormFieldDesc("ctrl+g cycles through enabled modes only", indent, m.focusIndex >= 99 && m.focusIndex <= 104))

	return b.String()
}
