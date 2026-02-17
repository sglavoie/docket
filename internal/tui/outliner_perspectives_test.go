package tui

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

func TestPerspectivePresetCount(t *testing.T) {
	presets := PerspectivePresets()
	if len(presets) != 9 {
		t.Fatalf("expected 9 presets, got %d", len(presets))
	}
	for i, p := range presets {
		if p.Key != i+1 {
			t.Errorf("preset %d: expected key %d, got %d", i, i+1, p.Key)
		}
		if p.Name == "" {
			t.Errorf("preset %d: empty name", i)
		}
	}
}

func TestPerspectiveByKey(t *testing.T) {
	for key := 1; key <= 9; key++ {
		p, ok := PerspectiveByKey(key)
		if !ok {
			t.Errorf("PerspectiveByKey(%d) returned false", key)
		}
		if p.Key != key {
			t.Errorf("PerspectiveByKey(%d).Key = %d", key, p.Key)
		}
	}

	for _, bad := range []int{0, -1, 10, 100} {
		_, ok := PerspectiveByKey(bad)
		if ok {
			t.Errorf("PerspectiveByKey(%d) should return false", bad)
		}
	}
}

func TestAllPerspective(t *testing.T) {
	p, _ := PerspectiveByKey(1)
	if p.Name != "All" {
		t.Fatalf("expected 'All', got %q", p.Name)
	}
	if p.Filter == nil {
		t.Fatal("All perspective should have a filter that excludes completed tasks")
	}
	// Should accept non-complete tasks
	if !p.Filter(&models.Task{Status: models.StatusTodo}) {
		t.Error("All should accept todo tasks")
	}
	if !p.Filter(&models.Task{Status: models.StatusInProgress}) {
		t.Error("All should accept in_progress tasks")
	}
	// Should reject completed tasks
	if p.Filter(&models.Task{Status: models.StatusDone}) {
		t.Error("All should reject done tasks")
	}
	if p.Filter(&models.Task{Status: models.StatusCanceled}) {
		t.Error("All should reject canceled tasks")
	}
}

func TestActivePerspective(t *testing.T) {
	p, _ := PerspectiveByKey(2)
	accept := &models.Task{Status: models.StatusInProgress}
	reject := []*models.Task{
		{Status: models.StatusTodo},
		{Status: models.StatusDone},
		{Status: models.StatusBlocked},
		{Status: models.StatusReview},
		{Status: models.StatusCanceled},
	}

	if !p.Filter(accept) {
		t.Error("Active should accept in_progress tasks")
	}
	for _, task := range reject {
		if p.Filter(task) {
			t.Errorf("Active should reject %s tasks", task.Status)
		}
	}
}

func TestNextPerspective(t *testing.T) {
	p, _ := PerspectiveByKey(3)

	// Accept: todo with priority 1 or 2
	for _, pri := range []int{1, 2} {
		task := &models.Task{Status: models.StatusTodo, Priority: pri}
		if !p.Filter(task) {
			t.Errorf("Next should accept todo priority %d", pri)
		}
	}

	// Reject: todo with priority 3-5
	for _, pri := range []int{3, 4, 5} {
		task := &models.Task{Status: models.StatusTodo, Priority: pri}
		if p.Filter(task) {
			t.Errorf("Next should reject todo priority %d", pri)
		}
	}

	// Reject: non-todo status even with high priority
	task := &models.Task{Status: models.StatusInProgress, Priority: 1}
	if p.Filter(task) {
		t.Error("Next should reject non-todo tasks")
	}
}

func TestInboxPerspective(t *testing.T) {
	p, _ := PerspectiveByKey(4)

	accept := &models.Task{Status: models.StatusTodo, Tags: nil}
	if !p.Filter(accept) {
		t.Error("Inbox should accept todo tasks with no tags")
	}

	acceptEmpty := &models.Task{Status: models.StatusTodo, Tags: []string{}}
	if !p.Filter(acceptEmpty) {
		t.Error("Inbox should accept todo tasks with empty tags slice")
	}

	rejectTags := &models.Task{Status: models.StatusTodo, Tags: []string{"work"}}
	if p.Filter(rejectTags) {
		t.Error("Inbox should reject todo tasks with tags")
	}

	rejectStatus := &models.Task{Status: models.StatusInProgress, Tags: nil}
	if p.Filter(rejectStatus) {
		t.Error("Inbox should reject non-todo tasks")
	}
}

func TestUrgentPerspective(t *testing.T) {
	p, _ := PerspectiveByKey(5)

	// Accept: priority 1 with active status
	for _, status := range []models.TaskStatus{models.StatusTodo, models.StatusInProgress} {
		task := &models.Task{Status: status, Priority: 1}
		if !p.Filter(task) {
			t.Errorf("Urgent should accept %s priority 1", status)
		}
	}

	// Accept: due within 3 days with active status
	soon := time.Now().Add(2 * 24 * time.Hour)
	task := &models.Task{Status: models.StatusTodo, Priority: 3, DueDate: &soon}
	if !p.Filter(task) {
		t.Error("Urgent should accept tasks due within 3 days")
	}

	// Accept: already past due
	past := time.Now().Add(-1 * time.Hour)
	taskPast := &models.Task{Status: models.StatusTodo, Priority: 3, DueDate: &past}
	if !p.Filter(taskPast) {
		t.Error("Urgent should accept overdue tasks")
	}

	// Reject: priority 3 with no due date
	taskLow := &models.Task{Status: models.StatusTodo, Priority: 3}
	if p.Filter(taskLow) {
		t.Error("Urgent should reject priority 3 tasks without due date")
	}

	// Reject: due far in the future, non-urgent priority
	far := time.Now().Add(30 * 24 * time.Hour)
	taskFar := &models.Task{Status: models.StatusTodo, Priority: 3, DueDate: &far}
	if p.Filter(taskFar) {
		t.Error("Urgent should reject tasks due far in future with low priority")
	}

	// Reject: wrong status even with priority 1
	taskDone := &models.Task{Status: models.StatusDone, Priority: 1}
	if p.Filter(taskDone) {
		t.Error("Urgent should reject done tasks")
	}
}

func TestWaitingPerspective(t *testing.T) {
	p, _ := PerspectiveByKey(6)

	accept := &models.Task{Status: models.StatusBlocked}
	if !p.Filter(accept) {
		t.Error("Waiting should accept blocked tasks")
	}

	for _, status := range []models.TaskStatus{
		models.StatusTodo, models.StatusInProgress, models.StatusDone, models.StatusReview,
	} {
		task := &models.Task{Status: status}
		if p.Filter(task) {
			t.Errorf("Waiting should reject %s tasks", status)
		}
	}
}

func TestSomedayPerspective(t *testing.T) {
	p, _ := PerspectiveByKey(7)

	for _, pri := range []int{4, 5} {
		task := &models.Task{Status: models.StatusTodo, Priority: pri}
		if !p.Filter(task) {
			t.Errorf("Someday should accept todo priority %d", pri)
		}
	}

	for _, pri := range []int{1, 2, 3} {
		task := &models.Task{Status: models.StatusTodo, Priority: pri}
		if p.Filter(task) {
			t.Errorf("Someday should reject todo priority %d", pri)
		}
	}

	task := &models.Task{Status: models.StatusInProgress, Priority: 5}
	if p.Filter(task) {
		t.Error("Someday should reject non-todo tasks")
	}
}

func TestCompletedPerspective(t *testing.T) {
	p, _ := PerspectiveByKey(8)

	accept := &models.Task{Status: models.StatusDone}
	if !p.Filter(accept) {
		t.Error("Completed should accept done tasks")
	}

	// Canceled is NOT done — this perspective is specifically StatusDone
	canceled := &models.Task{Status: models.StatusCanceled}
	if p.Filter(canceled) {
		t.Error("Completed should reject canceled tasks")
	}

	for _, status := range []models.TaskStatus{
		models.StatusTodo, models.StatusInProgress, models.StatusBlocked,
	} {
		task := &models.Task{Status: status}
		if p.Filter(task) {
			t.Errorf("Completed should reject %s tasks", status)
		}
	}
}

func TestReviewPerspective(t *testing.T) {
	p, _ := PerspectiveByKey(9)

	accept := &models.Task{Status: models.StatusReview}
	if !p.Filter(accept) {
		t.Error("Review should accept in_review tasks")
	}

	for _, status := range []models.TaskStatus{
		models.StatusTodo, models.StatusInProgress, models.StatusDone, models.StatusBlocked,
	} {
		task := &models.Task{Status: status}
		if p.Filter(task) {
			t.Errorf("Review should reject %s tasks", status)
		}
	}
}

// --- Integration tests: rebuildRows with perspective filtering ---

// buildTestProject creates a project with a tree of tasks for perspective testing.
// The tree structure:
//
//	epic (todo, priority 3, tags: ["backend"])
//	  ├── storyA (in_progress, priority 1)
//	  │   ├── taskA1 (todo, priority 1)
//	  │   └── taskA2 (done, priority 2)
//	  └── storyB (blocked, priority 4)
//	      └── taskB1 (in_review, priority 5)
//	bugRoot (todo, priority 2, no tags, no parent)
func buildTestProject() *models.Project {
	epic := &models.Task{
		ID:       "epic",
		ShortID:  1,
		Type:     models.TaskTypeEpic,
		Title:    "Epic",
		Status:   models.StatusTodo,
		Priority: 3,
		Tags:     []string{"backend"},
		Children: []*models.Task{},
	}
	storyA := &models.Task{
		ID:       "storyA",
		ShortID:  2,
		Type:     models.TaskTypeStory,
		Title:    "Story A",
		Status:   models.StatusInProgress,
		Priority: 1,
		Tags:     []string{"frontend"},
		Children: []*models.Task{},
		ParentID: &epic.ID,
	}
	taskA1 := &models.Task{
		ID:       "taskA1",
		ShortID:  3,
		Type:     models.TaskTypeTask,
		Title:    "Task A1",
		Status:   models.StatusTodo,
		Priority: 1,
		Tags:     []string{},
		Children: []*models.Task{},
		ParentID: &storyA.ID,
	}
	taskA2 := &models.Task{
		ID:       "taskA2",
		ShortID:  4,
		Type:     models.TaskTypeTask,
		Title:    "Task A2",
		Status:   models.StatusDone,
		Priority: 2,
		Tags:     []string{},
		Children: []*models.Task{},
		ParentID: &storyA.ID,
	}
	storyB := &models.Task{
		ID:       "storyB",
		ShortID:  5,
		Type:     models.TaskTypeStory,
		Title:    "Story B",
		Status:   models.StatusBlocked,
		Priority: 4,
		Tags:     []string{"ops"},
		Children: []*models.Task{},
		ParentID: &epic.ID,
	}
	taskB1 := &models.Task{
		ID:       "taskB1",
		ShortID:  6,
		Type:     models.TaskTypeTask,
		Title:    "Task B1",
		Status:   models.StatusReview,
		Priority: 5,
		Tags:     []string{},
		Children: []*models.Task{},
		ParentID: &storyB.ID,
	}
	bugRoot := &models.Task{
		ID:       "bugRoot",
		ShortID:  7,
		Type:     models.TaskTypeBug,
		Title:    "Bug Root",
		Status:   models.StatusTodo,
		Priority: 2,
		Tags:     nil,
		Children: []*models.Task{},
	}

	storyA.Children = []*models.Task{taskA1, taskA2}
	storyB.Children = []*models.Task{taskB1}
	epic.Children = []*models.Task{storyA, storyB}

	return &models.Project{
		Name:  "Test",
		Tasks: []*models.Task{epic, storyA, taskA1, taskA2, storyB, taskB1, bugRoot},
	}
}

func rowTitles(rows []outlinerRow) []string {
	titles := make([]string, len(rows))
	for i, r := range rows {
		if r.IsSectionHeader {
			// Use section name for headers, with "Active" for empty section
			displayName := r.SectionName
			if displayName == "" {
				displayName = "Active"
			}
			titles[i] = "[" + displayName + "]"
		} else {
			titles[i] = r.task.Title
		}
	}
	return titles
}

func assertRowTitles(t *testing.T, got []outlinerRow, want []string) {
	t.Helper()
	titles := rowTitles(got)
	if len(titles) != len(want) {
		t.Fatalf("expected %d rows %v, got %d rows %v", len(want), want, len(titles), titles)
	}
	for i := range want {
		if titles[i] != want[i] {
			t.Errorf("row %d: expected %q, got %q", i, want[i], titles[i])
		}
	}
}

func TestRebuildRows_AllPerspective(t *testing.T) {
	m := &OutlinerModel{project: buildTestProject(), activePerspective: 1, collapsedSections: make(map[string]bool)}
	m.rebuildRows()
	// Task A2 (done) is excluded by the All perspective filter
	assertRowTitles(t, m.rows, []string{
		"[Active]", "Epic", "Story A", "Task A1", "Story B", "Task B1", "Bug Root",
	})
}

func TestRebuildRows_ActivePerspective(t *testing.T) {
	m := &OutlinerModel{project: buildTestProject(), activePerspective: 2}
	m.rebuildRows()
	// Story A is in_progress. Epic is kept as ancestor.
	assertRowTitles(t, m.rows, []string{"Epic", "Story A"})
}

func TestRebuildRows_NextPerspective(t *testing.T) {
	m := &OutlinerModel{project: buildTestProject(), activePerspective: 3}
	m.rebuildRows()
	// Task A1 (todo, pri 1) and Bug Root (todo, pri 2) match.
	// Epic → Story A are kept as ancestors of Task A1.
	assertRowTitles(t, m.rows, []string{"Epic", "Story A", "Task A1", "Bug Root"})
}

func TestRebuildRows_InboxPerspective(t *testing.T) {
	m := &OutlinerModel{project: buildTestProject(), activePerspective: 4}
	m.rebuildRows()
	// Inbox: todo + no tags. Task A1 (todo, tags=[]) matches.
	// Bug Root (todo, tags=nil) matches.
	assertRowTitles(t, m.rows, []string{"Epic", "Story A", "Task A1", "Bug Root"})
}

func TestRebuildRows_UrgentPerspective(t *testing.T) {
	m := &OutlinerModel{project: buildTestProject(), activePerspective: 5}
	m.rebuildRows()
	// Task A1 (todo, pri 1) and Story A (in_progress, pri 1) match.
	assertRowTitles(t, m.rows, []string{"Epic", "Story A", "Task A1"})
}

func TestRebuildRows_WaitingPerspective(t *testing.T) {
	m := &OutlinerModel{project: buildTestProject(), activePerspective: 6}
	m.rebuildRows()
	// Story B is blocked. Epic is kept as ancestor.
	assertRowTitles(t, m.rows, []string{"Epic", "Story B"})
}

func TestRebuildRows_SomedayPerspective(t *testing.T) {
	m := &OutlinerModel{project: buildTestProject(), activePerspective: 7}
	m.rebuildRows()
	// No tasks match (todo + pri 4-5). Story B is pri 4 but blocked, not todo.
	assertRowTitles(t, m.rows, []string{})
}

func TestRebuildRows_CompletedPerspective(t *testing.T) {
	m := &OutlinerModel{project: buildTestProject(), activePerspective: 8}
	m.rebuildRows()
	// Task A2 is done. Epic → Story A are kept as ancestors.
	assertRowTitles(t, m.rows, []string{"Epic", "Story A", "Task A2"})
}

func TestRebuildRows_ReviewPerspective(t *testing.T) {
	m := &OutlinerModel{project: buildTestProject(), activePerspective: 9}
	m.rebuildRows()
	// Task B1 is in_review. Epic → Story B are kept as ancestors.
	assertRowTitles(t, m.rows, []string{"Epic", "Story B", "Task B1"})
}

func TestRebuildRows_ParentExcludedWhenNoDescendantsMatch(t *testing.T) {
	// Create a tree where a parent has children but none match the filter.
	parent := &models.Task{
		ID:       "parent",
		ShortID:  1,
		Type:     models.TaskTypeEpic,
		Title:    "Parent",
		Status:   models.StatusTodo,
		Priority: 3,
		Children: []*models.Task{},
	}
	child := &models.Task{
		ID:       "child",
		ShortID:  2,
		Type:     models.TaskTypeTask,
		Title:    "Child",
		Status:   models.StatusTodo,
		Priority: 3,
		Children: []*models.Task{},
		ParentID: &parent.ID,
	}
	parent.Children = []*models.Task{child}

	proj := &models.Project{
		Name:  "Test",
		Tasks: []*models.Task{parent, child},
	}

	// Active perspective: only in_progress. Neither parent nor child match.
	m := &OutlinerModel{project: proj, activePerspective: 2}
	m.rebuildRows()
	if len(m.rows) != 0 {
		t.Fatalf("expected 0 rows when no tasks match, got %d: %v", len(m.rows), rowTitles(m.rows))
	}
}

func TestRebuildRows_ParentPreservedForMatchingDescendant(t *testing.T) {
	// Parent doesn't match but grandchild does — both parent and child
	// should be preserved to maintain hierarchy context.
	grandparent := &models.Task{
		ID:       "gp",
		ShortID:  1,
		Type:     models.TaskTypeEpic,
		Title:    "Grandparent",
		Status:   models.StatusTodo,
		Priority: 3,
		Children: []*models.Task{},
	}
	mid := &models.Task{
		ID:       "mid",
		ShortID:  2,
		Type:     models.TaskTypeStory,
		Title:    "Middle",
		Status:   models.StatusTodo,
		Priority: 3,
		Children: []*models.Task{},
		ParentID: &grandparent.ID,
	}
	leaf := &models.Task{
		ID:       "leaf",
		ShortID:  3,
		Type:     models.TaskTypeTask,
		Title:    "Leaf",
		Status:   models.StatusDone,
		Priority: 3,
		Children: []*models.Task{},
		ParentID: &mid.ID,
	}
	mid.Children = []*models.Task{leaf}
	grandparent.Children = []*models.Task{mid}

	proj := &models.Project{
		Name:  "Test",
		Tasks: []*models.Task{grandparent, mid, leaf},
	}

	// Completed perspective: only done tasks match.
	m := &OutlinerModel{project: proj, activePerspective: 8}
	m.rebuildRows()
	assertRowTitles(t, m.rows, []string{"Grandparent", "Middle", "Leaf"})
}

func TestRebuildRows_DefaultPerspectiveIsAll(t *testing.T) {
	m := &OutlinerModel{project: buildTestProject(), activePerspective: 0, collapsedSections: make(map[string]bool)}
	m.rebuildRows()
	// activePerspective=0 defaults to "All" (key 1), showing non-complete tasks with section header.
	// 1 header + 6 non-done tasks (Task A2 is done and excluded)
	if len(m.rows) != 7 {
		t.Fatalf("default perspective should show 7 rows (1 header + 6 tasks), got %d", len(m.rows))
	}
	// First row should be the Active section header
	if !m.rows[0].IsSectionHeader {
		t.Errorf("first row should be section header")
	}
}

func TestRebuildRows_CursorClampedAfterFilter(t *testing.T) {
	m := &OutlinerModel{project: buildTestProject(), activePerspective: 1, collapsedSections: make(map[string]bool)}
	m.rebuildRows()
	m.cursor = 6 // last row in "All" perspective (header + 6 non-done tasks)

	// Switch to Someday (no matches) — cursor should clamp to 0.
	m.activePerspective = 7
	m.rebuildRows()
	if m.cursor != 0 {
		t.Errorf("cursor should clamp to 0 when rows empty, got %d", m.cursor)
	}
}
