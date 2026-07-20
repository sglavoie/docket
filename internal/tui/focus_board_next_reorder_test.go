package tui

import (
	"path/filepath"
	"strings"
	"testing"

	"charm.land/bubbles/v2/textinput"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

// newNextReorderBoard builds a Next-tab FocusBoardModel rooted at a writable
// data dir so persistNextOrder commits a real planning.json. It seeds the
// order index sparsely (indexIDs) plus optional stale non-Next entries
// (staleItems), then plants every StateNext task as a nextRow in insertion
// order — exactly what loadItems() would produce before the display sort.
func newNextReorderBoard(t *testing.T, seedIndex []string, staleItems []config.FocusQueueItem, nextTasks []*models.Task) (*FocusBoardModel, *config.RuntimeState, *models.Project) {
	t.Helper()
	dataDir := t.TempDir()
	cfg := newTestConfigAt(dataDir)
	project := testutil.NewTestProject("Reorder")
	project.Path = cfg.ProjectPath(project.Slug)
	setTestProjects(cfg, []*models.Project{project})
	runtime := testRuntimeAt(cfg.DataDir(), cfg)

	byID := make(map[string]*models.Task, len(nextTasks))
	for _, task := range nextTasks {
		byID[task.ID] = task
	}

	focusQueue := make([]FocusTask, 0, len(seedIndex))
	items := make([]config.FocusQueueItem, 0, len(seedIndex)+len(staleItems))
	for _, id := range seedIndex {
		task := byID[id]
		focusQueue = append(focusQueue, FocusTask{Task: task, Project: project})
		items = append(items, config.FocusQueueItem{TaskID: id, ProjectID: project.ID})
	}
	items = append(items, staleItems...)
	if err := commitPlanningToRuntime(runtime, func(p *config.PlanningState) {
		p.SetFocusQueue(items)
	}); err != nil {
		t.Fatalf("seed focus queue: %v", err)
	}

	dp := []FocusTask{}
	fq := focusQueue
	m := &FocusBoardModel{
		runtime:           runtime,
		projectService:    NewProjectService(),
		dailyPlan:         &dp,
		focusQueue:        &fq,
		dailyPlanIDs:      map[string]bool{},
		queueIDs:          map[string]bool{},
		carriedOverIDs:    map[string]bool{},
		addInput:          textinput.New(),
		editInput:         textinput.New(),
		FilterMixin:       FilterMixin{filter: newFilterState(), filterMachine: runtime.Machine()},
		sort:              NewSortState(),
		tabs:              TabState{Names: focusBoardTabNames, Active: boardTabNext},
		activePerspective: PerspectiveInactive,
	}
	for _, task := range nextTasks {
		m.nextRows = append(m.nextRows, boardRow{Task: task, Project: project, Source: project.Name})
	}
	m.applyFilter()
	return m, runtime, project
}

// persistedQueueIDs returns the on-disk Next order index task IDs, loaded through
// a fresh manager to prove the reorder survives a restart.
func persistedQueueIDs(t *testing.T, runtime *config.RuntimeState) []string {
	t.Helper()
	mgr := config.NewManager(filepath.Join(runtime.DataDir(), ".tt.json"))
	planning, err := mgr.DomainCoordinator().LoadPlanning()
	if err != nil {
		t.Fatalf("load planning: %v", err)
	}
	ids := make([]string, 0, len(planning.GetFocusQueue()))
	for _, item := range planning.GetFocusQueue() {
		ids = append(ids, item.TaskID)
	}
	return ids
}

// persistedDailyPlanIDs returns the on-disk Today order through a fresh manager.
func persistedDailyPlanIDs(t *testing.T, runtime *config.RuntimeState) []string {
	t.Helper()
	mgr := config.NewManager(filepath.Join(runtime.DataDir(), ".tt.json"))
	planning, err := mgr.DomainCoordinator().LoadPlanning()
	if err != nil {
		t.Fatalf("load planning: %v", err)
	}
	ids := make([]string, 0, len(planning.GetDailyPlan()))
	for _, item := range planning.GetDailyPlan() {
		ids = append(ids, item.TaskID)
	}
	return ids
}

func runtimeQueueIDs(runtime *config.RuntimeState) []string {
	items := runtime.Planning().GetFocusQueue()
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.TaskID)
	}
	return ids
}

func runtimeDailyPlanIDs(runtime *config.RuntimeState) []string {
	items := runtime.Planning().GetDailyPlan()
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.TaskID)
	}
	return ids
}

func focusTaskIDs(tasks []FocusTask) []string {
	ids := make([]string, 0, len(tasks))
	for _, task := range tasks {
		if task.Task != nil {
			ids = append(ids, task.Task.ID)
		}
	}
	return ids
}

func rowIDs(rows []boardRow) []string {
	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		if row.Task != nil {
			ids = append(ids, row.Task.ID)
		}
	}
	return ids
}

func filteredIDs(m *FocusBoardModel) []string {
	return rowIDs(m.filtered)
}

func assertIDOrder(t *testing.T, label string, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: length %d, want %d (%v vs %v)", label, len(got), len(want), got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%s: order %v, want %v", label, got, want)
		}
	}
}

func nextTask(t *testing.T, project *models.Project, id, title string, state models.WorkflowState) *models.Task {
	t.Helper()
	task, err := models.NewTask(id, len(project.Tasks)+1, models.TaskTypeTask, title)
	if err != nil {
		t.Fatalf("new task %q: %v", title, err)
	}
	task.WorkflowState = state
	project.Tasks = append(project.Tasks, task)
	return task
}

func newTodayReorderBoard(t *testing.T) (*FocusBoardModel, *config.RuntimeState) {
	t.Helper()
	m, runtime, project := newNextReorderBoard(t, nil, nil, nil)
	a := nextTask(t, project, "id-a", "Alpha", models.StateInProgress)
	b := nextTask(t, project, "id-b", "Bravo", models.StateInProgress)
	c := nextTask(t, project, "id-c", "Charlie", models.StateInProgress)
	a.Priority = 3
	b.Priority = 2
	c.Priority = 1

	dailyPlan := []FocusTask{
		{Task: a, Project: project},
		{Task: b, Project: project},
		{Task: c, Project: project},
	}
	items := []config.DailyPlanItem{
		{TaskID: a.ID, ProjectID: project.ID, PlannedDate: "2026-07-20"},
		{TaskID: b.ID, ProjectID: project.ID, PlannedDate: "2026-07-20"},
		{TaskID: c.ID, ProjectID: project.ID, PlannedDate: "2026-07-20"},
	}
	if err := commitPlanningToRuntime(runtime, func(p *config.PlanningState) {
		p.SetDailyPlan(items)
	}); err != nil {
		t.Fatalf("seed daily plan: %v", err)
	}

	m.tabs.Active = boardTabToday
	m.dailyPlan = &dailyPlan
	m.dailyPlanIDs = map[string]bool{a.ID: true, b.ID: true, c.ID: true}
	m.todayRows = []boardRow{
		{Task: a, Project: project, Priority: a.Priority, Source: project.Name},
		{Task: b, Project: project, Priority: b.Priority, Source: project.Name},
		{Task: c, Project: project, Priority: c.Priority, Source: project.Name},
	}
	m.sort = NewSortState()
	m.applyFilter()
	return m, runtime
}

// TestPlannerNextReorderPersistsCanonicalOrder is the FR-012-002 regression: the
// Planner Next tab exposes an adjacent reorder action (J/K) that edits the global
// Next order index. Reordering must persist a canonical order containing every
// live StateNext task and no stale non-Next entries, promote previously-unindexed
// members to explicit entries, and behave correctly when the visible list is a
// filtered subset — all without changing StateNext membership.
func TestPlannerNextReorderPersistsCanonicalOrder(t *testing.T) {
	newTasks := func(t *testing.T, project *models.Project) map[string]*models.Task {
		return map[string]*models.Task{
			"a": nextTask(t, project, "id-a", "Alpha", models.StateNext),
			"b": nextTask(t, project, "id-b", "Bravo", models.StateNext),
			"c": nextTask(t, project, "id-c", "Charlie", models.StateNext),
			"d": nextTask(t, project, "id-d", "Delta", models.StateNext),
			"e": nextTask(t, project, "id-e", "Echo", models.StateNext),
		}
	}

	t.Run("reorder drops stale non-Next entries and keeps every member", func(t *testing.T) {
		project := testutil.NewTestProject("seed")
		tasks := newTasks(t, project)
		// Seed index A,B,C plus a stale entry for a task that has left Next; D,E
		// are unindexed StateNext members.
		order := []*models.Task{tasks["a"], tasks["b"], tasks["c"], tasks["d"], tasks["e"]}
		m, runtime, _ := newNextReorderBoard(t,
			[]string{"id-a", "id-b", "id-c"},
			[]config.FocusQueueItem{{TaskID: "id-gone", ProjectID: project.ID}},
			order,
		)

		// Sanity: display order applies the seed index then appends unindexed.
		assertIDOrder(t, "initial filtered", filteredIDs(m), []string{"id-a", "id-b", "id-c", "id-d", "id-e"})

		// J on Charlie (cursor 2) moves it down past Delta.
		m.cursor = 2
		m.handleFocusBoardReorder(1)

		want := []string{"id-a", "id-b", "id-d", "id-c", "id-e"}
		assertIDOrder(t, "filtered after J", filteredIDs(m), want)
		assertIDOrder(t, "persisted after J", persistedQueueIDs(t, runtime), want)
		if m.cursor != 3 {
			t.Fatalf("cursor = %d, want 3 (follows moved task)", m.cursor)
		}
	})

	t.Run("previously-unindexed member can be reordered and becomes indexed", func(t *testing.T) {
		project := testutil.NewTestProject("seed")
		tasks := newTasks(t, project)
		order := []*models.Task{tasks["a"], tasks["b"], tasks["c"], tasks["d"], tasks["e"]}
		// Only A,B,C indexed; D,E unindexed.
		m, runtime, _ := newNextReorderBoard(t, []string{"id-a", "id-b", "id-c"}, nil, order)

		// K on Echo (cursor 4, unindexed) moves it up past Delta.
		m.cursor = 4
		m.handleFocusBoardReorder(-1)

		want := []string{"id-a", "id-b", "id-c", "id-e", "id-d"}
		assertIDOrder(t, "filtered after K", filteredIDs(m), want)
		// Every StateNext task now has an explicit order entry (D and E included).
		assertIDOrder(t, "persisted after K", persistedQueueIDs(t, runtime), want)
	})

	t.Run("filtered subset reorder preserves hidden members", func(t *testing.T) {
		project := testutil.NewTestProject("seed")
		tasks := newTasks(t, project)
		order := []*models.Task{tasks["a"], tasks["b"], tasks["c"], tasks["d"], tasks["e"]}
		m, runtime, _ := newNextReorderBoard(t, []string{"id-a", "id-b", "id-c"}, nil, order)

		// Simulate a filter that hides B, C, E: only A and D are visible.
		m.filtered = []boardRow{m.nextRows[0], m.nextRows[3]}
		m.cursor = 0

		// J on Alpha swaps it past Delta in the full canonical order, even though
		// B, C, E sit between them but are hidden.
		m.handleFocusBoardReorder(1)

		assertIDOrder(t, "filtered after J", filteredIDs(m), []string{"id-d", "id-a"})
		// Hidden members B, C, E must remain in the persisted order.
		assertIDOrder(t, "persisted after J", persistedQueueIDs(t, runtime),
			[]string{"id-d", "id-b", "id-c", "id-a", "id-e"})
	})

	t.Run("reorder never changes StateNext membership", func(t *testing.T) {
		project := testutil.NewTestProject("seed")
		tasks := newTasks(t, project)
		order := []*models.Task{tasks["a"], tasks["b"], tasks["c"], tasks["d"], tasks["e"]}
		m, _, _ := newNextReorderBoard(t, []string{"id-a", "id-b", "id-c"}, nil, order)

		m.cursor = 0
		m.handleFocusBoardReorder(1)

		for _, task := range order {
			if task.State() != models.StateNext {
				t.Fatalf("task %s left StateNext (now %s) after reorder", task.ID, task.State())
			}
		}
	})

	t.Run("reorder is a no-op on the Backlog tab", func(t *testing.T) {
		project := testutil.NewTestProject("seed")
		tasks := newTasks(t, project)
		order := []*models.Task{tasks["a"], tasks["b"], tasks["c"], tasks["d"], tasks["e"]}
		m, runtime, _ := newNextReorderBoard(t, []string{"id-a", "id-b", "id-c"}, nil, order)
		m.tabs.Active = boardTabBacklog
		m.cursor = 0

		before := persistedQueueIDs(t, runtime)
		m.handleFocusBoardReorder(1)
		assertIDOrder(t, "persisted unchanged", persistedQueueIDs(t, runtime), before)
	})
}

// TestPlannerNextReorderRejectsSecondarySort is the FR-013-002 regression. When a
// non-default sort is active, the visible Next list no longer matches the
// canonical FocusQueue order, so a J/K swap of view-adjacent rows would mutate the
// hidden canonical order and then vanish on the next filter refresh. Reorder must
// be rejected with an explicit status while a secondary sort is active, and only
// re-enabled once the Next tab is returned to its canonical order.
func TestPlannerNextReorderRejectsSecondarySort(t *testing.T) {
	project := testutil.NewTestProject("seed")
	a := nextTask(t, project, "id-a", "Alpha", models.StateNext)
	b := nextTask(t, project, "id-b", "Bravo", models.StateNext)
	c := nextTask(t, project, "id-c", "Charlie", models.StateNext)
	// Priorities chosen so an ascending priority sort reverses the canonical
	// A,B,C queue order into C,B,A.
	a.Priority = 3
	b.Priority = 2
	c.Priority = 1
	order := []*models.Task{a, b, c}
	m, runtime, _ := newNextReorderBoard(t, []string{"id-a", "id-b", "id-c"}, nil, order)

	// Canonical (default-sort) display order is the seeded index.
	assertIDOrder(t, "canonical filtered", filteredIDs(m), []string{"id-a", "id-b", "id-c"})

	// Activate a non-default sort: the visible list now diverges from the
	// canonical FocusQueue order.
	m.sort.Toggle(SortByPriority)
	m.applyFilter()
	assertIDOrder(t, "priority-sorted filtered", filteredIDs(m), []string{"id-c", "id-b", "id-a"})

	before := persistedQueueIDs(t, runtime)
	visBefore := filteredIDs(m)

	// J while a secondary sort is active must be rejected: an explicit status is
	// shown, the durable order is untouched, and the visible rows/cursor do not
	// move.
	m.cursor = 0
	m.statusMessage = ""
	m.handleFocusBoardReorder(1)
	if !strings.Contains(m.statusMessage, "reset to Next order") {
		t.Fatalf("status = %q, want Next-order reset guidance", m.statusMessage)
	}
	assertIDOrder(t, "persisted unchanged", persistedQueueIDs(t, runtime), before)
	assertIDOrder(t, "visible unchanged", filteredIDs(m), visBefore)
	if m.cursor != 0 {
		t.Fatalf("cursor moved to %d despite rejected reorder", m.cursor)
	}

	// Returning to canonical order re-enables reorder and it persists durably.
	m.sort.Reset()
	m.applyFilter()
	assertIDOrder(t, "restored canonical filtered", filteredIDs(m), []string{"id-a", "id-b", "id-c"})
	m.cursor = 0
	m.handleFocusBoardReorder(1)
	want := []string{"id-b", "id-a", "id-c"}
	assertIDOrder(t, "filtered after reset+J", filteredIDs(m), want)
	assertIDOrder(t, "persisted after reset+J", persistedQueueIDs(t, runtime), want)
}

// TestPlannerTodayReorderRejectsSecondarySort proves Today shares the same
// canonical-order guard as Next, with Today-specific reset guidance.
func TestPlannerTodayReorderRejectsSecondarySort(t *testing.T) {
	tests := []struct {
		name   string
		cursor int
		delta  int
	}{
		{name: "J", cursor: 0, delta: 1},
		{name: "K", cursor: 2, delta: -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, runtime := newTodayReorderBoard(t)
			m.sort.Toggle(SortByPriority)
			m.applyFilter()
			assertIDOrder(t, "priority-sorted filtered", filteredIDs(m), []string{"id-c", "id-b", "id-a"})

			persistedBefore := persistedDailyPlanIDs(t, runtime)
			runtimeBefore := runtimeDailyPlanIDs(runtime)
			sharedBefore := focusTaskIDs(*m.dailyPlan)
			todayBefore := rowIDs(m.todayRows)
			filteredBefore := filteredIDs(m)
			m.cursor = tt.cursor
			m.statusMessage = ""

			m.handleFocusBoardReorder(tt.delta)

			if !strings.Contains(m.statusMessage, "reset to Today order") {
				t.Fatalf("status = %q, want Today-order reset guidance", m.statusMessage)
			}
			assertIDOrder(t, "persisted unchanged", persistedDailyPlanIDs(t, runtime), persistedBefore)
			assertIDOrder(t, "runtime unchanged", runtimeDailyPlanIDs(runtime), runtimeBefore)
			assertIDOrder(t, "shared plan unchanged", focusTaskIDs(*m.dailyPlan), sharedBefore)
			assertIDOrder(t, "today rows unchanged", rowIDs(m.todayRows), todayBefore)
			assertIDOrder(t, "filtered unchanged", filteredIDs(m), filteredBefore)
			if m.cursor != tt.cursor {
				t.Fatalf("cursor moved to %d despite rejected reorder; want %d", m.cursor, tt.cursor)
			}
		})
	}
}

// TestPlannerReorderCommitFailureLeavesOrdersUnchanged proves neither horizon
// publishes candidate ordering before its checked planning commit succeeds.
func TestPlannerReorderCommitFailureLeavesOrdersUnchanged(t *testing.T) {
	failCommit := func(*config.RuntimeState, func(*config.PlanningState)) error {
		return config.ErrStaleObject
	}

	t.Run("Today", func(t *testing.T) {
		m, runtime := newTodayReorderBoard(t)
		m.planningCommit = failCommit
		m.cursor = 0

		persistedBefore := persistedDailyPlanIDs(t, runtime)
		runtimeBefore := runtimeDailyPlanIDs(runtime)
		sharedBefore := focusTaskIDs(*m.dailyPlan)
		todayBefore := rowIDs(m.todayRows)
		filteredBefore := filteredIDs(m)

		m.handleFocusBoardReorder(1)

		if !strings.Contains(m.statusMessage, config.ErrStaleObject.Error()) {
			t.Fatalf("status = %q, want stale-object error", m.statusMessage)
		}
		assertIDOrder(t, "persisted unchanged", persistedDailyPlanIDs(t, runtime), persistedBefore)
		assertIDOrder(t, "runtime unchanged", runtimeDailyPlanIDs(runtime), runtimeBefore)
		assertIDOrder(t, "shared plan unchanged", focusTaskIDs(*m.dailyPlan), sharedBefore)
		assertIDOrder(t, "today rows unchanged", rowIDs(m.todayRows), todayBefore)
		assertIDOrder(t, "filtered unchanged", filteredIDs(m), filteredBefore)
		if m.cursor != 0 {
			t.Fatalf("cursor moved to %d after failed commit", m.cursor)
		}
	})

	t.Run("Next", func(t *testing.T) {
		project := testutil.NewTestProject("seed")
		a := nextTask(t, project, "id-a", "Alpha", models.StateNext)
		b := nextTask(t, project, "id-b", "Bravo", models.StateNext)
		c := nextTask(t, project, "id-c", "Charlie", models.StateNext)
		m, runtime, _ := newNextReorderBoard(t, []string{"id-a", "id-b", "id-c"}, nil, []*models.Task{a, b, c})
		m.planningCommit = failCommit
		m.cursor = 0

		persistedBefore := persistedQueueIDs(t, runtime)
		runtimeBefore := runtimeQueueIDs(runtime)
		sharedBefore := focusTaskIDs(*m.focusQueue)
		nextBefore := rowIDs(m.nextRows)
		filteredBefore := filteredIDs(m)

		m.handleFocusBoardReorder(1)

		if !strings.Contains(m.statusMessage, config.ErrStaleObject.Error()) {
			t.Fatalf("status = %q, want stale-object error", m.statusMessage)
		}
		assertIDOrder(t, "persisted unchanged", persistedQueueIDs(t, runtime), persistedBefore)
		assertIDOrder(t, "runtime unchanged", runtimeQueueIDs(runtime), runtimeBefore)
		assertIDOrder(t, "shared queue unchanged", focusTaskIDs(*m.focusQueue), sharedBefore)
		assertIDOrder(t, "next rows unchanged", rowIDs(m.nextRows), nextBefore)
		assertIDOrder(t, "filtered unchanged", filteredIDs(m), filteredBefore)
		if m.cursor != 0 {
			t.Fatalf("cursor moved to %d after failed commit", m.cursor)
		}
	})
}
