package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

// buildJKFixture constructs a single-project outliner where p.Tasks order
// differs from bucketed display order. Storage order is A, B, C, D; visible
// order under the "All" perspective is Inbox: B, D then Next: A, C.
func buildJKFixture(t *testing.T) (*OutlinerModel, *models.Project, map[string]*models.Task) {
	t.Helper()

	a := &models.Task{ID: "a", Title: "A", WorkflowState: models.StateNext}
	b := &models.Task{ID: "b", Title: "B", WorkflowState: models.StateInbox}
	c := &models.Task{ID: "c", Title: "C", WorkflowState: models.StateNext}
	d := &models.Task{ID: "d", Title: "D", WorkflowState: models.StateInbox}

	proj := &models.Project{ID: "proj", Name: "Test", Tasks: []*models.Task{a, b, c, d}}
	proj.BuildTaskTree()

	m := minimalOutliner()
	m.project = proj
	m.activePerspective = 1 // "All" — keep all non-completed, non-snoozed tasks

	// Mimic post-rebuildRows state: section-bucketed.
	m.rows = []outlinerRow{
		{IsSectionHeader: true, SectionName: models.SectionInbox},
		{task: b},
		{task: d},
		{IsSeparator: true},
		{IsSectionHeader: true, SectionName: models.SectionNext},
		{task: a},
		{task: c},
	}

	return m, proj, map[string]*models.Task{"a": a, "b": b, "c": c, "d": d}
}

// projectTaskIDs returns the ID sequence of p.Tasks for stable assertions.
func projectTaskIDs(p *models.Project) []string {
	ids := make([]string, 0, len(p.Tasks))
	for _, t := range p.Tasks {
		ids = append(ids, t.ID)
	}
	return ids
}

func assertIDsEqual(t *testing.T, label string, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("%s: len = %d, want %d (got %v, want %v)", label, len(got), len(want), got, want)
		return
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("%s[%d]: got %q, want %q (full got %v, want %v)", label, i, got[i], want[i], got, want)
		}
	}
}

// TestOutlinerJ_SwapsWithVisuallyAdjacentSibling: cursor on B (Inbox);
// pressing J must swap with D (the next root in the *same* visible Inbox
// bucket), not with C (the next root in raw p.Tasks order, which lives in
// the Next bucket).
func TestOutlinerJ_SwapsWithVisuallyAdjacentSibling(t *testing.T) {
	m, proj, _ := buildJKFixture(t)
	m.cursor = 1 // row for B

	_, _, handled := m.handleTaskMutationKey("J", false)
	if !handled {
		t.Fatal("expected handled=true for J")
	}

	// SwapTasks(b, d) swaps positions 1 and 3 in p.Tasks → [a, d, c, b].
	assertIDsEqual(t, "p.Tasks", projectTaskIDs(proj), []string{"a", "d", "c", "b"})
}

// TestOutlinerK_SwapsWithVisuallyAdjacentSibling: cursor on D (Inbox);
// pressing K must swap with B (previous root in the same Inbox bucket).
func TestOutlinerK_SwapsWithVisuallyAdjacentSibling(t *testing.T) {
	m, proj, _ := buildJKFixture(t)
	m.cursor = 2 // row for D

	_, _, handled := m.handleTaskMutationKey("K", false)
	if !handled {
		t.Fatal("expected handled=true for K")
	}

	// SwapTasks(d, b) swaps positions 3 and 1 → [a, d, c, b].
	assertIDsEqual(t, "p.Tasks", projectTaskIDs(proj), []string{"a", "d", "c", "b"})
}

// TestOutlinerJ_NoOpWhenLastVisibleRoot: cursor on C (the *last* visible
// root row in m.rows). visibleSiblings returns [B, D, A, C]; C is idx 3.
// J finds no next sibling and must be a no-op (no SwapTasks call).
func TestOutlinerJ_NoOpWhenLastVisibleRoot(t *testing.T) {
	m, proj, _ := buildJKFixture(t)
	m.cursor = 6 // last row — C

	before := projectTaskIDs(proj)
	_, _, handled := m.handleTaskMutationKey("J", false)
	if !handled {
		t.Fatal("expected handled=true (key consumed) for J at last-root boundary")
	}

	assertIDsEqual(t, "p.Tasks unchanged", projectTaskIDs(proj), before)
}

// TestOutlinerK_NoOpWhenFirstVisibleRoot: cursor on B (first visible root
// row). K must be a no-op.
func TestOutlinerK_NoOpWhenFirstVisibleRoot(t *testing.T) {
	m, proj, _ := buildJKFixture(t)
	m.cursor = 1 // first task row — B

	before := projectTaskIDs(proj)
	_, _, handled := m.handleTaskMutationKey("K", false)
	if !handled {
		t.Fatal("expected handled=true (key consumed) for K at first-root boundary")
	}

	assertIDsEqual(t, "p.Tasks unchanged", projectTaskIDs(proj), before)
}

// TestOutlinerJ_BucketBoundaryIsNoOp: cursor on D (last task in the Inbox
// bucket). With the contiguous-run visibleSiblings, D is the last entry in
// its bucket's sibling list, so J is a no-op — it must NOT cross the
// section header to reach A in the Next bucket.
func TestOutlinerJ_BucketBoundaryIsNoOp(t *testing.T) {
	m, proj, _ := buildJKFixture(t)
	m.cursor = 2 // row for D (last in Inbox)

	before := projectTaskIDs(proj)
	_, _, handled := m.handleTaskMutationKey("J", false)
	if !handled {
		t.Fatal("expected handled=true (key consumed) for J at bucket boundary")
	}

	assertIDsEqual(t, "p.Tasks unchanged across bucket boundary", projectTaskIDs(proj), before)
}

// TestOutlinerMJ_MovesToBottomOfVisibleSiblings: cursor on A (row 5, first
// root in the Next bucket). With contiguous-run visibleSiblings, the run is
// [A, C]; mJ swaps A past C only.
func TestOutlinerMJ_MovesToBottomOfVisibleSiblings(t *testing.T) {
	m, proj, tasks := buildJKFixture(t)
	m.cursor = 5 // row for A

	_, _ = m.moveTaskToPosition(tasks["a"], false /* toBottom */)

	// siblings = [A, C], A at idx 0. SwapTasks(A, C) swaps p.Tasks idx 0 (a) and idx 2 (c) → [c, b, a, d].
	assertIDsEqual(t, "p.Tasks after mJ on A", projectTaskIDs(proj), []string{"c", "b", "a", "d"})
}

// TestOutlinerMK_MovesToTopOfVisibleSiblings: cursor on C (row 6, last root
// in the Next bucket). With contiguous-run visibleSiblings, the run is
// [A, C]; mK swaps C past A only.
func TestOutlinerMK_MovesToTopOfVisibleSiblings(t *testing.T) {
	m, proj, tasks := buildJKFixture(t)
	m.cursor = 6 // row for C

	_, _ = m.moveTaskToPosition(tasks["c"], true /* toTop */)

	// siblings = [A, C], C at idx 1. SwapTasks(C, A) swaps p.Tasks idx 2 (c) and idx 0 (a) → [c, b, a, d].
	assertIDsEqual(t, "p.Tasks after mK on C", projectTaskIDs(proj), []string{"c", "b", "a", "d"})
}

// TestOutlinerJ_CrossProject_StaysInsideSameProject covers cross-project
// mode: with two projects A (a1, a2) and B (b1, b2) all in [Inbox],
// pressing J on a1 must swap with a2 (same project, same section), not
// with b1 (visually next row across projects).
// seedInboxTask builds an Inbox task with an explicit unique ID for the
// disk-backed cross-project structural fixtures. Real task IDs are UUIDs
// (globally unique), so the app-layer coordinator resolves the owning project by
// task ID without ambiguity.
func seedInboxTask(t *testing.T, id string, shortID int, title string) *models.Task {
	t.Helper()
	task, err := models.NewTask(id, shortID, models.TaskTypeTask, title)
	if err != nil {
		t.Fatalf("NewTask %q: %v", id, err)
	}
	task.SetSection(models.SectionInbox)
	return task
}

// TestOutlinerJ_CrossProject_StaysInsideSameProject: pressing J on projA's first
// root commits a MoveDown through the single-task coordinator, reordering projA on
// disk while projB is untouched.
func TestOutlinerJ_CrossProject_StaysInsideSameProject(t *testing.T) {
	a1 := seedInboxTask(t, "a1", 1, "A1")
	a2 := seedInboxTask(t, "a2", 2, "A2")
	b1 := seedInboxTask(t, "b1", 1, "B1")
	b2 := seedInboxTask(t, "b2", 2, "B2")
	env := newKanbanBulkEnv(t, []*models.Task{a1, a2}, []*models.Task{b1, b2})
	m := newCrossOutlinerFromEnv(env)
	projA, projB := env.projects[0], env.projects[1]
	m.rows = []outlinerRow{
		{IsSectionHeader: true, SectionName: models.SectionInbox, ProjectID: projA.ID},
		{task: a1, ProjectID: projA.ID},
		{task: a2, ProjectID: projA.ID},
		{IsSeparator: true},
		{IsSectionHeader: true, SectionName: models.SectionInbox, ProjectID: projB.ID},
		{task: b1, ProjectID: projB.ID},
		{task: b2, ProjectID: projB.ID},
	}
	m.cursor = 1 // row for a1

	_, cmd, handled := m.handleTaskMutationKey("J", false)
	if !handled {
		t.Fatal("expected handled=true for J in cross-project mode")
	}
	runBatchResult(t, cmd)

	assertIDsEqual(t, "projA.Tasks", projectTaskIDs(env.reload(t, 0)), []string{"a2", "a1"})
	assertIDsEqual(t, "projB.Tasks unchanged", projectTaskIDs(env.reload(t, 1)), []string{"b1", "b2"})
}

// TestOutlinerJ_CrossProject_LastInProjectIsNoOp: cursor on a2 (last
// projA row). visibleSiblings filtered by projA returns [a1, a2]; idx=1
// is the last, so J is a no-op. Must NOT swap with b1 (next visual row).
func TestOutlinerJ_CrossProject_LastInProjectIsNoOp(t *testing.T) {
	projA := &models.Project{ID: "proj-A"}
	a1 := &models.Task{ID: "a1", WorkflowState: models.StateInbox}
	a2 := &models.Task{ID: "a2", WorkflowState: models.StateInbox}
	projA.Tasks = []*models.Task{a1, a2}
	projA.BuildTaskTree()

	projB := &models.Project{ID: "proj-B"}
	b1 := &models.Task{ID: "b1", WorkflowState: models.StateInbox}
	projB.Tasks = []*models.Task{b1}
	projB.BuildTaskTree()

	m := minimalOutliner()
	m.activePerspective = 1
	m.loadedProjects = []*models.Project{projA, projB}
	m.filterProjects = make(map[string]bool)
	m.rows = []outlinerRow{
		{IsSectionHeader: true, SectionName: models.SectionInbox, ProjectID: projA.ID},
		{task: a1, ProjectID: projA.ID},
		{task: a2, ProjectID: projA.ID},
		{IsSectionHeader: true, SectionName: models.SectionInbox, ProjectID: projB.ID},
		{task: b1, ProjectID: projB.ID},
	}
	m.cursor = 2 // row for a2 (last in projA)

	before := projectTaskIDs(projA)
	_, _, handled := m.handleTaskMutationKey("J", false)
	if !handled {
		t.Fatal("expected handled=true for cross-project J at project boundary")
	}

	assertIDsEqual(t, "projA.Tasks unchanged", projectTaskIDs(projA), before)
	assertIDsEqual(t, "projB.Tasks unchanged", projectTaskIDs(projB), []string{"b1"})
}

// TestOutlinerJ_BlockedAtCompletionBoundary: with two roots in the same
// bucket where the second is completed, pressing J on the uncompleted task
// must be blocked (MoveBlockedMsg emitted, no swap).
func TestOutlinerJ_BlockedAtCompletionBoundary(t *testing.T) {
	uc := &models.Task{ID: "uc", Title: "Uncompleted", WorkflowState: models.StateInbox}
	cp := &models.Task{ID: "cp", Title: "Completed", WorkflowState: models.StateDone}
	proj := &models.Project{ID: "proj", Tasks: []*models.Task{uc, cp}}
	proj.BuildTaskTree()

	m := minimalOutliner()
	m.project = proj
	m.activePerspective = 1
	m.rows = []outlinerRow{
		{IsSectionHeader: true, SectionName: models.SectionInbox},
		{task: uc},
		{task: cp},
	}
	m.cursor = 1 // row for uncompleted

	before := projectTaskIDs(proj)
	_, cmd, handled := m.handleTaskMutationKey("J", false)
	if !handled {
		t.Fatal("expected handled=true for J at completion boundary")
	}
	if cmd == nil {
		t.Fatal("expected a MoveBlockedMsg command to be returned")
	}
	if msg, ok := cmd().(MoveBlockedMsg); !ok {
		t.Fatalf("expected MoveBlockedMsg, got %T", cmd())
	} else if msg.Reason == "" {
		t.Error("expected MoveBlockedMsg.Reason to be set")
	}
	assertIDsEqual(t, "p.Tasks unchanged at J completion boundary", projectTaskIDs(proj), before)
}

// TestOutlinerK_BlockedAtCompletionBoundary: with the completed task placed
// below the uncompleted task, pressing K on the completed task must be
// blocked.
func TestOutlinerK_BlockedAtCompletionBoundary(t *testing.T) {
	uc := &models.Task{ID: "uc", Title: "Uncompleted", WorkflowState: models.StateInbox}
	cp := &models.Task{ID: "cp", Title: "Completed", WorkflowState: models.StateDone}
	proj := &models.Project{ID: "proj", Tasks: []*models.Task{uc, cp}}
	proj.BuildTaskTree()

	m := minimalOutliner()
	m.project = proj
	m.activePerspective = 1
	m.rows = []outlinerRow{
		{IsSectionHeader: true, SectionName: models.SectionInbox},
		{task: uc},
		{task: cp},
	}
	m.cursor = 2 // row for completed

	before := projectTaskIDs(proj)
	_, cmd, handled := m.handleTaskMutationKey("K", false)
	if !handled {
		t.Fatal("expected handled=true for K at completion boundary")
	}
	if cmd == nil {
		t.Fatal("expected a MoveBlockedMsg command to be returned")
	}
	if msg, ok := cmd().(MoveBlockedMsg); !ok {
		t.Fatalf("expected MoveBlockedMsg, got %T", cmd())
	} else if msg.Reason == "" {
		t.Error("expected MoveBlockedMsg.Reason to be set")
	}
	assertIDsEqual(t, "p.Tasks unchanged at K completion boundary", projectTaskIDs(proj), before)
}

// TestOutlinerJ_OnChildUsesParentChildren ensures the child short-circuit
// path is wired up: J on a child swaps with the next entry in
// Parent.Children, not by walking m.rows.
func TestOutlinerJ_OnChildUsesParentChildren(t *testing.T) {
	parent := &models.Task{ID: "p", Title: "P", WorkflowState: models.StateReady}
	c1 := &models.Task{ID: "c1", Title: "C1", Parent: parent, ParentID: &parent.ID, WorkflowState: models.StateReady}
	c2 := &models.Task{ID: "c2", Title: "C2", Parent: parent, ParentID: &parent.ID, WorkflowState: models.StateReady}
	parent.Children = []*models.Task{c1, c2}

	proj := &models.Project{ID: "proj", Tasks: []*models.Task{parent, c1, c2}}
	proj.BuildTaskTree()

	m := minimalOutliner()
	m.project = proj
	m.activePerspective = 1
	m.rows = []outlinerRow{
		{task: parent},
		{task: c1},
		{task: c2},
	}
	m.cursor = 1 // row for c1

	_, _, handled := m.handleTaskMutationKey("J", false)
	if !handled {
		t.Fatal("expected handled=true for J on child")
	}

	// SwapTasks swaps positions of c1 (idx 1) and c2 (idx 2) in p.Tasks → [p, c2, c1].
	assertIDsEqual(t, "p.Tasks after J on child", projectTaskIDs(proj), []string{"p", "c2", "c1"})
}

// TestOutlinerMoveTaskToPosition_CursorFollowsTask: after moveTaskToPosition
// swaps siblings and rebuildRows() repopulates rows by section, the cursor
// must follow the moved task to its new row — not remain at the original
// index, which would now point at a different task.
func TestOutlinerMoveTaskToPosition_CursorFollowsTask(t *testing.T) {
	m, _, tasks := buildJKFixture(t)

	// Locate row index of A in the bucketed layout before the move.
	cursorBefore := -1
	for i, r := range m.rows {
		if r.task != nil && r.task.ID == "a" {
			cursorBefore = i
			break
		}
	}
	if cursorBefore < 0 {
		t.Fatal("could not locate row for A before move")
	}
	m.cursor = cursorBefore

	// Move A to the bottom of its visible-sibling run.
	_, _ = m.moveTaskToPosition(tasks["a"], false /* toBottom */)

	// Re-find row index of A after rebuildRows (real flow) so we don't
	// depend on the exact bucketing — just on the cursor-tracking invariant.
	wantCursor := -1
	for i, r := range m.rows {
		if r.task != nil && r.task.ID == "a" {
			wantCursor = i
			break
		}
	}
	if wantCursor < 0 {
		t.Fatal("could not locate row for A after move (rebuildRows lost it)")
	}
	if m.cursor != wantCursor {
		t.Errorf("cursor = %d, want %d (cursor must follow the moved task)", m.cursor, wantCursor)
	}
}

// findTaskRowInProject scans m.rows for a row whose task carries the given
// task ID and ProjectID. Returns -1 when not found.
func findTaskRowInProject(m *OutlinerModel, projectID, taskID string) int {
	for i, r := range m.rows {
		if r.task != nil && r.ProjectID == projectID && r.task.ID == taskID {
			return i
		}
	}
	return -1
}

func taskChildIDs(task *models.Task) []string {
	if task == nil {
		return nil
	}
	ids := make([]string, 0, len(task.Children))
	for _, child := range task.Children {
		ids = append(ids, child.ID)
	}
	return ids
}

// seedNestedInboxTasks builds an epic → story → [shared, aux] subtree with the
// given ID prefix, all in the Inbox section, for the disk-backed nested cross
// fixtures. Prefixed unique IDs mirror real UUIDs so the coordinator resolves the
// owning project by task ID.
func seedNestedInboxTasks(t *testing.T, prefix string) []*models.Task {
	t.Helper()
	mk := func(id string, shortID int, typ models.TaskType, parentID *string) *models.Task {
		task, err := models.NewTask(prefix+id, shortID, typ, prefix+" "+id)
		if err != nil {
			t.Fatalf("NewTask %q: %v", prefix+id, err)
		}
		task.SetSection(models.SectionInbox)
		task.ParentID = parentID
		return task
	}
	epic := mk("epic", 1, models.TaskTypeEpic, nil)
	story := mk("story", 2, models.TaskTypeStory, &epic.ID)
	shared := mk("shared", 3, models.TaskTypeTask, &story.ID)
	aux := mk("aux", 4, models.TaskTypeBug, &story.ID)
	return []*models.Task{epic, story, shared, aux}
}

// newCrossNestedEnv seeds two projects with an epic → story → [shared, aux]
// subtree each (prefixes "a"/"b"), wires the cross-project outliner to the
// coordinator, and lays flat rows referencing the seeded pointers so J/K/move
// handlers resolve visible siblings. Structural mutations commit through the
// coordinator; assertions reload the committed project.
func newCrossNestedEnv(t *testing.T) (*OutlinerModel, *kanbanBulkEnv) {
	t.Helper()
	env := newKanbanBulkEnv(t, seedNestedInboxTasks(t, "a"), seedNestedInboxTasks(t, "b"))
	m := newCrossOutlinerFromEnv(env)
	projA, projB := env.projects[0], env.projects[1]
	aTasks, bTasks := projA.Tasks, projB.Tasks
	m.rows = []outlinerRow{
		{IsSectionHeader: true, SectionName: models.SectionInbox, ProjectID: projA.ID},
		{task: aTasks[0], ProjectID: projA.ID}, // a epic
		{task: aTasks[1], ProjectID: projA.ID}, // a story
		{task: aTasks[2], ProjectID: projA.ID}, // a shared
		{task: aTasks[3], ProjectID: projA.ID}, // a aux
		{IsSeparator: true},
		{IsSectionHeader: true, SectionName: models.SectionInbox, ProjectID: projB.ID},
		{task: bTasks[0], ProjectID: projB.ID}, // b epic
		{task: bTasks[1], ProjectID: projB.ID}, // b story
		{task: bTasks[2], ProjectID: projB.ID}, // b shared
		{task: bTasks[3], ProjectID: projB.ID}, // b aux
	}
	return m, env
}

// TestOutlinerJ_CrossProjectNestedChildReorders: J on projB's "bshared" child
// commits a MoveDown through the coordinator, reordering projB's story children
// on disk to [baux, bshared] while projA is untouched.
func TestOutlinerJ_CrossProjectNestedChildReorders(t *testing.T) {
	m, env := newCrossNestedEnv(t)
	m.cursor = findTaskRowInProject(m, env.projects[1].ID, "bshared")
	if m.cursor < 0 {
		t.Fatal("setup: could not locate projB shared child row")
	}

	_, cmd, handled := m.handleTaskMutationKey("J", false)
	if !handled {
		t.Fatal("expected handled=true for cross-project nested J")
	}
	runBatchResult(t, cmd)

	assertIDsEqual(t, "projA story.Children unchanged", taskChildIDs(env.reload(t, 0).GetTaskByID("astory")), []string{"ashared", "aaux"})
	assertIDsEqual(t, "projB story.Children after J", taskChildIDs(env.reload(t, 1).GetTaskByID("bstory")), []string{"baux", "bshared"})
}

// TestOutlinerK_CrossProjectNestedChildReorders: K on projB's "baux" child
// commits a MoveUp, reordering projB's story children to [baux, bshared].
func TestOutlinerK_CrossProjectNestedChildReorders(t *testing.T) {
	m, env := newCrossNestedEnv(t)
	m.cursor = findTaskRowInProject(m, env.projects[1].ID, "baux")
	if m.cursor < 0 {
		t.Fatal("setup: could not locate projB aux child row")
	}

	_, cmd, handled := m.handleTaskMutationKey("K", false)
	if !handled {
		t.Fatal("expected handled=true for cross-project nested K")
	}
	runBatchResult(t, cmd)

	assertIDsEqual(t, "projA story.Children unchanged", taskChildIDs(env.reload(t, 0).GetTaskByID("astory")), []string{"ashared", "aaux"})
	assertIDsEqual(t, "projB story.Children after K", taskChildIDs(env.reload(t, 1).GetTaskByID("bstory")), []string{"baux", "bshared"})
}

// TestOutlinerMJ_CrossProjectNestedChildReorders: move-to-bottom on projB's
// "bshared" child commits a MoveToBottom, ordering the story children [baux,
// bshared].
func TestOutlinerMJ_CrossProjectNestedChildReorders(t *testing.T) {
	m, env := newCrossNestedEnv(t)
	m.cursor = findTaskRowInProject(m, env.projects[1].ID, "bshared")
	if m.cursor < 0 {
		t.Fatal("setup: could not locate projB shared child row")
	}

	_, cmd := m.moveTaskToPosition(m.rows[m.cursor].task, false /* toBottom */)
	runBatchResult(t, cmd)

	assertIDsEqual(t, "projA story.Children unchanged", taskChildIDs(env.reload(t, 0).GetTaskByID("astory")), []string{"ashared", "aaux"})
	assertIDsEqual(t, "projB story.Children after mJ", taskChildIDs(env.reload(t, 1).GetTaskByID("bstory")), []string{"baux", "bshared"})
}

// TestOutlinerMK_CrossProjectNestedChildReorders: move-to-top on projB's "baux"
// child commits a MoveToTop, ordering the story children [baux, bshared].
func TestOutlinerMK_CrossProjectNestedChildReorders(t *testing.T) {
	m, env := newCrossNestedEnv(t)
	m.cursor = findTaskRowInProject(m, env.projects[1].ID, "baux")
	if m.cursor < 0 {
		t.Fatal("setup: could not locate projB aux child row")
	}

	_, cmd := m.moveTaskToPosition(m.rows[m.cursor].task, true /* toTop */)
	runBatchResult(t, cmd)

	assertIDsEqual(t, "projA story.Children unchanged", taskChildIDs(env.reload(t, 0).GetTaskByID("astory")), []string{"ashared", "aaux"})
	assertIDsEqual(t, "projB story.Children after mK", taskChildIDs(env.reload(t, 1).GetTaskByID("bstory")), []string{"baux", "bshared"})
}

// TestOutlinerK_CrossProject_ReordersCorrectProject: K on projB's second root
// reorders projB (by unique task ID) while projA stays untouched.
func TestOutlinerK_CrossProject_ReordersCorrectProject(t *testing.T) {
	a1 := seedInboxTask(t, "a1", 1, "A1")
	a2 := seedInboxTask(t, "a2", 2, "A2")
	b1 := seedInboxTask(t, "b1", 1, "B1")
	b2 := seedInboxTask(t, "b2", 2, "B2")
	env := newKanbanBulkEnv(t, []*models.Task{a1, a2}, []*models.Task{b1, b2})
	m := newCrossOutlinerFromEnv(env)
	projA, projB := env.projects[0], env.projects[1]
	m.rows = []outlinerRow{
		{IsSectionHeader: true, SectionName: models.SectionInbox, ProjectID: projA.ID},
		{task: a1, ProjectID: projA.ID},
		{task: a2, ProjectID: projA.ID},
		{IsSeparator: true},
		{IsSectionHeader: true, SectionName: models.SectionInbox, ProjectID: projB.ID},
		{task: b1, ProjectID: projB.ID},
		{task: b2, ProjectID: projB.ID},
	}
	m.cursor = 6 // row for projB's b2

	_, cmd, handled := m.handleTaskMutationKey("K", false)
	if !handled {
		t.Fatal("expected handled=true for cross-project K")
	}
	runBatchResult(t, cmd)

	assertIDsEqual(t, "projA.Tasks unchanged", projectTaskIDs(env.reload(t, 0)), []string{"a1", "a2"})
	assertIDsEqual(t, "projB.Tasks after K", projectTaskIDs(env.reload(t, 1)), []string{"b2", "b1"})
}

// buildExpandedParentFixture builds a single-project outliner reproducing the
// `trail` layout: in the Next bucket a root task, an expanded Epic (also a
// root) with two children, then two more root tasks. The Epic's children sit
// between the roots as visible rows, so only bounded-window sibling discovery
// lets J/K and move-to-top/bottom treat the roots and Epic as mutual siblings.
// Storage order in p.Tasks is root1, epic, c1, c2, root2, root3.
func buildExpandedParentFixture(t *testing.T) (*OutlinerModel, *models.Project, map[string]*models.Task) {
	t.Helper()

	root1 := &models.Task{ID: "root1", Title: "Root 1", WorkflowState: models.StateNext}
	epic := &models.Task{ID: "epic", Title: "Epic", WorkflowState: models.StateNext}
	c1 := &models.Task{ID: "c1", Title: "Child 1", WorkflowState: models.StateNext, ParentID: &epic.ID}
	c2 := &models.Task{ID: "c2", Title: "Child 2", WorkflowState: models.StateNext, ParentID: &epic.ID}
	root2 := &models.Task{ID: "root2", Title: "Root 2", WorkflowState: models.StateNext}
	root3 := &models.Task{ID: "root3", Title: "Root 3", WorkflowState: models.StateNext}

	proj := &models.Project{ID: "proj", Name: "Test", Tasks: []*models.Task{root1, epic, c1, c2, root2, root3}}
	proj.BuildTaskTree()

	m := minimalOutliner()
	m.project = proj
	m.activePerspective = 1 // "All" — keep all non-completed, non-snoozed tasks

	// Expanded display: the Epic's children are visible rows interleaved
	// between the root siblings, exactly as they appear after rebuildRows.
	m.rows = []outlinerRow{
		{IsSectionHeader: true, SectionName: models.SectionNext}, // 0
		{task: root1}, // 1
		{task: epic},  // 2 (expanded)
		{task: c1},    // 3 (expanded descendant)
		{task: c2},    // 4 (expanded descendant)
		{task: root2}, // 5
		{task: root3}, // 6
	}

	return m, proj, map[string]*models.Task{
		"root1": root1, "epic": epic, "c1": c1, "c2": c2, "root2": root2, "root3": root3,
	}
}

// assertCursorOnTask re-finds the row for taskID after a rebuild and asserts the
// cursor landed on it (single-project variant of assertCursorOnProjectTask).
func assertCursorOnTask(t *testing.T, m *OutlinerModel, taskID string) {
	t.Helper()
	want := -1
	for i, r := range m.rows {
		if r.task != nil && r.task.ID == taskID {
			want = i
			break
		}
	}
	if want < 0 {
		t.Fatalf("could not locate row for task %q after rebuild", taskID)
	}
	if m.cursor != want {
		t.Errorf("cursor = %d, want %d (cursor must follow task %q across the rebuild)", m.cursor, want, taskID)
	}
}

// TestOutlinerK_MovesRootAboveExpandedParent: with the Epic expanded, its child
// rows sit between the Epic and root2. Pressing K on root2 must swap it with the
// Epic (the previous visible sibling), skipping the interleaved child rows — not
// no-op because a child row happens to sit directly above.
func TestOutlinerK_MovesRootAboveExpandedParent(t *testing.T) {
	m, proj, tasks := buildExpandedParentFixture(t)
	m.cursor = 5 // row for root2 (directly below the Epic's last child)

	_, _, handled := m.handleTaskMutationKey("K", false)
	if !handled {
		t.Fatal("expected handled=true for K")
	}

	// SwapTasks(root2, epic) swaps p.Tasks idx 1 (epic) and idx 4 (root2). The
	// children keep their flat positions; the tree stays correct via ParentID.
	assertIDsEqual(t, "p.Tasks after K on root2", projectTaskIDs(proj), []string{"root1", "root2", "c1", "c2", "epic", "root3"})
	assertIDsEqual(t, "epic.Children preserved", taskChildIDs(tasks["epic"]), []string{"c1", "c2"})
	assertCursorOnTask(t, m, "root2")
}

// TestOutlinerJ_MovesExpandedParentBelowRoot: pressing J on the expanded Epic
// must swap it with root2 (the next visible sibling), stepping over the Epic's
// own child rows rather than treating them as a hard stop.
func TestOutlinerJ_MovesExpandedParentBelowRoot(t *testing.T) {
	m, proj, tasks := buildExpandedParentFixture(t)
	m.cursor = 2 // row for the expanded Epic

	_, _, handled := m.handleTaskMutationKey("J", false)
	if !handled {
		t.Fatal("expected handled=true for J")
	}

	// SwapTasks(epic, root2) swaps p.Tasks idx 1 (epic) and idx 4 (root2).
	assertIDsEqual(t, "p.Tasks after J on epic", projectTaskIDs(proj), []string{"root1", "root2", "c1", "c2", "epic", "root3"})
	assertIDsEqual(t, "epic.Children preserved", taskChildIDs(tasks["epic"]), []string{"c1", "c2"})
	assertCursorOnTask(t, m, "epic")
}

// TestOutlinerMK_MovesToTopAcrossExpandedDescendants: mK on root3 (the last root)
// must walk it to the top of its visible siblings, swapping past root2 and the
// expanded Epic (whose child rows are skipped, not counted as siblings).
func TestOutlinerMK_MovesToTopAcrossExpandedDescendants(t *testing.T) {
	m, proj, tasks := buildExpandedParentFixture(t)
	m.cursor = 6 // row for root3

	_, _ = m.moveTaskToPosition(tasks["root3"], true /* toTop */)

	// siblings = [root1, epic, root2, root3]; root3 swaps past root2, epic, root1.
	assertIDsEqual(t, "p.Tasks after mK on root3", projectTaskIDs(proj), []string{"root3", "root1", "c1", "c2", "epic", "root2"})
	assertIDsEqual(t, "epic.Children preserved", taskChildIDs(tasks["epic"]), []string{"c1", "c2"})
	assertCursorOnTask(t, m, "root3")
}

// TestOutlinerMJ_MovesToBottomAcrossExpandedDescendants: mJ on the expanded Epic
// must walk it to the bottom of its visible siblings, past root2 and root3,
// carrying its children with it (via ParentID) while the child rows never act
// as boundaries.
func TestOutlinerMJ_MovesToBottomAcrossExpandedDescendants(t *testing.T) {
	m, proj, tasks := buildExpandedParentFixture(t)
	m.cursor = 2 // row for the expanded Epic

	_, _ = m.moveTaskToPosition(tasks["epic"], false /* toBottom */)

	// siblings = [root1, epic, root2, root3]; epic swaps past root2, then root3.
	assertIDsEqual(t, "p.Tasks after mJ on epic", projectTaskIDs(proj), []string{"root1", "root2", "c1", "c2", "root3", "epic"})
	assertIDsEqual(t, "epic.Children preserved", taskChildIDs(tasks["epic"]), []string{"c1", "c2"})
	assertCursorOnTask(t, m, "epic")
}
