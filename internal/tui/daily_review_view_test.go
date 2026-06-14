package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

func newTestDailyReviewModel(cfg *config.Config) *DailyReviewModel {
	return &DailyReviewModel{
		ViewBundle:        ViewBundle{config: cfg},
		collapsedSections: make(map[int]bool),
		FilterMixin:       FilterMixin{filter: newFilterState()},
		tabs:              TabState{Names: dailyReviewTabNames},
	}
}

func newTestDailyReviewTask(t *testing.T, title string) *models.Task {
	t.Helper()
	task, err := models.NewTask(uuid.New().String(), 1, models.TaskTypeTask, title)
	if err != nil {
		t.Fatal(err)
	}
	return task
}

func TestDailyReview_EmptyState(t *testing.T) {
	cfg := &config.Config{Projects: []*models.Project{}}
	m := newTestDailyReviewModel(cfg)
	m.SetSize(80, 24)
	m.calculateNavigation()

	output := m.View().Content
	if output == "" {
		t.Error("expected non-empty output for empty daily review")
	}
	if !strings.Contains(output, "Review") {
		t.Error("expected output to contain 'Review' title")
	}
}

func TestDailyReview_SectionNavigation(t *testing.T) {
	cfg := &config.Config{Projects: []*models.Project{}}
	m := newTestDailyReviewModel(cfg)
	m.SetSize(80, 24)

	project, err := models.NewProject("p1", "Test", "")
	if err != nil {
		t.Fatal(err)
	}

	// Add overdue and due-today tasks (both on the "Due" tab, index 0)
	overdueTask := newTestDailyReviewTask(t, "Overdue task")
	past := time.Now().AddDate(0, 0, -5)
	overdueTask.DueDate = &past
	m.overdue = []CrossProjectTask{{Task: overdueTask, Project: project}}

	dueTodayTask := newTestDailyReviewTask(t, "Due today task")
	today := time.Now()
	dueTodayTask.DueDate = &today
	m.dueToday = []CrossProjectTask{{Task: dueTodayTask, Project: project}}

	m.calculateNavigation()

	// Default tab is "Due" (index 0), which shows overdue + dueToday sections
	// Each section has 1 header + 1 task = 2 items, total = 4
	if m.totalTasks != 4 {
		t.Errorf("expected 4 total navigable items, got %d", m.totalTasks)
	}

	if len(m.sectionOrder) != 2 {
		t.Errorf("expected 2 sections, got %d", len(m.sectionOrder))
	}
}

func TestDailyReview_ReviewCompletion(t *testing.T) {
	cfg := &config.Config{Projects: []*models.Project{}}
	m := newTestDailyReviewModel(cfg)
	m.SetSize(80, 24)

	m.reviewCompleted = true
	m.completedAt = time.Now()
	m.calculateNavigation()

	output := m.View().Content
	if !strings.Contains(output, "completed today at") {
		t.Error("expected output to contain completion message")
	}
}

func TestDailyReview_BuildTagCmdIncludesStandbyTasks(t *testing.T) {
	cfg := &config.Config{Projects: []*models.Project{}}
	m := newTestDailyReviewModel(cfg)

	project, err := models.NewProject("p1", "Test", "")
	if err != nil {
		t.Fatal(err)
	}

	// Create a task with a unique tag and put it only in inStandby
	standbyTask := newTestDailyReviewTask(t, "Standby task")
	standbyTask.Tags = []string{"standby-only-tag"}
	m.inStandby = []CrossProjectTask{{Task: standbyTask, Project: project}}

	// buildTagCmd collects tasks from all sections to build the tag picker.
	// Previously inStandby was omitted, so tags on standby tasks would be missing.
	cmd := m.buildTagCmd()
	if cmd == nil {
		t.Fatal("expected non-nil command from buildTagCmd")
	}

	// Execute the command and verify the tag appears in the message
	msg := cmd()
	tagMsg, ok := msg.(ShowTagFilterMsg)
	if !ok {
		t.Fatalf("expected ShowTagFilterMsg, got %T", msg)
	}
	found := false
	for _, tag := range tagMsg.Tags {
		if tag.Name == "standby-only-tag" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'standby-only-tag' from inStandby section to appear in tag picker; it was missing")
	}
}

func TestDailyReview_WithTasks_Render(t *testing.T) {
	cfg := &config.Config{Projects: []*models.Project{}}
	m := newTestDailyReviewModel(cfg)
	m.SetSize(120, 40)

	project, err := models.NewProject("p1", "Test Project", "")
	if err != nil {
		t.Fatal(err)
	}

	task := newTestDailyReviewTask(t, "Important task")
	task.Status = models.StatusInProgress
	m.inProgress = []CrossProjectTask{{Task: task, Project: project}}

	// Switch to Active tab (index 2) which shows in_progress and blocked
	m.tabs.Active = 2
	m.calculateNavigation()

	output := m.View().Content
	if !strings.Contains(output, "Important task") {
		t.Error("expected output to contain 'Important task'")
	}
}

func TestDailyReviewPersistentScrollPreservesWindowUntilCursorLeaves(t *testing.T) {
	cfg := &config.Config{Projects: []*models.Project{}}
	m := newTestDailyReviewModel(cfg)
	m.SetSize(80, 12)

	project, err := models.NewProject("p1", "Test Project", "")
	if err != nil {
		t.Fatal(err)
	}
	for range 24 {
		task := newTestDailyReviewTask(t, "Review task")
		task.Status = models.StatusInProgress
		m.inProgress = append(m.inProgress, CrossProjectTask{Task: task, Project: project})
	}

	m.tabs.Active = 2
	m.calculateNavigation()
	m.cursor = m.totalTasks - 1
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

func TestIsToday_CrossTimezone(t *testing.T) {
	now := time.Now()
	local := now.Location()

	// A time in UTC that represents the same instant as `now` is still "today"
	// in the local calendar, even when UTC and local fall on different dates.
	if !isToday(now.UTC()) {
		t.Errorf("expected isToday(now.UTC()) to be true; got false (now=%v utc=%v)", now, now.UTC())
	}

	// A time clearly in the past (~36h ago) must not be reported as today,
	// regardless of timezone shifts.
	past := now.Add(-36 * time.Hour).In(local)
	if isToday(past) {
		t.Errorf("expected isToday(36h ago) to be false; got true (past=%v)", past)
	}
}

func TestDailyReview_SortStability(t *testing.T) {
	cfg := &config.Config{Projects: []*models.Project{}}
	m := newTestDailyReviewModel(cfg)
	m.SetSize(80, 24)

	project, err := models.NewProject("p1", "Test", "")
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now()

	t.Run("byPriorityThenUpdatedAt_deterministic", func(t *testing.T) {
		// Two tasks with same priority and same UpdatedAt — should sort by ID.
		taskA := newTestDailyReviewTask(t, "Task A")
		taskA.Priority = 2
		taskA.UpdatedAt = now
		taskA.ID = "zzz-last"

		taskB := newTestDailyReviewTask(t, "Task B")
		taskB.Priority = 2
		taskB.UpdatedAt = now
		taskB.ID = "aaa-first"

		m.inProgress = []CrossProjectTask{
			{Task: taskA, Project: project},
			{Task: taskB, Project: project},
		}
		m.sortSections()

		// taskB (aaa-first) should come before taskA (zzz-last)
		if m.inProgress[0].Task.ID != "aaa-first" {
			t.Errorf("expected aaa-first first, got %s", m.inProgress[0].Task.ID)
		}
	})

	t.Run("byDueDateThenPriority_deterministic", func(t *testing.T) {
		due := now.AddDate(0, 0, 3)
		taskA := newTestDailyReviewTask(t, "Task A")
		taskA.Priority = 2
		taskA.DueDate = &due
		taskA.ID = "zzz-last"

		taskB := newTestDailyReviewTask(t, "Task B")
		taskB.Priority = 2
		taskB.DueDate = &due
		taskB.ID = "aaa-first"

		m.dueSoon = []CrossProjectTask{
			{Task: taskA, Project: project},
			{Task: taskB, Project: project},
		}
		m.sortSections()

		if m.dueSoon[0].Task.ID != "aaa-first" {
			t.Errorf("expected aaa-first first, got %s", m.dueSoon[0].Task.ID)
		}
	})

	t.Run("overdue_deterministic", func(t *testing.T) {
		due := now.AddDate(0, 0, -3)
		taskA := newTestDailyReviewTask(t, "Task A")
		taskA.DueDate = &due
		taskA.ID = "zzz-last"

		taskB := newTestDailyReviewTask(t, "Task B")
		taskB.DueDate = &due
		taskB.ID = "aaa-first"

		m.overdue = []CrossProjectTask{
			{Task: taskA, Project: project},
			{Task: taskB, Project: project},
		}
		m.sortSections()

		if m.overdue[0].Task.ID != "aaa-first" {
			t.Errorf("expected aaa-first first, got %s", m.overdue[0].Task.ID)
		}
	})

	t.Run("completed_deterministic", func(t *testing.T) {
		completedAt := now
		taskA := newTestDailyReviewTask(t, "Task A")
		taskA.CompletedAt = &completedAt
		taskA.ID = "zzz-last"

		taskB := newTestDailyReviewTask(t, "Task B")
		taskB.CompletedAt = &completedAt
		taskB.ID = "aaa-first"

		m.completed = []CrossProjectTask{
			{Task: taskA, Project: project},
			{Task: taskB, Project: project},
		}
		m.sortSections()

		if m.completed[0].Task.ID != "aaa-first" {
			t.Errorf("expected aaa-first first, got %s", m.completed[0].Task.ID)
		}
	})

	t.Run("old_deterministic", func(t *testing.T) {
		taskA := newTestDailyReviewTask(t, "Task A")
		taskA.UpdatedAt = now.AddDate(0, 0, -30)
		taskA.ID = "zzz-last"

		taskB := newTestDailyReviewTask(t, "Task B")
		taskB.UpdatedAt = now.AddDate(0, 0, -30)
		taskB.ID = "aaa-first"

		m.old = []CrossProjectTask{
			{Task: taskA, Project: project},
			{Task: taskB, Project: project},
		}
		m.sortSections()

		if m.old[0].Task.ID != "aaa-first" {
			t.Errorf("expected aaa-first first, got %s", m.old[0].Task.ID)
		}
	})
}
