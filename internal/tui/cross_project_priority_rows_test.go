package tui

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

// priorityTestEnv wires a minimal OutlinerModel with the given projects loaded
// so buildCrossPriorityRows can resolve projectByID lookups.
func priorityTestEnv(projects ...*models.Project) *OutlinerModel {
	m := minimalOutliner()
	m.loadedProjects = projects
	return m
}

// makePriorityTask builds a task with optional due-date offset for grouping.
func makePriorityTask(id, title string, daysOffset *int) *models.Task {
	task := makeTask(id, title, models.SectionNext)
	task.Priority = 3
	if daysOffset != nil {
		due := time.Now().AddDate(0, 0, *daysOffset)
		task.DueDate = &due
	}
	return task
}

func TestBuildCrossPriorityRowsGroupOrdering(t *testing.T) {
	projA := &models.Project{ID: "proj-a", Name: "A"}
	projB := &models.Project{ID: "proj-b", Name: "B"}

	overdue := makePriorityTask("o1", "Overdue", intPtr(-2))
	today := makePriorityTask("t1", "Today", intPtr(0))
	thisWeek := makePriorityTask("w1", "This week", intPtr(3))
	later := makePriorityTask("l1", "Later", intPtr(30))
	backlog := makePriorityTask("b1", "Backlog", nil)
	blocked := makePriorityTask("k1", "Blocked", nil)
	blocked.Status = models.StatusBlocked

	projA.Tasks = []*models.Task{overdue, thisWeek, backlog}
	projB.Tasks = []*models.Task{today, later, blocked}

	m := priorityTestEnv(projA, projB)
	groups := []projectTasksGroup{
		{projectID: projA.ID, tasks: projA.Tasks},
		{projectID: projB.ID, tasks: projB.Tasks},
	}

	m.buildCrossPriorityRows(groups)

	var headers []string
	for _, row := range m.rows {
		if row.IsSectionHeader {
			headers = append(headers, row.SectionName)
		}
	}

	want := []string{
		priorityGroupHeaderLabel(PriorityGroupOverdue),
		priorityGroupHeaderLabel(PriorityGroupToday),
		priorityGroupHeaderLabel(PriorityGroupThisWeek),
		priorityGroupHeaderLabel(PriorityGroupLater),
		priorityGroupHeaderLabel(PriorityGroupBacklog),
		priorityGroupHeaderLabel(PriorityGroupWaitingOn),
	}
	if len(headers) != len(want) {
		t.Fatalf("expected %d headers, got %d: %v", len(want), len(headers), headers)
	}
	for i, w := range want {
		if headers[i] != w {
			t.Errorf("header[%d]: want %q, got %q", i, w, headers[i])
		}
	}
}

func TestBuildCrossPriorityRowsEmptyGroupsSkipped(t *testing.T) {
	proj := &models.Project{ID: "proj-1", Name: "Solo"}
	only := makePriorityTask("t1", "Only", intPtr(0))
	proj.Tasks = []*models.Task{only}

	m := priorityTestEnv(proj)
	m.buildCrossPriorityRows([]projectTasksGroup{{projectID: proj.ID, tasks: proj.Tasks}})

	headerCount := 0
	for _, row := range m.rows {
		if row.IsSectionHeader {
			headerCount++
			if row.SectionName != priorityGroupHeaderLabel(PriorityGroupToday) {
				t.Errorf("unexpected header %q", row.SectionName)
			}
		}
	}
	if headerCount != 1 {
		t.Errorf("expected 1 header for non-empty group, got %d", headerCount)
	}
}

func TestBuildCrossPriorityRowsBlockedCollapsedHidesTasks(t *testing.T) {
	proj := &models.Project{ID: "proj-1", Name: "Solo"}
	today := makePriorityTask("t1", "Today", intPtr(0))
	blocked := makePriorityTask("b1", "Blocked", nil)
	blocked.Status = models.StatusBlocked
	proj.Tasks = []*models.Task{today, blocked}

	m := priorityTestEnv(proj)
	m.crossPriorityCollapsedGroups = map[PriorityGroup]bool{
		PriorityGroupWaitingOn: true,
	}
	m.buildCrossPriorityRows([]projectTasksGroup{{projectID: proj.ID, tasks: proj.Tasks}})

	var blockedHeader *outlinerRow
	taskRows := 0
	headerCount := 0
	for i := range m.rows {
		row := &m.rows[i]
		switch {
		case row.IsSectionHeader:
			headerCount++
			if row.SectionName == priorityGroupHeaderLabel(PriorityGroupWaitingOn) {
				blockedHeader = row
			}
		case row.task != nil:
			taskRows++
		}
	}
	if blockedHeader == nil {
		t.Fatal("blocked/waiting header should remain visible when collapsed")
	}
	if blockedHeader.SectionCount != 1 {
		t.Errorf("blocked header count: want 1, got %d", blockedHeader.SectionCount)
	}
	if headerCount != 2 {
		t.Errorf("expected 2 headers (today + blocked), got %d", headerCount)
	}
	// Today task only — blocked task is hidden under the collapsed header.
	if taskRows != 1 {
		t.Errorf("expected 1 task row (today only), got %d", taskRows)
	}
}

func TestBuildCrossPriorityRowsHeaderCollapseKey(t *testing.T) {
	proj := &models.Project{ID: "proj-1", Name: "Solo"}
	today := makePriorityTask("t1", "Today", intPtr(0))
	proj.Tasks = []*models.Task{today}

	m := priorityTestEnv(proj)
	m.buildCrossPriorityRows([]projectTasksGroup{{projectID: proj.ID, tasks: proj.Tasks}})

	if len(m.rows) == 0 || !m.rows[0].IsSectionHeader {
		t.Fatal("expected first row to be section header")
	}
	header := m.rows[0]
	wantKey := crossPriorityCollapseKey(PriorityGroupToday)
	if header.CollapseKey != wantKey {
		t.Errorf("CollapseKey: want %q, got %q", wantKey, header.CollapseKey)
	}
	if g, ok := priorityGroupFromCollapseKey(header.CollapseKey); !ok || g != PriorityGroupToday {
		t.Errorf("priorityGroupFromCollapseKey(%q): want (Today, true), got (%v, %v)", header.CollapseKey, g, ok)
	}
}

func TestBuildCrossPriorityRowsExcludesCompletedAndSnoozed(t *testing.T) {
	proj := &models.Project{ID: "proj-1", Name: "Solo"}
	visible := makePriorityTask("v1", "Visible", intPtr(0))

	done := makePriorityTask("d1", "Done", intPtr(0))
	done.Status = models.StatusDone

	canceled := makePriorityTask("c1", "Canceled", intPtr(0))
	canceled.Status = models.StatusCanceled

	snoozed := makePriorityTask("s1", "Snoozed", intPtr(0))
	future := time.Now().AddDate(0, 0, 5)
	snoozed.SnoozedUntil = &future

	proj.Tasks = []*models.Task{visible, done, canceled, snoozed}

	m := priorityTestEnv(proj)
	m.buildCrossPriorityRows([]projectTasksGroup{{projectID: proj.ID, tasks: proj.Tasks}})

	var seen []string
	for _, row := range m.rows {
		if row.task != nil {
			seen = append(seen, row.task.ID)
		}
	}
	if len(seen) != 1 || seen[0] != "v1" {
		t.Errorf("expected only visible task v1, got %v", seen)
	}
}

func TestBuildCrossPriorityRowsParentContainerVisible(t *testing.T) {
	proj := &models.Project{ID: "proj-1", Name: "Solo"}

	parent := makeTask("p1", "Parent epic", models.SectionNext)
	parent.Priority = 3
	parent.Type = models.TaskTypeEpic

	child := makePriorityTask("c1", "Child", intPtr(0))
	parentID := parent.ID
	child.ParentID = &parentID

	parent.Children = []*models.Task{child}
	proj.Tasks = []*models.Task{parent, child}

	m := priorityTestEnv(proj)
	m.buildCrossPriorityRows([]projectTasksGroup{{projectID: proj.ID, tasks: []*models.Task{parent}}})

	seen := make(map[string]bool)
	for _, row := range m.rows {
		if row.task != nil {
			seen[row.task.ID] = true
		}
	}
	if !seen["p1"] {
		t.Error("parent/container task should remain visible in priority rows")
	}
	if !seen["c1"] {
		t.Error("child task should also appear")
	}
}

func TestBuildCrossPriorityRowsTaskRowProjectID(t *testing.T) {
	projA := &models.Project{ID: "proj-a", Name: "A"}
	projB := &models.Project{ID: "proj-b", Name: "B"}

	tA := makePriorityTask("ta", "A task", intPtr(0))
	tB := makePriorityTask("tb", "B task", intPtr(0))
	projA.Tasks = []*models.Task{tA}
	projB.Tasks = []*models.Task{tB}

	m := priorityTestEnv(projA, projB)
	m.buildCrossPriorityRows([]projectTasksGroup{
		{projectID: projA.ID, tasks: projA.Tasks},
		{projectID: projB.ID, tasks: projB.Tasks},
	})

	got := make(map[string]string)
	for _, row := range m.rows {
		if row.task != nil {
			got[row.task.ID] = row.ProjectID
		}
	}
	if got["ta"] != projA.ID {
		t.Errorf("ta ProjectID: want %q, got %q", projA.ID, got["ta"])
	}
	if got["tb"] != projB.ID {
		t.Errorf("tb ProjectID: want %q, got %q", projB.ID, got["tb"])
	}
}

func TestBuildCrossPriorityRowsStableSortAcrossProjects(t *testing.T) {
	// Five priority-3 tasks today across two projects with identical urgency
	// scores. Stable sort must preserve the order in which they enter the
	// bucket (project A's tasks before project B's, then by insertion order).
	projA := &models.Project{ID: "proj-a", Name: "A"}
	projB := &models.Project{ID: "proj-b", Name: "B"}

	a1 := makePriorityTask("a1", "A1", intPtr(0))
	a2 := makePriorityTask("a2", "A2", intPtr(0))
	b1 := makePriorityTask("b1", "B1", intPtr(0))
	b2 := makePriorityTask("b2", "B2", intPtr(0))
	b3 := makePriorityTask("b3", "B3", intPtr(0))

	projA.Tasks = []*models.Task{a1, a2}
	projB.Tasks = []*models.Task{b1, b2, b3}

	m := priorityTestEnv(projA, projB)
	m.buildCrossPriorityRows([]projectTasksGroup{
		{projectID: projA.ID, tasks: projA.Tasks},
		{projectID: projB.ID, tasks: projB.Tasks},
	})

	var order []string
	for _, row := range m.rows {
		if row.task != nil {
			order = append(order, row.task.ID)
		}
	}
	want := []string{"a1", "a2", "b1", "b2", "b3"}
	if len(order) != len(want) {
		t.Fatalf("len: want %d, got %d (%v)", len(want), len(order), order)
	}
	for i, w := range want {
		if order[i] != w {
			t.Errorf("position %d: want %q, got %q (full %v)", i, w, order[i], order)
		}
	}
}

func TestBuildCrossPriorityRowsBlockedClassification(t *testing.T) {
	// Tasks with due dates that would otherwise place them in Today/Overdue
	// must be reclassified to Blocked/Waiting when blocked.
	proj := &models.Project{ID: "proj-1", Name: "Solo"}
	blockedToday := makePriorityTask("bt", "Blocked today", intPtr(0))
	blockedToday.Status = models.StatusBlocked
	proj.Tasks = []*models.Task{blockedToday}

	m := priorityTestEnv(proj)
	m.buildCrossPriorityRows([]projectTasksGroup{{projectID: proj.ID, tasks: proj.Tasks}})

	var headerNames []string
	for _, row := range m.rows {
		if row.IsSectionHeader {
			headerNames = append(headerNames, row.SectionName)
		}
	}
	if len(headerNames) != 1 || headerNames[0] != priorityGroupHeaderLabel(PriorityGroupWaitingOn) {
		t.Errorf("blocked task should land under Blocked/Waiting only, got headers %v", headerNames)
	}
}

func TestBuildCrossPriorityRowsSkipsUnknownProject(t *testing.T) {
	// projectByID returns nil for unknown project IDs; the builder must skip
	// that group rather than panic.
	known := &models.Project{ID: "proj-known", Name: "Known"}
	known.Tasks = []*models.Task{makePriorityTask("k1", "K1", intPtr(0))}

	m := priorityTestEnv(known)
	m.buildCrossPriorityRows([]projectTasksGroup{
		{projectID: "missing", tasks: []*models.Task{makePriorityTask("x1", "X1", intPtr(0))}},
		{projectID: known.ID, tasks: known.Tasks},
	})

	var ids []string
	for _, row := range m.rows {
		if row.task != nil {
			ids = append(ids, row.task.ID)
		}
	}
	if len(ids) != 1 || ids[0] != "k1" {
		t.Errorf("expected only k1 to appear, got %v", ids)
	}
}

func TestCrossProjectRebuildRowsPriorityModeIntegration(t *testing.T) {
	proj := &models.Project{ID: "proj-int", Name: "Int"}
	today := makePriorityTask("t1", "Today task", intPtr(0))
	done := makePriorityTask("d1", "Done task", intPtr(0))
	done.Status = models.StatusDone
	proj.Tasks = []*models.Task{today, done}
	proj.BuildTaskTree()

	m := &OutlinerModel{
		collapsedSections: make(map[string]bool),
		expandedNodes:     make(map[string]bool),
		MultiSelectState:  MultiSelectState{selectedTasks: make(map[string]bool)},
		activePerspective: PerspectiveAll,
		loadedProjects:    []*models.Project{proj},
	}
	m.enterCrossPriorityMode()
	m.crossProjectRebuildRows()

	var headers []string
	var taskIDs []string
	for _, row := range m.rows {
		if row.IsSectionHeader {
			headers = append(headers, row.SectionName)
		} else if row.task != nil {
			taskIDs = append(taskIDs, row.task.ID)
		}
	}

	if len(headers) != 1 || headers[0] != priorityGroupHeaderLabel(PriorityGroupToday) {
		t.Errorf("expected single Today header, got %v", headers)
	}
	if len(taskIDs) != 1 || taskIDs[0] != "t1" {
		t.Errorf("expected only t1 (done task excluded), got %v", taskIDs)
	}
}
