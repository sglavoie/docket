package tui

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

func TestTaskDecisionsSaveDecisionMutatesSynchronously(t *testing.T) {
	project, err := models.NewProject("project-1", "Decisions Project", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}

	task, err := models.NewTask("task-1", 1, models.TaskTypeTask, "Test task")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	project.Tasks = []*models.Task{task}

	model := NewTaskDecisionsModel(task, project)
	model.coordinator = newTestSecondaryCoordinators(project).Subview
	model.addMode = true
	model.categoryIndex = decisionCategoryIndex(models.DecisionCategoryArchitecture)
	model.textInput.SetValue("Use PostgreSQL")
	model.rationaleInput.SetValue("Better for complex queries")

	cmd := model.saveDecision()
	if cmd == nil {
		t.Fatal("expected non-nil command from saveDecision")
	}

	if model.addMode {
		t.Fatal("expected addMode to be false after saveDecision()")
	}
	if len(model.task.Decisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(model.task.Decisions))
	}
	if model.task.Decisions[0].Text != "Use PostgreSQL" {
		t.Fatalf("expected decision text %q, got %q", "Use PostgreSQL", model.task.Decisions[0].Text)
	}
	if model.task.Decisions[0].Rationale != "Better for complex queries" {
		t.Fatalf("expected rationale %q, got %q", "Better for complex queries", model.task.Decisions[0].Rationale)
	}
	if model.task.Decisions[0].Category != models.DecisionCategoryArchitecture {
		t.Fatalf("expected category %q, got %q", models.DecisionCategoryArchitecture, model.task.Decisions[0].Category)
	}
	if model.cursor != 0 {
		t.Fatalf("expected cursor at 0, got %d", model.cursor)
	}
	if model.err != nil {
		t.Fatalf("expected no error, got %v", model.err)
	}
}

func TestTaskDecisionsSaveDecisionEmptyText(t *testing.T) {
	project, err := models.NewProject("project-1", "Decisions Project", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}

	task, err := models.NewTask("task-1", 1, models.TaskTypeTask, "Test task")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	project.Tasks = []*models.Task{task}

	model := NewTaskDecisionsModel(task, project)
	model.coordinator = newTestSecondaryCoordinators(project).Subview
	model.addMode = true
	model.textInput.SetValue("   ")
	model.rationaleInput.SetValue("ignored")

	cmd := model.saveDecision()
	if cmd != nil {
		t.Fatal("expected nil command for empty decision")
	}
	if model.err == nil {
		t.Fatal("expected error for empty decision text")
	}
}

func TestTaskDecisionsDeleteMutatesSynchronously(t *testing.T) {
	project, err := models.NewProject("project-1", "Decisions Project", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}

	task, err := models.NewTask("task-1", 1, models.TaskTypeTask, "Test task")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	task.Decisions = []*models.Decision{
		{Text: "decision 1", Category: models.DecisionCategoryDesign, CreatedAt: time.Now()},
		{Text: "decision 2", Category: models.DecisionCategoryScope, CreatedAt: time.Now()},
	}
	project.Tasks = []*models.Task{task}

	model := NewTaskDecisionsModel(task, project)
	model.coordinator = newTestSecondaryCoordinators(project).Subview
	model.cursor = 1

	cmd := model.executeDelete()
	if cmd == nil {
		t.Fatal("expected non-nil command from executeDelete")
	}

	if len(model.task.Decisions) != 1 {
		t.Fatalf("expected 1 decision after delete, got %d", len(model.task.Decisions))
	}
	if model.task.Decisions[0].Text != "decision 1" {
		t.Fatalf("expected remaining decision to be %q, got %q", "decision 1", model.task.Decisions[0].Text)
	}
	if model.cursor != 0 {
		t.Fatalf("expected cursor at 0 after deleting last decision, got %d", model.cursor)
	}
}

func TestTaskDecisionsSaveEditMutatesSynchronously(t *testing.T) {
	project, err := models.NewProject("project-1", "Decisions Project", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}

	task, err := models.NewTask("task-1", 1, models.TaskTypeTask, "Test task")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	created := time.Date(2025, 1, 15, 10, 30, 0, 0, time.Local)
	task.Decisions = []*models.Decision{
		{
			Text:      "original text",
			Rationale: "original rationale",
			Category:  models.DecisionCategoryDesign,
			CreatedAt: created,
		},
	}
	project.Tasks = []*models.Task{task}

	model := NewTaskDecisionsModel(task, project)
	model.coordinator = newTestSecondaryCoordinators(project).Subview
	model.editMode = true
	model.editIndex = 0
	model.editCategoryIdx = decisionCategoryIndex(models.DecisionCategoryUX)
	model.editTextInput.SetValue("edited text")
	model.editRationale.SetValue("edited rationale")

	cmd := model.saveEditedDecision()
	if cmd == nil {
		t.Fatal("expected non-nil command from saveEditedDecision")
	}

	if model.editMode {
		t.Fatal("expected editMode to be false after saveEditedDecision()")
	}
	if model.task.Decisions[0].Text != "edited text" {
		t.Fatalf("expected decision text %q, got %q", "edited text", model.task.Decisions[0].Text)
	}
	if model.task.Decisions[0].Rationale != "edited rationale" {
		t.Fatalf("expected rationale %q, got %q", "edited rationale", model.task.Decisions[0].Rationale)
	}
	if model.task.Decisions[0].Category != models.DecisionCategoryUX {
		t.Fatalf("expected category %q, got %q", models.DecisionCategoryUX, model.task.Decisions[0].Category)
	}
	if !model.task.Decisions[0].CreatedAt.Equal(created) {
		t.Fatalf("expected timestamp to be preserved, got %v", model.task.Decisions[0].CreatedAt)
	}
	if model.err != nil {
		t.Fatalf("expected no error, got %v", model.err)
	}
}

func TestTaskDecisionsRenderEditModeOOB(t *testing.T) {
	project, err := models.NewProject("project-1", "Decisions Project", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}

	task, err := models.NewTask("task-1", 1, models.TaskTypeTask, "Test task")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	task.Decisions = []*models.Decision{
		{Text: "only decision", Category: models.DecisionCategoryDesign, CreatedAt: time.Now()},
	}
	project.Tasks = []*models.Task{task}

	model := NewTaskDecisionsModel(task, project)
	model.coordinator = newTestSecondaryCoordinators(project).Subview
	model.editMode = true
	model.editIndex = 0

	task.Decisions = nil

	view := model.View()
	if view.Content == "" {
		t.Fatal("expected non-empty view output")
	}
}

func TestTaskDecisionsEditExternalEditorFromEditModeOOB(t *testing.T) {
	project, err := models.NewProject("project-1", "Decisions Project", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}

	task, err := models.NewTask("task-1", 1, models.TaskTypeTask, "Test task")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	task.Decisions = []*models.Decision{
		{Text: "only decision", Category: models.DecisionCategoryDesign, CreatedAt: time.Now()},
	}
	project.Tasks = []*models.Task{task}

	model := NewTaskDecisionsModel(task, project)
	model.coordinator = newTestSecondaryCoordinators(project).Subview
	model.editMode = true
	model.editIndex = 0

	task.Decisions = nil

	cmd := model.editDecisionInExternalEditorFromEditMode()
	if cmd == nil {
		t.Fatal("expected non-nil error command when editIndex is out of bounds")
	}
	if model.editMode {
		t.Fatal("expected editMode to be canceled when decision was deleted externally")
	}
}

func TestTaskDecisionsSaveEditOOBAfterExternalDeletion(t *testing.T) {
	project, err := models.NewProject("project-1", "Decisions Project", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}

	task, err := models.NewTask("task-1", 1, models.TaskTypeTask, "Test task")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	task.Decisions = []*models.Decision{
		{Text: "original", Category: models.DecisionCategoryDesign, CreatedAt: time.Now()},
	}
	project.Tasks = []*models.Task{task}

	model := NewTaskDecisionsModel(task, project)
	model.coordinator = newTestSecondaryCoordinators(project).Subview
	model.editMode = true
	model.editIndex = 0
	model.editTextInput.SetValue("edited text")

	task.Decisions = nil

	cmd := model.saveEditedDecision()
	if cmd == nil {
		t.Fatal("expected non-nil error command when editIndex is out of bounds")
	}
	if model.editMode {
		t.Fatal("expected editMode to be canceled when decision was deleted externally")
	}
}

func TestTaskDecisionsToggleResolvedMutatesSynchronously(t *testing.T) {
	project, err := models.NewProject("project-1", "Decisions Project", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}

	task, err := models.NewTask("task-1", 1, models.TaskTypeTask, "Test task")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	task.Decisions = []*models.Decision{
		{Text: "decision", Category: models.DecisionCategoryDesign, CreatedAt: time.Now()},
	}
	project.Tasks = []*models.Task{task}

	model := NewTaskDecisionsModel(task, project)
	model.coordinator = newTestSecondaryCoordinators(project).Subview
	model.cursor = 0

	cmd := model.toggleResolved()
	if cmd == nil {
		t.Fatal("expected non-nil command from toggleResolved")
	}

	if !model.task.Decisions[0].Resolved {
		t.Fatal("expected decision to be resolved after toggleResolved()")
	}
}
