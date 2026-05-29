// Package planner provides pure-computation planning algorithms: topological
// sorting, critical path analysis, forward scheduling, and forecasting.
// It has no TUI imports and is usable from both CLI and TUI layers.
package planner

import (
	"time"

	"github.com/sglavoie/tt/internal/models"
)

// CriticalPathResult holds the output of critical path analysis.
type CriticalPathResult struct {
	// Path is the critical path tasks in topological order.
	Path []*models.Task
	// Length is the total length of the critical path in days.
	Length float64
	// OnPath maps task ID -> true for tasks on the critical path.
	OnPath map[string]bool
	// Slack maps task ID -> slack in days (latestStart - earliestStart).
	// Zero slack = on the critical path.
	Slack map[string]float64
}

// ScheduledTask holds a task with its computed earliest start and end dates.
type ScheduledTask struct {
	Task          *models.Task
	EarliestStart time.Time
	EarliestEnd   time.Time
	Slack         float64
	IsCritical    bool
}

// CriticalPath computes the critical path through the dependency graph of the
// given tasks using the CPM (Critical Path Method) algorithm.
//
// Duration for each task is taken from EstimatedDays when set, otherwise 1.0.
// Complete tasks are excluded (see TopologicalSort).
//
// Returns an empty result (not nil) when there are no schedulable tasks.
func CriticalPath(tasks []*models.Task) *CriticalPathResult {
	empty := &CriticalPathResult{
		OnPath: make(map[string]bool),
		Slack:  make(map[string]float64),
	}

	topoOrder, err := TopologicalSort(tasks)
	if err != nil || len(topoOrder) == 0 {
		return empty
	}

	taskMap := make(map[string]*models.Task, len(topoOrder))
	for _, t := range topoOrder {
		taskMap[t.ID] = t
	}

	// Forward pass: ES[t] = max(EF[pred]) over all predecessors.
	// ES = earliest start (days from t=0), EF = ES + duration.
	es := make(map[string]float64, len(topoOrder)) // all initialized to 0
	for _, t := range topoOrder {
		for _, depRef := range t.DependsOn {
			predID, ok := dependencyTaskID(depRef, taskMap)
			if !ok {
				continue
			}
			pred := taskMap[predID]
			predEF := es[pred.ID] + EstimateDuration(pred)
			if predEF > es[t.ID] {
				es[t.ID] = predEF
			}
		}
	}

	// Project duration = max(EF[t]) across all tasks.
	var projectDuration float64
	for _, t := range topoOrder {
		if ef := es[t.ID] + EstimateDuration(t); ef > projectDuration {
			projectDuration = ef
		}
	}

	// Build successor map for backward pass.
	succs := make(map[string][]string, len(topoOrder))
	for _, t := range topoOrder {
		for _, depRef := range t.DependsOn {
			if predID, ok := dependencyTaskID(depRef, taskMap); ok {
				succs[predID] = append(succs[predID], t.ID)
			}
		}
	}

	// Backward pass: LS[t] = LF[t] - duration[t].
	// LF for leaf nodes = projectDuration.
	// LF[t] = min(LS[succ]) over all successors.
	ls := make(map[string]float64, len(topoOrder))
	for _, t := range topoOrder {
		ls[t.ID] = projectDuration - EstimateDuration(t) // default for leaves
	}
	for i := len(topoOrder) - 1; i >= 0; i-- {
		t := topoOrder[i]
		for _, succID := range succs[t.ID] {
			// Latest I can finish = latest my successor can start.
			lf := ls[succID]
			myLS := lf - EstimateDuration(t)
			if myLS < ls[t.ID] {
				ls[t.ID] = myLS
			}
		}
	}

	// Slack = LS - ES. Zero (within epsilon) = critical path.
	const eps = 1e-9
	slack := make(map[string]float64, len(topoOrder))
	onPath := make(map[string]bool, len(topoOrder))
	for _, t := range topoOrder {
		s := ls[t.ID] - es[t.ID]
		slack[t.ID] = s
		if s < eps {
			onPath[t.ID] = true
		}
	}

	// Collect critical path in topological order.
	path := make([]*models.Task, 0)
	for _, t := range topoOrder {
		if onPath[t.ID] {
			path = append(path, t)
		}
	}

	return &CriticalPathResult{
		Path:   path,
		Length: projectDuration,
		OnPath: onPath,
		Slack:  slack,
	}
}

// ForwardSchedule assigns earliest start and end calendar dates to each
// incomplete task, respecting dependency order and estimated durations.
//
// startDate is the project start (used as t=0 for tasks with no explicit
// StartDate and no predecessor constraints).
//
// velocity scales how fast work gets done: actual duration = estimatedDays /
// velocity.  A velocity of 1.0 means tasks run at their nominal estimated
// speed; higher values compress the schedule.  Velocity ≤ 0 is treated as 1.0.
//
// Tasks that have an explicit StartDate receive max(startDate, predecessorEnd)
// as their actual start.
func ForwardSchedule(tasks []*models.Task, startDate time.Time, velocity float64) []ScheduledTask {
	if velocity <= 0 {
		velocity = 1.0
	}

	topoOrder, err := TopologicalSort(tasks)
	if err != nil || len(topoOrder) == 0 {
		return nil
	}

	cp := CriticalPath(tasks)

	taskMap := make(map[string]*models.Task, len(topoOrder))
	for _, t := range topoOrder {
		taskMap[t.ID] = t
	}

	// esDay[id] = earliest start offset in days from startDate.
	esDay := make(map[string]float64, len(topoOrder))

	for _, t := range topoOrder {
		// Explicit StartDate provides a lower bound on start offset.
		if t.StartDate != nil && !t.StartDate.IsZero() {
			offset := t.StartDate.Sub(startDate).Hours() / 24
			if offset > esDay[t.ID] {
				esDay[t.ID] = offset
			}
		}
		// Dependency constraints: must start after all predecessors finish.
		for _, depRef := range t.DependsOn {
			predID, ok := dependencyTaskID(depRef, taskMap)
			if !ok {
				continue
			}
			pred := taskMap[predID]
			predEnd := esDay[pred.ID] + EstimateDuration(pred)/velocity
			if predEnd > esDay[t.ID] {
				esDay[t.ID] = predEnd
			}
		}
	}

	result := make([]ScheduledTask, 0, len(topoOrder))
	for _, t := range topoOrder {
		durDays := EstimateDuration(t) / velocity
		start := startDate.Add(daysToduration(esDay[t.ID]))
		end := start.Add(daysToduration(durDays))
		// Slack from CriticalPath is in unscaled days; convert to calendar days
		// so it matches the velocity-adjusted start/end timestamps.
		result = append(result, ScheduledTask{
			Task:          t,
			EarliestStart: start,
			EarliestEnd:   end,
			Slack:         cp.Slack[t.ID] / velocity,
			IsCritical:    cp.OnPath[t.ID],
		})
	}
	return result
}

// daysToduration converts a float64 day count to a time.Duration. Values that
// would overflow time.Duration (~106k days) are clamped to the int64 limit so
// scheduling math never returns garbage negative durations.
func daysToduration(days float64) time.Duration {
	const maxDays = float64(1<<62) / (24 * float64(time.Hour))
	if days > maxDays {
		return time.Duration(1 << 62)
	}
	if days < -maxDays {
		return time.Duration(-(1 << 62))
	}
	return time.Duration(days * 24 * float64(time.Hour))
}

// SerialSchedule assigns calendar dates like ForwardSchedule but adds a
// single-resource constraint: independent tasks are chained sequentially
// instead of all starting at t=0.  Tasks are ordered by priority (ascending
// number = higher priority) then ID for determinism.
//
// Slack is derived from the serial schedule itself (difference between the
// latest possible start and the assigned start within the total serial
// makespan), not from standard CPM which assumes unlimited parallelism.
func SerialSchedule(tasks []*models.Task, startDate time.Time, velocity float64) []ScheduledTask {
	if velocity <= 0 {
		velocity = 1.0
	}

	topoOrder, err := topologicalSortByPriority(tasks)
	if err != nil || len(topoOrder) == 0 {
		return nil
	}

	taskMap := make(map[string]*models.Task, len(topoOrder))
	for _, t := range topoOrder {
		taskMap[t.ID] = t
	}

	// Forward pass with single-resource constraint.
	esDay := make(map[string]float64, len(topoOrder))
	var resourceAvailable float64

	for _, t := range topoOrder {
		earliest := resourceAvailable

		// Explicit StartDate lower bound.
		if t.StartDate != nil && !t.StartDate.IsZero() {
			offset := t.StartDate.Sub(startDate).Hours() / 24
			if offset > earliest {
				earliest = offset
			}
		}

		// Dependency constraints.
		for _, depRef := range t.DependsOn {
			predID, ok := dependencyTaskID(depRef, taskMap)
			if !ok {
				continue
			}
			pred := taskMap[predID]
			predEnd := esDay[pred.ID] + EstimateDuration(pred)/velocity
			if predEnd > earliest {
				earliest = predEnd
			}
		}

		esDay[t.ID] = earliest
		resourceAvailable = earliest + EstimateDuration(t)/velocity
	}

	// Total makespan = resourceAvailable after all tasks scheduled.
	makespan := resourceAvailable

	// Backward pass on serial order to compute slack.
	// LS[t] = min(LS[succ] for all successors, nextTaskStart) - but in serial
	// order the "next task" is an implicit successor.
	ls := make(map[string]float64, len(topoOrder))
	for i := len(topoOrder) - 1; i >= 0; i-- {
		t := topoOrder[i]
		dur := EstimateDuration(t) / velocity
		ls[t.ID] = makespan - dur // default: can slide to the end
	}

	// Build successor map.
	succs := make(map[string][]string, len(topoOrder))
	for _, t := range topoOrder {
		for _, depRef := range t.DependsOn {
			if predID, ok := dependencyTaskID(depRef, taskMap); ok {
				succs[predID] = append(succs[predID], t.ID)
			}
		}
	}
	// Also add serial-order successor edges (each task blocks the next).
	for i := 0; i < len(topoOrder)-1; i++ {
		succs[topoOrder[i].ID] = append(succs[topoOrder[i].ID], topoOrder[i+1].ID)
	}

	for i := len(topoOrder) - 1; i >= 0; i-- {
		t := topoOrder[i]
		dur := EstimateDuration(t) / velocity
		for _, succID := range succs[t.ID] {
			lf := ls[succID]
			myLS := lf - dur
			if myLS < ls[t.ID] {
				ls[t.ID] = myLS
			}
		}
	}

	result := make([]ScheduledTask, 0, len(topoOrder))
	const eps = 1e-9
	for _, t := range topoOrder {
		dur := EstimateDuration(t) / velocity
		start := startDate.Add(daysToduration(esDay[t.ID]))
		end := start.Add(daysToduration(dur))
		slack := ls[t.ID] - esDay[t.ID]
		if slack < eps {
			slack = 0
		}
		result = append(result, ScheduledTask{
			Task:          t,
			EarliestStart: start,
			EarliestEnd:   end,
			Slack:         slack,
			IsCritical:    slack < eps,
		})
	}
	return result
}
