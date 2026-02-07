package metrics

import (
	"sort"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

// computeBlocked calculates blocked duration metrics for all tasks.
// It scans activity logs for status transitions to/from "blocked" and
// computes total blocked time per task.
func computeBlocked(tasks []*models.Task, now time.Time, taskProject map[*models.Task]*models.Project) BlockedResult {
	var entries []BlockedEntry

	for _, t := range tasks {
		entry := computeBlockedEntry(t, now)
		if entry.TotalBlocked > 0 {
			entry.Project = taskProject[t]
			entries = append(entries, entry)
		}
	}

	// Sort by total blocked time descending
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].TotalBlocked > entries[j].TotalBlocked
	})

	result := BlockedResult{
		Entries:   entries,
		TaskCount: len(entries),
	}

	if len(entries) > 0 {
		var totalBlocked time.Duration
		for _, e := range entries {
			totalBlocked += e.TotalBlocked
		}
		result.TotalBlocked = totalBlocked
		result.AverageBlocked = totalBlocked / time.Duration(len(entries))
	}

	return result
}

// computeBlockedEntry scans a single task's activity for blocked periods.
// A period starts when NewValue="blocked" and ends when OldValue="blocked".
// If the task is currently blocked with an open period, it extends to now.
// If a "blocked" start has no end and the task is NOT currently blocked,
// the end transition was likely pruned — skip that partial period.
func computeBlockedEntry(t *models.Task, now time.Time) BlockedEntry {
	entry := BlockedEntry{Task: t}

	var blockedStart time.Time
	inBlockedPeriod := false

	for _, a := range t.Activity {
		if a.Field != "status" {
			continue
		}

		if a.NewValue == string(models.StatusBlocked) && !inBlockedPeriod {
			blockedStart = a.Timestamp
			inBlockedPeriod = true
		} else if a.OldValue == string(models.StatusBlocked) && inBlockedPeriod {
			duration := a.Timestamp.Sub(blockedStart)
			if duration > 0 {
				entry.TotalBlocked += duration
				entry.BlockedPeriods++
			}
			inBlockedPeriod = false
		}
	}

	// If still in a blocked period and task is currently blocked, extend to now
	if inBlockedPeriod && t.Status == models.StatusBlocked {
		duration := now.Sub(blockedStart)
		if duration > 0 {
			entry.TotalBlocked += duration
			entry.BlockedPeriods++
		}
	}
	// If inBlockedPeriod but task is NOT blocked, the end transition was pruned — skip

	return entry
}
