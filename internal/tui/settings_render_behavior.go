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
		return m.renderBehaviorSectionLimitsSection()
	case 5:
		return m.renderBehaviorStartupSection()
	case 6:
		return m.renderBehaviorHomeSection()
	default:
		return m.renderBehaviorRecentActivitySection()
	}
}

func (m *SettingsModel) renderBehaviorHomeSection() string {
	var b strings.Builder
	const labelWidth = 19
	const indent = 21
	base := subTabFieldStart(tabBehavior, 6)

	b.WriteString(renderFormLabelWithWidth("Parked task limit: ", labelWidth, m.focusIndex == base))
	b.WriteString(m.resumeCardLimitInput.View())
	b.WriteString(renderDefaultHint(m.resumeCardLimitInput.Value(), "5"))
	b.WriteString("\n")
	if m.focusIndex == base && m.validationErr != "" {
		b.WriteString(strings.Repeat(" ", indent))
		b.WriteString(ValidationErrorStyle.Render(m.validationErr))
		b.WriteString("\n")
	}
	b.WriteString(renderFormFieldDesc("Maximum parked tasks shown in Home Attention", indent, m.focusIndex == base))
	return b.String()
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
	b.WriteString(renderDefaultHint(m.recentActivityInput.Value(), "20"))
	b.WriteString("\n")

	if m.focusIndex == base+0 && m.validationErr != "" {
		b.WriteString(strings.Repeat(" ", indent))
		b.WriteString(ValidationErrorStyle.Render(m.validationErr))
		b.WriteString("\n")
	}
	b.WriteString(renderFormFieldDesc("Maximum tasks shown in recent activity", indent, m.focusIndex == base+0))

	b.WriteString("\n")

	showAllVal := "No"
	if m.recentActivityShowAll {
		showAllVal = "Yes"
	}
	b.WriteString(renderFormLabelWithWidth("Show completed: ", labelWidth, m.focusIndex == base+1))
	b.WriteString(renderFormToggle("Yes", "No", m.recentActivityShowAll, m.focusIndex == base+1))
	b.WriteString(renderDefaultHint(showAllVal, "No"))
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
	b.WriteString(renderDefaultHint(m.maxPinnedInput.Value(), "5"))
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
	b.WriteString(MutedStyle.Render("(1-50)"))
	b.WriteString(renderDefaultHint(m.maxRecentTaskVisitsInput.Value(), "15"))
	b.WriteString("\n")
	if m.focusIndex == base+0 && m.validationErr != "" {
		b.WriteString(strings.Repeat(" ", indent))
		b.WriteString(ValidationErrorStyle.Render(m.validationErr))
		b.WriteString("\n")
	}
	b.WriteString(renderFormFieldDesc("Maximum tasks shown in Recent Tasks overlay", indent, m.focusIndex == base+0))

	return b.String()
}

func (m *SettingsModel) renderBehaviorSectionLimitsSection() string {
	var b strings.Builder
	const labelWidth = 17
	const indent = 19
	base := subTabFieldStart(tabBehavior, 4)

	type limitField struct {
		label  string
		local  int
		input  *textinput.Model
		defVal string
	}
	fields := []limitField{
		{"Active: ", 0, &m.sectionLimitActiveInput, "5"},
		{"Next: ", 1, &m.sectionLimitNextInput, "5"},
		{"Inbox: ", 2, &m.sectionLimitInboxInput, "5"},
		{"Standby: ", 3, &m.sectionLimitStandbyInput, "5"},
		{"Waiting: ", 4, &m.sectionLimitWaitingInput, "3"},
		{"Someday: ", 5, &m.sectionLimitSomedayInput, "10"},
		{"Review: ", 6, &m.sectionLimitReviewInput, "5"},
	}
	for _, f := range fields {
		focusIdx := base + f.local
		b.WriteString(renderFormLabelWithWidth(f.label, labelWidth, m.focusIndex == focusIdx))
		b.WriteString(f.input.View())
		b.WriteString("  ")
		b.WriteString(MutedStyle.Render("(0=disabled, max 999)"))
		b.WriteString(renderDefaultHint(f.input.Value(), f.defVal))
		b.WriteString("\n")
		if m.focusIndex == focusIdx && m.validationErr != "" {
			b.WriteString(strings.Repeat(" ", indent))
			b.WriteString(ValidationErrorStyle.Render(m.validationErr))
			b.WriteString("\n")
		}
	}

	return b.String()
}

func (m *SettingsModel) renderBehaviorStartupSection() string {
	var b strings.Builder
	const labelWidth = 17
	const indent = 19
	base := subTabFieldStart(tabBehavior, 5)

	b.WriteString(renderFormLabelWithWidth("Start view: ", labelWidth, m.focusIndex == base+0))
	b.WriteString(renderEnumSelector(startViewOptions, m.startViewCursor, m.focusIndex == base+0, m.width-indent))
	b.WriteString(renderDefaultHint(startViewOptions[m.startViewCursor], "Today"))
	b.WriteString("\n")
	b.WriteString(renderFormFieldDesc("Destination shown when the application starts (default: Planner)", indent, m.focusIndex == base+0))

	return b.String()
}
