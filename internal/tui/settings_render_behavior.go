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
	case 6:
		return m.renderBehaviorStartupSection()
	case 7:
		return m.renderBehaviorTaskFormSection()
	case 8:
		return m.renderBehaviorFocusBoardSection()
	case 9:
		return m.renderBehaviorDashboardSection()
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

func (m *SettingsModel) renderBehaviorPopupsSection() string {
	var b strings.Builder
	const labelWidth = 17
	const indent = 19
	base := subTabFieldStart(tabBehavior, 4)

	b.WriteString(renderFormLabelWithWidth("Auto-dismiss: ", labelWidth, m.focusIndex == base+0))
	b.WriteString(m.popupTimeoutInput.View())
	b.WriteString("  ")
	b.WriteString(MutedStyle.Render("(0=stay, 1-30 seconds)"))
	b.WriteString(renderDefaultHint(m.popupTimeoutInput.Value(), "0"))
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
	base := subTabFieldStart(tabBehavior, 6)

	b.WriteString(renderFormLabelWithWidth("Start view: ", labelWidth, m.focusIndex == base+0))
	b.WriteString(renderEnumSelector(startViewOptions, m.startViewCursor, m.focusIndex == base+0, m.width-indent))
	b.WriteString(renderDefaultHint(startViewOptions[m.startViewCursor], "Today"))
	b.WriteString("\n")
	b.WriteString(renderFormFieldDesc("View shown when the application starts (default: Today cockpit)", indent, m.focusIndex == base+0))

	return b.String()
}

func (m *SettingsModel) renderBehaviorTaskFormSection() string {
	var b strings.Builder
	const labelWidth = 17
	const indent = 19
	base := subTabFieldStart(tabBehavior, 7)

	taskFormVal := "Expanded"
	if m.taskFormCollapsed {
		taskFormVal = "Collapsed"
	}
	b.WriteString(renderFormLabelWithWidth("Default mode: ", labelWidth, m.focusIndex == base+0))
	b.WriteString(renderFormToggle("Collapsed", "Expanded", m.taskFormCollapsed, m.focusIndex == base+0))
	b.WriteString(renderDefaultHint(taskFormVal, "Expanded"))
	b.WriteString("\n")
	b.WriteString(renderFormFieldDesc("Show only essential fields when opening the task form", indent, m.focusIndex == base+0))

	return b.String()
}

func (m *SettingsModel) renderBehaviorFocusBoardSection() string {
	var b strings.Builder
	const labelWidth = 17
	const indent = 19
	base := subTabFieldStart(tabBehavior, 8)

	b.WriteString(renderFormLabelWithWidth("Default mode: ", labelWidth, m.focusIndex == base+0))
	b.WriteString(renderEnumSelector(focusBoardModeOptions, m.focusBoardModeCursor, m.focusIndex == base+0, m.width-indent))
	b.WriteString(renderDefaultHint(focusBoardModeOptions[m.focusBoardModeCursor], "List"))
	b.WriteString("\n")
	b.WriteString(renderFormFieldDesc("Starting focus mode when Today opens (List/Tunnel/Zen)", indent, m.focusIndex == base+0))

	return b.String()
}

func (m *SettingsModel) renderBehaviorDashboardSection() string {
	var b strings.Builder
	const labelWidth = 17
	const indent = 19
	base := subTabFieldStart(tabBehavior, 9)

	b.WriteString(renderFormLabelWithWidth("Layout: ", labelWidth, m.focusIndex == base+0))
	b.WriteString(renderEnumSelector(dashboardLayoutOptions, m.dashboardLayoutCursor, m.focusIndex == base+0, m.width-indent))
	b.WriteString(renderDefaultHint(dashboardLayoutOptions[m.dashboardLayoutCursor], "Horizontal"))
	b.WriteString("\n")
	b.WriteString(renderFormFieldDesc("Dashboard bottom section layout (Horizontal=top/bottom, Vertical=side-by-side, List only=hide bottom)", indent, m.focusIndex == base+0))

	b.WriteString("\n")

	b.WriteString(renderFormLabelWithWidth("Horiz Split %: ", labelWidth, m.focusIndex == base+1))
	b.WriteString(m.dashboardHorizSplitInput.View())
	b.WriteString("  ")
	b.WriteString(MutedStyle.Render("(10-90)"))
	b.WriteString(renderDefaultHint(m.dashboardHorizSplitInput.Value(), "70"))
	b.WriteString("\n")
	if m.focusIndex == base+1 && m.validationErr != "" {
		b.WriteString(strings.Repeat(" ", indent))
		b.WriteString(ValidationErrorStyle.Render(m.validationErr))
		b.WriteString("\n")
	}
	b.WriteString(renderFormFieldDesc("Project-list height share in Horizontal layout", indent, m.focusIndex == base+1))

	b.WriteString("\n")

	b.WriteString(renderFormLabelWithWidth("Vert Split %: ", labelWidth, m.focusIndex == base+2))
	b.WriteString(m.dashboardVertSplitInput.View())
	b.WriteString("  ")
	b.WriteString(MutedStyle.Render("(10-90)"))
	b.WriteString(renderDefaultHint(m.dashboardVertSplitInput.Value(), "60"))
	b.WriteString("\n")
	if m.focusIndex == base+2 && m.validationErr != "" {
		b.WriteString(strings.Repeat(" ", indent))
		b.WriteString(ValidationErrorStyle.Render(m.validationErr))
		b.WriteString("\n")
	}
	b.WriteString(renderFormFieldDesc("Project-list width share in Vertical layout", indent, m.focusIndex == base+2))

	return b.String()
}
