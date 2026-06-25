package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/testutil"
)

func newFlowLoopFocusBoard(t *testing.T, statuses ...models.TaskStatus) (*FocusBoardModel, *models.Project, []*models.Task) {
	t.Helper()

	cfg := testutil.NewTestConfig()
	cfg.DataDir = t.TempDir()
	project := testutil.NewTestProject("Flow Loop")
	project.Path = cfg.GetProjectPath(project.Slug)
	cfg.Projects = []*models.Project{project}
	cfg.CurrentProject = project.ID

	tasks := make([]*models.Task, 0, len(statuses))
	for i, status := range statuses {
		task := cockpitTask(t, project, fmt.Sprintf("Flow task %d", i+1), status, models.SectionNext)
		tasks = append(tasks, task)
	}

	today := time.Now().Format("2006-01-02")
	dailyPlan := make([]FocusTask, 0, len(tasks))
	dailyPlanIDs := make(map[string]bool, len(tasks))
	planItems := make([]config.DailyPlanItem, 0, len(tasks))
	for _, task := range tasks {
		dailyPlan = append(dailyPlan, FocusTask{Task: task, Project: project})
		dailyPlanIDs[task.ID] = true
		planItems = append(planItems, config.DailyPlanItem{
			TaskID:      task.ID,
			ProjectID:   project.ID,
			PlannedDate: today,
		})
	}
	cfg.SetDailyPlan(planItems)

	if err := storage.SaveProject(project); err != nil {
		t.Fatalf("SaveProject: %v", err)
	}

	m := NewFocusBoardModel(cfg, nil, &dailyPlan, map[string]bool{}, dailyPlanIDs, map[string]bool{})
	m.tabs.Active = boardTabToday
	m.applyFilter()
	return m, project, tasks
}

func TestFocusBoardFlowLoopCompletionAdvancesToNextActionable(t *testing.T) {
	m, _, tasks := newFlowLoopFocusBoard(t, models.StatusTodo, models.StatusTodo)
	m.cursor = 0

	_, _ = m.markDone()

	if got := m.filtered[m.cursor].Task.ID; got != tasks[1].ID {
		t.Fatalf("cursor selected %q, want next actionable task %q", got, tasks[1].ID)
	}
	if !strings.Contains(m.statusMessage, "done") {
		t.Fatalf("statusMessage = %q, want completion status", m.statusMessage)
	}
	rec := m.computeNextActionRecommendation()
	if rec.Kind != RecommendationKindWorkOnTask || rec.Task == nil || rec.Task.ID != tasks[1].ID {
		t.Fatalf("recommendation after completion = %s/%v, want WorkOnTask for next task", rec.Kind, rec.Task)
	}
}

func TestFocusBoardFlowLoopDismissAdvancesToNextActionable(t *testing.T) {
	m, _, tasks := newFlowLoopFocusBoard(t, models.StatusTodo, models.StatusTodo)
	m.cursor = 0

	_, _ = m.dismiss()

	if got := m.filtered[m.cursor].Task.ID; got != tasks[1].ID {
		t.Fatalf("cursor selected %q, want next actionable task %q", got, tasks[1].ID)
	}
	if !strings.Contains(m.statusMessage, "canceled") {
		t.Fatalf("statusMessage = %q, want dismiss status", m.statusMessage)
	}
	rec := m.computeNextActionRecommendation()
	if rec.Kind != RecommendationKindWorkOnTask || rec.Task == nil || rec.Task.ID != tasks[1].ID {
		t.Fatalf("recommendation after dismiss = %s/%v, want WorkOnTask for next task", rec.Kind, rec.Task)
	}
}

func TestFocusBoardFlowLoopUnblockPreservesSelectedRowAndRefreshesRecommendation(t *testing.T) {
	m, _, tasks := newFlowLoopFocusBoard(t, models.StatusBlocked)
	m.cursor = 0

	before := m.computeNextActionRecommendation()
	if before.Kind != RecommendationKindReviewBlocked {
		t.Fatalf("precondition recommendation = %s, want ReviewBlocked", before.Kind)
	}

	_, _ = m.unblockSelectedTask()

	if got := m.filtered[m.cursor].Task.ID; got != tasks[0].ID {
		t.Fatalf("cursor selected %q, want unblocked task %q", got, tasks[0].ID)
	}
	if got := m.filtered[m.cursor].Task.Status; got != models.StatusTodo {
		t.Fatalf("unblocked task status = %s, want %s", got, models.StatusTodo)
	}
	after := m.computeNextActionRecommendation()
	if after.Kind != RecommendationKindWorkOnTask || after.Task == nil || after.Task.ID != tasks[0].ID {
		t.Fatalf("recommendation after unblock = %s/%v, want WorkOnTask for unblocked task", after.Kind, after.Task)
	}
}

func TestFocusBoardRefreshPreservesCursorByTaskIDAfterReorder(t *testing.T) {
	m, project, tasks := newFlowLoopFocusBoard(t, models.StatusTodo, models.StatusTodo)
	m.cursor = 1

	project.GetTaskByID(tasks[0].ID).ApplyTransition(models.StatusDone)
	if err := storage.SaveProject(project); err != nil {
		t.Fatalf("SaveProject: %v", err)
	}

	m.Update(FocusBoardRefreshMsg{})

	if got := m.filtered[m.cursor].Task.ID; got != tasks[1].ID {
		t.Fatalf("refresh selected %q, want preserved task %q", got, tasks[1].ID)
	}
}
