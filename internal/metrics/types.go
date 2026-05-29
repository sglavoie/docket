package metrics

import (
	"time"

	"github.com/sglavoie/tt/internal/models"
)

// TimeRange represents a predefined time window for metric computation.
type TimeRange int

const (
	TimeRange7d  TimeRange = iota // Last 7 days
	TimeRange30d                  // Last 30 days
	TimeRange90d                  // Last 90 days
	TimeRangeAll                  // All time
)

// AllTimeRanges returns all valid time ranges in order.
func AllTimeRanges() []TimeRange {
	return []TimeRange{TimeRange7d, TimeRange30d, TimeRange90d, TimeRangeAll}
}

// Label returns a human-readable label for the time range.
func (tr TimeRange) Label() string {
	switch tr {
	case TimeRange7d:
		return "7 days"
	case TimeRange30d:
		return "30 days"
	case TimeRange90d:
		return "90 days"
	case TimeRangeAll:
		return "All time"
	default:
		return "Unknown"
	}
}

// Days returns the number of days in the time range, or 0 for All.
func (tr TimeRange) Days() int {
	switch tr {
	case TimeRange7d:
		return 7
	case TimeRange30d:
		return 30
	case TimeRange90d:
		return 90
	case TimeRangeAll:
		return 0
	default:
		return 0
	}
}

// Cutoff returns the start time for this range relative to now.
// For TimeRangeAll, returns the zero time.
func (tr TimeRange) Cutoff(now time.Time) time.Time {
	if tr == TimeRangeAll {
		return time.Time{}
	}
	return now.AddDate(0, 0, -tr.Days())
}

// Next cycles to the next time range.
func (tr TimeRange) Next() TimeRange {
	ranges := AllTimeRanges()
	for i, r := range ranges {
		if r == tr {
			return ranges[(i+1)%len(ranges)]
		}
	}
	return TimeRange7d
}

// Prev cycles to the previous time range.
func (tr TimeRange) Prev() TimeRange {
	ranges := AllTimeRanges()
	for i, r := range ranges {
		if r == tr {
			return ranges[(i-1+len(ranges))%len(ranges)]
		}
	}
	return TimeRangeAll
}

// CompletionResult holds completion rate metrics.
type CompletionResult struct {
	Total     int       // Total resolved tasks (done + canceled) in range
	Completed int       // Tasks with StatusDone in range
	Rate      float64   // Completion rate (0-1), done/(done+canceled)
	Trend     []float64 // Rate per time bucket
	Labels    []string  // Bucket labels
}

// CycleTimeResult holds cycle time metrics.
type CycleTimeResult struct {
	Average time.Duration // Mean cycle time
	Median  time.Duration // Median cycle time
	P90     time.Duration // 90th percentile
	Count   int           // Number of tasks measured
	Trend   []float64     // Average hours per bucket
	Labels  []string      // Bucket labels
}

// ThroughputResult holds throughput metrics.
type ThroughputResult struct {
	Total     int       // Total tasks completed (done only) in range
	PerBucket []float64 // Completed tasks per time bucket
	Labels    []string  // Bucket labels
	AvgPerDay float64   // Average tasks completed per day
}

// BlockedEntry holds blocked duration info for a single task.
type BlockedEntry struct {
	Task           *models.Task
	Project        *models.Project
	TotalBlocked   time.Duration
	BlockedPeriods int
}

// BlockedResult holds blocked duration metrics.
type BlockedResult struct {
	Entries        []BlockedEntry // Sorted by TotalBlocked desc
	AverageBlocked time.Duration
	TotalBlocked   time.Duration
	TaskCount      int // Number of tasks that were blocked
}

// ProjectMetrics holds metrics for a single project.
type ProjectMetrics struct {
	Project    *models.Project
	Completion CompletionResult
	CycleTime  CycleTimeResult
	Throughput ThroughputResult
	Blocked    BlockedResult
}

// AggregateMetrics holds combined metrics across one or more projects.
type AggregateMetrics struct {
	Completion CompletionResult
	CycleTime  CycleTimeResult
	Throughput ThroughputResult
	Blocked    BlockedResult
	TimeRange  TimeRange
	IsGlobal   bool // true if aggregated across all projects
}

// ComputeAllAt computes all metrics using the given reference time (for testing).
// Optional extraTasks slices (e.g. archived tasks) are appended to the task pool
// for completion, cycle time, and throughput metrics. They are excluded from
// blocked metrics since archived tasks without project association aren't actionable.
//
// Weekly buckets use DefaultWeekStart (Monday). Use ComputeAllAtWithWeekStart
// to honor a user-configured week-start day.
func ComputeAllAt(projects []*models.Project, tr TimeRange, now time.Time, extraTasks ...[]*models.Task) *AggregateMetrics {
	return ComputeAllAtWithWeekStart(projects, tr, now, DefaultWeekStart, extraTasks...)
}

// ComputeAllAtWithWeekStart is identical to ComputeAllAt but lets callers
// pass an explicit week-start weekday (e.g. derived from a user's
// WeekEndDay setting via WeekStartFromWeekEnd) for weekly bucketing.
func ComputeAllAtWithWeekStart(projects []*models.Project, tr TimeRange, now time.Time, weekStart time.Weekday, extraTasks ...[]*models.Task) *AggregateMetrics {
	// projectTasks are the live project tasks (used for blocked metrics).
	var projectTasks []*models.Task
	taskProject := make(map[*models.Task]*models.Project)
	for _, p := range projects {
		if p == nil {
			continue
		}
		for _, t := range p.Tasks {
			if t == nil {
				continue
			}
			taskProject[t] = p
			projectTasks = append(projectTasks, t)
		}
	}

	// allTasks includes extra (e.g. archived) tasks for completion, cycle time,
	// and throughput metrics. Extra tasks are excluded from blocked metrics since
	// archived tasks without project association aren't actionable.
	allTasks := make([]*models.Task, 0, len(projectTasks))
	allTasks = append(allTasks, projectTasks...)
	for _, extra := range extraTasks {
		for _, t := range extra {
			if t == nil {
				continue
			}
			allTasks = append(allTasks, t)
		}
	}

	cutoff := tr.Cutoff(now)
	buckets := timeBucketsWithWeekStart(cutoff, now, bucketGranularity(tr), weekStart)

	return &AggregateMetrics{
		Completion: computeCompletion(allTasks, cutoff, now, buckets),
		CycleTime:  computeCycleTime(allTasks, cutoff, now, buckets),
		Throughput: computeThroughput(allTasks, cutoff, now, buckets),
		Blocked:    computeBlockedInRange(projectTasks, cutoff, now, taskProject),
		TimeRange:  tr,
		IsGlobal:   len(projects) > 1,
	}
}
