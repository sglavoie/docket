package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

// formatActivityEntry formats a timestamp as an activity entry.
func formatActivityEntry(label string, timestamp time.Time) string {
	absolute := timestamp.Format(util.DateFormat)
	relative := formatRelativeTime(timestamp)
	return fmt.Sprintf("%s: %s (%s)", MutedStyle.Render(label), absolute, relative)
}

// formatRelativeTime returns a human-readable relative time string.
func formatRelativeTime(t time.Time) string {
	duration := time.Since(t)

	switch {
	case duration < time.Minute:
		return "just now"
	case duration < time.Hour:
		mins := int(duration.Minutes())
		if mins == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", mins)
	case duration < util.Day:
		hours := int(duration.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	case duration < util.Week:
		// Calendar-day comparison avoids the wall-clock truncation issue
		// (e.g. 47h ago = 2 calendar days, not "1 day ago").
		days := util.DaysBetween(t, time.Now())
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	case duration < util.Month:
		weeks := int(duration.Hours() / 24 / 7)
		if weeks == 1 {
			return "1 week ago"
		}
		return fmt.Sprintf("%d weeks ago", weeks)
	case duration < util.Year:
		months := int(duration.Hours() / 24 / 30)
		if months == 1 {
			return "1 month ago"
		}
		return fmt.Sprintf("%d months ago", months)
	default:
		years := int(duration.Hours() / 24 / 365)
		if years == 1 {
			return "1 year ago"
		}
		return fmt.Sprintf("%d years ago", years)
	}
}

// formatTaskAttributes formats type-specific attributes for detail-panel display.
func formatTaskAttributes(task *models.Task, maxWidth int) []string {
	var lines []string

	switch task.Type {
	case models.TaskTypeBug:
		if task.BugAttrs != nil {
			if task.BugAttrs.Severity != "" {
				lines = append(lines, MutedStyle.Render("Severity: ")+string(task.BugAttrs.Severity))
			}
			if task.BugAttrs.Reproducible != "" {
				lines = append(lines, MutedStyle.Render("Reproducible: ")+string(task.BugAttrs.Reproducible))
			}
			if task.BugAttrs.Workaround != "" {
				preview := truncateToVisualWidth(task.BugAttrs.Workaround, maxWidth-12)
				preview = strings.ReplaceAll(preview, "\n", " ")
				lines = append(lines, MutedStyle.Render("Workaround: ")+preview)
			}
		}
	case models.TaskTypeIdea:
		if task.IdeaAttrs != nil {
			if task.IdeaAttrs.Category != "" {
				lines = append(lines, MutedStyle.Render("Category: ")+string(task.IdeaAttrs.Category))
			}
			if task.IdeaAttrs.Effort != "" {
				lines = append(lines, MutedStyle.Render("Effort: ")+string(task.IdeaAttrs.Effort))
			}
			if task.IdeaAttrs.Value != "" {
				lines = append(lines, MutedStyle.Render("Value: ")+string(task.IdeaAttrs.Value))
			}
			if task.IdeaAttrs.Validated {
				lines = append(lines, MutedStyle.Render("Validated: ")+"[x]")
			}
		}
	case models.TaskTypeTask:
		if task.TaskAttrs != nil {
			if task.TaskAttrs.Effort != "" {
				lines = append(lines, MutedStyle.Render("Effort: ")+string(task.TaskAttrs.Effort))
			}
			if task.TaskAttrs.WorkType != "" {
				lines = append(lines, MutedStyle.Render("WorkType: ")+string(task.TaskAttrs.WorkType))
			}
		}
	case models.TaskTypeStory:
		if task.StoryAttrs != nil {
			if task.StoryAttrs.StoryPoints > 0 {
				lines = append(lines, MutedStyle.Render("Points: ")+fmt.Sprintf("%d", task.StoryAttrs.StoryPoints))
			}
			if task.StoryAttrs.Persona != "" {
				preview := truncateToVisualWidth(task.StoryAttrs.Persona, maxWidth-9)
				preview = strings.ReplaceAll(preview, "\n", " ")
				lines = append(lines, MutedStyle.Render("Persona: ")+preview)
			}
		}
	case models.TaskTypeEpic:
		if task.EpicAttrs != nil {
			if task.EpicAttrs.Scope != "" {
				lines = append(lines, MutedStyle.Render("Scope: ")+string(task.EpicAttrs.Scope))
			}
			if task.EpicAttrs.Milestone != "" {
				preview := truncateToVisualWidth(task.EpicAttrs.Milestone, maxWidth-11)
				lines = append(lines, MutedStyle.Render("Milestone: ")+preview)
			}
			if task.EpicAttrs.SuccessMetrics != "" {
				preview := truncateToVisualWidth(task.EpicAttrs.SuccessMetrics, maxWidth-9)
				preview = strings.ReplaceAll(preview, "\n", " ")
				lines = append(lines, MutedStyle.Render("Metrics: ")+preview)
			}
		}
	}

	return lines
}
