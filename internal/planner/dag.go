// Package planner provides pure-computation planning algorithms: topological
// sorting, critical path analysis, forward scheduling, and forecasting.
// It has no TUI imports and is usable from both CLI and TUI layers.
package planner

import (
	"fmt"
	"sort"
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

// TopologicalSort returns incomplete tasks from the input slice in topological
// order using Kahn's algorithm on DependsOn edges.
//
// Complete tasks in the input are excluded from the result but are considered
// satisfied predecessors (a dependency on a complete task does not block the
// dependent).  Cross-project dependency refs (format "projectID:taskID") whose
// project-side task is not present in the input slice are treated as satisfied.
// Nil entries and tasks with empty IDs are silently skipped so callers can pass
// loosely-assembled cross-project slices without manually filtering them.
//
// Returns an error if a dependency cycle exists among the incomplete tasks.
func TopologicalSort(tasks []*models.Task) ([]*models.Task, error) {
	// Build map of incomplete tasks only.
	taskMap := make(map[string]*models.Task, len(tasks))
	for _, t := range tasks {
		if t == nil || t.ID == "" {
			continue
		}
		if !t.IsComplete() {
			taskMap[t.ID] = t
		}
	}

	// Build in-degree count and reverse-adjacency list (pred -> successors).
	// Only edges where the predecessor is also in taskMap (incomplete) count
	// toward in-degree; completed predecessors are treated as already satisfied.
	inDegree := make(map[string]int, len(taskMap))
	// adj maps predecessor task ID -> list of dependent task IDs.
	adj := make(map[string][]string, len(taskMap))
	for id := range taskMap {
		inDegree[id] = 0
	}
	for _, t := range taskMap {
		for _, depRef := range t.DependsOn {
			ref := models.ParseTaskRef(depRef)
			predID := ref.TaskID
			if _, ok := taskMap[predID]; ok {
				inDegree[t.ID]++
				adj[predID] = append(adj[predID], t.ID)
			}
		}
	}

	// Seed queue with zero in-degree nodes; sort for deterministic output.
	queue := make([]*models.Task, 0, len(taskMap))
	for id, t := range taskMap {
		if inDegree[id] == 0 {
			queue = append(queue, t)
		}
	}
	sortByID(queue)

	result := make([]*models.Task, 0, len(taskMap))
	for len(queue) > 0 {
		t := queue[0]
		queue = queue[1:]
		result = append(result, t)

		successors := adj[t.ID]
		sort.Strings(successors) // deterministic successor processing
		added := queue[:0:0]
		for _, succID := range successors {
			inDegree[succID]--
			if inDegree[succID] == 0 {
				added = append(added, taskMap[succID])
			}
		}
		if len(added) > 0 {
			sortByID(added)
			queue = append(queue, added...)
		}
	}

	if len(result) != len(taskMap) {
		return nil, fmt.Errorf("cycle detected in task dependencies")
	}
	return result, nil
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
	es := make(map[string]float64, len(topoOrder)) // all initialised to 0
	for _, t := range topoOrder {
		for _, depRef := range t.DependsOn {
			ref := models.ParseTaskRef(depRef)
			pred, ok := taskMap[ref.TaskID]
			if !ok {
				continue
			}
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
			ref := models.ParseTaskRef(depRef)
			if _, ok := taskMap[ref.TaskID]; ok {
				succs[ref.TaskID] = append(succs[ref.TaskID], t.ID)
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
			ref := models.ParseTaskRef(depRef)
			pred, ok := taskMap[ref.TaskID]
			if !ok {
				continue
			}
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
		result = append(result, ScheduledTask{
			Task:          t,
			EarliestStart: start,
			EarliestEnd:   end,
			Slack:         cp.Slack[t.ID],
			IsCritical:    cp.OnPath[t.ID],
		})
	}
	return result
}

// daysToduration converts a float64 day count to a time.Duration.
func daysToduration(days float64) time.Duration {
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
			ref := models.ParseTaskRef(depRef)
			pred, ok := taskMap[ref.TaskID]
			if !ok {
				continue
			}
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
			ref := models.ParseTaskRef(depRef)
			if _, ok := taskMap[ref.TaskID]; ok {
				succs[ref.TaskID] = append(succs[ref.TaskID], t.ID)
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

// topologicalSortByPriority is like TopologicalSort but breaks ties among
// zero-in-degree nodes by priority (ascending = higher priority) then ID.
func topologicalSortByPriority(tasks []*models.Task) ([]*models.Task, error) {
	taskMap := make(map[string]*models.Task, len(tasks))
	for _, t := range tasks {
		if t == nil || t.ID == "" {
			continue
		}
		if !t.IsComplete() {
			taskMap[t.ID] = t
		}
	}

	inDegree := make(map[string]int, len(taskMap))
	adj := make(map[string][]string, len(taskMap))
	for id := range taskMap {
		inDegree[id] = 0
	}
	for _, t := range taskMap {
		for _, depRef := range t.DependsOn {
			ref := models.ParseTaskRef(depRef)
			predID := ref.TaskID
			if _, ok := taskMap[predID]; ok {
				inDegree[t.ID]++
				adj[predID] = append(adj[predID], t.ID)
			}
		}
	}

	queue := make([]*models.Task, 0, len(taskMap))
	for id, t := range taskMap {
		if inDegree[id] == 0 {
			queue = append(queue, t)
		}
	}
	sortByPriorityThenID(queue)

	result := make([]*models.Task, 0, len(taskMap))
	for len(queue) > 0 {
		t := queue[0]
		queue = queue[1:]
		result = append(result, t)

		successors := adj[t.ID]
		sort.Strings(successors)
		var added []*models.Task
		for _, succID := range successors {
			inDegree[succID]--
			if inDegree[succID] == 0 {
				added = append(added, taskMap[succID])
			}
		}
		if len(added) > 0 {
			sortByPriorityThenID(added)
			queue = append(queue, added...)
			sortByPriorityThenID(queue)
		}
	}

	if len(result) != len(taskMap) {
		return nil, fmt.Errorf("cycle detected in task dependencies")
	}
	return result, nil
}

// sortByPriorityThenID sorts tasks by priority ascending (lower = higher
// priority), then by ID ascending for determinism.
func sortByPriorityThenID(tasks []*models.Task) {
	sort.Slice(tasks, func(i, j int) bool {
		if tasks[i].Priority != tasks[j].Priority {
			return tasks[i].Priority < tasks[j].Priority
		}
		return tasks[i].ID < tasks[j].ID
	})
}

// sortByID sorts a slice of tasks by ID for deterministic ordering.
func sortByID(tasks []*models.Task) {
	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].ID < tasks[j].ID
	})
}
