package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

func TestTaskForm_BugAttrs_FieldCount(t *testing.T) {
	m := newBugTaskForm()
	count := m.fieldCountForActiveTab()
	expected := formBasicsFieldCount + 3
	if count != expected {
		t.Errorf("Bug type field count: expected %d, got %d", expected, count)
	}
}

func TestTaskForm_BugAttrs_ReproducibleRow_OpensAttrPicker(t *testing.T) {
	m := newBugTaskForm()
	m = navigateToAttrRow(m, 0)
	m = sendFormKey(m, syntheticSpecialKey(tea.KeyEnter))
	if m.activeOverlay != formOverlayAttr {
		t.Errorf("expected formOverlayAttr, got %v", m.activeOverlay)
	}
	if m.attrPicker == nil {
		t.Error("expected attrPicker to be non-nil")
	}
	if m.attrPicker.field != "bugReproducible" {
		t.Errorf("expected field='bugReproducible', got %q", m.attrPicker.field)
	}
}

func TestTaskForm_BugAttrs_ReproduciblePicker_SelectYes_UpdatesDraft(t *testing.T) {
	m := newBugTaskForm()
	m = navigateToAttrRow(m, 0)
	m = sendFormKey(m, syntheticSpecialKey(tea.KeyEnter))
	m = sendFormKey(m, syntheticKey('j'))
	m = sendFormKey(m, syntheticSpecialKey(tea.KeyEnter))
	if m.draft.bugAttrs.Reproducible != models.BugReproducibleYes {
		t.Errorf("expected Reproducible=yes, got %v", m.draft.bugAttrs.Reproducible)
	}
	if !m.hasChanges {
		t.Error("expected hasChanges=true after attr change")
	}
	if m.activeOverlay != formOverlayNone {
		t.Error("expected overlay dismissed after selection")
	}
}

func TestTaskForm_BugAttrs_AttrPicker_EscCancels(t *testing.T) {
	m := newBugTaskForm()
	m = navigateToAttrRow(m, 0)
	m = sendFormKey(m, syntheticSpecialKey(tea.KeyEnter))
	m = sendFormKey(m, syntheticKey('j'))
	m = sendFormKey(m, syntheticSpecialKey(tea.KeyEscape))
	if m.draft.bugAttrs.Reproducible != "" {
		t.Errorf("cancel should not update draft; got %v", m.draft.bugAttrs.Reproducible)
	}
	if m.activeOverlay != formOverlayNone {
		t.Error("expected overlay dismissed after Esc")
	}
}

func TestTaskForm_BugAttrs_SeverityRow_OpensAttrPicker(t *testing.T) {
	m := newBugTaskForm()
	m = navigateToAttrRow(m, 1)
	m = sendFormKey(m, syntheticSpecialKey(tea.KeyEnter))
	if m.activeOverlay != formOverlayAttr {
		t.Errorf("expected formOverlayAttr for Severity, got %v", m.activeOverlay)
	}
	if m.attrPicker.field != "bugSeverity" {
		t.Errorf("expected field='bugSeverity', got %q", m.attrPicker.field)
	}
}

func TestTaskForm_BugAttrs_WorkaroundRow_EmitsShowTextEditorMsg(t *testing.T) {
	m := newBugTaskForm()
	m.draft.bugAttrs.Workaround = "restart server"
	m = navigateToAttrRow(m, 2)
	_, cmd := m.Update(syntheticSpecialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("expected Cmd from Enter on Workaround, got nil")
	}
	msg := cmd()
	editorMsg, ok := msg.(ShowTextEditorMsg)
	if !ok {
		t.Fatalf("expected ShowTextEditorMsg, got %T", msg)
	}
	if editorMsg.FieldName != "bugWorkaround" {
		t.Errorf("expected FieldName='bugWorkaround', got %q", editorMsg.FieldName)
	}
	if editorMsg.CurrentValue != "restart server" {
		t.Errorf("expected CurrentValue='restart server', got %q", editorMsg.CurrentValue)
	}
}

func TestTaskForm_TextEditorDoneMsg_BugWorkaround_UpdatesDraft(t *testing.T) {
	m := newBugTaskForm()
	m = sendFormMsg(m, TextEditorDoneMsg{FieldName: "bugWorkaround", Value: "reboot system", Canceled: false})
	if m.draft.bugAttrs.Workaround != "reboot system" {
		t.Errorf("expected bugWorkaround='reboot system', got %q", m.draft.bugAttrs.Workaround)
	}
	if !m.hasChanges {
		t.Error("expected hasChanges=true after workaround update")
	}
}

func TestTaskForm_BugAttrs_RenderedInBasicsView(t *testing.T) {
	m := newBugTaskForm()
	m.draft.bugAttrs.Severity = models.BugSeverityMajor
	view := m.View().Content
	if !strings.Contains(view, "Severity:") {
		t.Errorf("Basics view should contain 'Severity:' for Bug type; got:\n%s", view)
	}
	if !strings.Contains(view, "major") {
		t.Errorf("Basics view should show severity value 'major'; got:\n%s", view)
	}
}

func TestTaskForm_IdeaAttrs_FieldCount(t *testing.T) {
	m := newTestTaskForm()
	m.draft.taskType = models.TaskTypeIdea
	count := m.fieldCountForActiveTab()
	expected := formBasicsFieldCount + 4
	if count != expected {
		t.Errorf("Idea type field count: expected %d, got %d", expected, count)
	}
}

func TestTaskForm_IdeaAttrs_ValidatedPicker_OpensWithTwoOptions(t *testing.T) {
	m := newTestTaskForm()
	m.draft.taskType = models.TaskTypeIdea
	m = navigateToAttrRow(m, 3)
	m = sendFormKey(m, syntheticSpecialKey(tea.KeyEnter))
	if m.activeOverlay != formOverlayAttr {
		t.Errorf("expected formOverlayAttr for Validated, got %v", m.activeOverlay)
	}
	if m.attrPicker.field != "ideaValidated" {
		t.Errorf("expected field='ideaValidated', got %q", m.attrPicker.field)
	}
	if len(m.attrPicker.options) != 2 {
		t.Errorf("expected 2 options for Validated picker, got %d", len(m.attrPicker.options))
	}
}

func TestTaskForm_IdeaAttrs_ValidatedPicker_SelectValidated_UpdatesDraft(t *testing.T) {
	m := newTestTaskForm()
	m.draft.taskType = models.TaskTypeIdea
	m = navigateToAttrRow(m, 3)
	m = sendFormKey(m, syntheticSpecialKey(tea.KeyEnter))
	m = sendFormKey(m, syntheticKey('j'))
	m = sendFormKey(m, syntheticSpecialKey(tea.KeyEnter))
	if !m.draft.ideaAttrs.Validated {
		t.Error("expected ideaAttrs.Validated=true after picking 'validated'")
	}
}

func TestTaskForm_TaskAttrs_FieldCount(t *testing.T) {
	m := newTestTaskForm()
	if m.draft.taskType != models.TaskTypeTask {
		t.Skip("requires default type to be Task")
	}
	count := m.fieldCountForActiveTab()
	expected := formBasicsFieldCount + 2
	if count != expected {
		t.Errorf("Task type field count: expected %d, got %d", expected, count)
	}
}

func TestTaskForm_TaskAttrs_EffortRow_OpensAttrPicker(t *testing.T) {
	m := newTestTaskForm()
	if m.draft.taskType != models.TaskTypeTask {
		t.Skip("requires default type to be Task")
	}
	m = navigateToAttrRow(m, 0)
	m = sendFormKey(m, syntheticSpecialKey(tea.KeyEnter))
	if m.activeOverlay != formOverlayAttr {
		t.Errorf("expected formOverlayAttr, got %v", m.activeOverlay)
	}
	if m.attrPicker.field != "taskEffort" {
		t.Errorf("expected field='taskEffort', got %q", m.attrPicker.field)
	}
}

func TestTaskForm_StoryAttrs_PointsPicker_UpdatesDraft(t *testing.T) {
	m := newTestTaskForm()
	m.draft.taskType = models.TaskTypeStory
	m = navigateToAttrRow(m, 0)
	m = sendFormKey(m, syntheticSpecialKey(tea.KeyEnter))
	m = sendFormKey(m, syntheticKey('j'))
	m = sendFormKey(m, syntheticSpecialKey(tea.KeyEnter))
	if m.draft.storyAttrs.StoryPoints != 1 {
		t.Errorf("expected StoryPoints=1, got %d", m.draft.storyAttrs.StoryPoints)
	}
	if !m.hasChanges {
		t.Error("expected hasChanges=true after story points change")
	}
}

func TestTaskForm_StoryAttrs_PersonaRow_EmitsShowTextEditorMsg(t *testing.T) {
	m := newTestTaskForm()
	m.draft.taskType = models.TaskTypeStory
	m = navigateToAttrRow(m, 1)
	_, cmd := m.Update(syntheticSpecialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("expected Cmd from Enter on Persona, got nil")
	}
	msg := cmd()
	editorMsg, ok := msg.(ShowTextEditorMsg)
	if !ok {
		t.Fatalf("expected ShowTextEditorMsg, got %T", msg)
	}
	if editorMsg.FieldName != "storyPersona" {
		t.Errorf("expected FieldName='storyPersona', got %q", editorMsg.FieldName)
	}
}

func TestTaskForm_EpicAttrs_FieldCount(t *testing.T) {
	m := newTestTaskForm()
	m.draft.taskType = models.TaskTypeEpic
	count := m.fieldCountForActiveTab()
	expected := formBasicsFieldCount + 3
	if count != expected {
		t.Errorf("Epic type field count: expected %d, got %d", expected, count)
	}
}

func TestTaskForm_EpicAttrs_MilestoneRow_EmitsShowTextEditorMsg(t *testing.T) {
	m := newTestTaskForm()
	m.draft.taskType = models.TaskTypeEpic
	m = navigateToAttrRow(m, 1)
	_, cmd := m.Update(syntheticSpecialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("expected Cmd from Enter on Milestone, got nil")
	}
	resultMsg := cmd()
	editorMsg, ok := resultMsg.(ShowTextEditorMsg)
	if !ok {
		t.Fatalf("expected ShowTextEditorMsg, got %T", resultMsg)
	}
	if editorMsg.FieldName != "epicMilestone" {
		t.Errorf("expected FieldName='epicMilestone', got %q", editorMsg.FieldName)
	}
}

func TestTaskForm_TextEditorDoneMsg_EpicMilestone_UpdatesDraft(t *testing.T) {
	m := newTestTaskForm()
	m.draft.taskType = models.TaskTypeEpic
	m = sendFormMsg(m, TextEditorDoneMsg{FieldName: "epicMilestone", Value: "Q3 2026", Canceled: false})
	if m.draft.epicAttrs.Milestone != "Q3 2026" {
		t.Errorf("expected epicAttrs.Milestone='Q3 2026', got %q", m.draft.epicAttrs.Milestone)
	}
}

func TestTaskForm_TextEditorDoneMsg_EpicSuccessMetrics_UpdatesDraft(t *testing.T) {
	m := newTestTaskForm()
	m.draft.taskType = models.TaskTypeEpic
	m = sendFormMsg(m, TextEditorDoneMsg{FieldName: "epicSuccessMetrics", Value: "50% reduction", Canceled: false})
	if m.draft.epicAttrs.SuccessMetrics != "50% reduction" {
		t.Errorf("expected epicAttrs.SuccessMetrics='50%% reduction', got %q", m.draft.epicAttrs.SuccessMetrics)
	}
}

func TestTaskForm_TypeSwitch_ClampsAttrFocusIndex(t *testing.T) {
	m := newTestTaskForm()
	m.draft.taskType = models.TaskTypeIdea
	m.focusIndex = formBasicsFieldCount + 3

	m.applyFormOverlayResult(TaskFormTypePickedMsg{Type: models.TaskTypeBug})

	maxAllowed := formBasicsFieldCount + formAttrFieldCount(models.TaskTypeBug) - 1
	if m.focusIndex > maxAllowed {
		t.Errorf("focusIndex should be clamped to %d after type switch; got %d", maxAllowed, m.focusIndex)
	}
	if m.draft.taskType != models.TaskTypeBug {
		t.Errorf("expected taskType=Bug, got %v", m.draft.taskType)
	}
}

func TestTaskForm_TypeSwitch_PreservesAttrs(t *testing.T) {
	m := newBugTaskForm()
	m.draft.bugAttrs.Severity = models.BugSeverityMajor

	m.applyFormOverlayResult(TaskFormTypePickedMsg{Type: models.TaskTypeIdea})
	m.applyFormOverlayResult(TaskFormTypePickedMsg{Type: models.TaskTypeBug})

	if m.draft.bugAttrs.Severity != models.BugSeverityMajor {
		t.Errorf("bug attrs should be preserved across type switch; got %v", m.draft.bugAttrs.Severity)
	}
}
