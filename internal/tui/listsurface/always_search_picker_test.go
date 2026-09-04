package listsurface

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

func TestParentSelectorUpdate_AllowNoneSelection(t *testing.T) {
	parent, err := models.NewGenericTask("parent-1", 11, "Parent task")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}

	m := NewParentSelectorModel([]*models.Task{parent}, nil, true, presentation.ViewCrossProjectOutliner)
	m.Init()

	model, _ := m.Update(syntheticCtrlKey('d'))
	m = model.(*ParentSelectorModel)
	if m.CursorIndex() != 1 {
		t.Fatalf("ctrl+d should clamp to the parent entry, got cursor %d", m.CursorIndex())
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
	if m.CursorIndex() != 0 {
		t.Fatalf("ctrl+u should clamp back to None, got cursor %d", m.CursorIndex())
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
	parent, err := models.NewGenericTask("parent-1", 11, "Parent task")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}

	m := NewParentSelectorModel([]*models.Task{parent}, nil, true, presentation.ViewCrossProjectOutliner)
	m.SetSize(80, 24)

	view := m.View().Content
	if !strings.Contains(view, parentSelectorNoParentLabel) {
		t.Fatalf("expected no-parent label %q in view, got:\n%s", parentSelectorNoParentLabel, view)
	}
	if !strings.Contains(ansi.Strip(view), "Ctrl+X no parent") {
		t.Fatalf("expected Ctrl+X no-parent hint in view, got:\n%s", view)
	}
}

func TestParentSelectorUpdate_CtrlXClearsParent(t *testing.T) {
	parent, err := models.NewGenericTask("parent-1", 11, "Parent task")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}

	m := NewParentSelectorModel([]*models.Task{parent}, &parent.ID, true, presentation.ViewCrossProjectOutliner)
	m.ReparentTask = taskref.TaskIdentity{ProjectID: "project-1", TaskID: "task-1"}

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
	if msg.Source != presentation.ViewCrossProjectOutliner {
		t.Fatalf("expected source %v, got %v", presentation.ViewCrossProjectOutliner, msg.Source)
	}
	if msg.ReparentTask != (taskref.TaskIdentity{ProjectID: "project-1", TaskID: "task-1"}) {
		t.Fatalf("expected qualified reparent task, got %#v", msg.ReparentTask)
	}
}

func TestParentSelectorUpdate_CtrlXIgnoredWhenNoneDisallowed(t *testing.T) {
	parent, err := models.NewGenericTask("parent-1", 11, "Parent task")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}

	m := NewParentSelectorModel([]*models.Task{parent}, nil, false, presentation.ViewCrossProjectOutliner)

	_, cmd := m.Update(syntheticCtrlKey('x'))
	if cmd != nil {
		t.Fatal("ctrl+x should not emit a selection when no-parent is disallowed")
	}
}
