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

	a := &models.Task{ID: "a", Title: "A", Section: models.SectionNext, Status: models.StatusTodo}
	b := &models.Task{ID: "b", Title: "B", Section: models.SectionInbox, Status: models.StatusTodo}
	c := &models.Task{ID: "c", Title: "C", Section: models.SectionNext, Status: models.StatusTodo}
	d := &models.Task{ID: "d", Title: "D", Section: models.SectionInbox, Status: models.StatusTodo}

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
func TestOutlinerJ_CrossProject_StaysInsideSameProject(t *testing.T) {
	projA := &models.Project{ID: "proj-A", Name: "Alpha"}
	a1 := &models.Task{ID: "a1", Title: "A1", Section: models.SectionInbox, Status: models.StatusTodo}
	a2 := &models.Task{ID: "a2", Title: "A2", Section: models.SectionInbox, Status: models.StatusTodo}
	projA.Tasks = []*models.Task{a1, a2}
	projA.BuildTaskTree()

	projB := &models.Project{ID: "proj-B", Name: "Beta"}
	b1 := &models.Task{ID: "b1", Title: "B1", Section: models.SectionInbox, Status: models.StatusTodo}
	b2 := &models.Task{ID: "b2", Title: "B2", Section: models.SectionInbox, Status: models.StatusTodo}
	projB.Tasks = []*models.Task{b1, b2}
	projB.BuildTaskTree()

	m := minimalOutliner()
	m.project = nil // cross-project mode
	m.activePerspective = 1
	m.loadedProjects = []*models.Project{projA, projB}
	m.filterProjects = make(map[string]bool)
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

	// Force the cross rebuild path to refresh after mutation, but we won't
	// rely on its output — we assert directly against projA.Tasks.
	m.crossForceRebuild = true

	_, _, handled := m.handleTaskMutationKey("J", false)
	if !handled {
		t.Fatal("expected handled=true for J in cross-project mode")
	}

	// projA: SwapTasks(a1, a2) → [a2, a1]; projB untouched.
	assertIDsEqual(t, "projA.Tasks", projectTaskIDs(projA), []string{"a2", "a1"})
	assertIDsEqual(t, "projB.Tasks unchanged", projectTaskIDs(projB), []string{"b1", "b2"})
}

// TestOutlinerJ_CrossProject_LastInProjectIsNoOp: cursor on a2 (last
// projA row). visibleSiblings filtered by projA returns [a1, a2]; idx=1
// is the last, so J is a no-op. Must NOT swap with b1 (next visual row).
func TestOutlinerJ_CrossProject_LastInProjectIsNoOp(t *testing.T) {
	projA := &models.Project{ID: "proj-A"}
	a1 := &models.Task{ID: "a1", Section: models.SectionInbox, Status: models.StatusTodo}
	a2 := &models.Task{ID: "a2", Section: models.SectionInbox, Status: models.StatusTodo}
	projA.Tasks = []*models.Task{a1, a2}
	projA.BuildTaskTree()

	projB := &models.Project{ID: "proj-B"}
	b1 := &models.Task{ID: "b1", Section: models.SectionInbox, Status: models.StatusTodo}
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
	uc := &models.Task{ID: "uc", Title: "Uncompleted", Section: models.SectionInbox, Status: models.StatusTodo}
	cp := &models.Task{ID: "cp", Title: "Completed", Section: models.SectionInbox, Status: models.StatusDone}
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
	uc := &models.Task{ID: "uc", Title: "Uncompleted", Section: models.SectionInbox, Status: models.StatusTodo}
	cp := &models.Task{ID: "cp", Title: "Completed", Section: models.SectionInbox, Status: models.StatusDone}
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
	parent := &models.Task{ID: "p", Title: "P", Status: models.StatusTodo}
	c1 := &models.Task{ID: "c1", Title: "C1", Parent: parent, ParentID: &parent.ID, Status: models.StatusTodo}
	c2 := &models.Task{ID: "c2", Title: "C2", Parent: parent, ParentID: &parent.ID, Status: models.StatusTodo}
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

// buildCrossProjectDuplicateIDFixture sets up cross-project outliner with two
// projects, each holding a task with the same ID ("shared") plus a sibling
// ("aux"), all in the Inbox section. Both projects are loaded; m.project is
// nil; the perspective is "All". This proves cursor restoration after a
// cross-project J/K/mJ/mK must match by both ProjectID and TaskID.
func buildCrossProjectDuplicateIDFixture(t *testing.T) (m *OutlinerModel, projA, projB *models.Project) {
	t.Helper()

	projA = &models.Project{ID: "proj-A", Name: "Alpha"}
	aShared := &models.Task{ID: "shared", Title: "A-shared", Section: models.SectionInbox, Status: models.StatusTodo}
	aAux := &models.Task{ID: "aux", Title: "A-aux", Section: models.SectionInbox, Status: models.StatusTodo}
	projA.Tasks = []*models.Task{aShared, aAux}
	projA.BuildTaskTree()

	projB = &models.Project{ID: "proj-B", Name: "Beta"}
	bShared := &models.Task{ID: "shared", Title: "B-shared", Section: models.SectionInbox, Status: models.StatusTodo}
	bAux := &models.Task{ID: "aux", Title: "B-aux", Section: models.SectionInbox, Status: models.StatusTodo}
	projB.Tasks = []*models.Task{bShared, bAux}
	projB.BuildTaskTree()

	m = minimalOutliner()
	m.project = nil // cross-project mode
	m.activePerspective = PerspectiveAll
	m.loadedProjects = []*models.Project{projA, projB}
	m.filterProjects = make(map[string]bool)
	m.filter = NewFilterState()
	// Mimic the post-rebuild cross-project shape. The exact row count and
	// ordering here only need to be consistent with what
	// visibleSiblings()/handleTaskMutationKey see at the moment of the call;
	// rebuildRows() rewrites m.rows after the swap.
	m.rows = []outlinerRow{
		{IsSectionHeader: true, SectionName: models.SectionInbox},
		{IsProjectHeader: true, ProjectID: projA.ID},
		{task: aShared, ProjectID: projA.ID},
		{task: aAux, ProjectID: projA.ID},
		{IsProjectHeader: true, ProjectID: projB.ID},
		{task: bShared, ProjectID: projB.ID},
		{task: bAux, ProjectID: projB.ID},
	}
	m.crossForceRebuild = true
	return m, projA, projB
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

func assertCursorOnProjectTask(t *testing.T, m *OutlinerModel, projectID, taskID string) {
	t.Helper()
	wantCursor := findTaskRowInProject(m, projectID, taskID)
	if wantCursor < 0 {
		t.Fatalf("could not locate project=%q task=%q row after rebuild", projectID, taskID)
	}
	if m.cursor != wantCursor {
		gotPID := ""
		gotTID := ""
		if m.cursor >= 0 && m.cursor < len(m.rows) {
			gotPID = m.rows[m.cursor].ProjectID
			if m.rows[m.cursor].task != nil {
				gotTID = m.rows[m.cursor].task.ID
			}
		}
		t.Errorf("cursor = %d (project=%q task=%q), want %d (project=%q task=%q)",
			m.cursor, gotPID, gotTID, wantCursor, projectID, taskID)
	}
}

func buildCrossProjectNestedDuplicateIDFixture(t *testing.T) (m *OutlinerModel, projA, projB *models.Project) {
	t.Helper()

	newProject := func(projectID, name string) *models.Project {
		epic := &models.Task{
			ID:      "epic",
			Title:   name + " epic",
			Type:    models.TaskTypeEpic,
			Section: models.SectionInbox,
			Status:  models.StatusTodo,
		}
		story := &models.Task{
			ID:       "story",
			Title:    name + " story",
			Type:     models.TaskTypeStory,
			Section:  models.SectionInbox,
			Status:   models.StatusTodo,
			ParentID: &epic.ID,
		}
		shared := &models.Task{
			ID:       "shared",
			Title:    name + " shared child",
			Type:     models.TaskTypeTask,
			Section:  models.SectionInbox,
			Status:   models.StatusTodo,
			ParentID: &story.ID,
		}
		aux := &models.Task{
			ID:       "aux",
			Title:    name + " aux child",
			Type:     models.TaskTypeBug,
			Section:  models.SectionInbox,
			Status:   models.StatusTodo,
			ParentID: &story.ID,
		}
		project := &models.Project{
			ID:    projectID,
			Name:  name,
			Tasks: []*models.Task{epic, story, shared, aux},
		}
		project.BuildTaskTree()
		return project
	}

	projA = newProject("proj-A", "Alpha")
	projB = newProject("proj-B", "Beta")

	m = minimalOutliner()
	m.project = nil
	m.activePerspective = PerspectiveAll
	m.loadedProjects = []*models.Project{projA, projB}
	m.filterProjects = make(map[string]bool)
	m.filter = NewFilterState()
	m.rebuildRows()
	return m, projA, projB
}

// TestOutlinerJ_CrossProjectCursorTracking_DuplicateTaskID: cursor on projB's
// "shared". Pressing J swaps projB's [shared, aux] → [aux, shared]. After the
// rebuild, the cursor must follow projB's "shared" — not jump to projA's
// "shared", which also matches a bare-ID lookup.
func TestOutlinerJ_CrossProjectCursorTracking_DuplicateTaskID(t *testing.T) {
	m, projA, projB := buildCrossProjectDuplicateIDFixture(t)
	m.cursor = 5 // row for projB's "shared"

	_, _, handled := m.handleTaskMutationKey("J", false)
	if !handled {
		t.Fatal("expected handled=true for cross-project J")
	}

	assertIDsEqual(t, "projA.Tasks unchanged", projectTaskIDs(projA), []string{"shared", "aux"})
	assertIDsEqual(t, "projB.Tasks after J", projectTaskIDs(projB), []string{"aux", "shared"})

	wantCursor := findTaskRowInProject(m, projB.ID, "shared")
	if wantCursor < 0 {
		t.Fatal("could not locate projB's shared row after rebuild")
	}
	if m.cursor != wantCursor {
		gotPID := ""
		gotTID := ""
		if m.cursor >= 0 && m.cursor < len(m.rows) {
			gotPID = m.rows[m.cursor].ProjectID
			if m.rows[m.cursor].task != nil {
				gotTID = m.rows[m.cursor].task.ID
			}
		}
		t.Errorf("cursor = %d (project=%q task=%q), want %d (project=%q task=%q) — cursor must restore by (ProjectID, TaskID), not by TaskID alone",
			m.cursor, gotPID, gotTID, wantCursor, projB.ID, "shared")
	}
}

func TestOutlinerJ_CrossProjectNestedChildReordersAfterRebuild(t *testing.T) {
	m, projA, projB := buildCrossProjectNestedDuplicateIDFixture(t)
	m.cursor = findTaskRowInProject(m, projB.ID, "shared")
	if m.cursor < 0 {
		t.Fatal("setup: could not locate projB shared child row")
	}

	_, _, handled := m.handleTaskMutationKey("J", false)
	if !handled {
		t.Fatal("expected handled=true for cross-project nested J")
	}

	assertIDsEqual(t, "projA story.Children unchanged", taskChildIDs(projA.GetTaskByID("story")), []string{"shared", "aux"})
	assertIDsEqual(t, "projB story.Children after J", taskChildIDs(projB.GetTaskByID("story")), []string{"aux", "shared"})
	assertCursorOnProjectTask(t, m, projB.ID, "shared")
}

func TestOutlinerK_CrossProjectNestedChildReordersAfterRebuild(t *testing.T) {
	m, projA, projB := buildCrossProjectNestedDuplicateIDFixture(t)
	m.cursor = findTaskRowInProject(m, projB.ID, "aux")
	if m.cursor < 0 {
		t.Fatal("setup: could not locate projB aux child row")
	}

	_, _, handled := m.handleTaskMutationKey("K", false)
	if !handled {
		t.Fatal("expected handled=true for cross-project nested K")
	}

	assertIDsEqual(t, "projA story.Children unchanged", taskChildIDs(projA.GetTaskByID("story")), []string{"shared", "aux"})
	assertIDsEqual(t, "projB story.Children after K", taskChildIDs(projB.GetTaskByID("story")), []string{"aux", "shared"})
	assertCursorOnProjectTask(t, m, projB.ID, "aux")
}

func TestOutlinerMJ_CrossProjectNestedChildReordersAfterRebuild(t *testing.T) {
	m, projA, projB := buildCrossProjectNestedDuplicateIDFixture(t)
	m.cursor = findTaskRowInProject(m, projB.ID, "shared")
	if m.cursor < 0 {
		t.Fatal("setup: could not locate projB shared child row")
	}

	_, _ = m.moveTaskToPosition(m.rows[m.cursor].task, false /* toBottom */)

	assertIDsEqual(t, "projA story.Children unchanged", taskChildIDs(projA.GetTaskByID("story")), []string{"shared", "aux"})
	assertIDsEqual(t, "projB story.Children after mJ", taskChildIDs(projB.GetTaskByID("story")), []string{"aux", "shared"})
	assertCursorOnProjectTask(t, m, projB.ID, "shared")
}

func TestOutlinerMK_CrossProjectNestedChildReordersAfterRebuild(t *testing.T) {
	m, projA, projB := buildCrossProjectNestedDuplicateIDFixture(t)
	m.cursor = findTaskRowInProject(m, projB.ID, "aux")
	if m.cursor < 0 {
		t.Fatal("setup: could not locate projB aux child row")
	}

	_, _ = m.moveTaskToPosition(m.rows[m.cursor].task, true /* toTop */)

	assertIDsEqual(t, "projA story.Children unchanged", taskChildIDs(projA.GetTaskByID("story")), []string{"shared", "aux"})
	assertIDsEqual(t, "projB story.Children after mK", taskChildIDs(projB.GetTaskByID("story")), []string{"aux", "shared"})
	assertCursorOnProjectTask(t, m, projB.ID, "aux")
}

// TestOutlinerK_CrossProjectCursorTracking_DuplicateTaskID: cursor on projB's
// "aux". Pressing K swaps projB's [shared, aux] → [aux, shared]. After
// rebuild, cursor must follow projB's "aux", not jump to projA's "aux".
func TestOutlinerK_CrossProjectCursorTracking_DuplicateTaskID(t *testing.T) {
	m, projA, projB := buildCrossProjectDuplicateIDFixture(t)
	m.cursor = 6 // row for projB's "aux"

	_, _, handled := m.handleTaskMutationKey("K", false)
	if !handled {
		t.Fatal("expected handled=true for cross-project K")
	}

	assertIDsEqual(t, "projA.Tasks unchanged", projectTaskIDs(projA), []string{"shared", "aux"})
	assertIDsEqual(t, "projB.Tasks after K", projectTaskIDs(projB), []string{"aux", "shared"})

	wantCursor := findTaskRowInProject(m, projB.ID, "aux")
	if wantCursor < 0 {
		t.Fatal("could not locate projB's aux row after rebuild")
	}
	if m.cursor != wantCursor {
		gotPID := ""
		gotTID := ""
		if m.cursor >= 0 && m.cursor < len(m.rows) {
			gotPID = m.rows[m.cursor].ProjectID
			if m.rows[m.cursor].task != nil {
				gotTID = m.rows[m.cursor].task.ID
			}
		}
		t.Errorf("cursor = %d (project=%q task=%q), want %d (project=%q task=%q) — cursor must restore by (ProjectID, TaskID), not by TaskID alone",
			m.cursor, gotPID, gotTID, wantCursor, projB.ID, "aux")
	}
}

// TestOutlinerMJ_CrossProjectCursorTracking_DuplicateTaskID: cursor on projB's
// "shared". mJ (moveTaskToPosition toBottom=true → falls through to
// crossMoveTaskToPosition) moves projB's "shared" past its visible siblings.
// Cursor must follow projB's "shared", not projA's "shared".
func TestOutlinerMJ_CrossProjectCursorTracking_DuplicateTaskID(t *testing.T) {
	m, projA, projB := buildCrossProjectDuplicateIDFixture(t)
	m.cursor = 5 // row for projB's "shared"

	bShared := projB.GetTaskByID("shared")
	if bShared == nil {
		t.Fatal("setup: projB missing shared")
	}
	_, _ = m.moveTaskToPosition(bShared, false /* toBottom */)

	assertIDsEqual(t, "projA.Tasks unchanged", projectTaskIDs(projA), []string{"shared", "aux"})
	assertIDsEqual(t, "projB.Tasks after mJ", projectTaskIDs(projB), []string{"aux", "shared"})

	wantCursor := findTaskRowInProject(m, projB.ID, "shared")
	if wantCursor < 0 {
		t.Fatal("could not locate projB's shared row after rebuild")
	}
	if m.cursor != wantCursor {
		gotPID := ""
		gotTID := ""
		if m.cursor >= 0 && m.cursor < len(m.rows) {
			gotPID = m.rows[m.cursor].ProjectID
			if m.rows[m.cursor].task != nil {
				gotTID = m.rows[m.cursor].task.ID
			}
		}
		t.Errorf("cursor = %d (project=%q task=%q), want %d (project=%q task=%q) — cursor must restore by (ProjectID, TaskID), not by TaskID alone",
			m.cursor, gotPID, gotTID, wantCursor, projB.ID, "shared")
	}
}

// TestOutlinerMK_CrossProjectCursorTracking_DuplicateTaskID: cursor on projB's
// "aux". mK (toTop=true) moves projB's "aux" past its visible siblings.
// Cursor must follow projB's "aux", not projA's "aux".
func TestOutlinerMK_CrossProjectCursorTracking_DuplicateTaskID(t *testing.T) {
	m, projA, projB := buildCrossProjectDuplicateIDFixture(t)
	m.cursor = 6 // row for projB's "aux"

	bAux := projB.GetTaskByID("aux")
	if bAux == nil {
		t.Fatal("setup: projB missing aux")
	}
	_, _ = m.moveTaskToPosition(bAux, true /* toTop */)

	assertIDsEqual(t, "projA.Tasks unchanged", projectTaskIDs(projA), []string{"shared", "aux"})
	assertIDsEqual(t, "projB.Tasks after mK", projectTaskIDs(projB), []string{"aux", "shared"})

	wantCursor := findTaskRowInProject(m, projB.ID, "aux")
	if wantCursor < 0 {
		t.Fatal("could not locate projB's aux row after rebuild")
	}
	if m.cursor != wantCursor {
		gotPID := ""
		gotTID := ""
		if m.cursor >= 0 && m.cursor < len(m.rows) {
			gotPID = m.rows[m.cursor].ProjectID
			if m.rows[m.cursor].task != nil {
				gotTID = m.rows[m.cursor].task.ID
			}
		}
		t.Errorf("cursor = %d (project=%q task=%q), want %d (project=%q task=%q) — cursor must restore by (ProjectID, TaskID), not by TaskID alone",
			m.cursor, gotPID, gotTID, wantCursor, projB.ID, "aux")
	}
}
