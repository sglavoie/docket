package alerts

import (
	"fmt"
	"time"
)

// FilterDismissed returns a new AlertSnapshot with dismissed alerts removed.
// The dismissedSet maps DismissalKey strings to true for alerts that should
// be hidden. Groups that become empty after filtering are removed entirely.
func FilterDismissed(snapshot *AlertSnapshot, dismissedSet map[string]bool) *AlertSnapshot {
	if snapshot == nil {
		return nil
	}
	if len(dismissedSet) == 0 {
		return snapshot
	}

	var groups []AlertGroup
	total := 0
	for _, g := range snapshot.Groups {
		var kept []Alert
		for _, a := range g.Alerts {
			if !dismissedSet[a.DismissalKey()] {
				kept = append(kept, a)
			}
		}
		if len(kept) == 0 {
			continue
		}
		label := groupLabel(g.Category, len(kept))
		groups = append(groups, AlertGroup{
			Category: g.Category,
			Label:    label,
			Alerts:   kept,
		})
		total += len(kept)
	}

	return &AlertSnapshot{
		Groups:      groups,
		TotalCount:  total,
		EvaluatedAt: snapshot.EvaluatedAt,
	}
}

// groupLabel builds a display label for a group, adding a count suffix when
// there is more than one alert.
func groupLabel(category AlertCategory, count int) string {
	base := CategoryDisplayName(category)
	if count <= 1 {
		return base
	}
	return fmt.Sprintf("%s (%d)", base, count)
}

// CategoryDisplayName returns the human-readable label for a category.
// These match the labels used by the Evaluate engine.
func CategoryDisplayName(c AlertCategory) string {
	switch c {
	case CategoryDailyReview:
		return "Daily Review"
	case CategoryWeeklyReview:
		return "Weekly Review"
	case CategoryOverdue:
		return "Overdue"
	case CategoryDueSoon:
		return "Due Soon"
	case CategoryStale:
		return "Stale"
	case CategoryInbox:
		return "Inbox"
	case CategoryBlocked:
		return "Blocked"
	case CategoryProjectHealth:
		return "Project Health"
	default:
		return string(c)
	}
}

// ShouldExpireDismissal returns true if a dismissal should be considered expired
// based on the alert category and the time it was dismissed.
//
// Expiration policies:
//   - daily_review: expires at the next midnight after dismissedAt
//   - weekly_review: expires 7 days after dismissedAt
//   - inbox: expires 24 hours after dismissedAt
//   - overdue, due_soon, stale, blocked, project_health: never time-expires
//     (cleaned up by GC when the underlying condition resolves)
func ShouldExpireDismissal(category string, dismissedAt time.Time, now time.Time) bool {
	switch AlertCategory(category) {
	case CategoryDailyReview:
		nextMidnight := time.Date(
			dismissedAt.Year(), dismissedAt.Month(), dismissedAt.Day()+1,
			0, 0, 0, 0, dismissedAt.Location(),
		)
		return !now.Before(nextMidnight)
	case CategoryWeeklyReview:
		return now.Sub(dismissedAt) >= 7*24*time.Hour
	case CategoryInbox:
		return now.Sub(dismissedAt) >= 24*time.Hour
	default:
		return false
	}
}
