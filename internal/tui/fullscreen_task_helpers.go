package tui

import (
	"fmt"
	"image/color"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

// renderFullscreenTitle renders a title line for fullscreen views with optional subtitle.
// Pass an optional accent color to override the title foreground color.
func renderFullscreenTitle(title, subtitle string, accent ...color.Color) string {
	return renderFullscreenTitleContent(fullscreenTitleStyle(accent...).Render(title), subtitle)
}

func fullscreenTitleStyle(accent ...color.Color) lipgloss.Style {
	if len(accent) > 0 && accent[0] != nil {
		return lipgloss.NewStyle().Bold(true).Foreground(accent[0])
	}
	return TitleStyle
}

func renderFullscreenTitleContent(titleContent, subtitle string) string {
	t := titleContent
	if subtitle != "" {
		t += "  " + MutedStyle.Render(subtitle)
	}
	return t
}

func renderTaskScopedFullscreenTitle(prefix string, task *models.Task, subtitle string, width int, accent ...color.Color) string {
	titleStyle := fullscreenTitleStyle(accent...)
	if task == nil {
		return fitStyledChromeToWidth("fullscreen title", renderFullscreenTitleContent(titleStyle.Render(prefix), subtitle), width)
	}
	if width <= 0 {
		titleContent := titleStyle.Render(prefix) + TaskTitleStyleFor(task, TaskTitleStyleOptions{}).Render(task.Title)
		return renderFullscreenTitleContent(titleContent, subtitle)
	}

	taskTitleWidth := width - lipgloss.Width(titleStyle.Render(prefix))
	if subtitle != "" {
		taskTitleWidth -= lipgloss.Width("  " + MutedStyle.Render(subtitle))
	}
	// The task title is dense data: clip it intentionally to its own budget.
	taskTitle := truncateToVisualWidth(task.Title, max(taskTitleWidth, 0))
	titleContent := titleStyle.Render(prefix) + TaskTitleStyleFor(task, TaskTitleStyleOptions{}).Render(taskTitle)
	// The composed title line is chrome; once the data title is pre-fit, any
	// residual clip here means the prefix/subtitle chrome did not fit — audit it.
	return fitStyledChromeToWidth("fullscreen title", renderFullscreenTitleContent(titleContent, subtitle), width)
}

// renderFullscreenTitleWithCounter wraps renderFullscreenTitle with a
// right-aligned open-loops counter. The counter is dropped silently when
// width <= 0, counter == "", or there isn't enough room left after the title
// and subtitle to render it cleanly (the subtitle wins because it is
// view-state-specific).
func renderFullscreenTitleWithCounter(title, subtitle, counter string, width int, accent ...color.Color) string {
	return appendFullscreenTitleCounter(renderFullscreenTitle(title, subtitle, accent...), counter, width)
}

func appendFullscreenTitleCounter(base, counter string, width int) string {
	if counter == "" || width <= 0 {
		return base
	}
	rendered := MutedStyle.Render(counter)
	gap := width - lipgloss.Width(base) - lipgloss.Width(rendered)
	if gap <= 0 {
		return base
	}
	return base + strings.Repeat(" ", gap) + rendered
}

// renderPriorityBadge renders a priority indicator with color styling. An unset
// priority has no glyph of its own, so it renders nothing rather than borrowing
// the normal-priority icon.
func renderPriorityBadge(priority int) string {
	if models.ValidatePriority(priority, false) != nil {
		return ""
	}
	return GetPriorityStyle(priority).Render(PriorityIcon(priority))
}

// renderTaskHeaderContent builds the common task header content.
func renderTaskHeaderContent(task *models.Task, hasIncompleteDeps bool, showSnooze bool) string {
	state := task.State()
	statusStyle := GetStateStyle(state)

	if hasIncompleteDeps {
		statusStyle = BlockedByDepsStyle
	}

	title := TaskTitleStyleFor(task, TaskTitleStyleOptions{}).Render(task.Title)
	statusLine := statusStyle.Render(state.Icon() + " " + string(state))
	idLine := fmt.Sprintf("#%d", task.ShortID)
	if priorityBadge := renderPriorityBadge(task.Priority); priorityBadge != "" {
		idLine += " " + priorityBadge
	}

	content := title + "\n" + statusLine + " " + idLine

	if showSnooze && task.IsSnoozed() {
		snoozeStr := task.SnoozedUntil.Format("Jan 2")
		if task.SnoozedUntil.Year() != time.Now().Year() {
			snoozeStr = task.SnoozedUntil.Format("Jan 2 '06")
		}
		content += "\n" + MutedStyle.Render("zzz "+snoozeStr)
	}

	return content
}

// renderDueDateContentOpts builds the styled due date string. When hideRecurrence
// is true, the "Recurs:" line is omitted (useful when a dedicated Recurrence card
// is shown nearby).
func renderDueDateContentOpts(task *models.Task, hideRecurrence bool) string {
	dueAbsolute := task.DueDate.Format(util.DateFormat)
	dueRelative := formatRelativeDueVerbose(task.DueDate)
	dueLine := fmt.Sprintf("%s (%s)", dueAbsolute, dueRelative)

	if task.IsOverdue() {
		dueLine = OverdueStyle.Render(dueLine)
	} else if task.IsDueToday() {
		dueLine = DueTodayStyle.Render(dueLine)
	} else {
		dueLine = DueDateStyle.Render(dueLine)
	}

	if !hideRecurrence && task.Recurrence != nil {
		dueLine += "\n" + MutedStyle.Render("Recurs: "+task.Recurrence.String())
	}

	return dueLine
}

// renderMetadataContentOpts is like renderMetadataContent but can optionally hide
// the section lines (when they are shown elsewhere, e.g. in the header).
func renderMetadataContentOpts(task *models.Task, project *models.Project, innerWidth int, hideSection bool, showAttrs ...bool) string {
	var lines []string

	if len(task.Tags) > 0 {
		var tagParts []string
		for _, tag := range task.Tags {
			tagParts = append(tagParts, TagInlineStyle.Render("#"+tag))
		}
		lines = append(lines, strings.Join(tagParts, " "))
	}

	if !hideSection {
		sectionName := taskStateGroupLabel(task)
		if sectionName == "" {
			sectionName = "Active"
		}
		sectionStyle := lipgloss.NewStyle().Foreground(GetWorkflowStateGroupAccent(taskStateGroup(task)))
		sectionLine := MutedStyle.Render("Section: ") + sectionStyle.Render(sectionName)
		lines = append(lines, sectionLine)
	}

	return strings.Join(lines, "\n")
}
