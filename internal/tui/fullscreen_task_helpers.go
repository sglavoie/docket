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
	var titleStyle lipgloss.Style
	if len(accent) > 0 && accent[0] != nil {
		titleStyle = lipgloss.NewStyle().Bold(true).Foreground(accent[0])
	} else {
		titleStyle = TitleStyle
	}
	t := titleStyle.Render(title)
	if subtitle != "" {
		t += "  " + MutedStyle.Render(subtitle)
	}
	return t
}

// titleCounterAlertReserve mirrors the alert-indicator reserve used by
// kanban_view.go so the right-aligned open-loops counter never collides with
// the proactive-alert badge rendered by the app shell.
const titleCounterAlertReserve = 10

// renderFullscreenTitleWithCounter wraps renderFullscreenTitle with a
// right-aligned open-loops counter. The counter is dropped silently when
// width <= 0, counter == "", or there isn't enough room left after the title +
// subtitle + alert reserve to render it cleanly (the subtitle wins because it
// is view-state-specific).
func renderFullscreenTitleWithCounter(title, subtitle, counter string, width int, accent ...color.Color) string {
	base := renderFullscreenTitle(title, subtitle, accent...)
	if counter == "" || width <= 0 {
		return base
	}
	rendered := MutedStyle.Render(counter)
	gap := width - lipgloss.Width(base) - lipgloss.Width(rendered) - titleCounterAlertReserve
	if gap <= 0 {
		return base
	}
	return base + strings.Repeat(" ", gap) + rendered
}

// renderPriorityBadge renders a priority indicator with color styling.
func renderPriorityBadge(priority int) string {
	return GetPriorityStyle(priority).Render(PriorityIcon(priority))
}

// renderTaskHeaderContent builds the common task header content:
// title, type badge + status line, priority badge + ID, and optional snooze line.
func renderTaskHeaderContent(task *models.Task, hasIncompleteDeps bool, showSnooze bool) string {
	typeStyle := GetTypeStyle(string(task.Type))
	statusStyle := GetStatusStyle(string(task.Status))

	if hasIncompleteDeps {
		statusStyle = BlockedByDepsStyle
	}

	title := SidebarHeaderStyle.Render(task.Title)
	typeLine := GetTypeBadge(string(task.Type), task.Type.Icon()) + " " + typeStyle.Render(task.Type.HeadingPrefix())
	statusLine := statusStyle.Render(task.Status.Icon() + " " + string(task.Status))
	priorityBadge := renderPriorityBadge(task.Priority)
	idLine := fmt.Sprintf("#%d %s", task.ShortID, priorityBadge)

	content := title + "\n" + typeLine + " " + statusLine + " " + idLine

	if showSnooze && task.IsSnoozed() {
		snoozeStr := task.SnoozedUntil.Format("Jan 2")
		if task.SnoozedUntil.Year() != time.Now().Year() {
			snoozeStr = task.SnoozedUntil.Format("Jan 2 '06")
		}
		content += "\n" + MutedStyle.Render("zzz "+snoozeStr)
	}

	return content
}

// renderDueDateContent builds the styled due date string with overdue/today/normal
// coloring, and appends a recurrence line when the task has one.
// timeFormat is a Go time layout string (e.g. "15:04" or "3:04pm").
func renderDueDateContent(task *models.Task, timeFormat string) string {
	return renderDueDateContentOpts(task, false, timeFormat)
}

// renderDueDateContentOpts builds the styled due date string. When hideRecurrence
// is true, the "Recurs:" line is omitted (useful when a dedicated Recurrence card
// is shown nearby). timeFormat is a Go time layout string.
func renderDueDateContentOpts(task *models.Task, hideRecurrence bool, timeFormat string) string {
	dueAbsolute := task.DueDate.Format(util.DateFormat)
	dueAbsolute = AppendTimeIfHasTime(task, dueAbsolute, timeFormat)
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

// renderMetadataContent builds the tags, section, custom section, and type-specific
// attributes content shared by sidebar and task detail.
func renderMetadataContent(task *models.Task, project *models.Project, innerWidth int, showAttrs ...bool) string {
	return renderMetadataContentOpts(task, project, innerWidth, false, showAttrs...)
}

// renderMetadataContentOpts is like renderMetadataContent but can optionally hide
// the section/custom-section lines (when they are shown elsewhere, e.g. in the header).
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
		sectionName := task.Section
		if sectionName == "" && task.CustomSection != "" {
			sectionName = task.CustomSection
		} else if sectionName == "" {
			sectionName = "Active"
		}
		effectiveSection := task.Section
		if effectiveSection == "" && task.CustomSection != "" {
			effectiveSection = task.CustomSection
		}
		sectionColor := project.GetSectionColor(effectiveSection)
		sectionStyle := GetSectionColorStyle(sectionColor)
		sectionLine := MutedStyle.Render("Section: ") + sectionStyle.Render(sectionName)
		lines = append(lines, sectionLine)
	}

	if task.Source != "" {
		lines = append(lines, MutedStyle.Render("Source:  ")+task.Source)
	}
	if task.URL != "" {
		display := truncateToVisualWidth(task.URL, innerWidth-5)
		link := Hyperlink(task.URL, display)
		lines = append(lines, MutedStyle.Render("URL:     ")+link)
	}

	if len(showAttrs) == 0 || showAttrs[0] {
		attrLines := formatTaskAttributes(task, innerWidth)
		if len(attrLines) > 0 {
			lines = append(lines, "")
			lines = append(lines, attrLines...)
		}
	}

	return strings.Join(lines, "\n")
}
