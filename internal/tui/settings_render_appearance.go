package tui

import "strings"

func (m *SettingsModel) renderAppearanceTab() string {
	var b strings.Builder
	const labelWidth = 20
	const indent = 22
	base := tabFieldStart[tabAppearance]

	b.WriteString(renderFormLabelWithWidth("Zebra Striping: ", labelWidth, m.focusIndex == base+0))
	b.WriteString(renderFormToggle("Enabled", "Disabled", m.zebraStriping, m.focusIndex == base+0))
	b.WriteString("\n")
	b.WriteString(renderFormFieldDesc("Alternating row backgrounds in task list views", indent, m.focusIndex == base+0))

	b.WriteString("\n")

	b.WriteString(renderFormLabelWithWidth("Week-End Day: ", labelWidth, m.focusIndex == base+1))
	b.WriteString(renderEnumSelector(weekEndDayOptions, m.weekEndDayCursor, m.focusIndex == base+1, m.width-indent))
	b.WriteString("\n")
	b.WriteString(renderFormFieldDesc("Last day of the week for the 'eow' date token", indent, m.focusIndex == base+1))

	b.WriteString("\n")

	b.WriteString(renderFormLabelWithWidth("Time Format: ", labelWidth, m.focusIndex == base+2))
	b.WriteString(renderEnumSelector(timeFormatOptions, m.timeFormatCursor, m.focusIndex == base+2, m.width-indent))
	b.WriteString("\n")
	b.WriteString(renderFormFieldDesc("Clock format for tasks with a due time", indent, m.focusIndex == base+2))

	return b.String()
}
