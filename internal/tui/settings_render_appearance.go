package tui

import "strings"

func (m *SettingsModel) renderAppearanceTab() string {
	var b strings.Builder
	const labelWidth = 20
	const indent = 22

	b.WriteString(renderFormLabelWithWidth("Zebra Striping: ", labelWidth, m.focusIndex == 85))
	b.WriteString(renderFormToggle("Enabled", "Disabled", m.zebraStriping, m.focusIndex == 85))
	b.WriteString("\n")
	b.WriteString(renderFormFieldDesc("Alternating row backgrounds in task list views", indent, m.focusIndex == 85))

	return b.String()
}
