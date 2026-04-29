package planner

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

// newTask is a test helper that creates a task with the given ID.
func newTask(id string) *models.Task {
	t, err := models.NewTask(id, 1, models.TaskTypeTask, "Task "+id)
	if err != nil {
		panic(fmt.Sprintf("newTask: %v", err))
	}
	return t
}

// newTaskWithDuration creates a task with a given EstimatedDays.
func newTaskWithDuration(id string, days float64) *models.Task {
	t := newTask(id)
	t.EstimatedDays = days
	return t
}

// withDep adds a dependency ID to a task and returns the task.
func withDep(t *models.Task, depIDs ...string) *models.Task {
	for _, id := range depIDs {
		_ = t.AddDependency(id)
	}
	return t
}

// withStatus marks a task's status and returns the task.
func withStatus(t *models.Task, s models.TaskStatus) *models.Task {
	t.Status = s
	return t
}

// TestTopologicalLinearChain verifies A -> B -> C order.
func TestTopologicalLinearChain(t *testing.T) {
	a := newTask("a")
	b := withDep(newTask("b"), "a")
	c := withDep(newTask("c"), "b")

	order, err := TopologicalSort([]*models.Task{c, b, a})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(order) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(order))
	}
	pos := taskPositions(order)
	if pos["a"] >= pos["b"] || pos["b"] >= pos["c"] {
		t.Errorf("order violation: a=%d b=%d c=%d", pos["a"], pos["b"], pos["c"])
	}
}

// TestTopologicalDiamond verifies a diamond dependency: A, B->A, C->A, D->B,D->C.
func TestTopologicalDiamond(t *testing.T) {
	a := newTask("a")
	b := withDep(newTask("b"), "a")
	c := withDep(newTask("c"), "a")
	d := withDep(newTask("d"), "b", "c")

	order, err := TopologicalSort([]*models.Task{d, c, b, a})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(order) != 4 {
		t.Fatalf("expected 4 tasks, got %d", len(order))
	}
	pos := taskPositions(order)
	if pos["a"] >= pos["b"] {
		t.Errorf("a must come before b")
	}
	if pos["a"] >= pos["c"] {
		t.Errorf("a must come before c")
	}
	if pos["b"] >= pos["d"] {
		t.Errorf("b must come before d")
	}
	if pos["c"] >= pos["d"] {
		t.Errorf("c must come before d")
	}
}

// TestTopologicalNoDeps verifies tasks without dependencies are all returned.
func TestTopologicalNoDeps(t *testing.T) {
	tasks := []*models.Task{newTask("x"), newTask("y"), newTask("z")}
	order, err := TopologicalSort(tasks)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(order) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(order))
	}
}

// TestTopologicalSingleNode verifies a single task returns correctly.
func TestTopologicalSingleNode(t *testing.T) {
	order, err := TopologicalSort([]*models.Task{newTask("solo")})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(order) != 1 || order[0].ID != "solo" {
		t.Errorf("expected [solo], got %v", taskIDs(order))
	}
}

// TestTopologicalMixedCompleteIncomplete verifies complete tasks are excluded.
func TestTopologicalMixedCompleteIncomplete(t *testing.T) {
	done := withStatus(newTask("done"), models.StatusDone)
	todo := withDep(newTask("todo"), "done") // depends on a complete task

	order, err := TopologicalSort([]*models.Task{done, todo})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(order) != 1 || order[0].ID != "todo" {
		t.Errorf("expected only [todo], got %v", taskIDs(order))
	}
}

// TestTopologicalDisconnectedComponents verifies multiple independent trees.
func TestTopologicalDisconnectedComponents(t *testing.T) {
	// Component 1: p -> q
	p := newTask("p")
	q := withDep(newTask("q"), "p")
	// Component 2: r -> s
	r := newTask("r")
	s := withDep(newTask("s"), "r")

	order, err := TopologicalSort([]*models.Task{s, q, r, p})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(order) != 4 {
		t.Fatalf("expected 4 tasks, got %d", len(order))
	}
	pos := taskPositions(order)
	if pos["p"] >= pos["q"] {
		t.Errorf("p must come before q")
	}
	if pos["r"] >= pos["s"] {
		t.Errorf("r must come before s")
	}
}

// TestTopologicalCrossProjectDependency verifies cross-project refs are
// treated as satisfied (not in the slice) and don't block ordering.
func TestTopologicalCrossProjectDependency(t *testing.T) {
	// Task B depends on a task in project "proj-a" (not in this slice).
	b := withDep(newTask("b"), "proj-a:external-task")
	c := withDep(newTask("c"), "b")

	order, err := TopologicalSort([]*models.Task{c, b})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(order) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(order))
	}
	pos := taskPositions(order)
	if pos["b"] >= pos["c"] {
		t.Errorf("b must come before c")
	}
}

// TestTopologicalCrossProjectChainResolved verifies that when cross-project
// tasks ARE included in the slice (via CollectAllIncompleteTasks), ordering
// is correct across the unified flat slice.
func TestTopologicalCrossProjectChainResolved(t *testing.T) {
	// Task in project B depends on task in project A.
	taskA := newTask("task-a")
	taskB := withDep(newTask("task-b"), "task-a") // same-project lookup in unified slice

	order, err := TopologicalSort([]*models.Task{taskB, taskA})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	pos := taskPositions(order)
	if pos["task-a"] >= pos["task-b"] {
		t.Errorf("task-a must come before task-b")
	}
}

// TestCriticalPathLinear verifies the critical path for A(1) -> B(2) -> C(3).
// Expected length = 1+2+3 = 6; all tasks on critical path.
func TestCriticalPathLinear(t *testing.T) {
	a := newTaskWithDuration("a", 1)
	b := withDep(newTaskWithDuration("b", 2), "a")
	c := withDep(newTaskWithDuration("c", 3), "b")

	cp := CriticalPath([]*models.Task{a, b, c})

	const wantLen = 6.0
	if cp.Length != wantLen {
		t.Errorf("expected length %.1f, got %.1f", wantLen, cp.Length)
	}
	for _, id := range []string{"a", "b", "c"} {
		if !cp.OnPath[id] {
			t.Errorf("expected task %q on critical path", id)
		}
		if cp.Slack[id] != 0 {
			t.Errorf("expected zero slack for %q, got %f", id, cp.Slack[id])
		}
	}
}

// TestCriticalPathDiamond verifies slack for the non-critical path.
// A(1) -> B(1) -> D(1)
// A(1) -> C(3) -> D(1)
// Critical path: A->C->D length = 1+3+1 = 5; B has slack.
func TestCriticalPathDiamond(t *testing.T) {
	a := newTaskWithDuration("a", 1)
	b := withDep(newTaskWithDuration("b", 1), "a")
	c := withDep(newTaskWithDuration("c", 3), "a")
	d := withDep(newTaskWithDuration("d", 1), "b", "c")

	cp := CriticalPath([]*models.Task{a, b, c, d})

	const wantLen = 5.0
	if cp.Length != wantLen {
		t.Errorf("expected length %.1f, got %.1f", wantLen, cp.Length)
	}
	// a, c, d are on the critical path.
	for _, id := range []string{"a", "c", "d"} {
		if !cp.OnPath[id] {
			t.Errorf("expected %q on critical path", id)
		}
	}
	// b has slack = 2 (can start 2 days late without delaying the project).
	if cp.OnPath["b"] {
		t.Errorf("b should not be on critical path")
	}
	if cp.Slack["b"] <= 0 {
		t.Errorf("b should have positive slack, got %f", cp.Slack["b"])
	}
}

// TestCriticalPathNoDeps verifies single-task critical path.
func TestCriticalPathNoDeps(t *testing.T) {
	a := newTaskWithDuration("a", 2.5)
	cp := CriticalPath([]*models.Task{a})
	if cp.Length != 2.5 {
		t.Errorf("expected length 2.5, got %f", cp.Length)
	}
	if !cp.OnPath["a"] {
		t.Errorf("a should be on critical path")
	}
}

// TestCriticalPathDefaultDuration verifies fallback to 1.0 when EstimatedDays == 0.
func TestCriticalPathDefaultDuration(t *testing.T) {
	a := newTask("a") // EstimatedDays = 0, should default to 1.0
	b := withDep(newTask("b"), "a")
	cp := CriticalPath([]*models.Task{a, b})
	if cp.Length != 2.0 {
		t.Errorf("expected length 2.0, got %f", cp.Length)
	}
}

// TestCriticalPathSlackIsZeroForCriticalTasks verifies the slack invariant:
// slack == 0 iff the task is on the critical path.
func TestCriticalPathSlackIsZeroForCriticalTasks(t *testing.T) {
	a := newTaskWithDuration("a", 1)
	b := withDep(newTaskWithDuration("b", 1), "a") // short path
	c := withDep(newTaskWithDuration("c", 5), "a") // long path -> critical
	d := withDep(newTaskWithDuration("d", 1), "b", "c")

	cp := CriticalPath([]*models.Task{a, b, c, d})
	for id, onPath := range cp.OnPath {
		if onPath && cp.Slack[id] != 0 {
			t.Errorf("task %q on critical path but has non-zero slack %f", id, cp.Slack[id])
		}
	}
	for id, slack := range cp.Slack {
		if !cp.OnPath[id] && slack == 0 {
			t.Errorf("task %q not on critical path but has zero slack", id)
		}
	}
}

// TestForwardScheduleLinearChain verifies that chained tasks are scheduled
// sequentially with correct start/end dates.
func TestForwardScheduleLinearChain(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	a := newTaskWithDuration("a", 1)               // ends day 1
	b := withDep(newTaskWithDuration("b", 2), "a") // starts day 1, ends day 3
	c := withDep(newTaskWithDuration("c", 1), "b") // starts day 3, ends day 4

	sched := ForwardSchedule([]*models.Task{c, b, a}, start, 1.0)
	if len(sched) != 3 {
		t.Fatalf("expected 3 scheduled tasks, got %d", len(sched))
	}
	byID := scheduleByID(sched)

	// a: starts at 0, ends at 1 day
	if !byID["a"].EarliestStart.Equal(start) {
		t.Errorf("a: expected start %v, got %v", start, byID["a"].EarliestStart)
	}
	wantAEnd := start.Add(24 * time.Hour)
	if !byID["a"].EarliestEnd.Equal(wantAEnd) {
		t.Errorf("a: expected end %v, got %v", wantAEnd, byID["a"].EarliestEnd)
	}

	// b: starts where a ends
	if !byID["b"].EarliestStart.Equal(wantAEnd) {
		t.Errorf("b: expected start %v, got %v", wantAEnd, byID["b"].EarliestStart)
	}
}

// TestForwardScheduleVelocityScaling verifies that higher velocity compresses
// the schedule proportionally.
func TestForwardScheduleVelocityScaling(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	a := newTaskWithDuration("a", 4) // 4 days at velocity 1 = 4 days

	sched1 := ForwardSchedule([]*models.Task{a}, start, 1.0)
	sched2 := ForwardSchedule([]*models.Task{a}, start, 2.0) // twice as fast

	dur1 := sched1[0].EarliestEnd.Sub(sched1[0].EarliestStart)
	dur2 := sched2[0].EarliestEnd.Sub(sched2[0].EarliestStart)

	if dur2*2 != dur1 {
		t.Errorf("expected dur2 to be half dur1: dur1=%v dur2=%v", dur1, dur2)
	}
}

// TestForwardScheduleExplicitStartDate verifies that a task with an explicit
// StartDate respects it as a lower bound.
func TestForwardScheduleExplicitStartDate(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	later := time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)

	a := newTaskWithDuration("a", 1)
	a.StartDate = &later // explicitly scheduled for Jan 5

	sched := ForwardSchedule([]*models.Task{a}, start, 1.0)
	if len(sched) == 0 {
		t.Fatal("expected scheduled tasks")
	}
	if !sched[0].EarliestStart.Equal(later) {
		t.Errorf("expected start %v, got %v", later, sched[0].EarliestStart)
	}
}

// TestForwardScheduleZeroVelocityFallback verifies that zero/negative velocity
// is treated as 1.0 (no panic, sensible output).
func TestForwardScheduleZeroVelocityFallback(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	a := newTaskWithDuration("a", 2)

	schedZero := ForwardSchedule([]*models.Task{a}, start, 0)
	schedOne := ForwardSchedule([]*models.Task{a}, start, 1.0)

	if len(schedZero) != 1 || len(schedOne) != 1 {
		t.Fatal("expected 1 scheduled task")
	}
	if !schedZero[0].EarliestEnd.Equal(schedOne[0].EarliestEnd) {
		t.Errorf("zero velocity should behave like velocity=1: %v vs %v",
			schedZero[0].EarliestEnd, schedOne[0].EarliestEnd)
	}
}

// TestTopologicalPropertyRandomDAG is a property-based test that generates
// random DAGs and verifies that the topological order respects all edges and
// that the critical path length is consistent with the longest path.
func TestTopologicalPropertyRandomDAG(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	const (
		numTrials = 50
		maxNodes  = 12
	)

	for trial := 0; trial < numTrials; trial++ {
		n := rng.Intn(maxNodes) + 1
		tasks := make([]*models.Task, n)
		for i := 0; i < n; i++ {
			tasks[i] = newTaskWithDuration(fmt.Sprintf("t%d", i), float64(rng.Intn(5)+1))
		}
		// Add edges only from lower-indexed to higher-indexed nodes to guarantee
		// no cycles, then shuffle the slice to simulate unordered input.
		for i := 1; i < n; i++ {
			for j := 0; j < i; j++ {
				if rng.Intn(3) == 0 {
					_ = tasks[i].AddDependency(tasks[j].ID)
				}
			}
		}
		// Shuffle input order.
		rng.Shuffle(n, func(i, j int) { tasks[i], tasks[j] = tasks[j], tasks[i] })

		order, err := TopologicalSort(tasks)
		if err != nil {
			t.Errorf("trial %d: unexpected cycle error: %v", trial, err)
			continue
		}
		if len(order) != n {
			t.Errorf("trial %d: expected %d tasks, got %d", trial, n, len(order))
			continue
		}

		// Verify all edges are respected.
		pos := taskPositions(order)
		for _, task := range tasks {
			for _, depRef := range task.DependsOn {
				ref := models.ParseTaskRef(depRef)
				if _, ok := pos[ref.TaskID]; !ok {
					continue // dep not in slice (shouldn't happen here)
				}
				if pos[ref.TaskID] >= pos[task.ID] {
					t.Errorf("trial %d: dependency violation: %q (pos %d) must come before %q (pos %d)",
						trial, ref.TaskID, pos[ref.TaskID], task.ID, pos[task.ID])
				}
			}
		}

		// Verify critical path length equals longest path (brute-force check).
		cp := CriticalPath(tasks)
		longestPath := longestPathBruteForce(tasks)
		const eps = 1e-9
		if abs(cp.Length-longestPath) > eps {
			t.Errorf("trial %d: critical path length %f != brute-force longest path %f",
				trial, cp.Length, longestPath)
		}
	}
}

// longestPathBruteForce computes the longest path in a DAG by trying all
// source nodes and doing a DFS.  Used only in tests to verify CriticalPath.
func longestPathBruteForce(tasks []*models.Task) float64 {
	taskMap := make(map[string]*models.Task, len(tasks))
	for _, t := range tasks {
		if !t.IsComplete() {
			taskMap[t.ID] = t
		}
	}
	memo := make(map[string]float64)
	var dfs func(id string) float64
	dfs = func(id string) float64 {
		if v, ok := memo[id]; ok {
			return v
		}
		t := taskMap[id]
		if t == nil {
			return 0
		}
		best := EstimateDuration(t)
		for _, depRef := range t.DependsOn {
			ref := models.ParseTaskRef(depRef)
			if _, ok := taskMap[ref.TaskID]; !ok {
				continue
			}
			if v := dfs(ref.TaskID) + EstimateDuration(t); v > best {
				best = v
			}
		}
		memo[id] = best
		return best
	}
	var max float64
	for id := range taskMap {
		if v := dfs(id); v > max {
			max = v
		}
	}
	return max
}

// --- helpers ---

func taskPositions(order []*models.Task) map[string]int {
	pos := make(map[string]int, len(order))
	for i, t := range order {
		pos[t.ID] = i
	}
	return pos
}

func taskIDs(tasks []*models.Task) []string {
	ids := make([]string, len(tasks))
	for i, t := range tasks {
		ids[i] = t.ID
	}
	return ids
}

func scheduleByID(sched []ScheduledTask) map[string]ScheduledTask {
	m := make(map[string]ScheduledTask, len(sched))
	for _, s := range sched {
		m[s.Task.ID] = s
	}
	return m
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// TestCriticalPathWithEffort verifies CPM uses effort-based durations
// from TaskAttrs when EstimatedDays is not set.
func TestCriticalPathWithEffort(t *testing.T) {
	a := newTask("a")
	a.TaskAttrs = &models.TaskAttrs{Effort: models.EffortLarge} // 5 days
	b := withDep(newTask("b"), "a")
	b.TaskAttrs = &models.TaskAttrs{Effort: models.EffortSmall} // 0.5 days

	cp := CriticalPath([]*models.Task{a, b})

	// large=3.0 + small=0.5 = 3.5
	if abs(cp.Length-3.5) > 1e-9 {
		t.Errorf("expected length 3.5, got %f", cp.Length)
	}
	if !cp.OnPath["a"] || !cp.OnPath["b"] {
		t.Errorf("both tasks should be on the critical path")
	}
}

// TestSerialScheduleNoDeps verifies independent tasks are chained sequentially.
func TestSerialScheduleNoDeps(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	a := newTaskWithDuration("a", 2)
	b := newTaskWithDuration("b", 3)
	c := newTaskWithDuration("c", 1)

	sched := SerialSchedule([]*models.Task{c, b, a}, start, 1.0)
	if len(sched) != 3 {
		t.Fatalf("expected 3 scheduled tasks, got %d", len(sched))
	}

	// Verify each task starts where the previous one ends.
	for i := 1; i < len(sched); i++ {
		if !sched[i].EarliestStart.Equal(sched[i-1].EarliestEnd) {
			t.Errorf("task %d start (%v) != task %d end (%v)",
				i, sched[i].EarliestStart, i-1, sched[i-1].EarliestEnd)
		}
	}

	// First task starts at startDate.
	if !sched[0].EarliestStart.Equal(start) {
		t.Errorf("first task should start at %v, got %v", start, sched[0].EarliestStart)
	}
}

// TestSerialScheduleWithDeps verifies deps are respected AND no overlap.
func TestSerialScheduleWithDeps(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	a := newTaskWithDuration("a", 2)               // independent
	b := newTaskWithDuration("b", 1)               // independent
	c := withDep(newTaskWithDuration("c", 3), "a") // depends on a

	sched := SerialSchedule([]*models.Task{c, b, a}, start, 1.0)
	if len(sched) != 3 {
		t.Fatalf("expected 3 scheduled tasks, got %d", len(sched))
	}
	byID := scheduleByID(sched)

	// c must start after a ends.
	if byID["c"].EarliestStart.Before(byID["a"].EarliestEnd) {
		t.Errorf("c started before a ended: c.start=%v a.end=%v",
			byID["c"].EarliestStart, byID["a"].EarliestEnd)
	}

	// No two tasks overlap.
	for i := 0; i < len(sched); i++ {
		for j := i + 1; j < len(sched); j++ {
			si, sj := sched[i], sched[j]
			if si.EarliestStart.Before(sj.EarliestEnd) && sj.EarliestStart.Before(si.EarliestEnd) {
				t.Errorf("overlap between %s [%v,%v) and %s [%v,%v)",
					si.Task.ID, si.EarliestStart, si.EarliestEnd,
					sj.Task.ID, sj.EarliestStart, sj.EarliestEnd)
			}
		}
	}
}

// TestSerialSchedulePriority verifies higher-priority tasks are scheduled first.
func TestSerialSchedulePriority(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	low := newTaskWithDuration("low", 1)
	low.Priority = 3
	high := newTaskWithDuration("high", 1)
	high.Priority = 1
	med := newTaskWithDuration("med", 1)
	med.Priority = 2

	sched := SerialSchedule([]*models.Task{low, med, high}, start, 1.0)
	if len(sched) != 3 {
		t.Fatalf("expected 3, got %d", len(sched))
	}

	// Should be ordered: high, med, low.
	if sched[0].Task.ID != "high" {
		t.Errorf("expected first task to be 'high', got %q", sched[0].Task.ID)
	}
	if sched[1].Task.ID != "med" {
		t.Errorf("expected second task to be 'med', got %q", sched[1].Task.ID)
	}
	if sched[2].Task.ID != "low" {
		t.Errorf("expected third task to be 'low', got %q", sched[2].Task.ID)
	}
}
