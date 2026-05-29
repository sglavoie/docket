package metrics

import (
	"sort"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

// computeBlocked calculates blocked duration metrics for all tasks over all time.
func computeBlocked(tasks []*models.Task, now time.Time, taskProject map[*models.Task]*models.Project) BlockedResult {
	return computeBlockedInRange(tasks, time.Time{}, now, taskProject)
}

// computeBlockedInRange calculates blocked duration metrics for all tasks within
// the [cutoff, now] window. A zero cutoff keeps the all-time behavior.
func computeBlockedInRange(tasks []*models.Task, cutoff, now time.Time, taskProject map[*models.Task]*models.Project) BlockedResult {
	var entries []BlockedEntry

	for _, t := range tasks {
		entry := computeBlockedEntryInRange(t, cutoff, now)
		if entry.TotalBlocked > 0 {
			entry.Project = taskProject[t]
			entries = append(entries, entry)
		}
	}

	// Sort by total blocked time descending, breaking ties on Task.ID so that
	// repeated invocations on the same data produce a deterministic ordering
	// (the entries are presented in a UI list where rows must not shuffle).
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].TotalBlocked != entries[j].TotalBlocked {
			return entries[i].TotalBlocked > entries[j].TotalBlocked
		}
		return entries[i].Task.ID < entries[j].Task.ID
	})

	result := BlockedResult{
		Entries:   entries,
		TaskCount: len(entries),
	}

	if len(entries) > 0 {
		// Accumulate in float64 to avoid int64 nanosecond overflow on very
		// long-blocked task histories (time.Duration tops out around 292y).
		// Also compute the average via an incremental mean so the average
		// stays well-defined even when the sum saturates.
		var totalNs float64
		var meanNs float64
		for i, e := range entries {
			totalNs += float64(e.TotalBlocked)
			meanNs += (float64(e.TotalBlocked) - meanNs) / float64(i+1)
		}
		result.TotalBlocked = durationFromFloat(totalNs)
		result.AverageBlocked = durationFromFloat(meanNs)
	}

	return result
}

// computeBlockedEntryInRange scans a single task's activity for blocked periods
// within the [cutoff, now] window. A period starts when NewValue="blocked" and
// ends when OldValue="blocked". If the task is currently blocked with an open
// period, it extends to now. If a "blocked" start has no end and the task is
// NOT currently blocked, the end transition was likely pruned — skip that
// partial period.
func computeBlockedEntryInRange(t *models.Task, cutoff, now time.Time) BlockedEntry {
	entry := BlockedEntry{Task: t}

	activity := make([]*models.ActivityEntry, 0, len(t.Activity))
	for _, a := range t.Activity {
		if a.Field == "status" {
			activity = append(activity, a)
		}
	}
	sort.SliceStable(activity, func(i, j int) bool {
		return activity[i].Timestamp.Before(activity[j].Timestamp)
	})

	var blockedStart time.Time
	inBlockedPeriod := false

	for _, a := range activity {
		if a.NewValue == string(models.StatusBlocked) && !inBlockedPeriod {
			blockedStart = a.Timestamp
			inBlockedPeriod = true
		} else if a.OldValue == string(models.StatusBlocked) && inBlockedPeriod {
			duration := blockedDurationInRange(blockedStart, a.Timestamp, cutoff, now)
			if duration > 0 {
				entry.TotalBlocked = addDurationSaturating(entry.TotalBlocked, duration)
				entry.BlockedPeriods++
			}
			inBlockedPeriod = false
		}
	}

	// If still in a blocked period and task is currently blocked, extend to now
	if inBlockedPeriod && t.Status == models.StatusBlocked {
		duration := blockedDurationInRange(blockedStart, now, cutoff, now)
		if duration > 0 {
			entry.TotalBlocked = addDurationSaturating(entry.TotalBlocked, duration)
			entry.BlockedPeriods++
		}
	}
	// If inBlockedPeriod but task is NOT blocked, the end transition was pruned — skip

	return entry
}

func blockedDurationInRange(start, end, cutoff, now time.Time) time.Duration {
	if end.After(now) {
		end = now
	}
	if !cutoff.IsZero() && start.Before(cutoff) {
		start = cutoff
	}
	if !end.After(start) {
		return 0
	}
	return end.Sub(start)
}

func addDurationSaturating(total, next time.Duration) time.Duration {
	const maxDuration = time.Duration(1<<63 - 1)
	if next > maxDuration-total {
		return maxDuration
	}
	return total + next
}
