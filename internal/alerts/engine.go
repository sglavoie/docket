package alerts

import (
	"fmt"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

// Evaluate runs all enabled checkers against the provided project data and returns an AlertSnapshot.
func Evaluate(cfg AlertConfig, projects []ProjectData) *AlertSnapshot {
	now := time.Now()
	var groups []AlertGroup
	totalCount := 0

	type checker struct {
		enabled  bool
		category AlertCategory
		label    string
		fn       func(AlertConfig, []ProjectData, time.Time) []Alert
	}

	checkers := []checker{
		{cfg.DailyReviewEnabled, CategoryDailyReview, "Daily Review", checkDailyReviewReminder},
		{cfg.WeeklyReviewEnabled, CategoryWeeklyReview, "Weekly Review", checkWeeklyReviewReminder},
		{cfg.OverdueEnabled, CategoryOverdue, "Overdue", checkOverdueTasks},
		{cfg.DueSoonEnabled, CategoryDueSoon, "Due Soon", checkDueSoonTasks},
		{cfg.StaleEnabled, CategoryStale, "Stale", checkStaleTasks},
		{cfg.InboxEnabled, CategoryInbox, "Inbox", checkInboxBacklog},
		{cfg.BlockedEnabled, CategoryBlocked, "Blocked", checkBlockedTasks},
		{cfg.ProjectHealthEnabled, CategoryProjectHealth, "Project Health", checkProjectHealth},
	}

	for _, c := range checkers {
		if !c.enabled {
			continue
		}
		alerts := c.fn(cfg, projects, now)
		if len(alerts) == 0 {
			continue
		}
		label := c.label
		if len(alerts) > 1 {
			label = fmt.Sprintf("%s (%d)", c.label, len(alerts))
		}
		groups = append(groups, AlertGroup{
			Category: c.category,
			Label:    label,
			Alerts:   alerts,
		})
		totalCount += len(alerts)
	}

	return &AlertSnapshot{
		Groups:      groups,
		TotalCount:  totalCount,
		EvaluatedAt: now,
	}
}

func checkDailyReviewReminder(cfg AlertConfig, _ []ProjectData, now time.Time) []Alert {
	if cfg.LastDailyReviewAt == nil {
		return []Alert{{
			Category: CategoryDailyReview,
			Title:    "Daily review never completed",
			Details:  "Open the daily review with ;d and press R when done",
			Severity: SeverityWarning,
		}}
	}

	last := *cfg.LastDailyReviewAt
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	lastDay := time.Date(last.Year(), last.Month(), last.Day(), 0, 0, 0, 0, last.Location())

	// Already reviewed today
	if lastDay.Equal(today) {
		return nil
	}

	// Current hour >= configured hour and no review today
	if now.Hour() >= cfg.DailyReviewHour {
		return []Alert{{
			Category: CategoryDailyReview,
			Title:    "Daily review pending",
			Details:  fmt.Sprintf("Last completed %s", formatTimeAgo(now, last)),
			Severity: SeverityInfo,
		}}
	}

	// Fallback: more than 26 hours since last review
	if now.Sub(last) > 26*time.Hour {
		return []Alert{{
			Category: CategoryDailyReview,
			Title:    "Daily review overdue",
			Details:  fmt.Sprintf("Last completed %s", formatTimeAgo(now, last)),
			Severity: SeverityWarning,
		}}
	}

	return nil
}

func checkWeeklyReviewReminder(cfg AlertConfig, _ []ProjectData, now time.Time) []Alert {
	if cfg.LastWeeklyReviewAt == nil {
		return []Alert{{
			Category: CategoryWeeklyReview,
			Title:    "Weekly review never completed",
			Details:  "Open the weekly review with ;w",
			Severity: SeverityWarning,
		}}
	}

	last := *cfg.LastWeeklyReviewAt

	// More than 8 days since last review (fallback)
	if now.Sub(last) > 8*24*time.Hour {
		return []Alert{{
			Category: CategoryWeeklyReview,
			Title:    "Weekly review overdue",
			Details:  fmt.Sprintf("Last completed %s", formatTimeAgo(now, last)),
			Severity: SeverityWarning,
		}}
	}

	// Current weekday matches configured day and no review this week
	if int(now.Weekday()) == cfg.WeeklyReviewDay {
		lastDay := time.Date(last.Year(), last.Month(), last.Day(), 0, 0, 0, 0, last.Location())
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		if lastDay.Before(today) {
			return []Alert{{
				Category: CategoryWeeklyReview,
				Title:    "Weekly review day",
				Details:  fmt.Sprintf("Last completed %s", formatTimeAgo(now, last)),
				Severity: SeverityInfo,
			}}
		}
	}

	return nil
}

func checkOverdueTasks(cfg AlertConfig, projects []ProjectData, now time.Time) []Alert {
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	var alerts []Alert

	for _, pd := range projects {
		for _, task := range pd.Tasks {
			if !task.Status.IsActive() || task.DueDate == nil || task.IsSnoozedAt(now) {
				continue
			}
			dueDay := time.Date(task.DueDate.Year(), task.DueDate.Month(), task.DueDate.Day(), 0, 0, 0, 0, task.DueDate.Location())
			if dueDay.Before(today) {
				severity := SeverityWarning
				if now.Sub(dueDay) > 7*24*time.Hour {
					severity = SeverityUrgent
				}
				alerts = append(alerts, Alert{
					Category:    CategoryOverdue,
					Title:       task.Title,
					Details:     fmt.Sprintf("Due %s", dueDay.Format("Jan 2")),
					TaskID:      task.ID,
					ProjectID:   pd.Project.ID,
					ProjectName: pd.Project.Name,
					Severity:    severity,
				})
			}
		}
	}

	return alerts
}

func checkDueSoonTasks(cfg AlertConfig, projects []ProjectData, now time.Time) []Alert {
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	horizon := today.AddDate(0, 0, cfg.DueSoonDays)
	var alerts []Alert

	for _, pd := range projects {
		for _, task := range pd.Tasks {
			if !task.Status.IsActive() || task.DueDate == nil || task.IsSnoozedAt(now) {
				continue
			}
			dueDay := time.Date(task.DueDate.Year(), task.DueDate.Month(), task.DueDate.Day(), 0, 0, 0, 0, task.DueDate.Location())
			// Due today or in the future but within the horizon (not overdue)
			if !dueDay.Before(today) && !dueDay.After(horizon) {
				alerts = append(alerts, Alert{
					Category:    CategoryDueSoon,
					Title:       task.Title,
					Details:     fmt.Sprintf("Due %s", dueDay.Format("Jan 2")),
					TaskID:      task.ID,
					ProjectID:   pd.Project.ID,
					ProjectName: pd.Project.Name,
					Severity:    SeverityInfo,
				})
			}
		}
	}

	return alerts
}

func checkStaleTasks(cfg AlertConfig, projects []ProjectData, now time.Time) []Alert {
	staleThreshold := now.AddDate(0, 0, -cfg.StaleDays)
	var alerts []Alert

	for _, pd := range projects {
		for _, task := range pd.Tasks {
			if !task.Status.IsActive() || task.IsSnoozedAt(now) || task.Section == models.SectionSomeday {
				continue
			}
			if task.UpdatedAt.Before(staleThreshold) {
				alerts = append(alerts, Alert{
					Category:    CategoryStale,
					Title:       task.Title,
					Details:     fmt.Sprintf("No updates for %d days", util.DaysBetween(task.UpdatedAt, now)),
					TaskID:      task.ID,
					ProjectID:   pd.Project.ID,
					ProjectName: pd.Project.Name,
					Severity:    SeverityInfo,
				})
			}
		}
	}

	return alerts
}

func checkInboxBacklog(cfg AlertConfig, projects []ProjectData, now time.Time) []Alert {
	var alerts []Alert

	for _, pd := range projects {
		inboxCount := 0
		for _, task := range pd.Tasks {
			if task.Status.IsActive() && task.Section == models.SectionInbox && !task.IsSnoozedAt(now) {
				inboxCount++
			}
		}
		if inboxCount >= cfg.InboxThreshold {
			alerts = append(alerts, Alert{
				Category:    CategoryInbox,
				Title:       fmt.Sprintf("%d inbox tasks", inboxCount),
				Details:     "Consider triaging inbox items",
				Count:       inboxCount,
				ProjectID:   pd.Project.ID,
				ProjectName: pd.Project.Name,
				Severity:    SeverityInfo,
			})
		}
	}

	return alerts
}

func checkBlockedTasks(cfg AlertConfig, projects []ProjectData, now time.Time) []Alert {
	var alerts []Alert

	for _, pd := range projects {
		for _, task := range pd.Tasks {
			if task.Status != models.StatusBlocked || task.IsSnoozedAt(now) {
				continue
			}
			// Use the activity log to find when the task last became blocked,
			// rather than UpdatedAt which resets on any metadata edit.
			blockedSince := findLastBlockedTransition(task)
			// Avoid emitting a spurious "Blocked for ~700000 days" alert when
			// the activity log is empty and UpdatedAt is zero (e.g. partially
			// constructed task via JSON missing the field).
			if blockedSince.IsZero() {
				continue
			}
			daysSinceBlocked := util.DaysBetween(blockedSince, now)
			if daysSinceBlocked >= cfg.BlockedDays {
				alerts = append(alerts, Alert{
					Category:    CategoryBlocked,
					Title:       task.Title,
					Details:     fmt.Sprintf("Blocked for %d days", daysSinceBlocked),
					TaskID:      task.ID,
					ProjectID:   pd.Project.ID,
					ProjectName: pd.Project.Name,
					Severity:    SeverityWarning,
				})
			}
		}
	}

	return alerts
}

// findLastBlockedTransition scans the task's activity log backwards to find
// the most recent transition to "blocked" status. Falls back to UpdatedAt
// if the activity log doesn't contain the transition (e.g., pruned).
func findLastBlockedTransition(task *models.Task) time.Time {
	// Scan backwards to find the most recent transition *to* blocked
	for i := len(task.Activity) - 1; i >= 0; i-- {
		entry := task.Activity[i]
		if entry.Field == "status" && entry.NewValue == string(models.StatusBlocked) {
			return entry.Timestamp
		}
	}
	// Fallback: activity log was pruned or task was created as blocked
	return task.UpdatedAt
}

func checkProjectHealth(cfg AlertConfig, projects []ProjectData, now time.Time) []Alert {
	staleThreshold := now.AddDate(0, 0, -cfg.ProjectStaleDays)
	var alerts []Alert

	for _, pd := range projects {
		if len(pd.Tasks) == 0 {
			continue
		}

		// Find most recent task update
		var latestUpdate time.Time
		for _, task := range pd.Tasks {
			if task.UpdatedAt.After(latestUpdate) {
				latestUpdate = task.UpdatedAt
			}
		}

		// If no task has a real UpdatedAt (all zero), skip rather than
		// emit a spurious "N days ago" alert with a ~10^5-day count.
		if latestUpdate.IsZero() {
			continue
		}

		if latestUpdate.Before(staleThreshold) {
			alerts = append(alerts, Alert{
				Category:    CategoryProjectHealth,
				Title:       "No activity",
				Details:     fmt.Sprintf("Last update %d days ago", util.DaysBetween(latestUpdate, now)),
				ProjectID:   pd.Project.ID,
				ProjectName: pd.Project.Name,
				Severity:    SeverityInfo,
			})
		}
	}

	return alerts
}

// formatTimeAgo returns a human-readable relative time string.
func formatTimeAgo(now, t time.Time) string {
	d := now.Sub(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		mins := int(d.Minutes())
		if mins == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", mins)
	case d < 24*time.Hour:
		hours := int(d.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	default:
		days := int(d.Hours() / 24)
		if days == 1 {
			return "yesterday"
		}
		return fmt.Sprintf("%d days ago", days)
	}
}
