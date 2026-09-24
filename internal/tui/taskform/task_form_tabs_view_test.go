package taskform

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/tui/status"
)

func TestTaskForm_TabSwitchForward(t *testing.T) {
	m := newTestTaskForm()
	if m.activeTab != formTabBasics {
		t.Fatalf("expected initial tab %d, got %d", formTabBasics, m.activeTab)
	}
	m = sendFormKey(m, syntheticShiftKey(tea.KeyRight))
	if m.activeTab != formTabSchedule {
		t.Errorf("after ]: expected tab %d, got %d", formTabSchedule, m.activeTab)
	}
	m = sendFormKey(m, syntheticShiftKey(tea.KeyRight))
	if m.activeTab != formTabChildren {
		t.Errorf("after ]]: expected tab %d, got %d", formTabChildren, m.activeTab)
	}
	m = sendFormKey(m, syntheticShiftKey(tea.KeyRight))
	if m.activeTab != formTabExtras {
		t.Errorf("after ]]]: expected tab %d, got %d", formTabExtras, m.activeTab)
	}
	m = sendFormKey(m, syntheticShiftKey(tea.KeyRight))
	if m.activeTab != formTabBasics {
		t.Errorf("after ]]]]: expected tab %d, got %d", formTabBasics, m.activeTab)
	}
}

func TestTaskForm_TabSwitchBackward(t *testing.T) {
	m := newTestTaskForm()
	m = sendFormKey(m, syntheticShiftKey(tea.KeyLeft))
	if m.activeTab != formTabExtras {
		t.Errorf("[: expected wrap to tab %d, got %d", formTabExtras, m.activeTab)
	}
}

func TestTaskForm_TabSwitchWrapsForward(t *testing.T) {
	m := newTestTaskForm()
	for range formTabCount {
		m = sendFormKey(m, syntheticShiftKey(tea.KeyRight))
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
	m = sendFormKey(m, syntheticShiftKey(tea.KeyRight))
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

func TestTaskForm_ChildrenFocusCyclesAcrossSectionHeaders(t *testing.T) {
	m := newTestTaskForm()
	for range formTabCount {
		m = sendFormKey(m, syntheticShiftKey(tea.KeyRight))
	}
	m.activeTab = formTabChildren
	m.focusIndex = 0
	m = sendFormKey(m, syntheticSpecialKey(tea.KeyTab))
	if m.focusIndex != 1 {
		t.Errorf("children focus after Tab: expected 1, got %d", m.focusIndex)
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
	m.focusIndex = 2
	view := m.View().Content
	if !strings.Contains(view, "Priority") {
		t.Errorf("help pane should show 'Priority' header at focusIndex 2; got:\n%s", view)
	}
}

func TestTaskForm_HelpPaneHiddenDuringOverlay(t *testing.T) {
	m := newTestTaskForm()
	m.activeOverlay = formOverlayPriority
	view := m.View().Content
	if strings.Contains(view, "│") {
		t.Errorf("help pane should be hidden during overlay; got:\n%s", view)
	}
}

func TestTaskForm_EscWithoutChanges_EmitsBackMsg(t *testing.T) {
	m := newTestTaskForm()
	_, cmd := m.Update(syntheticSpecialKey(tea.KeyEscape))
	if cmd == nil {
		t.Fatal("expected a Cmd from Esc without changes, got nil")
	}
	msg := cmd()
	if _, ok := msg.(status.BackMsg); !ok {
		t.Errorf("expected BackMsg, got %T", msg)
	}
}

func TestTaskForm_EscWithChanges_SetsConfirmDiscard(t *testing.T) {
	m := newTestTaskForm()
	m.hasChanges = true
	m = sendFormKey(m, syntheticSpecialKey(tea.KeyEscape))
	if !m.ConfirmDiscard {
		t.Error("expected ConfirmDiscard=true after Esc with hasChanges")
	}
}

func TestTaskForm_ConfirmDiscard_YEmitsBackMsg(t *testing.T) {
	m := newTestTaskForm()
	m.hasChanges = true
	m.ConfirmDiscard = true
	_, cmd := m.Update(syntheticKey('y'))
	if cmd == nil {
		t.Fatal("expected Cmd from confirm-y, got nil")
	}
	msg := cmd()
	if _, ok := msg.(status.BackMsg); !ok {
		t.Errorf("expected BackMsg, got %T", msg)
	}
}

func TestTaskForm_ConfirmDiscard_NCancels(t *testing.T) {
	m := newTestTaskForm()
	m.hasChanges = true
	m.ConfirmDiscard = true
	m = sendFormKey(m, syntheticKey('n'))
	if m.ConfirmDiscard {
		t.Error("expected ConfirmDiscard=false after 'n'")
	}
}

func TestTaskForm_CreateModeEscWithTitle_ShowsConfirmDiscard(t *testing.T) {
	m := newTestTaskForm()
	m.titleInput.SetValue("Some title")
	m = sendFormKey(m, syntheticSpecialKey(tea.KeyEscape))
	if !m.ConfirmDiscard {
		t.Error("expected ConfirmDiscard=true after Esc with non-empty title in create mode")
	}
}

func TestTaskForm_ShouldCaptureEsc(t *testing.T) {
	m := newTestTaskForm()
	if !m.ShouldCaptureEsc() {
		t.Error("TaskFormModel.ShouldCaptureEsc() should return true")
	}
}
