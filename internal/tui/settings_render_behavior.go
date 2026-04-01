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

	b.WriteString(renderFormLabelWithWidth("Task limit: ", labelWidth, m.focusIndex == 20))
	b.WriteString(m.recentActivityInput.View())
	b.WriteString("  ")
	b.WriteString(MutedStyle.Render("(0 = no limit)"))
	b.WriteString("\n")

	if m.focusIndex == 20 && m.validationErr != "" {
		b.WriteString(strings.Repeat(" ", indent))
		b.WriteString(ValidationErrorStyle.Render(m.validationErr))
		b.WriteString("\n")
	}
	b.WriteString(renderFormFieldDesc("Maximum tasks shown in recent activity", indent, m.focusIndex == 20))

	b.WriteString("\n")

	b.WriteString(renderFormLabelWithWidth("Show completed: ", labelWidth, m.focusIndex == 21))
	b.WriteString(renderFormToggle("Yes", "No", m.recentActivityShowAll, m.focusIndex == 21))
	b.WriteString("\n")
	b.WriteString(renderFormFieldDesc("Include completed tasks in recent activity view", indent, m.focusIndex == 21))

	return b.String()
}

func (m *SettingsModel) renderBehaviorLinkedTasksSection() string {
	var b strings.Builder
	const labelWidth = 17
	const indent = 19

	b.WriteString(renderFormLabelWithWidth("After creation: ", labelWidth, m.focusIndex == 22))
	b.WriteString(renderFormToggle("Jump to new", "Stay on source", m.linkedTaskJump, m.focusIndex == 22))
	b.WriteString("\n")
	b.WriteString(renderFormFieldDesc("Where to navigate after creating a linked task", indent, m.focusIndex == 22))

	return b.String()
}

func (m *SettingsModel) renderBehaviorPinnedTasksSection() string {
	var b strings.Builder
	const labelWidth = 17
	const indent = 19

	b.WriteString(renderFormLabelWithWidth("Bar limit: ", labelWidth, m.focusIndex == 23))
	b.WriteString(m.maxPinnedInput.View())
	b.WriteString("  ")
	b.WriteString(MutedStyle.Render("(0=disable, 1-20)"))
	b.WriteString("\n")
	if m.focusIndex == 23 && m.validationErr != "" {
		b.WriteString(strings.Repeat(" ", indent))
		b.WriteString(ValidationErrorStyle.Render(m.validationErr))
		b.WriteString("\n")
	}
	b.WriteString(renderFormFieldDesc("Maximum pinned tasks displayed in the bar", indent, m.focusIndex == 23))

	return b.String()
}

func (m *SettingsModel) renderBehaviorRecentTasksSection() string {
	var b strings.Builder
	const labelWidth = 17
	const indent = 19

	b.WriteString(renderFormLabelWithWidth("Task limit: ", labelWidth, m.focusIndex == 24))
	b.WriteString(m.maxRecentTaskVisitsInput.View())
	b.WriteString("  ")
	b.WriteString(MutedStyle.Render("(1-50, default 15)"))
	b.WriteString("\n")
	if m.focusIndex == 24 && m.validationErr != "" {
		b.WriteString(strings.Repeat(" ", indent))
		b.WriteString(ValidationErrorStyle.Render(m.validationErr))
		b.WriteString("\n")
	}
	b.WriteString(renderFormFieldDesc("Maximum tasks shown in quick-switch overlay (Ctrl+R)", indent, m.focusIndex == 24))

	return b.String()
}

func (m *SettingsModel) renderBehaviorPopupsSection() string {
	var b strings.Builder
	const labelWidth = 17
	const indent = 19

	b.WriteString(renderFormLabelWithWidth("Auto-dismiss: ", labelWidth, m.focusIndex == 25))
	b.WriteString(m.popupTimeoutInput.View())
	b.WriteString("  ")
	b.WriteString(MutedStyle.Render("(0=stay, 1-30 seconds)"))
	b.WriteString("\n")
	if m.focusIndex == 25 && m.validationErr != "" {
		b.WriteString(strings.Repeat(" ", indent))
		b.WriteString(ValidationErrorStyle.Render(m.validationErr))
		b.WriteString("\n")
	}
	b.WriteString(renderFormFieldDesc("Seconds before prefix popups auto-dismiss (0=disabled)", indent, m.focusIndex == 25))

	return b.String()
}

func (m *SettingsModel) renderBehaviorSectionLimitsSection() string {
	var b strings.Builder
	const labelWidth = 17
	const indent = 19

	type limitField struct {
		label    string
		focusIdx int
		input    *textinput.Model
	}
	fields := []limitField{
		{"Active: ", 26, &m.sectionLimitActiveInput},
		{"Next: ", 27, &m.sectionLimitNextInput},
		{"Inbox: ", 28, &m.sectionLimitInboxInput},
		{"Standby: ", 29, &m.sectionLimitStandbyInput},
		{"Waiting: ", 30, &m.sectionLimitWaitingInput},
		{"Someday: ", 31, &m.sectionLimitSomedayInput},
		{"Review: ", 32, &m.sectionLimitReviewInput},
	}
	for _, f := range fields {
		b.WriteString(renderFormLabelWithWidth(f.label, labelWidth, m.focusIndex == f.focusIdx))
		b.WriteString(f.input.View())
		b.WriteString("  ")
		b.WriteString(MutedStyle.Render("(0=disabled, max 999)"))
		b.WriteString("\n")
		if m.focusIndex == f.focusIdx && m.validationErr != "" {
			b.WriteString(strings.Repeat(" ", indent))
			b.WriteString(ValidationErrorStyle.Render(m.validationErr))
			b.WriteString("\n")
		}
	}

	return b.String()
}
