package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/sglavoie/tt/internal/util"
)

// renderHeader renders the header with progress information.
func (m *GlobalInboxProcessorModel) renderHeader() string {
	processed, skipped, remaining, total := m.getProgressStats()
	subtitle := fmt.Sprintf("%d/%d tasks from %d projects (%d processed, %d skipped, %d remaining)",
		m.currentIndex+1, total, len(m.projectCache), processed, skipped, remaining)
	title := renderFullscreenTitle("Global Inbox Processing", subtitle)

	// Progress bar based on current position
	var percent float64
	if total > 0 {
		percent = float64(m.currentIndex+1) / float64(total) * 100
	}
	barWidth := 30
	bar := RenderProgressBar(percent, barWidth)
	progressBar := fmt.Sprintf("%s %.0f%%", bar, percent)

	return fmt.Sprintf("%s\n%s", title, progressBar)
}

// innerWidth returns the available content width inside a box (border + padding).
func (m *GlobalInboxProcessorModel) innerWidth() int {
	// 2 border chars + 4 padding chars (2 each side)
	w := m.width - 6
	if w < 20 {
		w = 20
	}
	return w
}

// renderTaskDisplay renders the current task details.
func (m *GlobalInboxProcessorModel) renderTaskDisplay(git *GlobalInboxTask) string {
	var b strings.Builder
	inner := m.innerWidth()

	task := git.Task
	project := git.Project

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

	// Project context
	b.WriteString(MutedStyle.Render(fmt.Sprintf("Project: %s", project.Name)))
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
	b.WriteString(m.renderCurrentSettings(git))

	// Wrap in compact box style; use selected (blue border) if triaged
	content := b.String()
	if m.willLeaveInbox(task) {
		return inboxSelectedBoxStyle().Render(content)
	}
	return inboxBoxStyle().Render(content)
}

// renderCurrentSettings renders the current task settings as a vertical label-value layout.
func (m *GlobalInboxProcessorModel) renderCurrentSettings(git *GlobalInboxTask) string {
	task := git.Task
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

// renderOperationsPanel renders the available operations.
func (m *GlobalInboxProcessorModel) renderOperationsPanel() string {
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
func (m *GlobalInboxProcessorModel) renderFooter() string {
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
	return renderStatusBarFooter(strings.Join(parts, "  "), "", m.width)
}

// renderEmptyState renders the state when current task is nil.
func (m *GlobalInboxProcessorModel) renderEmptyState() string {
	content := SuccessStyle.Render("✓ All inboxes are empty! All tasks have been triaged.")
	return BoxStyle.Render(content)
}

// renderEmptyInbox renders the view when no inbox tasks exist.
func (m *GlobalInboxProcessorModel) renderEmptyInbox() string {
	var b strings.Builder
	b.WriteString(renderFullscreenTitle("Global Inbox Processing", ""))
	b.WriteString("\n\n")
	content := SuccessStyle.Render("✓ All inboxes are empty! No tasks to process across any project.")
	b.WriteString(BoxStyle.Render(content))
	b.WriteString("\n\n")
	b.WriteString(HelpKeyStyle.Render("q") + MutedStyle.Render(": return to dashboard"))
	return b.String()
}

// renderPrefixPopup renders the which-key style popup for prefix key sequences.
// Returns empty string if no prefix is pending.
func (m *GlobalInboxProcessorModel) renderPrefixPopup() string {
	return RenderPrefixPopup(m.pendingPrefix, PrefixHints{
		",": StatusPrefixHints,
		"g": NavigationPrefixHints,
	}, m.width)
}
