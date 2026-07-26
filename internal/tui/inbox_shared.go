package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
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
	w := max(width-6, 20)
	return w
}

// renderSettingLine renders a single label: value line with right-aligned label.
func renderSettingLine(label, value string) string {
	padded := fmt.Sprintf("%*s", FormLabelWidth, label)
	return MutedStyle.Render(padded+":") + " " + value
}

// inboxSnapshot stores task state for undo in inbox processors. The task's
// lifecycle is captured as a single canonical WorkflowState; the remaining
// fields are orthogonal task attributes that inbox triage also mutates and must
// restore. The retired Section/Status/Stage/Active shims are no longer captured
// — ApplyWorkflowState reconstitutes them from the stored state on restore.
type inboxSnapshot struct {
	priority     int
	tags         []string
	dueDate      *time.Time
	hasTime      bool
	dependsOn    []string
	state        models.WorkflowState
	completedAt  *time.Time
	snoozedUntil *time.Time
}

// captureInboxSnapshot creates a snapshot of task state for undo.
func captureInboxSnapshot(task *models.Task) *inboxSnapshot {
	var dueDate *time.Time
	if task.DueDate != nil {
		d := *task.DueDate
		dueDate = &d
	}

	var completedAt *time.Time
	if task.CompletedAt != nil {
		c := *task.CompletedAt
		completedAt = &c
	}

	var snoozedUntil *time.Time
	if task.SnoozedUntil != nil {
		s := *task.SnoozedUntil
		snoozedUntil = &s
	}

	return &inboxSnapshot{
		priority:     task.Priority,
		tags:         append([]string{}, task.Tags...),
		dueDate:      dueDate,
		hasTime:      task.HasTime,
		dependsOn:    append([]string{}, task.DependsOn...),
		state:        task.State(),
		completedAt:  completedAt,
		snoozedUntil: snoozedUntil,
	}
}

// restoreInboxSnapshot restores a task from a snapshot. The lifecycle is
// reconstituted from the stored WorkflowState via ApplyWorkflowState (which
// keeps the derived Section/Status coherent); CompletedAt is restored
// afterward since ApplyWorkflowState intentionally leaves the terminal
// timestamp untouched.
func restoreInboxSnapshot(task *models.Task, snap *inboxSnapshot) {
	task.Priority = snap.priority
	task.Tags = append([]string{}, snap.tags...)
	task.DependsOn = append([]string{}, snap.dependsOn...)
	if snap.dueDate != nil {
		d := *snap.dueDate
		task.DueDate = &d
	} else {
		task.DueDate = nil
	}
	task.HasTime = snap.hasTime
	task.ApplyWorkflowState(snap.state)
	if snap.completedAt != nil {
		c := *snap.completedAt
		task.CompletedAt = &c
	} else {
		task.CompletedAt = nil
	}
	if snap.snoozedUntil != nil {
		s := *snap.snoozedUntil
		task.SnoozedUntil = &s
	} else {
		task.SnoozedUntil = nil
	}
	task.UpdatedAt = time.Now()
}

func isProcessableInboxTask(task *models.Task) bool {
	return task != nil &&
		task.InSection(models.SectionInbox) &&
		!task.IsTerminalState() &&
		!task.IsSnoozed()
}

// willLeaveInbox returns true if the task will no longer appear in inbox after save.
func willLeaveInbox(task *models.Task) bool {
	return !task.InSection(models.SectionInbox) || task.IsComplete()
}

// renderInboxTaskDisplay renders the current task details for both local and global inbox.
// Pass project=nil to omit the project line (local inbox).
func renderInboxTaskDisplay(task *models.Task, width int, willLeave bool, project *models.Project, settings *config.Settings) string {
	var b strings.Builder
	inner := inboxInnerWidth(width)

	// Title: #ID prefix on first line, wraps to 2 lines then truncates
	prefix := fmt.Sprintf("#%d  ", task.ShortID)
	titleWidth := inner - lipgloss.Width(prefix)
	if willLeave {
		titleWidth -= lipgloss.Width("  ↗ Triaged")
	}
	titleWidth = max(titleWidth, 1)
	breadcrumb := compactAncestorBreadcrumb(taskAncestorBreadcrumb(task), max(titleWidth-4, 0))
	breadcrumbRendered := ""
	if breadcrumb != "" {
		breadcrumbRendered = MutedStyle.Render(breadcrumb) + " "
		titleWidth = max(titleWidth-lipgloss.Width(breadcrumbRendered), 1)
	}
	wrapped := lipgloss.NewStyle().Width(titleWidth).Render(task.Title)
	rawLines := strings.Split(wrapped, "\n")
	if len(rawLines) > 2 {
		rawLines = rawLines[:2]
		rawLines[1] = truncateToVisualWidth(strings.TrimRight(rawLines[1], " "), titleWidth)
	}
	titleLines := make([]string, 2)
	copy(titleLines, rawLines)
	titleStyle := TaskTitleStyleFor(task, TaskTitleStyleOptions{})
	firstLine := BoldStyle.Render(prefix) + breadcrumbRendered + titleStyle.Render(titleLines[0])
	if willLeave {
		firstLine += "  " + TriagedBadgeStyle.Render("↗ Triaged")
	}
	b.WriteString(firstLine)
	indent := strings.Repeat(" ", len(prefix)+lipgloss.Width(breadcrumbRendered))
	b.WriteString("\n")
	b.WriteString(indent + titleStyle.Render(titleLines[1]))
	b.WriteString("\n")

	// Project context (global inbox only)
	if project != nil {
		b.WriteString(MutedStyle.Render("Project: "))
		b.WriteString(projectColorStyle(project).Render(project.Name))
		b.WriteString("\n")
	}

	// Type badge, status badge, created date, priority, due date
	typeIcon := GetTypeBadge(string(task.Type), task.Type.Icon())
	statusIcon := GetStateBadge(task.State())
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
			lines[2] = truncateToVisualWidth(strings.TrimRight(lines[2], " "), inner)
		}
		copy(descLines, lines)
	}
	b.WriteString(MutedStyle.Render(strings.Join(descLines, "\n")))
	b.WriteString("\n")

	// Current settings as mini table
	b.WriteString(renderInboxCurrentSettings(task, settings))

	// Wrap in compact box style; use selected (blue border) if triaged
	content := b.String()
	if willLeave {
		return inboxSelectedBoxStyle().Width(inner).Render(content)
	}
	return inboxBoxStyle().Width(inner).Render(content)
}

// renderInboxCurrentSettings renders the current task settings as a vertical label-value layout.
func renderInboxCurrentSettings(task *models.Task, settings *config.Settings) string {
	var lines []string

	section := taskSectionBucket(task)
	if section == "" {
		section = "(no section)"
	}
	sectionStyle := lipgloss.NewStyle().Foreground(GetWorkflowStateGroupAccent(taskStateGroup(task)))
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

// renderInboxOperationsPanel renders the triage panel around the central
// question "Where should this task go next?". Primary direct-key actions live
// in the footer; this panel surfaces the routing/resolution/refinement prefix
// vocabulary that answers the question, chunked into four groups (Route,
// Schedule, Resolve, Refine) so power users see the intent column at a glance.
func renderInboxOperationsPanel() string {
	var b strings.Builder

	// UnsetMarginBottom keeps the question's accent color but drops the trailing
	// blank line that SubtitleStyle injects, so the four group rows can fit the
	// 80x24 budget without the global inbox spilling past the height ceiling.
	b.WriteString(SubtitleStyle.UnsetMarginBottom().Render("Where should this task go next?"))
	b.WriteString("\n")

	type op struct{ key, desc string }
	groups := []struct {
		label string
		ops   []op
	}{
		{"Route", []op{{"m*", "Route"}, {"c*", "Linked"}}},
		{"Schedule", []op{{"w", "Today"}, {"d*", "Schedule"}, {"z*", "Snooze"}}},
		{"Resolve", []op{{"D", "Done"}, {",*", "Status"}}},
		{"Refine", []op{{"e", "Edit"}, {"T", "Tags"}, {"!/1-5", "Priority"}}},
	}

	labelWidth := 0
	for _, g := range groups {
		if w := lipgloss.Width(g.label); w > labelWidth {
			labelWidth = w
		}
	}

	var lines []string
	for _, g := range groups {
		var parts []string
		for _, o := range g.ops {
			parts = append(parts, HelpKeyStyle.Render(o.key)+MutedStyle.Render(": "+o.desc))
		}
		padding := strings.Repeat(" ", labelWidth-lipgloss.Width(g.label))
		label := BoldStyle.Render(g.label) + padding
		lines = append(lines, label+"  "+strings.Join(parts, "  "))
	}
	lines = append(lines, MutedStyle.Render("More: ")+
		HelpKeyStyle.Render("t*")+MutedStyle.Render(" task ops · ")+
		HelpKeyStyle.Render("?")+MutedStyle.Render(" help · ")+
		HelpKeyStyle.Render(":")+MutedStyle.Render(" palette"))

	content := strings.Join(lines, "\n")
	return DashboardCardStyle.Render(b.String() + content)
}

func renderInboxOperationsPanelCompact(width int) string {
	parts := []string{
		HelpKeyStyle.Render("m*") + MutedStyle.Render(": Route"),
		HelpKeyStyle.Render("w") + MutedStyle.Render(": Today"),
		HelpKeyStyle.Render("d*") + MutedStyle.Render(": Schedule"),
		HelpKeyStyle.Render("D") + MutedStyle.Render(": Done"),
		HelpKeyStyle.Render("T") + MutedStyle.Render(": Tags"),
		HelpKeyStyle.Render("?") + MutedStyle.Render(": Help"),
	}
	line := MutedStyle.Render("Next: ") + strings.Join(parts, MutedStyle.Render(" · "))
	return truncateToVisualWidth(line, width)
}

// renderInboxFooter renders the footer with the primary triage vocabulary —
// the six single-tap actions shared by both inbox processors plus Esc.
// Navigation and prefix subkeys are discoverable via ? (help) and : (palette).
func renderInboxFooter(width int) string {
	controls := []struct{ key, desc string }{
		{"n", "Skip"},
		{"e", "Edit"},
		{"D", "Done"},
		{"T", "Tags"},
		{"u", "Undo"},
		{"?", "Help"},
		{"esc", "Back"},
	}
	var parts []string
	for _, c := range controls {
		parts = append(parts, HelpKeyStyle.Render(c.key)+MutedStyle.Render(": "+c.desc))
	}
	return renderStatusBarFooter(strings.Join(parts, "  "), "", width)
}
