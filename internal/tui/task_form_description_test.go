package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/sglavoie/tt/internal/tui/chrome"
)

func TestTaskForm_BasicsTab_DescriptionEmpty_ShowsNone(t *testing.T) {
	m := newTestTaskForm()
	m = navigateToField(m, 1)
	view := m.View().Content
	if !strings.Contains(view, "(none)") {
		t.Errorf("empty description should show '(none)'; got:\n%s", view)
	}
}

func TestTaskForm_BasicsTab_DescriptionFilled_ShowsPreview(t *testing.T) {
	m := newTestTaskForm()
	m.draft.desc = "This is the description"
	m = navigateToField(m, 1)
	view := m.View().Content
	if !strings.Contains(view, "This is the description") {
		t.Errorf("Basics tab should show description preview; got:\n%s", view)
	}
}

func TestTaskForm_BasicsTab_EnterOnDescription_EmitsShowTextEditorMsg(t *testing.T) {
	m := newTestTaskForm()
	m.draft.desc = "current desc"
	m = navigateToField(m, 1)
	_, cmd := m.Update(syntheticSpecialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("expected Cmd from Enter on Description, got nil")
	}
	msg := cmd()
	editorMsg, ok := msg.(chrome.ShowTextEditorMsg)
	if !ok {
		t.Fatalf("expected ShowTextEditorMsg, got %T", msg)
	}
	if editorMsg.FieldName != "formDesc" {
		t.Errorf("expected FieldName='formDesc', got %q", editorMsg.FieldName)
	}
	if editorMsg.CurrentValue != "current desc" {
		t.Errorf("expected CurrentValue='current desc', got %q", editorMsg.CurrentValue)
	}
}

func TestTaskForm_TextEditorDoneMsg_FormDesc_UpdatesDraft(t *testing.T) {
	m := newTestTaskForm()
	m = sendFormMsg(m, chrome.TextEditorDoneMsg{FieldName: "formDesc", Value: "new description", Canceled: false})
	if m.draft.desc != "new description" {
		t.Errorf("expected draft.desc='new description', got %q", m.draft.desc)
	}
	if !m.hasChanges {
		t.Error("expected hasChanges=true after description update")
	}
}

func TestTaskForm_TextEditorDoneMsg_Canceled_NoChange(t *testing.T) {
	m := newTestTaskForm()
	m.draft.desc = "original"
	m = sendFormMsg(m, chrome.TextEditorDoneMsg{FieldName: "formDesc", Value: "new", Canceled: true})
	if m.draft.desc != "original" {
		t.Errorf("canceled TextEditorDoneMsg should not update draft; got %q", m.draft.desc)
	}
}
