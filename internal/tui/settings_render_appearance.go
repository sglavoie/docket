package tui

import "strings"

func (m *SettingsModel) renderAppearanceTab() string {
	var b strings.Builder
	const labelWidth = 20
	const indent = 22
	base := tabFieldStart[tabAppearance]

	zebraVal := "Disabled"
	if m.zebraStriping {
		zebraVal = "Enabled"
	}
	b.WriteString(renderFormLabelWithWidth("Zebra Striping: ", labelWidth, m.focusIndex == base+0))
	b.WriteString(renderFormToggle("Enabled", "Disabled", m.zebraStriping, m.focusIndex == base+0))
	b.WriteString(renderDefaultHint(zebraVal, "Disabled"))
	b.WriteString("\n")
	b.WriteString(renderFormFieldDesc("Alternating backgrounds in task rows", indent, m.focusIndex == base+0))

	b.WriteString("\n")

	b.WriteString(renderFormLabelWithWidth("Week-End Day: ", labelWidth, m.focusIndex == base+1))
	b.WriteString(renderEnumSelector(weekEndDayOptions, m.weekEndDayCursor, m.focusIndex == base+1, m.width-indent))
	b.WriteString(renderDefaultHint(weekEndDayOptions[m.weekEndDayCursor], "Sunday"))
	b.WriteString("\n")
	b.WriteString(renderFormFieldDesc("Last day of the week for the 'eow' date token", indent, m.focusIndex == base+1))

	b.WriteString("\n")

	b.WriteString(renderFormLabelWithWidth("Time Format: ", labelWidth, m.focusIndex == base+2))
	b.WriteString(renderEnumSelector(timeFormatOptions, m.timeFormatCursor, m.focusIndex == base+2, m.width-indent))
	b.WriteString(renderDefaultHint(timeFormatOptions[m.timeFormatCursor], "24-hour (17:00)"))
	b.WriteString("\n")
	b.WriteString(renderFormFieldDesc("Clock format for tasks with a due time", indent, m.focusIndex == base+2))

	b.WriteString("\n")

	autoThemeVal := "Disabled"
	if m.autoTheme {
		autoThemeVal = "Enabled"
	}
	b.WriteString(renderFormLabelWithWidth("Auto Theme: ", labelWidth, m.focusIndex == base+3))
	b.WriteString(renderFormToggle("Enabled", "Disabled", m.autoTheme, m.focusIndex == base+3))
	b.WriteString(renderDefaultHint(autoThemeVal, "Disabled"))
	b.WriteString("\n")
	b.WriteString(renderFormFieldDesc("Sync theme with macOS system appearance", indent, m.focusIndex == base+3))

	b.WriteString("\n")

	useEmojiVal := "Disabled"
	if m.useEmoji {
		useEmojiVal = "Enabled"
	}
	b.WriteString(renderFormLabelWithWidth("Use Emoji: ", labelWidth, m.focusIndex == base+4))
	b.WriteString(renderFormToggle("Enabled", "Disabled", m.useEmoji, m.focusIndex == base+4))
	b.WriteString(renderDefaultHint(useEmojiVal, "Disabled"))
	b.WriteString("\n")
	b.WriteString(renderFormFieldDesc("Use emoji icons instead of Unicode symbols", indent, m.focusIndex == base+4))

	return b.String()
}
