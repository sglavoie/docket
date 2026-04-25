package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/testutil"
)

func TestTaskForm_TabSwitchForward(t *testing.T) {
	m := newTestTaskForm()
	if m.activeTab != formTabBasics {
		t.Fatalf("expected initial tab %d, got %d", formTabBasics, m.activeTab)
	}
	m = sendFormKey(m, syntheticKey(']'))
	if m.activeTab != formTabSchedule {
		t.Errorf("after ]: expected tab %d, got %d", formTabSchedule, m.activeTab)
	}
	m = sendFormKey(m, syntheticKey(']'))
	if m.activeTab != formTabChildren {
		t.Errorf("after ]]: expected tab %d, got %d", formTabChildren, m.activeTab)
	}
	m = sendFormKey(m, syntheticKey(']'))
	if m.activeTab != formTabDetails {
		t.Errorf("after ]]]: expected tab %d, got %d", formTabDetails, m.activeTab)
	}
}

func TestTaskForm_TabSwitchBackward(t *testing.T) {
	m := newTestTaskForm()
	m = sendFormKey(m, syntheticKey('['))
	if m.activeTab != formTabDetails {
		t.Errorf("[: expected wrap to tab %d, got %d", formTabDetails, m.activeTab)
	}
}

func TestTaskForm_TabSwitchWrapsForward(t *testing.T) {
	m := newTestTaskForm()
	for range formTabCount {
		m = sendFormKey(m, syntheticKey(']'))
	}
	if m.activeTab != formTabBasics {
		t.Errorf("full cycle: expected tab %d, got %d", formTabBasics, m.activeTab)
	}
}

func TestTaskForm_TabSwitchResetsFocusIndex(t *testing.T) {
	m := newTestTaskForm()
	m = sendFormKey(m, syntheticSpecialKey(tea.KeyTab))
	if m.focusIndex != 1 {
		t.Fatalf("expected focusIndex 1, got %d", m.focusIndex)
	}
	m = sendFormKey(m, syntheticKey(']'))
	if m.focusIndex != 0 {
		t.Errorf("tab switch should reset focusIndex; got %d", m.focusIndex)
	}
}

func TestTaskForm_FocusCyclesForward_Basics(t *testing.T) {
	m := newTestTaskForm()
	count := m.fieldCountForActiveTab()
	for range count {
		m = sendFormKey(m, syntheticSpecialKey(tea.KeyTab))
	}
	if m.focusIndex != 0 {
		t.Errorf("expected focus wrap to 0 after %d Tab presses, got %d", count, m.focusIndex)
	}
}

func TestTaskForm_FocusCyclesBackward_Basics(t *testing.T) {
	m := newTestTaskForm()
	lastIdx := m.fieldCountForActiveTab() - 1
	m = sendFormKey(m, tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
	if m.focusIndex != lastIdx {
		t.Errorf("shift+tab from 0: expected %d, got %d", lastIdx, m.focusIndex)
	}
}

func TestTaskForm_FocusNoOpOnEmptyTab(t *testing.T) {
	m := newTestTaskForm()
	for range formTabCount {
		m = sendFormKey(m, syntheticKey(']'))
	}
	m.activeTab = formTabChildren
	m.focusIndex = 0
	m = sendFormKey(m, syntheticSpecialKey(tea.KeyTab))
	if m.focusIndex != 0 {
		t.Errorf("focus on empty tab: expected 0, got %d", m.focusIndex)
	}
}

func TestTaskForm_ViewContainsCreateTitle(t *testing.T) {
	m := newTestTaskForm()
	view := m.View().Content
	if !strings.Contains(view, "Create Task") {
		t.Errorf("Create view should contain 'Create Task'; got:\n%s", view)
	}
}

func TestTaskForm_ViewContainsTabBar(t *testing.T) {
	m := newTestTaskForm()
	view := m.View().Content
	for _, name := range formTabNames {
		if !strings.Contains(view, name) {
			t.Errorf("view should contain tab name %q; got:\n%s", name, view)
		}
	}
}

func TestTaskForm_HelpPaneShownWhenWide(t *testing.T) {
	m := newTestTaskForm()
	view := m.View().Content
	if !strings.Contains(view, "Title") {
		t.Errorf("help pane should show field header 'Title' at width 80; got:\n%s", view)
	}
}

func TestTaskForm_HelpPaneHiddenWhenNarrow(t *testing.T) {
	m := newTestTaskForm()
	m.width = 50
	view := m.View().Content
	if strings.Contains(view, "│") {
		t.Errorf("help pane separator should be hidden at width 50; got:\n%s", view)
	}
}

func TestTaskForm_HelpPaneFieldHeader_Priority(t *testing.T) {
	m := newTestTaskForm()
	m.focusIndex = 3
	view := m.View().Content
	if !strings.Contains(view, "Priority") {
		t.Errorf("help pane should show 'Priority' header at focusIndex 3; got:\n%s", view)
	}
}

func TestTaskForm_HelpPaneHiddenDuringOverlay(t *testing.T) {
	m := newTestTaskForm()
	m.activeOverlay = formOverlayType
	view := m.View().Content
	if strings.Contains(view, "│") {
		t.Errorf("help pane should be hidden during overlay; got:\n%s", view)
	}
}

func TestTaskForm_EditTitle_ContainsShortID(t *testing.T) {
	project := testutil.NewTestProjectWithTasks("Test Project")
	cfg := testutil.NewTestConfig()
	if len(project.Tasks) == 0 {
		t.Skip("test project has no tasks")
	}
	task := project.Tasks[0]
	task.ShortID = 42

	m := NewTaskFormModelEdit(project, task, cfg, nil)
	view := m.View().Content
	if !strings.Contains(view, "#42") {
		t.Errorf("Edit view should contain '#42'; got:\n%s", view)
	}
	if !strings.Contains(view, "Edit Task") {
		t.Errorf("Edit view should contain 'Edit Task'; got:\n%s", view)
	}
}

func TestTaskForm_EditTitle_DirtyMarker(t *testing.T) {
	project := testutil.NewTestProjectWithTasks("Test Project")
	cfg := testutil.NewTestConfig()
	if len(project.Tasks) == 0 {
		t.Skip("test project has no tasks")
	}
	task := project.Tasks[0]
	task.ShortID = 7

	m := NewTaskFormModelEdit(project, task, cfg, nil)
	m.hasChanges = true
	view := m.View().Content
	expected := fmt.Sprintf("#%d*", task.ShortID)
	if !strings.Contains(view, expected) {
		t.Errorf("dirty Edit view should contain %q; got:\n%s", expected, view)
	}
}

func TestTaskForm_EscWithoutChanges_EmitsBackMsg(t *testing.T) {
	m := newTestTaskForm()
	_, cmd := m.Update(syntheticSpecialKey(tea.KeyEscape))
	if cmd == nil {
		t.Fatal("expected a Cmd from Esc without changes, got nil")
	}
	msg := cmd()
	if _, ok := msg.(BackMsg); !ok {
		t.Errorf("expected BackMsg, got %T", msg)
	}
}

func TestTaskForm_EscWithChanges_SetsConfirmDiscard(t *testing.T) {
	m := newTestTaskForm()
	m.hasChanges = true
	m = sendFormKey(m, syntheticSpecialKey(tea.KeyEscape))
	if !m.confirmDiscard {
		t.Error("expected confirmDiscard=true after Esc with hasChanges")
	}
}

func TestTaskForm_ConfirmDiscard_YEmitsBackMsg(t *testing.T) {
	m := newTestTaskForm()
	m.hasChanges = true
	m.confirmDiscard = true
	_, cmd := m.Update(syntheticKey('y'))
	if cmd == nil {
		t.Fatal("expected Cmd from confirm-y, got nil")
	}
	msg := cmd()
	if _, ok := msg.(BackMsg); !ok {
		t.Errorf("expected BackMsg, got %T", msg)
	}
}

func TestTaskForm_ConfirmDiscard_NCancels(t *testing.T) {
	m := newTestTaskForm()
	m.hasChanges = true
	m.confirmDiscard = true
	m = sendFormKey(m, syntheticKey('n'))
	if m.confirmDiscard {
		t.Error("expected confirmDiscard=false after 'n'")
	}
}

func TestTaskForm_EditModeEscNoChanges_EmitsBackMsg(t *testing.T) {
	project := testutil.NewTestProjectWithTasks("Test Project")
	cfg := testutil.NewTestConfig()
	if len(project.Tasks) == 0 {
		t.Skip("test project has no tasks")
	}
	task := project.Tasks[0]
	m := NewTaskFormModelEdit(project, task, cfg, nil)

	if m.activeTab != formTabBasics || m.focusIndex != 0 {
		t.Fatalf("expected focus on Basics/0, got tab=%d focus=%d", m.activeTab, m.focusIndex)
	}
	if m.hasChanges {
		t.Fatal("hasChanges should be false initially in edit mode")
	}

	_, cmd := m.Update(syntheticSpecialKey(tea.KeyEscape))
	if cmd == nil {
		t.Fatal("expected a Cmd from Esc without changes in edit mode, got nil")
	}
	msg := cmd()
	if _, ok := msg.(BackMsg); !ok {
		t.Errorf("expected BackMsg from Esc in edit mode with no changes, got %T", msg)
	}
	if m.confirmDiscard {
		t.Error("confirmDiscard should be false when Esc is pressed in edit mode without changes")
	}
}

func TestTaskForm_CreateModeEscWithTitle_ShowsConfirmDiscard(t *testing.T) {
	m := newTestTaskForm()
	m.titleInput.SetValue("Some title")
	m = sendFormKey(m, syntheticSpecialKey(tea.KeyEscape))
	if !m.confirmDiscard {
		t.Error("expected confirmDiscard=true after Esc with non-empty title in create mode")
	}
}

func TestTaskForm_ShouldCaptureEsc(t *testing.T) {
	m := newTestTaskForm()
	if !m.ShouldCaptureEsc() {
		t.Error("TaskFormModel.ShouldCaptureEsc() should return true")
	}
}

func TestTaskForm_ViewIDString(t *testing.T) {
	if ViewTaskForm.String() != "TaskForm" {
		t.Errorf("expected 'TaskForm', got %q", ViewTaskForm.String())
	}
}
