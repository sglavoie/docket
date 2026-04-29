package format

import (
	"fmt"
	"strings"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

// WriteMetadata generates the metadata comment for a task.
func WriteMetadata(task *models.Task) string { //nolint:gocyclo
	var parts []string

	if task.ID != "" {
		parts = append(parts, fmt.Sprintf("id: %s", task.ID))
	}

	parts = append(parts, fmt.Sprintf("status: %s", task.Status))
	parts = append(parts, fmt.Sprintf("priority: %d", task.Priority))

	if task.ParentID != nil && *task.ParentID != "" {
		parts = append(parts, fmt.Sprintf("parent: %s", *task.ParentID))
	}

	if len(task.Tags) > 0 {
		parts = append(parts, fmt.Sprintf("tags: %s", strings.Join(task.Tags, ";")))
	}

	if task.Section != "" {
		parts = append(parts, fmt.Sprintf("section: %s", task.Section))
	}

	if task.CustomSection != "" {
		parts = append(parts, fmt.Sprintf("custom_section: %s", task.CustomSection))
	}

	if !task.CreatedAt.IsZero() {
		parts = append(parts, fmt.Sprintf("created: %s", task.CreatedAt.Format(util.DateTimeFormat)))
	}

	if !task.UpdatedAt.IsZero() && !task.UpdatedAt.Equal(task.CreatedAt) {
		parts = append(parts, fmt.Sprintf("updated: %s", task.UpdatedAt.Format(util.DateTimeFormat)))
	}

	if task.CompletedAt != nil {
		parts = append(parts, fmt.Sprintf("completed: %s", task.CompletedAt.Format(util.DateTimeFormat)))
	}

	if task.DueDate != nil {
		if task.HasTime {
			parts = append(parts, fmt.Sprintf("due: %s", task.DueDate.Format(util.DateTimeDueFormat)))
		} else {
			parts = append(parts, fmt.Sprintf("due: %s", task.DueDate.Format(util.DateFormat)))
		}
	}

	if task.StartDate != nil {
		parts = append(parts, fmt.Sprintf("start: %s", task.StartDate.Format(util.DateFormat)))
	}

	if task.EstimatedDays > 0 {
		parts = append(parts, fmt.Sprintf("estimated_days: %g", task.EstimatedDays))
	}

	if len(task.DependsOn) > 0 {
		parts = append(parts, fmt.Sprintf("depends: %s", strings.Join(task.DependsOn, ";")))
	}

	if len(task.RelatedTo) > 0 {
		parts = append(parts, fmt.Sprintf("related: %s", strings.Join(task.RelatedTo, ";")))
	}

	if task.Pinned {
		parts = append(parts, "pinned: true")
		if task.PinnedAt != nil {
			parts = append(parts, fmt.Sprintf("pinned_at: %s", task.PinnedAt.Format(pinnedAtFormat)))
		}
	}

	if task.Active {
		parts = append(parts, "active: true")
	}

	if task.Dimmed {
		parts = append(parts, "dimmed: true")
	}

	if task.SnoozedUntil != nil {
		parts = append(parts, fmt.Sprintf("snoozed: %s", task.SnoozedUntil.Format(util.DateFormat)))
	}

	if task.Recurrence != nil {
		parts = append(parts, fmt.Sprintf("recur: %s", FormatRecurrence(task.Recurrence)))
		if task.Recurrence.Skipped > 0 {
			parts = append(parts, fmt.Sprintf("recur.skipped: %d", task.Recurrence.Skipped))
		}
		if task.Recurrence.Streak > 0 {
			parts = append(parts, fmt.Sprintf("recur.streak: %d", task.Recurrence.Streak))
		}
		if task.Recurrence.MaxStreak > 0 {
			parts = append(parts, fmt.Sprintf("recur.max_streak: %d", task.Recurrence.MaxStreak))
		}
		if task.Recurrence.OriginalDay > 0 {
			parts = append(parts, fmt.Sprintf("recur.original_day: %d", task.Recurrence.OriginalDay))
		}
	}

	if len(task.Attachments) > 0 {
		var attStrings []string
		for _, att := range task.Attachments {
			var attStr string
			if att.IsURL {
				attStr = "url:" + att.Filename
			} else if att.IsCodeRef {
				attStr = "code:" + att.Filename
			} else {
				attStr = att.Filename
			}
			if att.OriginalPath != "" {
				if att.IsCodeRef && att.LineNumber > 0 {
					attStr += fmt.Sprintf("||%s:%d", att.OriginalPath, att.LineNumber)
				} else {
					attStr += "||" + att.OriginalPath
				}
			}
			timestamp := fmt.Sprintf("@%d", att.AddedAt.Unix())
			attStrings = append(attStrings, attStr+timestamp)
		}
		parts = append(parts, fmt.Sprintf("attachments: %s", strings.Join(attStrings, ";")))
	}

	if len(task.Checklist) > 0 {
		var checklistStrings []string
		for _, item := range task.Checklist {
			marker := "[ ]"
			if item.Done {
				marker = "[x]"
			}
			checklistStrings = append(checklistStrings, escapeChecklistText(item.Text)+marker)
		}
		parts = append(parts, fmt.Sprintf("checklist: %s", strings.Join(checklistStrings, ";")))
	}

	if task.Stage != "" {
		parts = append(parts, fmt.Sprintf("stage: %s", task.Stage))
	}

	if task.Source != "" {
		parts = append(parts, fmt.Sprintf("source: %s", task.Source))
	}

	if task.URL != "" {
		parts = append(parts, fmt.Sprintf("url: %s", task.URL))
	}

	if task.BugAttrs != nil && !task.BugAttrs.IsEmpty() {
		if task.BugAttrs.Reproducible != "" {
			parts = append(parts, fmt.Sprintf("bug.reproducible: %s", task.BugAttrs.Reproducible))
		}
		if task.BugAttrs.Severity != "" {
			parts = append(parts, fmt.Sprintf("bug.severity: %s", task.BugAttrs.Severity))
		}
		if task.BugAttrs.Workaround != "" {
			parts = append(parts, fmt.Sprintf("bug.workaround: %s", escapeNewlines(task.BugAttrs.Workaround)))
		}
	}

	if task.IdeaAttrs != nil && !task.IdeaAttrs.IsEmpty() {
		if task.IdeaAttrs.Category != "" {
			parts = append(parts, fmt.Sprintf("idea.category: %s", task.IdeaAttrs.Category))
		}
		if task.IdeaAttrs.Effort != "" {
			parts = append(parts, fmt.Sprintf("idea.effort: %s", task.IdeaAttrs.Effort))
		}
		if task.IdeaAttrs.Value != "" {
			parts = append(parts, fmt.Sprintf("idea.value: %s", task.IdeaAttrs.Value))
		}
		if task.IdeaAttrs.Validated {
			parts = append(parts, "idea.validated: true")
		}
	}

	if task.TaskAttrs != nil && !task.TaskAttrs.IsEmpty() {
		if task.TaskAttrs.Effort != "" {
			parts = append(parts, fmt.Sprintf("task.effort: %s", task.TaskAttrs.Effort))
		}
		if task.TaskAttrs.WorkType != "" {
			parts = append(parts, fmt.Sprintf("task.worktype: %s", task.TaskAttrs.WorkType))
		}
	}

	if task.StoryAttrs != nil && !task.StoryAttrs.IsEmpty() {
		if task.StoryAttrs.StoryPoints > 0 {
			parts = append(parts, fmt.Sprintf("story.points: %d", task.StoryAttrs.StoryPoints))
		}
		if task.StoryAttrs.Persona != "" {
			parts = append(parts, fmt.Sprintf("story.persona: %s", escapeNewlines(task.StoryAttrs.Persona)))
		}
	}

	if task.EpicAttrs != nil && !task.EpicAttrs.IsEmpty() {
		if task.EpicAttrs.Scope != "" {
			parts = append(parts, fmt.Sprintf("epic.scope: %s", task.EpicAttrs.Scope))
		}
		if task.EpicAttrs.Milestone != "" {
			parts = append(parts, fmt.Sprintf("epic.milestone: %s", task.EpicAttrs.Milestone))
		}
		if task.EpicAttrs.SuccessMetrics != "" {
			parts = append(parts, fmt.Sprintf("epic.metrics: %s", escapeNewlines(task.EpicAttrs.SuccessMetrics)))
		}
		if task.EpicAttrs.InitialChildCount > 0 {
			parts = append(parts, fmt.Sprintf("epic.initial_children: %d", task.EpicAttrs.InitialChildCount))
		}
		if task.EpicAttrs.ScopeBaselinedAt != nil {
			parts = append(parts, fmt.Sprintf("epic.scope_baselined: %s", task.EpicAttrs.ScopeBaselinedAt.Format(util.DateFormat)))
		}
	}

	if len(task.ScoreDimensions) > 0 {
		keys := sortedKeys(task.ScoreDimensions)
		for _, k := range keys {
			if k == "" {
				continue
			}
			parts = append(parts, fmt.Sprintf("score.%s: %d", k, task.ScoreDimensions[k]))
		}
	}

	return fmt.Sprintf("<!-- %s -->", strings.Join(parts, ", "))
}

// FormatRecurrence converts a RecurrencePattern to its string representation for storage.
func FormatRecurrence(r *models.RecurrencePattern) string {
	if r == nil {
		return ""
	}

	var unit string
	switch r.Unit {
	case models.RecurrenceDaily:
		unit = "d"
	case models.RecurrenceWeekly:
		unit = "w"
	case models.RecurrenceMonthly:
		unit = "m"
	case models.RecurrenceYearly:
		unit = "y"
	default:
		unit = "d"
	}

	base := fmt.Sprintf("%d%s", r.Interval, unit)

	if r.Unit == models.RecurrenceWeekly && len(r.Weekdays) > 0 {
		dayNames := []string{"sun", "mon", "tue", "wed", "thu", "fri", "sat"}
		var days []string
		for _, d := range r.Weekdays {
			if d >= 0 && d < 7 {
				days = append(days, dayNames[d])
			}
		}
		if len(days) > 0 {
			base = base + ":" + strings.Join(days, ",")
		}
	}

	if r.Mode == models.RecurModeFromCompletion {
		return base + "~"
	}
	return base
}
