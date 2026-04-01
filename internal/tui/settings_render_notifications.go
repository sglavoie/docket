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

	b.WriteString(renderFormLabelWithWidth("Daily Review: ", labelWidth, m.focusIndex == 69))
	b.WriteString(renderFormToggle("Enabled", "Disabled", m.alertDailyReviewEnabled, m.focusIndex == 69))
	b.WriteString("\n")
	b.WriteString(renderFormFieldDesc("Remind to do daily review", indent, m.focusIndex == 69))

	b.WriteString("\n")

	b.WriteString(renderFormLabelWithWidth("Review Hour: ", labelWidth, m.focusIndex == 70))
	b.WriteString(m.alertDailyReviewHourInput.View())
	b.WriteString("  ")
	b.WriteString(MutedStyle.Render("(0-23)"))
	b.WriteString("\n")
	if m.focusIndex == 70 && m.validationErr != "" {
		b.WriteString(strings.Repeat(" ", indent))
		b.WriteString(ValidationErrorStyle.Render(m.validationErr))
		b.WriteString("\n")
	}
	b.WriteString(renderFormFieldDesc("Hour of day to trigger daily review reminder", indent, m.focusIndex == 70))

	b.WriteString("\n")

	b.WriteString(renderFormLabelWithWidth("Weekly Review: ", labelWidth, m.focusIndex == 71))
	b.WriteString(renderFormToggle("Enabled", "Disabled", m.alertWeeklyReviewEnabled, m.focusIndex == 71))
	b.WriteString("\n")
	b.WriteString(renderFormFieldDesc("Remind to do weekly review", indent, m.focusIndex == 71))

	b.WriteString("\n")

	b.WriteString(renderFormLabelWithWidth("Review Day: ", labelWidth, m.focusIndex == 72))
	b.WriteString(renderEnumSelector(weeklyReviewDayOptions, m.alertWeeklyReviewDay, m.focusIndex == 72))
	b.WriteString("\n")
	b.WriteString(renderFormFieldDesc("Day of week for weekly review reminder", indent, m.focusIndex == 72))

	return b.String()
}

func (m *SettingsModel) renderNotificationsTasksSection() string {
	var b strings.Builder
	const labelWidth = 22
	const indent = 24

	b.WriteString(renderFormLabelWithWidth("Overdue: ", labelWidth, m.focusIndex == 73))
	b.WriteString(renderFormToggle("Enabled", "Disabled", m.alertOverdueEnabled, m.focusIndex == 73))
	b.WriteString("\n")
	b.WriteString(renderFormFieldDesc("Alert for tasks past their due date", indent, m.focusIndex == 73))

	b.WriteString("\n")

	b.WriteString(renderFormLabelWithWidth("Due Soon: ", labelWidth, m.focusIndex == 74))
	b.WriteString(renderFormToggle("Enabled", "Disabled", m.alertDueSoonEnabled, m.focusIndex == 74))
	b.WriteString("\n")

	b.WriteString(renderFormLabelWithWidth("Due Soon (days): ", labelWidth, m.focusIndex == 75))
	b.WriteString(m.alertDueSoonDaysInput.View())
	b.WriteString("\n")
	if m.focusIndex == 75 && m.validationErr != "" {
		b.WriteString(strings.Repeat(" ", indent))
		b.WriteString(ValidationErrorStyle.Render(m.validationErr))
		b.WriteString("\n")
	}
	b.WriteString(renderFormFieldDesc("Alert when tasks are due within this many days", indent, m.focusIndex == 75))

	b.WriteString("\n")

	b.WriteString(renderFormLabelWithWidth("Stale: ", labelWidth, m.focusIndex == 76))
	b.WriteString(renderFormToggle("Enabled", "Disabled", m.alertStaleEnabled, m.focusIndex == 76))
	b.WriteString("\n")

	b.WriteString(renderFormLabelWithWidth("Stale (days): ", labelWidth, m.focusIndex == 77))
	b.WriteString(m.alertStaleDaysInput.View())
	b.WriteString("\n")
	if m.focusIndex == 77 && m.validationErr != "" {
		b.WriteString(strings.Repeat(" ", indent))
		b.WriteString(ValidationErrorStyle.Render(m.validationErr))
		b.WriteString("\n")
	}
	b.WriteString(renderFormFieldDesc("Days without updates before a task is stale", indent, m.focusIndex == 77))

	b.WriteString("\n")

	b.WriteString(renderFormLabelWithWidth("Inbox Backlog: ", labelWidth, m.focusIndex == 78))
	b.WriteString(renderFormToggle("Enabled", "Disabled", m.alertInboxEnabled, m.focusIndex == 78))
	b.WriteString("\n")

	b.WriteString(renderFormLabelWithWidth("Inbox Threshold: ", labelWidth, m.focusIndex == 79))
	b.WriteString(m.alertInboxThresholdInput.View())
	b.WriteString("\n")
	if m.focusIndex == 79 && m.validationErr != "" {
		b.WriteString(strings.Repeat(" ", indent))
		b.WriteString(ValidationErrorStyle.Render(m.validationErr))
		b.WriteString("\n")
	}
	b.WriteString(renderFormFieldDesc("Alert when inbox has more than this many tasks", indent, m.focusIndex == 79))

	b.WriteString("\n")

	b.WriteString(renderFormLabelWithWidth("Blocked: ", labelWidth, m.focusIndex == 80))
	b.WriteString(renderFormToggle("Enabled", "Disabled", m.alertBlockedEnabled, m.focusIndex == 80))
	b.WriteString("\n")

	b.WriteString(renderFormLabelWithWidth("Blocked (days): ", labelWidth, m.focusIndex == 81))
	b.WriteString(m.alertBlockedDaysInput.View())
	b.WriteString("\n")
	if m.focusIndex == 81 && m.validationErr != "" {
		b.WriteString(strings.Repeat(" ", indent))
		b.WriteString(ValidationErrorStyle.Render(m.validationErr))
		b.WriteString("\n")
	}
	b.WriteString(renderFormFieldDesc("Days a task must be blocked before alerting", indent, m.focusIndex == 81))

	return b.String()
}

func (m *SettingsModel) renderNotificationsProjectsSection() string {
	var b strings.Builder
	const labelWidth = 22
	const indent = 24

	b.WriteString(renderFormLabelWithWidth("Project Health: ", labelWidth, m.focusIndex == 82))
	b.WriteString(renderFormToggle("Enabled", "Disabled", m.alertProjectHealthEnabled, m.focusIndex == 82))
	b.WriteString("\n")

	b.WriteString(renderFormLabelWithWidth("Stale (days): ", labelWidth, m.focusIndex == 83))
	b.WriteString(m.alertProjectStaleDaysInput.View())
	b.WriteString("\n")
	if m.focusIndex == 83 && m.validationErr != "" {
		b.WriteString(strings.Repeat(" ", indent))
		b.WriteString(ValidationErrorStyle.Render(m.validationErr))
		b.WriteString("\n")
	}
	b.WriteString(renderFormFieldDesc("Days without task updates to flag project health", indent, m.focusIndex == 83))

	return b.String()
}
