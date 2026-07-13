package planner

import (
	"time"

	"github.com/sglavoie/tt/internal/metrics"
	"github.com/sglavoie/tt/internal/models"
)

// Confidence represents how reliable a forecast is, based on the number of
// completed tasks the velocity estimate is derived from.
type Confidence int

const (
	ConfidenceNone   Confidence = iota // < 3 completed children — no basis for a forecast
	ConfidenceLow                      // 3–9 completed children
	ConfidenceMedium                   // 10–29 completed children
	ConfidenceHigh                     // 30+ completed children
)

// String returns a human-readable label for the confidence level.
func (c Confidence) String() string {
	switch c {
	case ConfidenceNone:
		return "none"
	case ConfidenceLow:
		return "low"
	case ConfidenceMedium:
		return "medium"
	case ConfidenceHigh:
		return "high"
	default:
		return "unknown"
	}
}

// ForecastResult holds the output of ForecastCompletion.
type ForecastResult struct {
	RemainingCount        int        // Number of incomplete direct children
	RemainingWeightedDays float64    // Total effort-weighted days remaining
	Velocity              float64    // Velocity used for the forecast (weighted tasks/day)
	ForecastDate          time.Time  // Projected completion date (zero if velocity == 0)
	Confidence            Confidence // Reliability indicator based on completed child count
}

// Effort multipliers (days per unit).
const (
	effortDaysSmall   = 0.5
	effortDaysMedium  = 1.0
	effortDaysLarge   = 3.0
	effortDaysUnknown = 1.0
)

// childEffortDays returns the effort-weighted day estimate for a task.
//
// Priority: TaskAttrs.Effort > IdeaAttrs.Effort > StoryAttrs.StoryPoints >
// Task.EstimatedDays > 1.0. Returns (days, true) when explicit effort data is
// present; (days, false) when falling back to EstimatedDays or the default —
// the boolean signals whether the task contributed effort data for velocity
// weighting purposes. Returns (effortDaysUnknown, false) when t is nil so
// callers can pass loose slices without pre-filtering.
func childEffortDays(t *models.Task) (days float64, hasEffortData bool) {
	if t == nil {
		return effortDaysUnknown, false
	}
	if t.TaskAttrs != nil && t.TaskAttrs.Effort != "" {
		switch t.TaskAttrs.Effort {
		case models.EffortSmall:
			return effortDaysSmall, true
		case models.EffortMedium:
			return effortDaysMedium, true
		case models.EffortLarge:
			return effortDaysLarge, true
		case models.EffortUnknown:
			return effortDaysUnknown, true
		}
	}
	if t.IdeaAttrs != nil && t.IdeaAttrs.Effort != "" {
		switch t.IdeaAttrs.Effort {
		case models.EffortSmall:
			return effortDaysSmall, true
		case models.EffortMedium:
			return effortDaysMedium, true
		case models.EffortLarge:
			return effortDaysLarge, true
		case models.EffortUnknown:
			return effortDaysUnknown, true
		}
	}
	if t.StoryAttrs != nil && t.StoryAttrs.StoryPoints > 0 {
		return float64(t.StoryAttrs.StoryPoints) * 0.5, true
	}
	// No explicit effort data — use EstimatedDays or default.
	if t.EstimatedDays > 0 {
		return t.EstimatedDays, false
	}
	return effortDaysUnknown, false
}

func summaryEffortDays(summary metrics.TaskSummary) (days float64, hasEffortData bool) {
	if summary.Effort != "" {
		switch summary.Effort {
		case models.EffortSmall:
			return effortDaysSmall, true
		case models.EffortMedium:
			return effortDaysMedium, true
		case models.EffortLarge:
			return effortDaysLarge, true
		case models.EffortUnknown:
			return effortDaysUnknown, true
		}
	}
	if summary.StoryPoints > 0 {
		return float64(summary.StoryPoints) * 0.5, true
	}
	if summary.EstimatedDays > 0 {
		return summary.EstimatedDays, false
	}
	return effortDaysUnknown, false
}

// confidenceForCount maps a completed-child count to a Confidence level.
func confidenceForCount(completed int) Confidence {
	switch {
	case completed < 3:
		return ConfidenceNone
	case completed < 10:
		return ConfidenceLow
	case completed < 30:
		return ConfidenceMedium
	default:
		return ConfidenceHigh
	}
}

// ForecastCompletion estimates when parent's direct children will all be done.
//
// velocity is effort-weighted tasks per day (use WeightedVelocity to obtain
// it).  now is the reference time for the forecast date.
//
// When velocity is zero the ForecastDate is the zero time (no forecast
// possible), but RemainingWeightedDays and Confidence are still populated.
func ForecastCompletion(parent *models.Task, velocity float64, now time.Time) *ForecastResult {
	if parent == nil {
		return &ForecastResult{Velocity: velocity}
	}
	var remainingDays float64
	var remainingCount int
	var completedCount int

	for _, child := range parent.Children {
		if child == nil {
			continue
		}
		if child.IsComplete() {
			completedCount++
			continue
		}
		remainingCount++
		days, _ := childEffortDays(child)
		remainingDays += days
	}

	result := &ForecastResult{
		RemainingCount:        remainingCount,
		RemainingWeightedDays: remainingDays,
		Velocity:              velocity,
		Confidence:            confidenceForCount(completedCount),
	}

	if velocity > 0 && remainingDays > 0 {
		daysToComplete := remainingDays / velocity
		// Cap absurd values to prevent time.Duration overflow (~106k days = ~292 years).
		const maxForecastDays = 100000
		if daysToComplete > maxForecastDays {
			daysToComplete = maxForecastDays
		}
		result.ForecastDate = now.Add(daysToduration(daysToComplete))
	}

	return result
}

// WeightedVelocity computes an effort-weighted velocity (effort-days per day)
// from historical throughput data and a slice of tasks.
//
// Algorithm:
//  1. Collect completed tasks from tasks that have effort data.
//  2. If any effort-annotated completed tasks exist, compute:
//     weighted_velocity = sum(effort_days for completed tasks with effort) /
//     (throughput.Total / throughput.AvgPerDay)
//  3. Fall back to throughput.AvgPerDay when no effort data exists or
//     throughput has zero tasks.
//
// Returns 0 if throughput.AvgPerDay is 0 and no effort data is available.
// Nil entries in tasks are skipped.
func WeightedVelocity(throughput metrics.ThroughputResult, tasks []*models.Task) float64 {
	return WeightedVelocityWithSummaries(throughput, metrics.TaskSummariesFromTasks(tasks))
}

// WeightedVelocityWithSummaries computes an effort-weighted velocity from
// historical throughput data and compact task summaries. It lets archive-aware
// callers include derived archive index summaries without constructing full
// archived task bodies.
func WeightedVelocityWithSummaries(throughput metrics.ThroughputResult, summaryGroups ...[]metrics.TaskSummary) float64 {
	if throughput.AvgPerDay <= 0 {
		return 0
	}

	// Total calendar days covered by the throughput window.
	totalDays := float64(throughput.Total) / throughput.AvgPerDay

	var sumEffortDays float64
	var effortTaskCount int
	for _, summaries := range summaryGroups {
		for _, summary := range summaries {
			if !summary.WorkflowState.IsTerminal() {
				continue
			}
			days, hasEffort := summaryEffortDays(summary)
			if !hasEffort {
				continue
			}
			sumEffortDays += days
			effortTaskCount++
		}
	}

	if effortTaskCount == 0 || totalDays <= 0 {
		return throughput.AvgPerDay
	}

	// Scale: only the effort-annotated fraction of completed tasks was measured.
	// Compute the fraction of total completed tasks that had effort annotations
	// and extrapolate the weighted rate.
	effortFraction := float64(effortTaskCount) / float64(throughput.Total)
	if effortFraction <= 0 {
		return throughput.AvgPerDay
	}

	// effort-weighted velocity = effort_days_completed / days_in_window
	// But we only have effort data for effortTaskCount tasks, so we extrapolate
	// assuming the same average effort for the remaining tasks.
	avgEffortPerTask := sumEffortDays / float64(effortTaskCount)
	// Estimate total effort days completed = throughput.Total * avgEffortPerTask.
	totalEffortDays := float64(throughput.Total) * avgEffortPerTask
	return totalEffortDays / totalDays
}
