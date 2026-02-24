package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/sglavoie/tt/internal/config"
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

// inboxInnerWidth returns the available content width inside a box (border + padding).
func inboxInnerWidth(width int) int {
	// 2 border chars + 4 padding chars (2 each side)
	w := width - 6
	if w < 20 {
		w = 20
	}
	return w
}

// renderSettingLine renders a single label: value line with right-aligned label.
func renderSettingLine(label, value string) string {
	padded := fmt.Sprintf("%*s", FormLabelWidth, label)
	return MutedStyle.Render(padded+":") + " " + value
}

// inboxSnapshot stores task state for undo in inbox processors.
type inboxSnapshot struct {
	section   string
	priority  int
	tags      []string
	dueDate   *time.Time
	dependsOn []string
}

// captureInboxSnapshot creates a snapshot of task state for undo.
func captureInboxSnapshot(task *models.Task) *inboxSnapshot {
	var dueDate *time.Time
	if task.DueDate != nil {
		d := *task.DueDate
		dueDate = &d
	}

	return &inboxSnapshot{
		section:   task.Section,
		priority:  task.Priority,
		tags:      append([]string{}, task.Tags...),
		dueDate:   dueDate,
		dependsOn: append([]string{}, task.DependsOn...),
	}
}

// restoreInboxSnapshot restores a task from a snapshot.
func restoreInboxSnapshot(task *models.Task, snap *inboxSnapshot) {
	task.Section = snap.section
	task.Priority = snap.priority
	task.Tags = append([]string{}, snap.tags...)
	task.DependsOn = append([]string{}, snap.dependsOn...)
	if snap.dueDate != nil {
		d := *snap.dueDate
		task.DueDate = &d
	} else {
		task.DueDate = nil
	}
	task.UpdatedAt = time.Now()
}

// willLeaveInbox returns true if the task will no longer appear in inbox after save.
func willLeaveInbox(task *models.Task) bool {
	return task.Section != models.SectionInbox || task.IsComplete()
}

// renderInboxTaskDisplay renders the current task details for both local and global inbox.
// Pass projectName="" to omit the project line (local inbox).
func renderInboxTaskDisplay(task *models.Task, width int, willLeave bool, projectName string, cfg *config.Config) string {
	var b strings.Builder
	inner := inboxInnerWidth(width)

	// Title: #ID prefix on first line, wraps to 2 lines then truncates
	prefix := fmt.Sprintf("#%d  ", task.ShortID)
	titleWidth := inner - len(prefix)
	if willLeave {
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
	if willLeave {
		firstLine += "  " + TriagedBadgeStyle.Render("↗ Triaged")
	}
	b.WriteString(BoldStyle.Render(firstLine))
	indent := strings.Repeat(" ", len(prefix))
	b.WriteString("\n")
	b.WriteString(BoldStyle.Render(indent + titleLines[1]))
	b.WriteString("\n")

	// Project context (global inbox only)
	if projectName != "" {
		b.WriteString(MutedStyle.Render(fmt.Sprintf("Project: %s", projectName)))
		b.WriteString("\n")
	}

	// Type badge, status badge, created date, priority, due date
	typeIcon := GetTypeBadge(string(task.Type), task.Type.Icon())
	statusIcon := GetStatusBadge(string(task.Status), task.Status.Icon())
	metadata := fmt.Sprintf("%s %s  Created %s  Priority %s",
		typeIcon, statusIcon,
		MutedStyle.Render(task.CreatedAt.Format("Jan 2")),
		GetPriorityStyle(task.Priority).Render(renderPriorityStarsShared(task.Priority)),
	)
	if task.DueDate != nil {
		metadata += fmt.Sprintf("  Due %s", DueDateStyle.Render(task.DueDate.Format("Jan 2")))
	}
	b.WriteString(metadata)
	b.WriteString("\n")

	// Description: always reserve 3 lines for consistent box height
	descLines := make([]string, 3)
	if task.Description != "" {
		rendered := renderMarkdown(task.Description, inner)
		lines := strings.Split(rendered, "\n")
		if len(lines) > 3 {
			lines = lines[:3]
			lines[2] = util.Truncate(lines[2], inner-3) + "..."
		}
		copy(descLines, lines)
	}
	b.WriteString(MutedStyle.Render(strings.Join(descLines, "\n")))
	b.WriteString("\n")

	// Current settings as mini table
	b.WriteString(renderInboxCurrentSettings(task, cfg))

	// Wrap in compact box style; use selected (blue border) if triaged
	content := b.String()
	if willLeave {
		return inboxSelectedBoxStyle().Render(content)
	}
	return inboxBoxStyle().Render(content)
}

// renderInboxCurrentSettings renders the current task settings as a vertical label-value layout.
func renderInboxCurrentSettings(task *models.Task, cfg *config.Config) string {
	var lines []string

	section := task.Section
	if section == "" {
		section = "(no section)"
	}
	sectionColorName := cfg.GetGlobalSectionColor(section)
	sectionStyle := GetSectionColorStyle(sectionColorName)
	lines = append(lines, renderSettingLine("Section", sectionStyle.Render(section)))
	lines = append(lines, renderSettingLine("Priority", GetPriorityStyle(task.Priority).Render(renderPriorityStarsShared(task.Priority))))

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

// renderInboxOperationsPanel renders the available operations.
func renderInboxOperationsPanel() string {
	var b strings.Builder

	b.WriteString(SubtitleStyle.Render("Operations"))
	b.WriteString("\n")

	ops := []struct{ key, desc string }{
		{"s", "Section"}, {"1-5", "Priority"}, {"t", "Manage tags"},
		{"c", "Create subtask"}, {"e", "Edit"}, {"d", "Due date"},
		{"w", "Waiting"}, {"b", "Dependencies"}, {"D", "Delete"}, {"u", "Undo"},
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

// renderInboxFooter renders the footer with navigation controls.
func renderInboxFooter(width int) string {
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
	return renderStatusBarFooter(strings.Join(parts, "  "), "", width)
}
