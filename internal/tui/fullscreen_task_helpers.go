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

// renderInlineTags renders a compact inline tag string showing at most 2 tags,
// with per-tag truncation at 8 chars and an overflow count.
// Returns an empty string if there are no tags. Result is prefixed with a space.
func renderInlineTags(tags []string) string {
	s := renderTruncatedTags(tags, 2, 8)
	if s == "" {
		return ""
	}
	return " " + s
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
func renderDueDateContent(task *models.Task) string {
	return renderDueDateContentOpts(task, false)
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

// renderMetadataContent builds the tags, section, custom section, and type-specific
// attributes content shared by sidebar and task detail.
func renderMetadataContent(task *models.Task, project *models.Project, innerWidth int, showAttrs ...bool) string {
	var lines []string

	if len(task.Tags) > 0 {
		var tagParts []string
		for _, tag := range task.Tags {
			tagParts = append(tagParts, TagInlineStyle.Render("#"+tag))
		}
		lines = append(lines, strings.Join(tagParts, " "))
	}

	sectionName := task.Section
	if sectionName == "" {
		sectionName = "Active"
	}
	sectionColor := project.GetSectionColor(task.Section)
	sectionStyle := GetSectionColorStyle(sectionColor)
	sectionLine := MutedStyle.Render("Section: ") + sectionStyle.Render(sectionName)
	lines = append(lines, sectionLine)

	if task.CustomSection != "" {
		csColor := project.GetSectionColor(task.CustomSection)
		csStyle := GetSectionColorStyle(csColor)
		csLine := MutedStyle.Render("Custom:  ") + csStyle.Render(task.CustomSection)
		lines = append(lines, csLine)
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
