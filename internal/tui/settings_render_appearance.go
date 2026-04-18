package tui

import "strings"

func (m *SettingsModel) renderAppearanceTab() string {
	var b strings.Builder
	const labelWidth = 20
	const indent = 22

	b.WriteString(renderFormLabelWithWidth("Zebra Striping: ", labelWidth, m.focusIndex == 91))
	b.WriteString(renderFormToggle("Enabled", "Disabled", m.zebraStriping, m.focusIndex == 91))
	b.WriteString("\n")
	b.WriteString(renderFormFieldDesc("Alternating row backgrounds in task list views", indent, m.focusIndex == 91))

	b.WriteString("\n")

	b.WriteString(renderFormLabelWithWidth("Week-End Day: ", labelWidth, m.focusIndex == 92))
	b.WriteString(renderEnumSelector(weekEndDayOptions, m.weekEndDayCursor, m.focusIndex == 92))
	b.WriteString("\n")
	b.WriteString(renderFormFieldDesc("Last day of the week for the 'eow' date token", indent, m.focusIndex == 92))

	b.WriteString("\n")

	b.WriteString(renderFormLabelWithWidth("Time Format: ", labelWidth, m.focusIndex == 93))
	b.WriteString(renderEnumSelector(timeFormatOptions, m.timeFormatCursor, m.focusIndex == 93))
	b.WriteString("\n")
	b.WriteString(renderFormFieldDesc("Clock format for tasks with a due time", indent, m.focusIndex == 93))

	return b.String()
}
