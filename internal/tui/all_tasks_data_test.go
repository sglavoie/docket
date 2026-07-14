package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

// newMinimalAllTasksVM constructs an AllTasksViewModel for unit tests without
// triggering disk I/O. Callers populate groups and call calculateNavigation()
// themselves if they need navigation state to be valid.
func newMinimalAllTasksVM() *AllTasksViewModel {
	return &AllTasksViewModel{
		collapsedGroups: make(map[string]bool),
		projectCache:    make(map[string]*models.Project),
	}
}

// makeGroup is a test helper that builds an AllTasksGroup from a slice of tasks
// paired with a project, all wrapped in CrossProjectTask.
func makeGroup(name string, proj *models.Project, tasks ...*models.Task) AllTasksGroup {
	g := AllTasksGroup{Name: name}
	for _, t := range tasks {
		g.Tasks = append(g.Tasks, CrossProjectTask{Task: t, Project: proj})
	}
	return g
}

// ── totalNavigableItems ───────────────────────────────────────────────────────

func TestTotalNavigableItemsEmpty(t *testing.T) {
	m := newMinimalAllTasksVM()
	m.calculateNavigation()
	if got := m.totalNavigableItems(); got != 0 {
		t.Errorf("empty model: want 0, got %d", got)
	}
}

func TestTotalNavigableItemsHeadersAndTasks(t *testing.T) {
	proj := testutil.NewTestProject("alpha")
	t1 := testutil.NewTestTask("Task 1", models.TaskTypeTask)
	t2 := testutil.NewTestTask("Task 2", models.TaskTypeTask)

	m := newMinimalAllTasksVM()
	m.groups = []AllTasksGroup{
		makeGroup("Group A", proj, t1),
		makeGroup("Group B", proj, t2),
	}
	m.calculateNavigation()

	// 2 groups × (1 header + 1 task) = 4
	if got := m.totalNavigableItems(); got != 4 {
		t.Errorf("want 4 navigable items, got %d", got)
	}
}

func TestTotalNavigableItemsCollapsedGroup(t *testing.T) {
	proj := testutil.NewTestProject("beta")
	t1 := testutil.NewTestTask("Task A", models.TaskTypeTask)
	t2 := testutil.NewTestTask("Task B", models.TaskTypeTask)

	m := newMinimalAllTasksVM()
	m.groups = []AllTasksGroup{
		makeGroup("Group A", proj, t1, t2),
	}
	m.collapsedGroups["Group A"] = true
	m.calculateNavigation()

	// 1 header, tasks hidden because collapsed
	if got := m.totalNavigableItems(); got != 1 {
		t.Errorf("collapsed group: want 1, got %d", got)
	}
}

// ── totalVisibleTaskCount ─────────────────────────────────────────────────────

func TestTotalVisibleTaskCount(t *testing.T) {
	proj := testutil.NewTestProject("gamma")
	tasks := []*models.Task{
		testutil.NewTestTask("T1", models.TaskTypeTask),
		testutil.NewTestTask("T2", models.TaskTypeTask),
		testutil.NewTestTask("T3", models.TaskTypeTask),
	}

	m := newMinimalAllTasksVM()
	m.groups = []AllTasksGroup{
		makeGroup("Open", proj, tasks[0], tasks[1]),
		makeGroup("Closed", proj, tasks[2]),
	}
	m.calculateNavigation()

	if got := m.totalVisibleTaskCount(); got != 3 {
		t.Errorf("want 3 visible tasks, got %d", got)
	}
}

func TestTotalVisibleTaskCountCollapsed(t *testing.T) {
	proj := testutil.NewTestProject("delta")
	m := newMinimalAllTasksVM()
	m.groups = []AllTasksGroup{
		makeGroup("Open", proj,
			testutil.NewTestTask("T1", models.TaskTypeTask),
			testutil.NewTestTask("T2", models.TaskTypeTask),
		),
		makeGroup("Closed", proj,
			testutil.NewTestTask("T3", models.TaskTypeTask),
		),
	}
	m.collapsedGroups["Closed"] = true
	m.calculateNavigation()

	// Only "Open" tasks are visible (2); "Closed" is collapsed
	if got := m.totalVisibleTaskCount(); got != 2 {
		t.Errorf("want 2 visible tasks, got %d", got)
	}
}

// ── getTaskAtCursor ───────────────────────────────────────────────────────────

func TestGetTaskAtCursorOnHeader(t *testing.T) {
	proj := testutil.NewTestProject("epsilon")
	m := newMinimalAllTasksVM()
	m.groups = []AllTasksGroup{
		makeGroup("G", proj, testutil.NewTestTask("T", models.TaskTypeTask)),
	}
	m.calculateNavigation()

	m.cursor = 0 // pointing at the header
	if got := m.getTaskAtCursor(); got != nil {
		t.Errorf("cursor on header should return nil, got %v", got)
	}
}

func TestGetTaskAtCursorOnTask(t *testing.T) {
	proj := testutil.NewTestProject("zeta")
	task := testutil.NewTestTask("My Task", models.TaskTypeTask)

	m := newMinimalAllTasksVM()
	m.groups = []AllTasksGroup{makeGroup("G", proj, task)}
	m.calculateNavigation()

	m.cursor = 1 // pos 0 = header, pos 1 = task
	item := m.getTaskAtCursor()
	if item == nil {
		t.Fatal("expected non-nil task at cursor=1")
	}
	if item.Task != task {
		t.Errorf("got wrong task: want %q, got %q", task.Title, item.Task.Title)
	}
}

func TestGetTaskAtCursorSecondGroup(t *testing.T) {
	proj := testutil.NewTestProject("eta")
	t1 := testutil.NewTestTask("First", models.TaskTypeTask)
	t2 := testutil.NewTestTask("Second", models.TaskTypeTask)

	m := newMinimalAllTasksVM()
	m.groups = []AllTasksGroup{
		makeGroup("G1", proj, t1),
		makeGroup("G2", proj, t2),
	}
	m.calculateNavigation()

	// layout: 0=G1 header, 1=t1, 2=G2 header, 3=t2
	m.cursor = 3
	item := m.getTaskAtCursor()
	if item == nil {
		t.Fatal("expected non-nil task at cursor=3")
	}
	if item.Task != t2 {
		t.Errorf("got wrong task: want %q, got %q", t2.Title, item.Task.Title)
	}
}

// ── goToNextGroup ─────────────────────────────────────────────────────────────

func TestGoToNextGroupMovesToNextHeader(t *testing.T) {
	proj := testutil.NewTestProject("theta")
	m := newMinimalAllTasksVM()
	m.groups = []AllTasksGroup{
		makeGroup("G1", proj, testutil.NewTestTask("T1", models.TaskTypeTask)),
		makeGroup("G2", proj, testutil.NewTestTask("T2", models.TaskTypeTask)),
	}
	m.calculateNavigation()

	m.cursor = 0 // at G1 header
	m.goToNextGroup()

	// Should move to G2 header at position 2
	if m.cursor != 2 {
		t.Errorf("want cursor=2 (G2 header), got %d", m.cursor)
	}
}

func TestGoToNextGroupWraps(t *testing.T) {
	proj := testutil.NewTestProject("iota")
	m := newMinimalAllTasksVM()
	m.groups = []AllTasksGroup{
		makeGroup("G1", proj, testutil.NewTestTask("T1", models.TaskTypeTask)),
		makeGroup("G2", proj, testutil.NewTestTask("T2", models.TaskTypeTask)),
	}
	m.calculateNavigation()

	m.cursor = 2 // at G2 header (last group)
	m.goToNextGroup()

	// Should wrap to G1 header at position 0
	if m.cursor != 0 {
		t.Errorf("want cursor=0 (wrapped to G1), got %d", m.cursor)
	}
}

// ── projectCount ──────────────────────────────────────────────────────────────

func TestProjectCountEmpty(t *testing.T) {
	m := newMinimalAllTasksVM()
	if got := m.projectCount(); got != 0 {
		t.Errorf("want 0, got %d", got)
	}
}

func TestProjectCountMultiple(t *testing.T) {
	p1 := testutil.NewTestProject("proj-one")
	p2 := testutil.NewTestProject("proj-two")

	m := newMinimalAllTasksVM()
	m.projectCache[p1.ID] = p1
	m.projectCache[p2.ID] = p2

	if got := m.projectCount(); got != 2 {
		t.Errorf("want 2, got %d", got)
	}
}

// ── loadedWithCompleted tracking ─────────────────────────────────────────────

func TestLoadedWithCompletedTracked(t *testing.T) {
	m := newMinimalAllTasksVM()
	// Default tab is built-in (index 0), so IncludeCompleted should be false
	m.tabEntries = []TabEntry{
		{Name: "All", IsBuiltIn: true, BuiltInID: 0},
	}
	m.tabs.Names = []string{"All"}
	m.tabs.Active = 0

	// Simulate loadAllTasks behavior: loadedWithCompleted should match
	// the active tab's IncludeCompleted setting
	includeCompleted := m.activeTabIncludesCompleted()
	m.loadedWithCompleted = includeCompleted

	if m.loadedWithCompleted {
		t.Error("built-in tab should set loadedWithCompleted=false")
	}
}

func TestActiveTabIncludesCompletedOutOfBounds(t *testing.T) {
	m := newMinimalAllTasksVM()
	m.tabEntries = nil
	m.tabs.Active = 5 // out of bounds

	if m.activeTabIncludesCompleted() {
		t.Error("out-of-bounds tab should return false for IncludeCompleted")
	}
}

// TestFindNavigableIndexMissing verifies that findNavigableIndex returns -1
// for IDs that no longer exist in the visible groups, which is the input
// condition that reloadData must guard against by clamping the cursor.
func TestFindNavigableIndexMissing(t *testing.T) {
	proj := testutil.NewTestProject("kappa")
	t1 := testutil.NewTestTask("T1", models.TaskTypeTask)

	m := newMinimalAllTasksVM()
	m.groups = []AllTasksGroup{makeGroup("G", proj, t1)}
	m.calculateNavigation()

	if got := m.findNavigableIndex("does-not-exist"); got != -1 {
		t.Errorf("want -1 for missing id, got %d", got)
	}
}

// TestClampCursorAfterShrink verifies the fix for the cursor-lingering bug:
// after reloadData drops the task previously under the cursor (e.g. status
// transitioned it out of the visible set), the cursor must be clamped back
// into range rather than left pointing past the end of the navigable list.
func TestClampCursorAfterShrink(t *testing.T) {
	proj := testutil.NewTestProject("lambda")
	t1 := testutil.NewTestTask("T1", models.TaskTypeTask)
	t2 := testutil.NewTestTask("T2", models.TaskTypeTask)

	m := newMinimalAllTasksVM()
	m.groups = []AllTasksGroup{makeGroup("G", proj, t1, t2)}
	m.calculateNavigation()
	// layout: 0=header, 1=t1, 2=t2 — cursor sits on the last task
	m.cursor = 2

	// Simulate the list shrinking to a single task (t2 removed).
	m.groups = []AllTasksGroup{makeGroup("G", proj, t1)}
	m.calculateNavigation()
	m.clampCursor()

	// Total navigable items is now 2 (header + t1); cursor must be <= 1.
	if m.cursor >= m.totalNavigableItems() {
		t.Errorf("cursor %d not clamped to navigable range %d", m.cursor, m.totalNavigableItems())
	}
}

func TestAllTasksPersistentScrollPreservesWindowUntilCursorLeaves(t *testing.T) {
	proj := testutil.NewTestProject("scroll")
	tasks := make([]*models.Task, 24)
	items := make([]CrossProjectTask, 0, len(tasks))
	for i := range tasks {
		tasks[i] = testutil.NewTestTask("Task", models.TaskTypeTask)
		items = append(items, CrossProjectTask{Task: tasks[i], Project: proj})
	}

	m := newMinimalAllTasksVM()
	cfg := newTestConfig()
	m.PrimaryViewBundle = NewPrimaryViewBundle(nil, testRuntimeAt(cfg.DataDir(), cfg))
	m.FilterMixin = FilterMixin{filter: newFilterState()}
	m.projectFilter = make(map[string]bool)
	m.tabs = TabState{Names: []string{"All"}}
	m.tabEntries = []TabEntry{{Name: "All", IsBuiltIn: true}}
	m.allTabEntries = m.tabEntries
	m.allTasks = items
	m.groups = []AllTasksGroup{{Name: "Open", Tasks: items}}
	m.SetSize(80, 12)
	m.calculateNavigation()

	m.cursor = m.totalNavigableItems() - 1
	_ = m.View()
	bottomOffset := m.scrollOffset
	if bottomOffset == 0 {
		t.Fatal("expected bottom render to establish a non-zero scroll offset")
	}

	m.cursor--
	_ = m.View()
	if m.scrollOffset != bottomOffset {
		t.Fatalf("scroll offset changed while cursor stayed visible: got %d, want %d", m.scrollOffset, bottomOffset)
	}

	m.cursor = 0
	_ = m.View()
	if m.scrollOffset != 0 {
		t.Fatalf("scroll offset should clamp when cursor jumps above viewport: got %d", m.scrollOffset)
	}
}
