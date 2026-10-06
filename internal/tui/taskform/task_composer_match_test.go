package taskform

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
)

func TestCaptureInspectionPreservesBatchDraftAndCaret(t *testing.T) {
	parent := "parent"
	project := &models.Project{ID: "web", Slug: "web", Name: "Web", Tasks: []*models.Task{
		{ID: "root", ShortID: 1, Title: "Fix login", WorkflowState: models.StateBacklog},
		{ID: "child", ShortID: 2, ParentID: &parent, Title: "Fix login", WorkflowState: models.StateDoing,
			ResumeNote: "Start with the redirect", Description: strings.Repeat("Details with Unicode café. ", 80)},
		{ID: parent, ShortID: 7, Title: "Release"},
	}}
	m := NewTaskComposerModel(TaskComposerPreset{Project: project,
		Shared: TaskComposerSharedAttributes{ParentID: &parent}, InitialText: "Fix login\nAnother task"}, nil)
	m.Input.MoveToBegin()
	m.Input.SetCursorColumn(3)
	value, line, col := m.Input.Value(), m.Input.Line(), m.Input.Column()
	m.Update(tea.KeyPressMsg{Code: 'o', Mod: tea.ModCtrl})
	if m.matchingTaskPreview == nil {
		t.Fatal("matching task did not open")
	}
	if !strings.Contains(m.matchingTaskText, "web#2") || !strings.Contains(m.matchingTaskText, "Start with the redirect") {
		t.Fatalf("inspected wrong task: %s", m.matchingTaskText)
	}
	for _, width := range []int{60, 100} {
		m.SetSize(width, 18)
		if text := ansi.Strip(m.View().Content); !strings.Contains(text, "Matching task") {
			t.Fatalf("missing inspection: %s", text)
		}
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyPgDown})
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.SubmissionInFlight {
		t.Fatal("inspection submitted the draft")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.matchingTaskPreview != nil || m.Input.Value() != value || m.Input.Line() != line || m.Input.Column() != col {
		t.Fatal("return lost draft or caret")
	}
	m.Input.SetValue("No match")
	m.Update(tea.KeyPressMsg{Code: 'o', Mod: tea.ModCtrl})
	if m.matchingTaskPreview != nil {
		t.Fatal("opened stale match")
	}
}

func TestHostCapturePreviewIncludesDeferralPriorityAndDuplicate(t *testing.T) {
	home := &models.Project{ID: "home", Name: "Home", Slug: "home"}
	web := &models.Project{ID: "web", Name: "Web", Slug: "web", Tasks: []*models.Task{
		{ID: "existing", ShortID: 42, Title: "Fix login", WorkflowState: models.StateWaiting},
	}}
	preset := TaskComposerPreset{Project: home, AllProjects: []*models.Project{home, web}, AllowProjectSwitch: true}
	text, errs := CapturePreviewText(preset, nil, "Fix login %web !1 @@2030-01-02")
	if len(errs) != 0 {
		t.Fatalf("preview errors: %+v", errs)
	}
	for _, want := range []string{"Project: Web", "Defer until:", "Jan 2 2030", "Priority: High", "Matching title: web#42 · Waiting"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in %s", want, text)
		}
	}
	text, errs = CapturePreviewText(preset, nil, "Fix login %web !1\nOther !2")
	if len(errs) != 0 || !strings.Contains(text, "Priority: Mixed") || !strings.Contains(text, "Line 1: Matching title:") {
		t.Fatalf("batch preview: %s, %+v", text, errs)
	}
	text, errs = CapturePreviewText(preset, nil, "Fix login %missing !1")
	if len(errs) == 0 || text != "" {
		t.Fatalf("invalid preview: %s, %+v", text, errs)
	}
}
