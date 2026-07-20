package tui

import (
	"strings"
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

	m := NewParentSelectorModel([]*models.Task{parent}, nil, true, ViewOutliner)
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

func TestParentSelectorView_AllowNoneLabelsNoParent(t *testing.T) {
	parent, err := models.NewTask("parent-1", 11, models.TaskTypeStory, "Parent task")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}

	m := NewParentSelectorModel([]*models.Task{parent}, nil, true, ViewOutliner)
	m.SetSize(80, 24)

	view := m.View().Content
	if !strings.Contains(view, parentSelectorNoParentLabel) {
		t.Fatalf("expected no-parent label %q in view, got:\n%s", parentSelectorNoParentLabel, view)
	}
	if !strings.Contains(view, "Ctrl+X no parent") {
		t.Fatalf("expected Ctrl+X no-parent hint in view, got:\n%s", view)
	}
}

func TestParentSelectorUpdate_CtrlXClearsParent(t *testing.T) {
	parent, err := models.NewTask("parent-1", 11, models.TaskTypeStory, "Parent task")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}

	m := NewParentSelectorModel([]*models.Task{parent}, &parent.ID, true, ViewCrossProjectOutliner)
	m.reparentTaskID = "task-1"

	_, cmd := m.Update(syntheticCtrlKey('x'))
	if cmd == nil {
		t.Fatal("ctrl+x should emit a parent selection message")
	}
	msg, ok := cmd().(ParentSelectedMsg)
	if !ok {
		t.Fatalf("expected ParentSelectedMsg, got %T", cmd())
	}
	if msg.ParentID != nil {
		t.Fatalf("expected nil parent from ctrl+x, got %#v", msg.ParentID)
	}
	if msg.Source != ViewCrossProjectOutliner {
		t.Fatalf("expected source %v, got %v", ViewCrossProjectOutliner, msg.Source)
	}
	if msg.ReparentTaskID != "task-1" {
		t.Fatalf("expected reparent task ID %q, got %q", "task-1", msg.ReparentTaskID)
	}
}

func TestParentSelectorUpdate_CtrlXIgnoredWhenNoneDisallowed(t *testing.T) {
	parent, err := models.NewTask("parent-1", 11, models.TaskTypeStory, "Parent task")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}

	m := NewParentSelectorModel([]*models.Task{parent}, nil, false, ViewOutliner)

	_, cmd := m.Update(syntheticCtrlKey('x'))
	if cmd != nil {
		t.Fatal("ctrl+x should not emit a selection when no-parent is disallowed")
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
	if msg.ProjectID != projectB.ID || msg.ProjectName != projectB.Name {
		t.Fatalf("expected selected project %q, got %#v", projectB.ID, msg)
	}
}
