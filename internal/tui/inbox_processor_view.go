package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

// inboxBoxStyle returns a compact box style with no vertical padding.
func inboxBoxStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorBorder).
		Padding(0, 2)
}

// inboxSelectedBoxStyle returns a compact selected box style with no vertical padding.
func inboxSelectedBoxStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorPrimary).
		Padding(0, 2)
}

// renderHeader renders the header with progress information.
func (m *InboxProcessorModel) renderHeader() string {
	processed, skipped, remaining, total := m.getProgressStats()
	title := TitleStyle.Render("Inbox Processing")

	// Progress bar based on current position
	var percent float64
	if total > 0 {
		percent = float64(m.currentIndex+1) / float64(total) * 100
	}
	barWidth := 30
	bar := RenderProgressBar(percent, barWidth)
	progressBar := fmt.Sprintf("%s %.0f%%", bar, percent)

	progress := MutedStyle.Render(fmt.Sprintf(
		"Processing %d/%d tasks (%d processed, %d skipped, %d remaining)",
		m.currentIndex+1, total, processed, skipped, remaining,
	))
	return fmt.Sprintf("%s\n%s\n%s", title, progressBar, progress)
}

// innerWidth returns the available content width inside a box (border + padding).
func (m *InboxProcessorModel) innerWidth() int {
	// 2 border chars + 4 padding chars (2 each side)
	w := m.width - 6
	if w < 20 {
		w = 20
	}
	return w
}

// renderTaskDisplay renders the current task details.
func (m *InboxProcessorModel) renderTaskDisplay(task *models.Task) string {
	var b strings.Builder
	inner := m.innerWidth()

	// Title: #ID prefix on first line, wraps to 2 lines then truncates
	prefix := fmt.Sprintf("#%d  ", task.ShortID)
	titleWidth := inner - len(prefix)
	if m.willLeaveInbox(task) {
		titleWidth -= len("  ↗ Triaged")
	}
	wrapped := lipgloss.NewStyle().Width(titleWidth).Render(task.Title)
	rawLines := strings.Split(wrapped, "\n")
	if len(rawLines) > 2 {
		rawLines = rawLines[:2]
		rawLines[1] = util.Truncate(rawLines[1], titleWidth-3) + "..."
	}
	titleLines := make([]string, 2)
	copy(titleLines, rawLines)
	firstLine := prefix + titleLines[0]
	if m.willLeaveInbox(task) {
		firstLine += "  " + TriagedBadgeStyle.Render("↗ Triaged")
	}
	b.WriteString(BoldStyle.Render(firstLine))
	indent := strings.Repeat(" ", len(prefix))
	b.WriteString("\n")
	b.WriteString(BoldStyle.Render(indent + titleLines[1]))
	b.WriteString("\n")

	// Line 2: type badge, status badge, created date, priority, due date
	typeIcon := GetTypeBadge(string(task.Type), task.Type.Icon())
	statusIcon := GetStatusBadge(string(task.Status), task.Status.Icon())
	metadata := fmt.Sprintf("%s %s  Created %s  Priority %s",
		typeIcon, statusIcon,
		MutedStyle.Render(task.CreatedAt.Format("Jan 2")),
		GetPriorityStyle(task.Priority).Render(renderPriorityStars(task.Priority)),
	)
	if task.DueDate != nil {
		metadata += fmt.Sprintf("  Due %s", DueDateStyle.Render(task.DueDate.Format("Jan 2")))
	}
	b.WriteString(metadata)
	b.WriteString("\n")

	// Description: always reserve 3 lines for consistent box height
	descLines := make([]string, 3)
	if task.Description != "" {
		wrapped := lipgloss.NewStyle().Width(inner).Render(task.Description)
		lines := strings.Split(wrapped, "\n")
		if len(lines) > 3 {
			lines = lines[:3]
			lines[2] = util.Truncate(lines[2], inner-3) + "..."
		}
		copy(descLines, lines)
	}
	b.WriteString(MutedStyle.Render(strings.Join(descLines, "\n")))
	b.WriteString("\n")

	// Current settings as mini table
	b.WriteString(m.renderCurrentSettings(task))

	// Wrap in compact box style; use selected (blue border) if triaged
	content := b.String()
	if m.willLeaveInbox(task) {
		return inboxSelectedBoxStyle().Render(content)
	}
	return inboxBoxStyle().Render(content)
}

// renderCurrentSettings renders the current task settings as a vertical label-value layout.
func (m *InboxProcessorModel) renderCurrentSettings(task *models.Task) string {
	var lines []string

	section := task.Section
	if section == "" {
		section = "(no section)"
	}
	sectionColorName := m.config.GetGlobalSectionColor(section)
	sectionStyle := GetSectionColorStyle(sectionColorName)
	lines = append(lines, renderSettingLine("Section", sectionStyle.Render(section)))
	lines = append(lines, renderSettingLine("Priority", GetPriorityStyle(task.Priority).Render(renderPriorityStars(task.Priority))))

	if len(task.Tags) > 0 {
		lines = append(lines, renderSettingLine("Tags", strings.Join(task.Tags, ", ")))
	} else {
		lines = append(lines, renderSettingLine("Tags", "none"))
	}

	if len(task.DependsOn) > 0 {
		lines = append(lines, renderSettingLine("Deps", fmt.Sprintf("%d", len(task.DependsOn))))
	}

	return strings.Join(lines, "\n")
}

// renderSettingLine renders a single label: value line with right-aligned label.
func renderSettingLine(label, value string) string {
	padded := fmt.Sprintf("%*s", FormLabelWidth, label)
	return MutedStyle.Render(padded+":") + " " + value
}

// renderOperationsPanel renders the available operations.
func (m *InboxProcessorModel) renderOperationsPanel() string {
	var b strings.Builder

	b.WriteString(SubtitleStyle.Render("Operations"))
	b.WriteString("\n")

	ops := []struct{ key, desc string }{
		{"s", "Section"}, {"1-5", "Priority"}, {"t", "Manage tags"},
		{"c", "Create subtask"}, {"e", "Edit"}, {"D", "Due date"},
		{"w", "Waiting"}, {"b", "Dependencies"}, {"x", "Delete"}, {"u", "Undo"},
	}

	var lines []string
	for i := 0; i < len(ops); i += 3 {
		var parts []string
		for j := i; j < i+3 && j < len(ops); j++ {
			parts = append(parts, HelpKeyStyle.Render(ops[j].key)+MutedStyle.Render(": "+ops[j].desc))
		}
		lines = append(lines, strings.Join(parts, "  "))
	}

	content := strings.Join(lines, "\n")
	return DashboardCardStyle.Render(b.String() + content)
}

// renderFooter renders the footer with navigation controls.
func (m *InboxProcessorModel) renderFooter() string {
	controls := []struct{ key, desc string }{
		{"n", "Skip"},
		{"j/k", "Navigate"},
		{"gg/G", "First/last"},
		{",s/d/b/x/o", "Status"},
		{"q", "Quit"},
	}
	var parts []string
	for _, c := range controls {
		parts = append(parts, HelpKeyStyle.Render(c.key)+MutedStyle.Render(": "+c.desc))
	}
	return strings.Join(parts, "  ")
}

// renderEmptyState renders the state when current task is nil.
func (m *InboxProcessorModel) renderEmptyState() string {
	content := SuccessStyle.Render("✓ Inbox is empty! All tasks have been triaged.")
	return BoxStyle.Render(content)
}

// renderEmptyInbox renders the view when no inbox tasks exist.
func (m *InboxProcessorModel) renderEmptyInbox() string {
	var b strings.Builder
	b.WriteString(TitleStyle.Render("Inbox Processing"))
	b.WriteString("\n\n")
	content := SuccessStyle.Render("✓ Inbox is empty! All tasks have been triaged.")
	b.WriteString(BoxStyle.Render(content))
	b.WriteString("\n\n")
	b.WriteString(HelpKeyStyle.Render("q") + MutedStyle.Render(": return to project tree"))
	return b.String()
}

// renderPriorityStars renders priority as stars.
func renderPriorityStars(priority int) string {
	if priority <= 0 {
		return ""
	}
	return strings.Repeat("★", priority)
}

// renderPrefixPopup renders the which-key style popup for prefix key sequences.
// Returns empty string if no prefix is pending.
func (m *InboxProcessorModel) renderPrefixPopup() string {
	return RenderPrefixPopup(m.pendingPrefix, PrefixHints{
		",": StatusPrefixHints,
		"g": NavigationPrefixHints,
	}, m.width)
}
