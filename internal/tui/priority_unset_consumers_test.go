package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/testutil"
)

// weeklyReviewTaskAt builds a Ready task whose activity timestamps are old
// enough to reach every stale-driven weekly review section.
func weeklyReviewTaskAt(t *testing.T, id string, shortID, priority int, title string, updated time.Time) *models.Task {
	t.Helper()
	task, err := models.NewTask(id, shortID, models.TaskTypeTask, title)
	if err != nil {
		t.Fatal(err)
	}
	task.WorkflowState = models.StateReady
	task.Priority = priority
	task.CreatedAt = updated
	task.UpdatedAt = updated
	return task
}

func TestWeeklyReviewLoadDataExcludesUnsetPriorityFromHighPriority(t *testing.T) {
	mockRepo := testutil.NewMockRepository()
	oldRepo := storage.DefaultRepository
	storage.DefaultRepository = mockRepo
	defer func() { storage.DefaultRepository = oldRepo }()

	old := time.Now().AddDate(0, 0, -30)

	storedProject, err := models.NewProject("p1", "Priority Contract", "")
	if err != nil {
		t.Fatal(err)
	}
	storedProject.CreatedAt = old
	storedProject.Tasks = []*models.Task{
		weeklyReviewTaskAt(t, "unset-a", 1, models.PriorityUnset, "Unset A", old),
		weeklyReviewTaskAt(t, "unset-b", 2, models.PriorityUnset, "Unset B", old),
		weeklyReviewTaskAt(t, "p1", 3, 1, "Critical", old),
		weeklyReviewTaskAt(t, "p3", 4, models.PriorityDefault, "Medium", old),
	}
	mockRepo.Projects[storedProject.ID] = storedProject

	configProject := &models.Project{ID: storedProject.ID, Name: storedProject.Name}
	m := newTestWeeklyReviewModel(testConfigWithProjects(configProject))
	m.loadData()

	// One high-priority task out of four active tasks is 25%, below the 40%
	// drift threshold. Counting the two unset tasks as high would reach 75%.
	if m.priorityDriftPct != 0 || m.priorityDriftWarning != "" {
		t.Errorf("priority drift fired at %d%% (%q); unset priority must not count as P1/P2",
			m.priorityDriftPct, m.priorityDriftWarning)
	}

	if len(m.staleProjects) != 1 {
		t.Fatalf("staleProjects count = %d, want 1", len(m.staleProjects))
	}
	if got := m.staleProjects[0].HighPriorityTodo; got != 1 {
		t.Errorf("HighPriorityTodo = %d, want 1 (unset priority is not high priority)", got)
	}

	var mismatched []string
	for _, task := range m.priorityMismatch {
		mismatched = append(mismatched, task.Task.Title)
	}
	if len(mismatched) != 1 || mismatched[0] != "Critical" {
		t.Errorf("priorityMismatch = %v, want only the P1 task", mismatched)
	}
}

func TestWeeklyReviewSortOrdersUnsetPriorityAsDefault(t *testing.T) {
	due := time.Date(2026, 5, 30, 0, 0, 0, 0, time.UTC)
	project := &models.Project{ID: "p1", Name: "P"}

	mk := func(id string, priority int) WeeklyReviewTask {
		return WeeklyReviewTask{
			Task:    &models.Task{ID: id, Priority: priority, DueDate: &due},
			Project: project,
		}
	}

	m := newTestWeeklyReviewModel(newTestConfig())
	// IDs are chosen so the unset task only lands last when its priority
	// resolves to PriorityDefault and the tie breaks on ID.
	m.priorityMismatch = []WeeklyReviewTask{mk("z-unset", models.PriorityUnset), mk("a-p3", models.PriorityDefault), mk("m-p1", 1)}
	m.upcomingWeek = []WeeklyReviewTask{mk("z-unset", models.PriorityUnset), mk("a-p3", models.PriorityDefault), mk("m-p1", 1)}
	m.sortSections()

	want := []string{"m-p1", "a-p3", "z-unset"}
	assertWeeklyTaskOrder(t, "priorityMismatch", m.priorityMismatch, want)
	assertWeeklyTaskOrder(t, "upcomingWeek", m.upcomingWeek, want)
}

func assertWeeklyTaskOrder(t *testing.T, name string, tasks []WeeklyReviewTask, want []string) {
	t.Helper()
	got := make([]string, len(tasks))
	for i, task := range tasks {
		got[i] = task.Task.ID
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("%s order = %v, want %v", name, got, want)
	}
}

func TestDailyReviewSectionOrderingResolvesUnsetPriority(t *testing.T) {
	due := time.Date(2026, 5, 30, 0, 0, 0, 0, time.UTC)
	updated := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	project := &models.Project{ID: "p1", Name: "P"}

	mk := func(id string, priority int) CrossProjectTask {
		return CrossProjectTask{
			Task:    &models.Task{ID: id, Priority: priority, DueDate: &due, UpdatedAt: updated},
			Project: project,
		}
	}
	input := func() []CrossProjectTask {
		return []CrossProjectTask{mk("z-unset", models.PriorityUnset), mk("a-p3", models.PriorityDefault), mk("m-p1", 1)}
	}
	want := []string{"m-p1", "a-p3", "z-unset"}

	for name, less := range map[string]crossTaskLess{
		"byPriorityThenID":        byPriorityThenID,
		"byPriorityThenUpdatedAt": byPriorityThenUpdatedAt,
		"byDueDateThenPriority":   byDueDateThenPriority,
	} {
		tasks := input()
		sortDailyReviewSection(tasks, less)
		got := make([]string, len(tasks))
		for i, task := range tasks {
			got[i] = task.Task.ID
		}
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Errorf("%s order = %v, want %v", name, got, want)
		}
	}
}

func TestTaskJumperPreviewRendersUnsetPriorityWithoutWarning(t *testing.T) {
	project, err := models.NewProject("p1", "Jumper", "")
	if err != nil {
		t.Fatal(err)
	}
	for i, spec := range []struct {
		title    string
		priority int
	}{
		{"Unset", models.PriorityUnset},
		{"Critical", 1},
		{"Medium", models.PriorityDefault},
	} {
		task, err := models.NewTask(spec.title, i+1, models.TaskTypeTask, spec.title)
		if err != nil {
			t.Fatal(err)
		}
		task.Priority = spec.priority
		project.Tasks = append(project.Tasks, task)
	}

	m := NewTaskJumperModel(project)
	m.SetSize(120, 40)

	for i, want := range []string{
		NormalStyle.Render("unset"),
		WarningIconStyle.Render("1"),
		NormalStyle.Render("3"),
	} {
		m.cursor = i
		preview := m.renderPreview()
		if !strings.Contains(preview, MutedStyle.Render("Priority:")+" "+want) {
			t.Errorf("preview for %q missing priority %q:\n%q", project.Tasks[i].Title, want, preview)
		}
	}

	m.cursor = 0
	preview := m.renderPreview()
	if strings.Contains(preview, "Priority: 0") || strings.Contains(preview, WarningIconStyle.Render("unset")) {
		t.Errorf("unset priority must render as plain 'unset', got:\n%q", preview)
	}
}
