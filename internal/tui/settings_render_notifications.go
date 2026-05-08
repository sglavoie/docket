package tui

import "strings"

func (m *SettingsModel) renderNotificationsTab() string {
	switch m.subTab {
	case 1:
		return m.renderNotificationsTasksSection()
	case 2:
		return m.renderNotificationsProjectsSection()
	default:
		return m.renderNotificationsReviewsSection()
	}
}

func (m *SettingsModel) renderNotificationsReviewsSection() string {
	var b strings.Builder
	const labelWidth = 22
	const indent = 24
	base := subTabFieldStart(tabNotifications, 0)

	b.WriteString(renderFormLabelWithWidth("Daily Review: ", labelWidth, m.focusIndex == base+0))
	b.WriteString(renderFormToggle("Enabled", "Disabled", m.alertDailyReviewEnabled, m.focusIndex == base+0))
	b.WriteString("\n")
	b.WriteString(renderFormFieldDesc("Remind to do daily review", indent, m.focusIndex == base+0))

	b.WriteString("\n")

	b.WriteString(renderFormLabelWithWidth("Review Hour: ", labelWidth, m.focusIndex == base+1))
	b.WriteString(m.alertDailyReviewHourInput.View())
	b.WriteString("  ")
	b.WriteString(MutedStyle.Render("(0-23)"))
	b.WriteString("\n")
	if m.focusIndex == base+1 && m.validationErr != "" {
		b.WriteString(strings.Repeat(" ", indent))
		b.WriteString(ValidationErrorStyle.Render(m.validationErr))
		b.WriteString("\n")
	}
	b.WriteString(renderFormFieldDesc("Hour of day to trigger daily review reminder", indent, m.focusIndex == base+1))

	b.WriteString("\n")

	b.WriteString(renderFormLabelWithWidth("Weekly Review: ", labelWidth, m.focusIndex == base+2))
	b.WriteString(renderFormToggle("Enabled", "Disabled", m.alertWeeklyReviewEnabled, m.focusIndex == base+2))
	b.WriteString("\n")
	b.WriteString(renderFormFieldDesc("Remind to do weekly review", indent, m.focusIndex == base+2))

	b.WriteString("\n")

	b.WriteString(renderFormLabelWithWidth("Review Day: ", labelWidth, m.focusIndex == base+3))
	b.WriteString(renderEnumSelector(weeklyReviewDayOptions, m.alertWeeklyReviewDay, m.focusIndex == base+3, m.width-indent))
	b.WriteString("\n")
	b.WriteString(renderFormFieldDesc("Day of week for weekly review reminder", indent, m.focusIndex == base+3))

	return b.String()
}

func (m *SettingsModel) renderNotificationsTasksSection() string {
	var b strings.Builder
	const labelWidth = 22
	const indent = 24
	base := subTabFieldStart(tabNotifications, 1)

	b.WriteString(renderFormLabelWithWidth("Overdue: ", labelWidth, m.focusIndex == base+0))
	b.WriteString(renderFormToggle("Enabled", "Disabled", m.alertOverdueEnabled, m.focusIndex == base+0))
	b.WriteString("\n")
	b.WriteString(renderFormFieldDesc("Alert for tasks past their due date", indent, m.focusIndex == base+0))

	b.WriteString("\n")

	b.WriteString(renderFormLabelWithWidth("Due Soon: ", labelWidth, m.focusIndex == base+1))
	b.WriteString(renderFormToggle("Enabled", "Disabled", m.alertDueSoonEnabled, m.focusIndex == base+1))
	b.WriteString("\n")

	b.WriteString(renderFormLabelWithWidth("Due Soon (days): ", labelWidth, m.focusIndex == base+2))
	b.WriteString(m.alertDueSoonDaysInput.View())
	b.WriteString("\n")
	if m.focusIndex == base+2 && m.validationErr != "" {
		b.WriteString(strings.Repeat(" ", indent))
		b.WriteString(ValidationErrorStyle.Render(m.validationErr))
		b.WriteString("\n")
	}
	b.WriteString(renderFormFieldDesc("Alert when tasks are due within this many days", indent, m.focusIndex == base+2))

	b.WriteString("\n")

	b.WriteString(renderFormLabelWithWidth("Stale: ", labelWidth, m.focusIndex == base+3))
	b.WriteString(renderFormToggle("Enabled", "Disabled", m.alertStaleEnabled, m.focusIndex == base+3))
	b.WriteString("\n")

	b.WriteString(renderFormLabelWithWidth("Stale (days): ", labelWidth, m.focusIndex == base+4))
	b.WriteString(m.alertStaleDaysInput.View())
	b.WriteString("\n")
	if m.focusIndex == base+4 && m.validationErr != "" {
		b.WriteString(strings.Repeat(" ", indent))
		b.WriteString(ValidationErrorStyle.Render(m.validationErr))
		b.WriteString("\n")
	}
	b.WriteString(renderFormFieldDesc("Days without updates before a task is stale", indent, m.focusIndex == base+4))

	b.WriteString("\n")

	b.WriteString(renderFormLabelWithWidth("Inbox Backlog: ", labelWidth, m.focusIndex == base+5))
	b.WriteString(renderFormToggle("Enabled", "Disabled", m.alertInboxEnabled, m.focusIndex == base+5))
	b.WriteString("\n")

	b.WriteString(renderFormLabelWithWidth("Inbox Threshold: ", labelWidth, m.focusIndex == base+6))
	b.WriteString(m.alertInboxThresholdInput.View())
	b.WriteString("\n")
	if m.focusIndex == base+6 && m.validationErr != "" {
		b.WriteString(strings.Repeat(" ", indent))
		b.WriteString(ValidationErrorStyle.Render(m.validationErr))
		b.WriteString("\n")
	}
	b.WriteString(renderFormFieldDesc("Alert when inbox has more than this many tasks", indent, m.focusIndex == base+6))

	b.WriteString("\n")

	b.WriteString(renderFormLabelWithWidth("Blocked: ", labelWidth, m.focusIndex == base+7))
	b.WriteString(renderFormToggle("Enabled", "Disabled", m.alertBlockedEnabled, m.focusIndex == base+7))
	b.WriteString("\n")

	b.WriteString(renderFormLabelWithWidth("Blocked (days): ", labelWidth, m.focusIndex == base+8))
	b.WriteString(m.alertBlockedDaysInput.View())
	b.WriteString("\n")
	if m.focusIndex == base+8 && m.validationErr != "" {
		b.WriteString(strings.Repeat(" ", indent))
		b.WriteString(ValidationErrorStyle.Render(m.validationErr))
		b.WriteString("\n")
	}
	b.WriteString(renderFormFieldDesc("Days a task must be blocked before alerting", indent, m.focusIndex == base+8))

	return b.String()
}

func (m *SettingsModel) renderNotificationsProjectsSection() string {
	var b strings.Builder
	const labelWidth = 22
	const indent = 24
	base := subTabFieldStart(tabNotifications, 2)

	b.WriteString(renderFormLabelWithWidth("Project Health: ", labelWidth, m.focusIndex == base+0))
	b.WriteString(renderFormToggle("Enabled", "Disabled", m.alertProjectHealthEnabled, m.focusIndex == base+0))
	b.WriteString("\n")

	b.WriteString(renderFormLabelWithWidth("Stale (days): ", labelWidth, m.focusIndex == base+1))
	b.WriteString(m.alertProjectStaleDaysInput.View())
	b.WriteString("\n")
	if m.focusIndex == base+1 && m.validationErr != "" {
		b.WriteString(strings.Repeat(" ", indent))
		b.WriteString(ValidationErrorStyle.Render(m.validationErr))
		b.WriteString("\n")
	}
	b.WriteString(renderFormFieldDesc("Days without task updates to flag project health", indent, m.focusIndex == base+1))

	return b.String()
}
