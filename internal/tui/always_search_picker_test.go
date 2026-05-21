package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

func TestProjectPickerUpdate_CtrlPNAndEscBehavior(t *testing.T) {
	projectA, err := models.NewProject("proj-a", "Alpha", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject A: %v", err)
	}
	projectB, err := models.NewProject("proj-b", "Beta", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject B: %v", err)
	}
	projectC, err := models.NewProject("proj-c", "Gamma", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject C: %v", err)
	}

	m := NewProjectPickerModel([]*models.Project{projectA, projectB, projectC})

	model, _ := m.Update(syntheticCtrlKey('n'))
	m = model.(*ProjectPickerModel)
	if m.Cursor != 1 {
		t.Fatalf("ctrl+n should move cursor to 1, got %d", m.Cursor)
	}

	model, _ = m.Update(syntheticCtrlKey('p'))
	m = model.(*ProjectPickerModel)
	if m.Cursor != 0 {
		t.Fatalf("ctrl+p should move cursor back to 0, got %d", m.Cursor)
	}

	model, _ = m.Update(syntheticKey('b'))
	m = model.(*ProjectPickerModel)
	if m.SearchQuery() != "b" {
		t.Fatalf("expected search query %q, got %q", "b", m.SearchQuery())
	}
	if len(m.Filtered) != 1 || m.Filtered[0].Project.ID != projectB.ID {
		t.Fatalf("expected only Beta to match, got %#v", m.Filtered)
	}

	model, cmd := m.Update(syntheticSpecialKey(tea.KeyEsc))
	m = model.(*ProjectPickerModel)
	if cmd != nil {
		t.Fatal("first esc with a query should clear search, not cancel")
	}
	if m.SearchQuery() != "" {
		t.Fatalf("expected cleared search query, got %q", m.SearchQuery())
	}
	if len(m.Filtered) != 3 {
		t.Fatalf("expected all projects after clearing search, got %d", len(m.Filtered))
	}

	_, cmd = m.Update(syntheticSpecialKey(tea.KeyEsc))
	if cmd == nil {
		t.Fatal("second esc should cancel the picker")
	}
	msg, ok := cmd().(ProjectPickerDoneMsg)
	if !ok {
		t.Fatalf("expected ProjectPickerDoneMsg, got %T", cmd())
	}
	if !msg.Canceled || msg.Project != nil {
		t.Fatalf("expected canceled project picker result, got %#v", msg)
	}
}

func TestProjectSelectorUpdate_ReturnsMoveMessage(t *testing.T) {
	sourceProject, err := models.NewProject("source", "Source", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject source: %v", err)
	}
	destProject, err := models.NewProject("dest", "Destination", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject dest: %v", err)
	}
	task, err := models.NewTask("task-1", 7, models.TaskTypeTask, "Move me")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}

	m := NewProjectSelectorModel([]*models.Project{sourceProject, destProject}, task, sourceProject)
	_, cmd := m.Update(syntheticSpecialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("enter should return a move command")
	}

	msg, ok := cmd().(MoveTaskToProjectMsg)
	if !ok {
		t.Fatalf("expected MoveTaskToProjectMsg, got %T", cmd())
	}
	if msg.Task != task || msg.SourceProject != sourceProject || msg.DestProjectID != destProject.ID {
		t.Fatalf("unexpected move message: %#v", msg)
	}
}

func TestParentSelectorUpdate_AllowNoneSelection(t *testing.T) {
	parent, err := models.NewTask("parent-1", 11, models.TaskTypeStory, "Parent task")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}

	m := NewParentSelectorModel([]*models.Task{parent}, nil, true, ViewProjectTree)
	m.Init()

	model, _ := m.Update(syntheticCtrlKey('d'))
	m = model.(*ParentSelectorModel)
	if m.Cursor != 1 {
		t.Fatalf("ctrl+d should clamp to the parent entry, got cursor %d", m.Cursor)
	}

	_, cmd := m.Update(syntheticSpecialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("enter should emit a parent selection message")
	}
	msg, ok := cmd().(ParentSelectedMsg)
	if !ok {
		t.Fatalf("expected ParentSelectedMsg, got %T", cmd())
	}
	if msg.ParentID == nil || *msg.ParentID != parent.ID {
		t.Fatalf("expected selected parent %q, got %#v", parent.ID, msg.ParentID)
	}

	model, _ = m.Update(syntheticCtrlKey('u'))
	m = model.(*ParentSelectorModel)
	if m.Cursor != 0 {
		t.Fatalf("ctrl+u should clamp back to None, got cursor %d", m.Cursor)
	}

	_, cmd = m.Update(syntheticSpecialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("enter on None should still emit a selection message")
	}
	msg, ok = cmd().(ParentSelectedMsg)
	if !ok {
		t.Fatalf("expected ParentSelectedMsg, got %T", cmd())
	}
	if msg.ParentID != nil {
		t.Fatalf("expected nil parent when None is selected, got %#v", msg.ParentID)
	}
}

func TestCustomSectionPickerUpdate_SelectsUnsetSection(t *testing.T) {
	project, err := models.NewProject("proj-1", "Sections", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}
	task, err := models.NewTask("task-1", 5, models.TaskTypeTask, "Sectioned task")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	task.CustomSection = "Focus"
	project.Tasks = []*models.Task{task}

	m := NewCustomSectionPickerModel(task, project, []*models.Project{project})
	m.Cursor = 0

	_, cmd := m.Update(syntheticSpecialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("enter should emit a custom section result")
	}
	msg, ok := cmd().(CustomSectionPickerDoneMsg)
	if !ok {
		t.Fatalf("expected CustomSectionPickerDoneMsg, got %T", cmd())
	}
	if msg.Canceled {
		t.Fatalf("expected selecting unset to succeed, got %#v", msg)
	}
	if msg.Section != "" {
		t.Fatalf("expected empty section for unset, got %q", msg.Section)
	}
}

func TestGlobalNoteMoveProjectSelectorUpdate_SelectsProject(t *testing.T) {
	projectA, err := models.NewProject("proj-a", "Alpha", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject A: %v", err)
	}
	projectB, err := models.NewProject("proj-b", "Beta", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject B: %v", err)
	}

	m := NewGlobalNoteMoveProjectSelectorModel([]*models.Project{projectA, projectB})
	model, _ := m.Update(syntheticCtrlKey('j'))
	m = model.(*GlobalNoteMoveProjectSelectorModel)
	if m.Cursor != 1 {
		t.Fatalf("ctrl+j should move cursor to 1, got %d", m.Cursor)
	}

	_, cmd := m.Update(syntheticSpecialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("enter should emit a project selection")
	}
	msg, ok := cmd().(GlobalNoteMoveDoneMsg)
	if !ok {
		t.Fatalf("expected GlobalNoteMoveDoneMsg, got %T", cmd())
	}
	if msg.Project != projectB {
		t.Fatalf("expected selected project %q, got %#v", projectB.ID, msg.Project)
	}
}
