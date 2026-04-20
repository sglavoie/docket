package tui

import (
	"strings"

	"charm.land/bubbles/v2/textinput"
)

func (m *SettingsModel) renderBehaviorTab() string {
	switch m.subTab {
	case 1:
		return m.renderBehaviorLinkedTasksSection()
	case 2:
		return m.renderBehaviorPinnedTasksSection()
	case 3:
		return m.renderBehaviorRecentTasksSection()
	case 4:
		return m.renderBehaviorPopupsSection()
	case 5:
		return m.renderBehaviorSectionLimitsSection()
	default:
		return m.renderBehaviorRecentActivitySection()
	}
}

func (m *SettingsModel) renderBehaviorRecentActivitySection() string {
	var b strings.Builder
	const labelWidth = 17
	const indent = 19
	base := subTabFieldStart(tabBehavior, 0)

	b.WriteString(renderFormLabelWithWidth("Task limit: ", labelWidth, m.focusIndex == base+0))
	b.WriteString(m.recentActivityInput.View())
	b.WriteString("  ")
	b.WriteString(MutedStyle.Render("(0 = no limit)"))
	b.WriteString("\n")

	if m.focusIndex == base+0 && m.validationErr != "" {
		b.WriteString(strings.Repeat(" ", indent))
		b.WriteString(ValidationErrorStyle.Render(m.validationErr))
		b.WriteString("\n")
	}
	b.WriteString(renderFormFieldDesc("Maximum tasks shown in recent activity", indent, m.focusIndex == base+0))

	b.WriteString("\n")

	b.WriteString(renderFormLabelWithWidth("Show completed: ", labelWidth, m.focusIndex == base+1))
	b.WriteString(renderFormToggle("Yes", "No", m.recentActivityShowAll, m.focusIndex == base+1))
	b.WriteString("\n")
	b.WriteString(renderFormFieldDesc("Include completed tasks in recent activity view", indent, m.focusIndex == base+1))

	return b.String()
}

func (m *SettingsModel) renderBehaviorLinkedTasksSection() string {
	var b strings.Builder
	const labelWidth = 17
	const indent = 19
	base := subTabFieldStart(tabBehavior, 1)

	b.WriteString(renderFormLabelWithWidth("After creation: ", labelWidth, m.focusIndex == base+0))
	b.WriteString(renderFormToggle("Jump to new", "Stay on source", m.linkedTaskJump, m.focusIndex == base+0))
	b.WriteString("\n")
	b.WriteString(renderFormFieldDesc("Where to navigate after creating a linked task", indent, m.focusIndex == base+0))

	return b.String()
}

func (m *SettingsModel) renderBehaviorPinnedTasksSection() string {
	var b strings.Builder
	const labelWidth = 17
	const indent = 19
	base := subTabFieldStart(tabBehavior, 2)

	b.WriteString(renderFormLabelWithWidth("Bar limit: ", labelWidth, m.focusIndex == base+0))
	b.WriteString(m.maxPinnedInput.View())
	b.WriteString("  ")
	b.WriteString(MutedStyle.Render("(0=disable, 1-20)"))
	b.WriteString("\n")
	if m.focusIndex == base+0 && m.validationErr != "" {
		b.WriteString(strings.Repeat(" ", indent))
		b.WriteString(ValidationErrorStyle.Render(m.validationErr))
		b.WriteString("\n")
	}
	b.WriteString(renderFormFieldDesc("Maximum pinned tasks displayed in the bar", indent, m.focusIndex == base+0))

	return b.String()
}

func (m *SettingsModel) renderBehaviorRecentTasksSection() string {
	var b strings.Builder
	const labelWidth = 17
	const indent = 19
	base := subTabFieldStart(tabBehavior, 3)

	b.WriteString(renderFormLabelWithWidth("Task limit: ", labelWidth, m.focusIndex == base+0))
	b.WriteString(m.maxRecentTaskVisitsInput.View())
	b.WriteString("  ")
	b.WriteString(MutedStyle.Render("(1-50, default 15)"))
	b.WriteString("\n")
	if m.focusIndex == base+0 && m.validationErr != "" {
		b.WriteString(strings.Repeat(" ", indent))
		b.WriteString(ValidationErrorStyle.Render(m.validationErr))
		b.WriteString("\n")
	}
	b.WriteString(renderFormFieldDesc("Maximum tasks shown in quick-switch overlay (Ctrl+R)", indent, m.focusIndex == base+0))

	return b.String()
}

func (m *SettingsModel) renderBehaviorPopupsSection() string {
	var b strings.Builder
	const labelWidth = 17
	const indent = 19
	base := subTabFieldStart(tabBehavior, 4)

	b.WriteString(renderFormLabelWithWidth("Auto-dismiss: ", labelWidth, m.focusIndex == base+0))
	b.WriteString(m.popupTimeoutInput.View())
	b.WriteString("  ")
	b.WriteString(MutedStyle.Render("(0=stay, 1-30 seconds)"))
	b.WriteString("\n")
	if m.focusIndex == base+0 && m.validationErr != "" {
		b.WriteString(strings.Repeat(" ", indent))
		b.WriteString(ValidationErrorStyle.Render(m.validationErr))
		b.WriteString("\n")
	}
	b.WriteString(renderFormFieldDesc("Seconds before prefix popups auto-dismiss (0=disabled)", indent, m.focusIndex == base+0))

	return b.String()
}

func (m *SettingsModel) renderBehaviorSectionLimitsSection() string {
	var b strings.Builder
	const labelWidth = 17
	const indent = 19
	base := subTabFieldStart(tabBehavior, 5)

	type limitField struct {
		label string
		local int
		input *textinput.Model
	}
	fields := []limitField{
		{"Active: ", 0, &m.sectionLimitActiveInput},
		{"Next: ", 1, &m.sectionLimitNextInput},
		{"Inbox: ", 2, &m.sectionLimitInboxInput},
		{"Standby: ", 3, &m.sectionLimitStandbyInput},
		{"Waiting: ", 4, &m.sectionLimitWaitingInput},
		{"Someday: ", 5, &m.sectionLimitSomedayInput},
		{"Review: ", 6, &m.sectionLimitReviewInput},
	}
	for _, f := range fields {
		focusIdx := base + f.local
		b.WriteString(renderFormLabelWithWidth(f.label, labelWidth, m.focusIndex == focusIdx))
		b.WriteString(f.input.View())
		b.WriteString("  ")
		b.WriteString(MutedStyle.Render("(0=disabled, max 999)"))
		b.WriteString("\n")
		if m.focusIndex == focusIdx && m.validationErr != "" {
			b.WriteString(strings.Repeat(" ", indent))
			b.WriteString(ValidationErrorStyle.Render(m.validationErr))
			b.WriteString("\n")
		}
	}

	return b.String()
}
