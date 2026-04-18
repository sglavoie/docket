package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

// ── helpers ───────────────────────────────────────────────────────────────────

func newTestTaskForm() *TaskFormModel {
	project := testutil.NewTestProjectWithTasks("Test Project")
	cfg := testutil.NewTestConfig()
	return NewTaskFormModelCreate(project, cfg, nil, nil)
}

func sendFormKey(m *TaskFormModel, key tea.KeyPressMsg) *TaskFormModel {
	next, _ := m.Update(key)
	return next.(*TaskFormModel)
}

// ── tab switching ─────────────────────────────────────────────────────────────

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
	// Wraps from Basics → Details (last tab).
	if m.activeTab != formTabDetails {
		t.Errorf("[: expected wrap to tab %d, got %d", formTabDetails, m.activeTab)
	}
}

func TestTaskForm_TabSwitchWrapsForward(t *testing.T) {
	m := newTestTaskForm()
	for i := 0; i < formTabCount; i++ {
		m = sendFormKey(m, syntheticKey(']'))
	}
	// After a full cycle we should be back at Basics.
	if m.activeTab != formTabBasics {
		t.Errorf("full cycle: expected tab %d, got %d", formTabBasics, m.activeTab)
	}
}

func TestTaskForm_TabSwitchResetsFocusIndex(t *testing.T) {
	m := newTestTaskForm()
	// Move focus forward on Basics tab.
	m = sendFormKey(m, syntheticSpecialKey(tea.KeyTab))
	if m.focusIndex != 1 {
		t.Fatalf("expected focusIndex 1, got %d", m.focusIndex)
	}
	// Switching tab resets focusIndex to 0.
	m = sendFormKey(m, syntheticKey(']'))
	if m.focusIndex != 0 {
		t.Errorf("tab switch should reset focusIndex; got %d", m.focusIndex)
	}
}

// ── focus wrap-around ──────────────────────────────────────────────────────────

func TestTaskForm_FocusCyclesForward_Basics(t *testing.T) {
	m := newTestTaskForm()
	// Use the actual field count for the current type (includes attr rows).
	count := m.fieldCountForActiveTab()
	for i := 0; i < count; i++ {
		m = sendFormKey(m, syntheticSpecialKey(tea.KeyTab))
	}
	// After cycling through all fields, focusIndex wraps back to 0.
	if m.focusIndex != 0 {
		t.Errorf("expected focus wrap to 0 after %d Tab presses, got %d", count, m.focusIndex)
	}
}

func TestTaskForm_FocusCyclesBackward_Basics(t *testing.T) {
	m := newTestTaskForm()
	// Shift+Tab from index 0 should wrap to the last field (includes attr rows).
	lastIdx := m.fieldCountForActiveTab() - 1
	m = sendFormKey(m, tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
	if m.focusIndex != lastIdx {
		t.Errorf("shift+tab from 0: expected %d, got %d", lastIdx, m.focusIndex)
	}
}

func TestTaskForm_FocusNoOpOnEmptyTab(t *testing.T) {
	m := newTestTaskForm()
	// Children tab has 0 fields in Session 1.
	for i := 0; i < formTabCount; i++ {
		m = sendFormKey(m, syntheticKey(']'))
	}
	// We're back on Basics; jump to Children.
	m.activeTab = formTabChildren
	m.focusIndex = 0
	// Tab on empty tab should leave focusIndex unchanged.
	m = sendFormKey(m, syntheticSpecialKey(tea.KeyTab))
	if m.focusIndex != 0 {
		t.Errorf("focus on empty tab: expected 0, got %d", m.focusIndex)
	}
}

// ── title / chrome render ──────────────────────────────────────────────────────

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
	m := newTestTaskForm() // default width=80 >= helpPaneMinWidth
	view := m.View().Content
	// Field-specific header for focusIndex 0 on Basics tab is "Title".
	if !strings.Contains(view, "Title") {
		t.Errorf("help pane should show field header 'Title' at width 80; got:\n%s", view)
	}
}

func TestTaskForm_HelpPaneHiddenWhenNarrow(t *testing.T) {
	m := newTestTaskForm()
	m.width = 50 // below helpPaneMinWidth
	view := m.View().Content
	// With narrow width, help pane header should not appear.
	// The word "Title" would also appear as a form field label, so check for
	// a help-pane-specific phrase that only the pane produces.
	if strings.Contains(view, "│") {
		t.Errorf("help pane separator should be hidden at width 50; got:\n%s", view)
	}
}

func TestTaskForm_HelpPaneFieldHeader_Priority(t *testing.T) {
	m := newTestTaskForm()
	m.focusIndex = 3 // Priority
	view := m.View().Content
	if !strings.Contains(view, "Priority") {
		t.Errorf("help pane should show 'Priority' header at focusIndex 3; got:\n%s", view)
	}
}

func TestTaskForm_HelpPaneHiddenDuringOverlay(t *testing.T) {
	m := newTestTaskForm()
	m.activeOverlay = formOverlayType // any non-none overlay
	view := m.View().Content
	// Help pane separator should not appear during overlay.
	if strings.Contains(view, "│") {
		t.Errorf("help pane should be hidden during overlay; got:\n%s", view)
	}
}

func TestTaskForm_EditTitle_ContainsShortID(t *testing.T) {
	project := testutil.NewTestProjectWithTasks("Test Project")
	cfg := testutil.NewTestConfig()
	// Get first task from project.
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

// ── dirty-state / Esc ─────────────────────────────────────────────────────────

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
	// In edit mode, pressing Esc on the title field without making changes
	// should emit BackMsg, not show a discard confirmation. The title is
	// pre-filled so `draft.title != ""` is always true; only hasChanges matters.
	project := testutil.NewTestProjectWithTasks("Test Project")
	cfg := testutil.NewTestConfig()
	if len(project.Tasks) == 0 {
		t.Skip("test project has no tasks")
	}
	task := project.Tasks[0]
	m := NewTaskFormModelEdit(project, task, cfg, nil)

	// Title field is focused by default (Basics tab, field 0).
	if m.activeTab != formTabBasics || m.focusIndex != 0 {
		t.Fatalf("expected focus on Basics/0, got tab=%d focus=%d", m.activeTab, m.focusIndex)
	}
	// hasChanges is false because nothing was edited.
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
	// In create mode, pressing Esc when a title has been typed should show
	// the discard confirmation, even if hasChanges hasn't been marked.
	m := newTestTaskForm()
	// Simulate typing a title by setting both the textarea and draft.
	m.titleInput.SetValue("Some title")
	m = sendFormKey(m, syntheticSpecialKey(tea.KeyEscape))
	if !m.confirmDiscard {
		t.Error("expected confirmDiscard=true after Esc with non-empty title in create mode")
	}
}

// ── interface compliance ───────────────────────────────────────────────────────

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

// ── Session 2 helpers ─────────────────────────────────────────────────────────

// navigateToField sets focusIndex directly and syncs titleInput focus.
func navigateToField(m *TaskFormModel, idx int) *TaskFormModel {
	m.focusIndex = idx
	m.focusTitleInput()
	return m
}

// sendFormMsg sends a non-key message to the form.
func sendFormMsg(m *TaskFormModel, msg tea.Msg) *TaskFormModel {
	next, _ := m.Update(msg)
	return next.(*TaskFormModel)
}

// ── Type picker overlay ───────────────────────────────────────────────────────

func TestTaskForm_EnterOnType_OpensTypePicker(t *testing.T) {
	m := newTestTaskForm()
	m = navigateToField(m, 2)
	m = sendFormKey(m, syntheticSpecialKey(tea.KeyEnter))
	if m.activeOverlay != formOverlayType {
		t.Errorf("expected formOverlayType, got %v", m.activeOverlay)
	}
	if m.typePicker == nil {
		t.Error("expected typePicker to be non-nil")
	}
}

func TestTaskForm_TypePicker_SelectBug_UpdatesDraft(t *testing.T) {
	m := newTestTaskForm()
	m = navigateToField(m, 2)
	m = sendFormKey(m, syntheticSpecialKey(tea.KeyEnter))
	// AllTaskTypes order: Epic(0), Story(1), Task(2, default), Bug(3), Idea(4)
	// Default cursor is at Task(2). One 'j' press moves to Bug(3).
	m = sendFormKey(m, syntheticKey('j'))
	m = sendFormKey(m, syntheticSpecialKey(tea.KeyEnter))
	if m.draft.taskType != models.TaskTypeBug {
		t.Errorf("expected draft.taskType=Bug, got %v", m.draft.taskType)
	}
	if !m.hasChanges {
		t.Error("expected hasChanges=true after type change")
	}
	if m.activeOverlay != formOverlayNone {
		t.Error("expected overlay dismissed after selection")
	}
}

func TestTaskForm_TypePicker_EscCancels(t *testing.T) {
	m := newTestTaskForm()
	m = navigateToField(m, 2)
	original := m.draft.taskType
	m = sendFormKey(m, syntheticSpecialKey(tea.KeyEnter))
	m = sendFormKey(m, syntheticSpecialKey(tea.KeyEscape))
	if m.draft.taskType != original {
		t.Errorf("cancel should preserve taskType; got %v", m.draft.taskType)
	}
	if m.activeOverlay != formOverlayNone {
		t.Error("expected overlay dismissed after Esc")
	}
}

// ── Priority picker overlay ───────────────────────────────────────────────────

func TestTaskForm_EnterOnPriority_OpensPriorityPicker(t *testing.T) {
	m := newTestTaskForm()
	m = navigateToField(m, 3)
	m = sendFormKey(m, syntheticSpecialKey(tea.KeyEnter))
	if m.activeOverlay != formOverlayPriority {
		t.Errorf("expected formOverlayPriority, got %v", m.activeOverlay)
	}
}

func TestTaskForm_PriorityPicker_DirectKey_UpdatesDraft(t *testing.T) {
	m := newTestTaskForm()
	m = navigateToField(m, 3)
	m = sendFormKey(m, syntheticSpecialKey(tea.KeyEnter))
	m = sendFormKey(m, syntheticKey('1')) // pick critical
	if m.draft.priority != 1 {
		t.Errorf("expected priority=1 (critical), got %d", m.draft.priority)
	}
	if !m.hasChanges {
		t.Error("expected hasChanges=true after priority change")
	}
	if m.activeOverlay != formOverlayNone {
		t.Error("expected overlay dismissed after direct key selection")
	}
}

func TestTaskForm_PriorityPicker_EscCancels(t *testing.T) {
	m := newTestTaskForm()
	m = navigateToField(m, 3)
	original := m.draft.priority
	m = sendFormKey(m, syntheticSpecialKey(tea.KeyEnter))
	m = sendFormKey(m, syntheticSpecialKey(tea.KeyEscape))
	if m.draft.priority != original {
		t.Errorf("cancel should preserve priority; got %d", m.draft.priority)
	}
}

// ── Flags picker overlay ──────────────────────────────────────────────────────

func TestTaskForm_EnterOnFlags_OpensFlagsPicker(t *testing.T) {
	m := newTestTaskForm()
	m = navigateToField(m, 6)
	m = sendFormKey(m, syntheticSpecialKey(tea.KeyEnter))
	if m.activeOverlay != formOverlayFlags {
		t.Errorf("expected formOverlayFlags, got %v", m.activeOverlay)
	}
}

func TestTaskForm_FlagsPicker_Toggle_UpdatesDraft(t *testing.T) {
	m := newTestTaskForm()
	m = navigateToField(m, 6)
	m = sendFormKey(m, syntheticSpecialKey(tea.KeyEnter)) // open (cursor=0=Pinned)
	m = sendFormKey(m, syntheticSpecialKey(tea.KeySpace)) // toggle Pinned
	m = sendFormKey(m, syntheticSpecialKey(tea.KeyEnter)) // confirm
	if !m.draft.pinned {
		t.Error("expected draft.pinned=true after toggling Pinned and confirming")
	}
	if !m.hasChanges {
		t.Error("expected hasChanges=true after flags change")
	}
}

func TestTaskForm_FlagsPicker_EscCancels(t *testing.T) {
	m := newTestTaskForm()
	m = navigateToField(m, 6)
	m = sendFormKey(m, syntheticSpecialKey(tea.KeyEnter))
	m = sendFormKey(m, syntheticSpecialKey(tea.KeySpace)) // toggle but don't confirm
	m = sendFormKey(m, syntheticSpecialKey(tea.KeyEscape))
	if m.draft.pinned {
		t.Error("cancel should not update draft.pinned")
	}
}

// ── External overlay result messages ─────────────────────────────────────────

func TestTaskForm_ParentSelectedMsg_UpdatesDraft(t *testing.T) {
	m := newTestTaskForm()
	parentID := "some-parent-id"
	m = sendFormMsg(m, ParentSelectedMsg{ParentID: &parentID, Source: ViewTaskForm})
	if m.draft.parentID == nil || *m.draft.parentID != parentID {
		t.Errorf("expected draft.parentID=%q, got %v", parentID, m.draft.parentID)
	}
	if !m.hasChanges {
		t.Error("expected hasChanges=true")
	}
}

func TestTaskForm_ParentSelectedMsg_WrongSource_NoChange(t *testing.T) {
	m := newTestTaskForm()
	parentID := "some-parent-id"
	m = sendFormMsg(m, ParentSelectedMsg{ParentID: &parentID, Source: ViewDashboard})
	if m.draft.parentID != nil {
		t.Error("wrong-source ParentSelectedMsg should not update draft")
	}
}

func TestTaskForm_SectionPickerDoneMsg_UpdatesDraft(t *testing.T) {
	m := newTestTaskForm()
	m = sendFormMsg(m, SectionPickerDoneMsg{Section: "Next", Canceled: false})
	if m.draft.section != "Next" {
		t.Errorf("expected draft.section=Next, got %q", m.draft.section)
	}
	if !m.hasChanges {
		t.Error("expected hasChanges=true")
	}
}

func TestTaskForm_SectionPickerDoneMsg_Canceled_NoChange(t *testing.T) {
	m := newTestTaskForm()
	m.draft.section = "Now"
	m = sendFormMsg(m, SectionPickerDoneMsg{Section: "Later", Canceled: true})
	if m.draft.section != "Now" {
		t.Errorf("canceled section picker should not update draft; got %q", m.draft.section)
	}
}

func TestTaskForm_TagSelectorDoneMsg_UpdatesDraft(t *testing.T) {
	m := newTestTaskForm()
	tags := []string{"go", "backend"}
	m = sendFormMsg(m, TagSelectorDoneMsg{SelectedTags: tags, Canceled: false})
	if len(m.draft.tags) != 2 || m.draft.tags[0] != "go" {
		t.Errorf("expected draft.tags=%v, got %v", tags, m.draft.tags)
	}
	if !m.hasChanges {
		t.Error("expected hasChanges=true")
	}
}

func TestTaskForm_TagSelectorDoneMsg_Canceled_NoChange(t *testing.T) {
	m := newTestTaskForm()
	m.draft.tags = []string{"existing"}
	m = sendFormMsg(m, TagSelectorDoneMsg{SelectedTags: []string{"new"}, Canceled: true})
	if len(m.draft.tags) != 1 || m.draft.tags[0] != "existing" {
		t.Errorf("canceled tag selector should not update draft; got %v", m.draft.tags)
	}
}

// ── Basics tab rendering ──────────────────────────────────────────────────────

func TestTaskForm_BasicsTab_RendersLabels(t *testing.T) {
	m := newTestTaskForm()
	m.activeTab = formTabBasics
	view := m.View().Content
	for _, label := range []string{"Title:", "Description:", "Type:", "Priority:", "Parent:", "Section:", "Flags:", "Tags:"} {
		if !strings.Contains(view, label) {
			t.Errorf("Basics tab should contain label %q; got:\n%s", label, view)
		}
	}
}

func TestTaskForm_BasicsTab_OverlayVisible_WhenTypePickerActive(t *testing.T) {
	m := newTestTaskForm()
	m = navigateToField(m, 2)
	m = sendFormKey(m, syntheticSpecialKey(tea.KeyEnter))
	view := m.View().Content
	if !strings.Contains(view, "Set Type") {
		t.Errorf("view should show 'Set Type' when type picker is active; got:\n%s", view)
	}
}

// ── Schedule tab helpers ──────────────────────────────────────────────────────

// navigateToScheduleField sets the form to the Schedule tab and the given focusIndex.
func navigateToScheduleField(m *TaskFormModel, idx int) *TaskFormModel {
	m.activeTab = formTabSchedule
	m.focusIndex = idx
	m.syncInputFocus()
	return m
}

// ── Schedule tab rendering ────────────────────────────────────────────────────

func TestTaskForm_ScheduleTab_RendersLabels(t *testing.T) {
	m := newTestTaskForm()
	m = navigateToScheduleField(m, 0)
	view := m.View().Content
	for _, label := range []string{"Due:", "Recurrence:"} {
		if !strings.Contains(view, label) {
			t.Errorf("Schedule tab should contain label %q; got:\n%s", label, view)
		}
	}
}

func TestTaskForm_ScheduleTab_Due_ShowsPlaceholder_WhenEmpty(t *testing.T) {
	m := newTestTaskForm()
	m = navigateToScheduleField(m, 0)
	view := m.View().Content
	// Ctrl+D presets hint should appear in the help pane when Due field is focused.
	if !strings.Contains(view, "Ctrl+D") || !strings.Contains(view, "presets") {
		t.Errorf("Due field help pane should show 'Ctrl+D' and 'presets' when focused; got:\n%s", view)
	}
}

func TestTaskForm_ScheduleTab_Due_TypedValue_SetsHasChanges(t *testing.T) {
	m := newTestTaskForm()
	m = navigateToScheduleField(m, 0)
	m = sendFormKey(m, syntheticKey('t')) // 't' → textinput
	if !m.hasChanges {
		t.Error("typing in Due field should set hasChanges=true")
	}
}

func TestTaskForm_ScheduleTab_Due_DKey_OpensDuePicker(t *testing.T) {
	m := newTestTaskForm()
	m = navigateToScheduleField(m, 0)
	m = sendFormKey(m, syntheticCtrlKey('d'))
	if m.activeOverlay != formOverlayDue {
		t.Errorf("expected formOverlayDue, got %v", m.activeOverlay)
	}
	if m.duePicker == nil {
		t.Error("expected duePicker to be non-nil")
	}
}

func TestTaskForm_ScheduleTab_DuePicker_TodayKey_UpdatesDraft(t *testing.T) {
	m := newTestTaskForm()
	m = navigateToScheduleField(m, 0)
	m = sendFormKey(m, syntheticCtrlKey('d')) // open picker
	m = sendFormKey(m, syntheticKey('t'))     // pick "today"
	if m.draft.dueDate == nil {
		t.Error("expected draft.dueDate to be set after picking today")
	}
	if m.draft.dueInput == "" {
		t.Error("expected draft.dueInput to be non-empty after picking today")
	}
	if !m.hasChanges {
		t.Error("expected hasChanges=true after due date picked")
	}
	if m.activeOverlay != formOverlayNone {
		t.Error("expected overlay dismissed after selection")
	}
}

func TestTaskForm_ScheduleTab_DuePicker_ClearKey_ClearsDraft(t *testing.T) {
	m := newTestTaskForm()
	m = navigateToScheduleField(m, 0)
	// Pre-set a due date.
	m.draft.dueInput = "2026-01-15"
	m = sendFormKey(m, syntheticCtrlKey('d')) // open picker
	m = sendFormKey(m, syntheticKey('0'))     // pick "clear"
	if m.draft.dueDate != nil {
		t.Error("expected draft.dueDate=nil after clear")
	}
	if m.draft.dueInput != "" {
		t.Errorf("expected draft.dueInput empty after clear, got %q", m.draft.dueInput)
	}
}

func TestTaskForm_ScheduleTab_DuePicker_EscCancels(t *testing.T) {
	m := newTestTaskForm()
	m = navigateToScheduleField(m, 0)
	m = sendFormKey(m, syntheticCtrlKey('d')) // open picker
	m = sendFormKey(m, syntheticSpecialKey(tea.KeyEscape))
	if m.activeOverlay != formOverlayNone {
		t.Error("expected overlay dismissed after Esc")
	}
	if m.draft.dueDate != nil {
		t.Error("cancel should not set dueDate")
	}
}

func TestTaskForm_ScheduleTab_EnterOnRecurrence_EmitsShowRecurrenceEditorMsg(t *testing.T) {
	m := newTestTaskForm()
	m = navigateToScheduleField(m, 1) // Recurrence row
	_, cmd := m.Update(syntheticSpecialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("expected Cmd from Enter on Recurrence, got nil")
	}
	msg := cmd()
	if _, ok := msg.(ShowRecurrenceEditorMsg); !ok {
		t.Errorf("expected ShowRecurrenceEditorMsg, got %T", msg)
	}
}

func TestTaskForm_ScheduleTab_RecurrenceEditorDoneMsg_UpdatesDraft(t *testing.T) {
	m := newTestTaskForm()
	pattern := &models.RecurrencePattern{Interval: 1, Unit: models.RecurrenceDaily}
	m = sendFormMsg(m, RecurrenceEditorDoneMsg{Pattern: pattern, Canceled: false})
	if m.draft.recurrence == nil {
		t.Fatal("expected draft.recurrence to be set")
	}
	if m.draft.recurrence.Interval != 1 {
		t.Errorf("expected recurrence interval=1, got %d", m.draft.recurrence.Interval)
	}
	if !m.hasChanges {
		t.Error("expected hasChanges=true after recurrence updated")
	}
}

func TestTaskForm_ScheduleTab_RecurrenceEditorDoneMsg_Canceled_NoChange(t *testing.T) {
	m := newTestTaskForm()
	m.draft.recurrence = nil
	m = sendFormMsg(m, RecurrenceEditorDoneMsg{Pattern: &models.RecurrencePattern{Interval: 7}, Canceled: true})
	if m.draft.recurrence != nil {
		t.Error("canceled RecurrenceEditorDoneMsg should not update draft")
	}
}

func TestTaskForm_ScheduleTab_DuePicker_Render_ContainsTitleText(t *testing.T) {
	m := newTestTaskForm()
	m = navigateToScheduleField(m, 0)
	m = sendFormKey(m, syntheticCtrlKey('d')) // open overlay
	view := m.View().Content
	if !strings.Contains(view, "Set Due Date") {
		t.Errorf("view should show 'Set Due Date' when due picker is active; got:\n%s", view)
	}
}

func TestTaskForm_ScheduleTab_NLDateRoundTrip(t *testing.T) {
	m := newTestTaskForm()
	m = navigateToScheduleField(m, 0)
	m = sendFormKey(m, syntheticCtrlKey('d'))
	// Pick "tomorrow" via 'm' key.
	m = sendFormKey(m, syntheticKey('m'))
	if m.draft.dueDate == nil {
		t.Fatal("expected dueDate set after picking tomorrow")
	}
	// The stored dueInput should be a formatted date string.
	if m.draft.dueInput == "" {
		t.Error("expected dueInput non-empty after picking tomorrow")
	}
}

// ── Title textarea dirty state ────────────────────────────────────────────────

func TestTaskForm_TypingTitle_SetsHasChanges(t *testing.T) {
	m := newTestTaskForm()
	// Field 0 (Title) is focused by default on Basics tab.
	if m.activeTab != formTabBasics || m.focusIndex != 0 {
		t.Skip("test requires focus on title field")
	}
	m = sendFormKey(m, syntheticKey('H'))
	if !m.hasChanges {
		t.Error("typing in title should set hasChanges=true")
	}
}

// ── Session 4 helpers ─────────────────────────────────────────────────────────

// navigateToAttrRow sets the form to Basics tab at the given attr-section local index.
func navigateToAttrRow(m *TaskFormModel, localIdx int) *TaskFormModel {
	m.activeTab = formTabBasics
	m.focusIndex = formBasicsFieldCount + localIdx
	m.focusTitleInput()
	return m
}

// navigateToDetailsField sets the form to the Details tab and the given focusIndex.
func navigateToDetailsField(m *TaskFormModel, idx int) *TaskFormModel {
	m.activeTab = formTabDetails
	m.focusIndex = idx
	m.syncInputFocus()
	return m
}

// newBugTaskForm creates a TaskFormModel with type=Bug.
func newBugTaskForm() *TaskFormModel {
	m := newTestTaskForm()
	m.draft.taskType = models.TaskTypeBug
	return m
}

// ── Details tab rendering ─────────────────────────────────────────────────────

func TestTaskForm_DetailsTab_RendersLabels(t *testing.T) {
	m := newTestTaskForm()
	m = navigateToDetailsField(m, 0)
	view := m.View().Content
	for _, label := range []string{"URL:", "Source:"} {
		if !strings.Contains(view, label) {
			t.Errorf("Details tab should contain label %q; got:\n%s", label, view)
		}
	}
}

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
	editorMsg, ok := msg.(ShowTextEditorMsg)
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
	m = sendFormMsg(m, TextEditorDoneMsg{FieldName: "formDesc", Value: "new description", Canceled: false})
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
	m = sendFormMsg(m, TextEditorDoneMsg{FieldName: "formDesc", Value: "new", Canceled: true})
	if m.draft.desc != "original" {
		t.Errorf("canceled TextEditorDoneMsg should not update draft; got %q", m.draft.desc)
	}
}

func TestTaskForm_DetailsTab_URLTyping_SetsHasChanges(t *testing.T) {
	m := newTestTaskForm()
	m = navigateToDetailsField(m, 0)
	before := m.hasChanges
	m = sendFormKey(m, syntheticKey('h'))
	if m.hasChanges == before && !m.hasChanges {
		t.Error("typing in URL field should set hasChanges=true")
	}
}

func TestTaskForm_DetailsTab_SourceTyping_SetsHasChanges(t *testing.T) {
	m := newTestTaskForm()
	m = navigateToDetailsField(m, 1)
	m = sendFormKey(m, syntheticKey('s'))
	if !m.hasChanges {
		t.Error("typing in Source field should set hasChanges=true")
	}
}

// ── Type-attrs: Bug ───────────────────────────────────────────────────────────

func TestTaskForm_BugAttrs_FieldCount(t *testing.T) {
	m := newBugTaskForm()
	count := m.fieldCountForActiveTab()
	expected := formBasicsFieldCount + 3 // Bug has 3 attr fields
	if count != expected {
		t.Errorf("Bug type field count: expected %d, got %d", expected, count)
	}
}

func TestTaskForm_BugAttrs_ReproducibleRow_OpensAttrPicker(t *testing.T) {
	m := newBugTaskForm()
	m = navigateToAttrRow(m, 0) // Reproducible
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
	m = sendFormKey(m, syntheticSpecialKey(tea.KeyEnter)) // open picker (cursor=0="none")
	m = sendFormKey(m, syntheticKey('j'))                 // move to "yes"
	m = sendFormKey(m, syntheticSpecialKey(tea.KeyEnter)) // confirm
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
	m = sendFormKey(m, syntheticSpecialKey(tea.KeyEnter)) // open picker
	m = sendFormKey(m, syntheticKey('j'))                 // move cursor
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
	m = navigateToAttrRow(m, 1) // Severity
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
	m = navigateToAttrRow(m, 2) // Workaround
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

// ── Type-attrs: Idea ──────────────────────────────────────────────────────────

func TestTaskForm_IdeaAttrs_FieldCount(t *testing.T) {
	m := newTestTaskForm()
	m.draft.taskType = models.TaskTypeIdea
	count := m.fieldCountForActiveTab()
	expected := formBasicsFieldCount + 4 // Idea has 4 attr fields
	if count != expected {
		t.Errorf("Idea type field count: expected %d, got %d", expected, count)
	}
}

func TestTaskForm_IdeaAttrs_ValidatedPicker_OpensWithTwoOptions(t *testing.T) {
	m := newTestTaskForm()
	m.draft.taskType = models.TaskTypeIdea
	m = navigateToAttrRow(m, 3) // Validated
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
	m = sendFormKey(m, syntheticSpecialKey(tea.KeyEnter)) // open (cursor=0="not validated")
	m = sendFormKey(m, syntheticKey('j'))                 // move to "validated"
	m = sendFormKey(m, syntheticSpecialKey(tea.KeyEnter)) // confirm
	if !m.draft.ideaAttrs.Validated {
		t.Error("expected ideaAttrs.Validated=true after picking 'validated'")
	}
}

// ── Type-attrs: Task ──────────────────────────────────────────────────────────

func TestTaskForm_TaskAttrs_FieldCount(t *testing.T) {
	m := newTestTaskForm()
	// Default type is Task; ensure it has attrs.
	if m.draft.taskType != models.TaskTypeTask {
		t.Skip("requires default type to be Task")
	}
	count := m.fieldCountForActiveTab()
	expected := formBasicsFieldCount + 2 // Task has 2 attr fields
	if count != expected {
		t.Errorf("Task type field count: expected %d, got %d", expected, count)
	}
}

func TestTaskForm_TaskAttrs_EffortRow_OpensAttrPicker(t *testing.T) {
	m := newTestTaskForm() // default type = Task
	if m.draft.taskType != models.TaskTypeTask {
		t.Skip("requires default type to be Task")
	}
	m = navigateToAttrRow(m, 0) // Effort
	m = sendFormKey(m, syntheticSpecialKey(tea.KeyEnter))
	if m.activeOverlay != formOverlayAttr {
		t.Errorf("expected formOverlayAttr, got %v", m.activeOverlay)
	}
	if m.attrPicker.field != "taskEffort" {
		t.Errorf("expected field='taskEffort', got %q", m.attrPicker.field)
	}
}

// ── Type-attrs: Story ─────────────────────────────────────────────────────────

func TestTaskForm_StoryAttrs_PointsPicker_UpdatesDraft(t *testing.T) {
	m := newTestTaskForm()
	m.draft.taskType = models.TaskTypeStory
	m = navigateToAttrRow(m, 0)                           // StoryPoints
	m = sendFormKey(m, syntheticSpecialKey(tea.KeyEnter)) // open picker (cursor=0="none")
	m = sendFormKey(m, syntheticKey('j'))                 // move to "1"
	m = sendFormKey(m, syntheticSpecialKey(tea.KeyEnter)) // confirm
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
	m = navigateToAttrRow(m, 1) // Persona
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

// ── Type-attrs: Epic ──────────────────────────────────────────────────────────

func TestTaskForm_EpicAttrs_FieldCount(t *testing.T) {
	m := newTestTaskForm()
	m.draft.taskType = models.TaskTypeEpic
	count := m.fieldCountForActiveTab()
	expected := formBasicsFieldCount + 3 // Epic has 3 attr fields
	if count != expected {
		t.Errorf("Epic type field count: expected %d, got %d", expected, count)
	}
}

func TestTaskForm_EpicAttrs_MilestoneRow_EmitsShowTextEditorMsg(t *testing.T) {
	m := newTestTaskForm()
	m.draft.taskType = models.TaskTypeEpic
	m = navigateToAttrRow(m, 1) // Milestone
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

// ── Session 5 helpers ─────────────────────────────────────────────────────────

// navigateToChildrenTab sets the form to the Children tab, focusIndex 0.
func navigateToChildrenTab(m *TaskFormModel) *TaskFormModel {
	m.activeTab = formTabChildren
	m.focusIndex = 0
	return m
}

// newEpicTaskForm creates a TaskFormModel with type=Epic (supports subtasks).
func newEpicTaskForm() *TaskFormModel {
	m := newTestTaskForm()
	m.draft.taskType = models.TaskTypeEpic
	return m
}

func TestTaskForm_TextEditorDoneMsg_EpicSuccessMetrics_UpdatesDraft(t *testing.T) {
	m := newTestTaskForm()
	m.draft.taskType = models.TaskTypeEpic
	m = sendFormMsg(m, TextEditorDoneMsg{FieldName: "epicSuccessMetrics", Value: "50% reduction", Canceled: false})
	if m.draft.epicAttrs.SuccessMetrics != "50% reduction" {
		t.Errorf("expected epicAttrs.SuccessMetrics='50%% reduction', got %q", m.draft.epicAttrs.SuccessMetrics)
	}
}

// ── Type switch clamps focusIndex ─────────────────────────────────────────────

func TestTaskForm_TypeSwitch_ClampsAttrFocusIndex(t *testing.T) {
	m := newTestTaskForm()
	m.draft.taskType = models.TaskTypeIdea
	// Simulate focus on Idea's last attr row (Validated = local idx 3 → focusIndex 10).
	m.focusIndex = formBasicsFieldCount + 3

	// Simulate switching to Bug (3 attr fields) via applyFormOverlayResult.
	m.applyFormOverlayResult(TaskFormTypePickedMsg{Type: models.TaskTypeBug})

	// Bug max attr focusIndex = formBasicsFieldCount + 3 - 1 = 9.
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

	// Switch to Idea.
	m.applyFormOverlayResult(TaskFormTypePickedMsg{Type: models.TaskTypeIdea})
	// Switch back to Bug.
	m.applyFormOverlayResult(TaskFormTypePickedMsg{Type: models.TaskTypeBug})

	if m.draft.bugAttrs.Severity != models.BugSeverityMajor {
		t.Errorf("bug attrs should be preserved across type switch; got %v", m.draft.bugAttrs.Severity)
	}
}

// ── NewTaskFormModelEdit loads attrs ──────────────────────────────────────────

func TestTaskForm_EditMode_LoadsBugAttrs(t *testing.T) {
	project := testutil.NewTestProjectWithTasks("Test Project")
	cfg := testutil.NewTestConfig()
	if len(project.Tasks) == 0 {
		t.Skip("test project has no tasks")
	}
	task := project.Tasks[0]
	task.Type = models.TaskTypeBug
	task.BugAttrs = &models.BugAttrs{
		Severity:     models.BugSeverityMajor,
		Reproducible: models.BugReproducibleYes,
		Workaround:   "some workaround",
	}

	m := NewTaskFormModelEdit(project, task, cfg, nil)
	if m.draft.bugAttrs.Severity != models.BugSeverityMajor {
		t.Errorf("expected Severity=major, got %v", m.draft.bugAttrs.Severity)
	}
	if m.draft.bugAttrs.Reproducible != models.BugReproducibleYes {
		t.Errorf("expected Reproducible=yes, got %v", m.draft.bugAttrs.Reproducible)
	}
	if m.draft.bugAttrs.Workaround != "some workaround" {
		t.Errorf("expected Workaround='some workaround', got %q", m.draft.bugAttrs.Workaround)
	}
}

// ── Children tab: field count ──────────────────────────���──────────────────────

func TestTaskForm_Children_FieldCount_DefaultTask(t *testing.T) {
	m := newTestTaskForm()
	m = navigateToChildrenTab(m)
	// Task type cannot have subtasks; only checklist header present.
	got := m.fieldCountForActiveTab()
	if got != 1 {
		t.Errorf("Children tab field count for Task type: expected 1 (header only), got %d", got)
	}
}

func TestTaskForm_Children_FieldCount_ExpandedChecklist(t *testing.T) {
	m := newTestTaskForm()
	m = navigateToChildrenTab(m)
	m.draft.checklist = []models.ChecklistItem{{Text: "a"}, {Text: "b"}}
	m.checklistExpanded = true
	got := m.fieldCountForActiveTab()
	if got != 3 { // header + 2 items
		t.Errorf("Children tab field count with 2 checklist items: expected 3, got %d", got)
	}
}

func TestTaskForm_Children_FieldCount_EpicWithSubtasks(t *testing.T) {
	m := newEpicTaskForm()
	m = navigateToChildrenTab(m)
	m.draft.subtasks = []formSubtaskDraft{{title: "s1", taskType: models.TaskTypeTask}}
	m.subtasksExpanded = true
	got := m.fieldCountForActiveTab()
	// 1 (cl header) + 0 (cl items, collapsed) + 1 (st header) + 1 (st item) = 3
	if got != 3 {
		t.Errorf("Children tab Epic with 1 subtask expanded: expected 3, got %d", got)
	}
}

// ── Children tab: toggle sections ────────────────────────────────────────────

func TestTaskForm_Children_CKey_TogglesChecklistExpand(t *testing.T) {
	m := newTestTaskForm()
	m = navigateToChildrenTab(m)
	if m.checklistExpanded {
		t.Fatal("checklist should start collapsed")
	}
	m = sendFormKey(m, syntheticKey('c'))
	if !m.checklistExpanded {
		t.Error("c should expand checklist section")
	}
	m = sendFormKey(m, syntheticKey('c'))
	if m.checklistExpanded {
		t.Error("c again should collapse checklist section")
	}
}

func TestTaskForm_Children_SKey_TogglesSubtasksExpand_EpicType(t *testing.T) {
	m := newEpicTaskForm()
	m = navigateToChildrenTab(m)
	m = sendFormKey(m, syntheticKey('S'))
	if !m.subtasksExpanded {
		t.Error("S should expand subtasks section for Epic type")
	}
}

func TestTaskForm_Children_SKey_NoOpForTaskType(t *testing.T) {
	m := newTestTaskForm() // Task type — cannot have subtasks
	m = navigateToChildrenTab(m)
	m = sendFormKey(m, syntheticKey('S'))
	if m.subtasksExpanded {
		t.Error("S should be no-op for Task type (no subtasks)")
	}
}

// ── Children tab: checklist add/edit/delete ───────────────────���───────────────

func TestTaskForm_Children_CtrlA_AddsChecklistItemAndOpensOverlay(t *testing.T) {
	m := newTestTaskForm()
	m = navigateToChildrenTab(m)
	m = sendFormKey(m, syntheticCtrlKey('a'))
	if len(m.draft.checklist) != 1 {
		t.Fatalf("expected 1 checklist item after Ctrl+A, got %d", len(m.draft.checklist))
	}
	if m.activeOverlay != formOverlayChecklistEdit {
		t.Errorf("expected formOverlayChecklistEdit after Ctrl+A, got %v", m.activeOverlay)
	}
	if !m.checklistExpanded {
		t.Error("checklist should be expanded after Ctrl+A")
	}
	if !m.hasChanges {
		t.Error("expected hasChanges=true after Ctrl+A")
	}
}

func TestTaskForm_Children_ChecklistEdit_Commit_UpdatesDraft(t *testing.T) {
	m := newTestTaskForm()
	m = navigateToChildrenTab(m)
	m = sendFormKey(m, syntheticCtrlKey('a'))
	// Type text into the overlay input.
	m = sendFormKey(m, syntheticKey('H'))
	m = sendFormKey(m, syntheticKey('i'))
	m = sendFormKey(m, syntheticSpecialKey(tea.KeyEnter))
	if m.activeOverlay != formOverlayNone {
		t.Error("overlay should be dismissed after Enter")
	}
	if m.draft.checklist[0].Text == "" {
		t.Error("checklist item text should be set after committing overlay")
	}
}

func TestTaskForm_Children_ChecklistEdit_Cancel_RemovesNewItem(t *testing.T) {
	m := newTestTaskForm()
	m = navigateToChildrenTab(m)
	m = sendFormKey(m, syntheticCtrlKey('a'))
	if len(m.draft.checklist) != 1 {
		t.Fatalf("precondition: expected 1 checklist item")
	}
	// Esc cancels and removes the new (unsaved) item.
	m = sendFormKey(m, syntheticSpecialKey(tea.KeyEscape))
	if m.activeOverlay != formOverlayNone {
		t.Error("overlay should be dismissed after Esc")
	}
	if len(m.draft.checklist) != 0 {
		t.Errorf("cancelled new checklist item should be removed; got %d items", len(m.draft.checklist))
	}
}

func TestTaskForm_Children_ChecklistEdit_CancelExisting_PreservesItem(t *testing.T) {
	m := newTestTaskForm()
	m.draft.checklist = []models.ChecklistItem{{Text: "existing"}}
	m.checklistExpanded = true
	m = navigateToChildrenTab(m)
	m.focusIndex = 1 // focus first item
	// Enter opens edit overlay for existing item.
	m = sendFormKey(m, syntheticSpecialKey(tea.KeyEnter))
	if m.activeOverlay != formOverlayChecklistEdit {
		t.Fatalf("expected overlay open, got %v", m.activeOverlay)
	}
	// Esc cancels — existing item stays.
	m = sendFormKey(m, syntheticSpecialKey(tea.KeyEscape))
	if len(m.draft.checklist) != 1 || m.draft.checklist[0].Text != "existing" {
		t.Errorf("cancel edit of existing item should preserve it; got %v", m.draft.checklist)
	}
}

func TestTaskForm_Children_CtrlD_DeletesChecklistItem(t *testing.T) {
	m := newTestTaskForm()
	m.draft.checklist = []models.ChecklistItem{{Text: "a"}, {Text: "b"}}
	m.checklistExpanded = true
	m = navigateToChildrenTab(m)
	m.focusIndex = 1 // first item
	m = sendFormKey(m, syntheticCtrlKey('d'))
	if len(m.draft.checklist) != 1 {
		t.Errorf("expected 1 checklist item after Ctrl+D, got %d", len(m.draft.checklist))
	}
	if m.draft.checklist[0].Text != "b" {
		t.Errorf("expected remaining item 'b', got %q", m.draft.checklist[0].Text)
	}
	if !m.hasChanges {
		t.Error("expected hasChanges=true after Ctrl+D")
	}
}

func TestTaskForm_Children_CtrlJ_ReordersChecklistDown(t *testing.T) {
	m := newTestTaskForm()
	m.draft.checklist = []models.ChecklistItem{{Text: "first"}, {Text: "second"}}
	m.checklistExpanded = true
	m = navigateToChildrenTab(m)
	m.focusIndex = 1 // focus "first"
	m = sendFormKey(m, syntheticCtrlKey('j'))
	if m.draft.checklist[0].Text != "second" {
		t.Errorf("after Ctrl+J: expected 'second' at index 0, got %q", m.draft.checklist[0].Text)
	}
	if m.draft.checklist[1].Text != "first" {
		t.Errorf("after Ctrl+J: expected 'first' at index 1, got %q", m.draft.checklist[1].Text)
	}
	if m.focusIndex != 2 {
		t.Errorf("focusIndex should advance to 2, got %d", m.focusIndex)
	}
}

func TestTaskForm_Children_CtrlK_ReordersChecklistUp(t *testing.T) {
	m := newTestTaskForm()
	m.draft.checklist = []models.ChecklistItem{{Text: "first"}, {Text: "second"}}
	m.checklistExpanded = true
	m = navigateToChildrenTab(m)
	m.focusIndex = 2 // focus "second"
	m = sendFormKey(m, syntheticCtrlKey('k'))
	if m.draft.checklist[0].Text != "second" {
		t.Errorf("after Ctrl+K: expected 'second' at index 0, got %q", m.draft.checklist[0].Text)
	}
	if m.focusIndex != 1 {
		t.Errorf("focusIndex should move to 1, got %d", m.focusIndex)
	}
}

func TestTaskForm_Children_Space_TogglesChecklistDone(t *testing.T) {
	m := newTestTaskForm()
	m.draft.checklist = []models.ChecklistItem{{Text: "item", Done: false}}
	m.checklistExpanded = true
	m = navigateToChildrenTab(m)
	m.focusIndex = 1
	m = sendFormKey(m, syntheticSpecialKey(tea.KeySpace))
	if !m.draft.checklist[0].Done {
		t.Error("Space should toggle checklist item Done=true")
	}
	if !m.hasChanges {
		t.Error("expected hasChanges=true after toggling Done")
	}
}

// ── Children tab: subtask add/edit/delete ───────────────────────���──────────────

func TestTaskForm_Children_CtrlT_AddsSubtaskAndOpensOverlay(t *testing.T) {
	m := newEpicTaskForm()
	m = navigateToChildrenTab(m)
	m = sendFormKey(m, syntheticCtrlKey('t'))
	if len(m.draft.subtasks) != 1 {
		t.Fatalf("expected 1 subtask after Ctrl+T, got %d", len(m.draft.subtasks))
	}
	if m.activeOverlay != formOverlaySubtaskEdit {
		t.Errorf("expected formOverlaySubtaskEdit after Ctrl+T, got %v", m.activeOverlay)
	}
	if !m.subtasksExpanded {
		t.Error("subtasks should be expanded after Ctrl+T")
	}
}

func TestTaskForm_Children_CtrlT_NoOpForTaskType(t *testing.T) {
	m := newTestTaskForm() // Task type
	m = navigateToChildrenTab(m)
	m = sendFormKey(m, syntheticCtrlKey('t'))
	if len(m.draft.subtasks) != 0 {
		t.Error("Ctrl+T should be no-op for Task type")
	}
}

func TestTaskForm_Children_SubtaskEdit_Commit_UpdatesDraft(t *testing.T) {
	m := newEpicTaskForm()
	m = navigateToChildrenTab(m)
	m = sendFormKey(m, syntheticCtrlKey('t'))
	// Type a title.
	m = sendFormKey(m, syntheticKey('F'))
	m = sendFormKey(m, syntheticKey('o'))
	m = sendFormKey(m, syntheticKey('o'))
	// Change type to Bug (j moves cursor down once).
	m = sendFormKey(m, syntheticKey('j'))
	m = sendFormKey(m, syntheticSpecialKey(tea.KeyEnter))
	if m.activeOverlay != formOverlayNone {
		t.Error("overlay should be dismissed after Enter")
	}
	if len(m.draft.subtasks) != 1 {
		t.Fatalf("expected 1 subtask, got %d", len(m.draft.subtasks))
	}
	if m.draft.subtasks[0].taskType != models.TaskTypeBug {
		t.Errorf("expected subtask type Bug, got %v", m.draft.subtasks[0].taskType)
	}
}

func TestTaskForm_Children_SubtaskEdit_Cancel_RemovesNewSubtask(t *testing.T) {
	m := newEpicTaskForm()
	m = navigateToChildrenTab(m)
	m = sendFormKey(m, syntheticCtrlKey('t'))
	m = sendFormKey(m, syntheticSpecialKey(tea.KeyEscape))
	if len(m.draft.subtasks) != 0 {
		t.Errorf("cancelled new subtask should be removed; got %d", len(m.draft.subtasks))
	}
}

func TestTaskForm_Children_CtrlO_TogglesSubtaskOptional(t *testing.T) {
	m := newEpicTaskForm()
	m.draft.subtasks = []formSubtaskDraft{{title: "s1", taskType: models.TaskTypeTask, optional: false}}
	m.subtasksExpanded = true
	m = navigateToChildrenTab(m)
	// Focus the subtask item: subtasks header is at childrenSubtasksHeaderIdx(), item is +1
	m.focusIndex = m.childrenSubtasksHeaderIdx() + 1
	m = sendFormKey(m, syntheticCtrlKey('o'))
	if !m.draft.subtasks[0].optional {
		t.Error("Ctrl+O should set subtask optional=true")
	}
	if !m.hasChanges {
		t.Error("expected hasChanges=true after toggle optional")
	}
}

func TestTaskForm_Children_SubtaskReorder(t *testing.T) {
	m := newEpicTaskForm()
	m.draft.subtasks = []formSubtaskDraft{
		{title: "first", taskType: models.TaskTypeTask},
		{title: "second", taskType: models.TaskTypeBug},
	}
	m.subtasksExpanded = true
	m = navigateToChildrenTab(m)
	stStart := m.childrenSubtasksHeaderIdx() + 1
	m.focusIndex = stStart // focus "first"
	m = sendFormKey(m, syntheticCtrlKey('j'))
	if m.draft.subtasks[0].title != "second" {
		t.Errorf("after Ctrl+J: expected 'second' at index 0, got %q", m.draft.subtasks[0].title)
	}
	if m.draft.subtasks[1].title != "first" {
		t.Errorf("after Ctrl+J: expected 'first' at index 1, got %q", m.draft.subtasks[1].title)
	}
}

// ── Children tab: rendering ──────────────────────────��────────────────────────

func TestTaskForm_Children_RendersChecklistSectionHeader(t *testing.T) {
	m := newTestTaskForm()
	m = navigateToChildrenTab(m)
	view := m.View().Content
	if !strings.Contains(view, "Checklist") {
		t.Errorf("Children tab should render Checklist section header; got:\n%s", view)
	}
}

func TestTaskForm_Children_RendersChecklistItems_WhenExpanded(t *testing.T) {
	m := newTestTaskForm()
	m.draft.checklist = []models.ChecklistItem{{Text: "buy milk", Done: false}, {Text: "done thing", Done: true}}
	m.checklistExpanded = true
	m = navigateToChildrenTab(m)
	view := m.View().Content
	if !strings.Contains(view, "buy milk") {
		t.Errorf("Children tab should show checklist item text; got:\n%s", view)
	}
	if !strings.Contains(view, "☑") {
		t.Errorf("Children tab should show ☑ for done item; got:\n%s", view)
	}
}

func TestTaskForm_Children_EpicRendersSubtasksSection(t *testing.T) {
	m := newEpicTaskForm()
	m = navigateToChildrenTab(m)
	view := m.View().Content
	if !strings.Contains(view, "Subtasks") {
		t.Errorf("Children tab for Epic should render Subtasks section; got:\n%s", view)
	}
}

func TestTaskForm_Children_TaskTypeHidesSubtasksSection(t *testing.T) {
	m := newTestTaskForm() // Task type
	m = navigateToChildrenTab(m)
	view := m.View().Content
	// Subtasks section should NOT appear for plain Task type in create mode.
	if strings.Contains(view, "▼ Subtasks") || strings.Contains(view, "▶ Subtasks") {
		t.Errorf("Children tab for Task type should not show Subtasks section header; got:\n%s", view)
	}
}

// ── Children tab: Edit mode ────────────────────────────────────────────────────

func TestTaskForm_EditMode_LoadsChecklist(t *testing.T) {
	project := testutil.NewTestProjectWithTasks("Test Project")
	cfg := testutil.NewTestConfig()
	if len(project.Tasks) == 0 {
		t.Skip("test project has no tasks")
	}
	task := project.Tasks[0]
	task.Checklist = []models.ChecklistItem{{Text: "do this", Done: false}, {Text: "done that", Done: true}}

	m := NewTaskFormModelEdit(project, task, cfg, nil)
	if len(m.draft.checklist) != 2 {
		t.Fatalf("expected 2 checklist items loaded from task, got %d", len(m.draft.checklist))
	}
	if m.draft.checklist[0].Text != "do this" {
		t.Errorf("expected first item 'do this', got %q", m.draft.checklist[0].Text)
	}
	if !m.draft.checklist[1].Done {
		t.Error("expected second item Done=true")
	}
}

func TestTaskForm_EditMode_SubtasksHiddenWithHint(t *testing.T) {
	project := testutil.NewTestProjectWithTasks("Test Project")
	cfg := testutil.NewTestConfig()
	if len(project.Tasks) == 0 {
		t.Skip("test project has no tasks")
	}
	task := project.Tasks[0]
	task.Type = models.TaskTypeEpic
	m := NewTaskFormModelEdit(project, task, cfg, nil)
	m.activeTab = formTabChildren
	view := m.View().Content
	if !strings.Contains(view, "Parent") {
		t.Errorf("Edit mode Children tab should contain hint about Parent field; got:\n%s", view)
	}
}

// ── Session 6 helpers ─────────────────────────────────────────────────────────

// newSaveableTestForm returns a TaskFormModel (create mode) backed by a real
// temp project directory so that saveProjectSync can write to disk.
func newSaveableTestForm(t *testing.T) (*TaskFormModel, func()) {
	t.Helper()
	tmpDir, cleanup := testutil.TempProjectDir(t)
	project := testutil.NewTestProject("Save Test")
	project.Path = filepath.Join(tmpDir, "save-test-project")
	if err := os.MkdirAll(project.Path, 0o755); err != nil {
		t.Fatalf("failed to create project dir: %v", err)
	}
	cfg := testutil.NewTestConfig()
	m := NewTaskFormModelCreate(project, cfg, nil, nil)
	return m, cleanup
}

// newSaveableEditForm returns a TaskFormModel (edit mode) backed by a real
// temp project directory for save tests.
func newSaveableEditForm(t *testing.T) (*TaskFormModel, func()) {
	t.Helper()
	tmpDir, cleanup := testutil.TempProjectDir(t)
	project := testutil.NewTestProjectWithTasks("Save Test")
	project.Path = filepath.Join(tmpDir, "save-edit-project")
	if err := os.MkdirAll(project.Path, 0o755); err != nil {
		t.Fatalf("failed to create project dir: %v", err)
	}
	cfg := testutil.NewTestConfig()
	if len(project.Tasks) == 0 {
		t.Fatal("test project has no tasks")
	}
	task := project.Tasks[0]
	m := NewTaskFormModelEdit(project, task, cfg, nil)
	return m, cleanup
}

// ── Session 6: validation ─────────────────────────────────────────────────────

func TestTaskForm_Save_EmptyTitle_SetsErrorJumpsToBasics(t *testing.T) {
	m := newTestTaskForm()
	// Leave title empty.
	m.draft.title = ""
	m.titleInput.SetValue("")
	_, cmd := m.Update(syntheticCtrlKey('s'))
	if cmd != nil {
		t.Error("expected nil Cmd on validation failure")
	}
	if m.err == nil {
		t.Error("expected m.err to be set after empty title save")
	}
	if m.activeTab != formTabBasics {
		t.Errorf("expected activeTab=Basics on title error, got %d", m.activeTab)
	}
	if m.focusIndex != 0 {
		t.Errorf("expected focusIndex=0 on title error, got %d", m.focusIndex)
	}
}

func TestTaskForm_Save_InvalidDueDate_SetsErrorJumpsToSchedule(t *testing.T) {
	m := newTestTaskForm()
	// Set title in both draft and textarea so commitAllTextInputs doesn't blank it.
	m.draft.title = "valid title"
	m.titleInput.SetValue("valid title")
	m.draft.dueInput = "not-a-date-xyz"
	m.dueTextInput.SetValue("not-a-date-xyz")
	// Call handleSave directly to avoid the textarea ctrl+s path overwriting draft.
	cmd := m.handleSave()
	if cmd != nil {
		t.Error("expected nil Cmd on due date parse failure")
	}
	if m.err == nil {
		t.Error("expected m.err to be set after bad due date")
	}
	if m.activeTab != formTabSchedule {
		t.Errorf("expected activeTab=Schedule on due-date error, got %d", m.activeTab)
	}
	if m.focusIndex != 0 {
		t.Errorf("expected focusIndex=0 on due-date error, got %d", m.focusIndex)
	}
}

func TestTaskForm_Save_CommitsTextInputsBeforeSave(t *testing.T) {
	m := newTestTaskForm()
	// Type directly into the textarea without committing.
	m.titleInput.SetValue("committed title")
	// Trigger save (will fail validation with empty title because SetValue
	// doesn't update draft.title — handleSave must commit it first).
	m.handleSave()
	if m.draft.title != "committed title" {
		t.Errorf("handleSave should commit titleInput; got %q", m.draft.title)
	}
}

func TestTaskForm_Save_ClearsDueDateOnNoneInput(t *testing.T) {
	m := newTestTaskForm()
	// Set a valid dueDate first.
	m.draft.title = "some task"
	m.draft.dueInput = "none"
	if err := m.parseDraftDueDate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.draft.dueDate != nil {
		t.Error("parseDraftDueDate('none') should clear dueDate")
	}
}

func TestTaskForm_Save_ParsesDueDateFromInput(t *testing.T) {
	m := newTestTaskForm()
	m.draft.dueInput = "2026-06-15"
	if err := m.parseDraftDueDate(); err != nil {
		t.Fatalf("unexpected error parsing ISO date: %v", err)
	}
	if m.draft.dueDate == nil {
		t.Fatal("expected dueDate to be set")
	}
	if m.draft.dueDate.Year() != 2026 || int(m.draft.dueDate.Month()) != 6 || m.draft.dueDate.Day() != 15 {
		t.Errorf("parsed wrong date: got %v", m.draft.dueDate)
	}
}

func TestTaskForm_Save_ParsesDueDateWithTime(t *testing.T) {
	m := newTestTaskForm()
	m.draft.dueInput = "2026-06-15T14:30"
	if err := m.parseDraftDueDate(); err != nil {
		t.Fatalf("unexpected error parsing date+time: %v", err)
	}
	if m.draft.dueDate == nil {
		t.Fatal("expected dueDate to be set")
	}
	if !m.draft.hasTime {
		t.Error("expected hasTime=true for input with time component")
	}
	if m.draft.dueDate.Hour() != 14 || m.draft.dueDate.Minute() != 30 {
		t.Errorf("expected 14:30, got %02d:%02d", m.draft.dueDate.Hour(), m.draft.dueDate.Minute())
	}
}

func TestTaskForm_Save_ParsesDueDateWithTimeSuffix(t *testing.T) {
	m := newTestTaskForm()
	m.draft.dueInput = "tomorrow 5pm"
	if err := m.parseDraftDueDate(); err != nil {
		t.Fatalf("unexpected error parsing 'tomorrow 5pm': %v", err)
	}
	if m.draft.dueDate == nil {
		t.Fatal("expected dueDate to be set")
	}
	if !m.draft.hasTime {
		t.Error("expected hasTime=true for 'tomorrow 5pm'")
	}
	if m.draft.dueDate.Hour() != 17 {
		t.Errorf("expected hour=17, got %d", m.draft.dueDate.Hour())
	}
}

func TestTaskForm_Save_DateOnlyDoesNotSetHasTime(t *testing.T) {
	m := newTestTaskForm()
	m.draft.dueInput = "2026-06-15"
	if err := m.parseDraftDueDate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.draft.hasTime {
		t.Error("expected hasTime=false for date-only input")
	}
}

func TestTaskForm_Save_ClearDueDateClearsHasTime(t *testing.T) {
	m := newTestTaskForm()
	m.draft.hasTime = true
	m.draft.dueInput = "none"
	if err := m.parseDraftDueDate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.draft.dueDate != nil {
		t.Error("expected dueDate=nil")
	}
	if m.draft.hasTime {
		t.Error("expected hasTime=false after clearing due date")
	}
}

func TestTaskForm_Edit_PreservesHasTime(t *testing.T) {
	project := &models.Project{
		ID:   "test",
		Name: "Test",
		Path: t.TempDir(),
	}
	dueDate := time.Date(2026, 6, 15, 14, 30, 0, 0, time.Local)
	task := &models.Task{
		ID:      "task-1",
		ShortID: 1,
		Type:    models.TaskTypeTask,
		Title:   "Test task",
		Status:  models.StatusTodo,
		DueDate: &dueDate,
		HasTime: true,
	}
	project.Tasks = []*models.Task{task}

	m := NewTaskFormModelEdit(project, task, nil, nil)
	if !m.draft.hasTime {
		t.Error("expected draft.hasTime=true when editing task with HasTime=true")
	}
	if m.draft.dueInput != "2026-06-15T14:30" {
		t.Errorf("expected dueInput='2026-06-15T14:30', got %q", m.draft.dueInput)
	}
}

func TestTaskForm_DueDatePresetPickerClearsHasTime(t *testing.T) {
	m := newTestTaskForm()
	m.draft.hasTime = true // simulate a previous time-of-day entry
	tomorrow := time.Date(2026, 6, 16, 0, 0, 0, 0, time.Local)
	pickedMsg := TaskFormDueDatePickedMsg{Date: &tomorrow}
	m.activeOverlay = formOverlayDue
	m.duePicker = newFormDueDatePicker(nil)
	// Directly call applyFormOverlayResult to simulate the picker result.
	m.applyFormOverlayResult(pickedMsg)
	if m.draft.hasTime {
		t.Error("expected hasTime=false after preset picker selection")
	}
	if m.draft.dueDate == nil {
		t.Error("expected dueDate to be set")
	}
}

func TestDateInput_PrefillPreservesHasTime(t *testing.T) {
	project := testutil.NewTestProjectWithTasks("Test Project")
	dueWithTime := time.Date(2026, 6, 15, 14, 30, 0, 0, time.Local)

	task := &models.Task{
		ID:      "test-due-time",
		ShortID: 99,
		Title:   "Test task",
		DueDate: &dueWithTime,
		HasTime: true,
	}

	m := NewDateInputModel(task, project)

	// The pre-filled input should include the time component (ISO format).
	got := m.input.Value()
	want := "2026-06-15T14:30"
	if got != want {
		t.Errorf("DateInputModel prefill with HasTime=true: got %q, want %q", got, want)
	}
}

func TestDateInput_PrefillDateOnly(t *testing.T) {
	project := testutil.NewTestProjectWithTasks("Test Project")
	due := time.Date(2026, 6, 15, 0, 0, 0, 0, time.Local)

	task := &models.Task{
		ID:      "test-due-date",
		ShortID: 98,
		Title:   "Test task",
		DueDate: &due,
		HasTime: false,
	}

	m := NewDateInputModel(task, project)

	// The pre-filled input should be date-only.
	got := m.input.Value()
	want := "2026-06-15"
	if got != want {
		t.Errorf("DateInputModel prefill with HasTime=false: got %q, want %q", got, want)
	}
}

func TestTaskForm_Save_BuildAttrs_BugType(t *testing.T) {
	m := newBugTaskForm()
	m.draft.bugAttrs.Severity = models.BugSeverityMajor
	m.draft.bugAttrs.Reproducible = models.BugReproducibleYes
	bug, idea, task, story, epic := m.buildDraftTaskAttrs()
	if bug == nil {
		t.Fatal("expected bugAttrs to be non-nil")
	}
	if bug.Severity != models.BugSeverityMajor {
		t.Errorf("expected Severity=major, got %v", bug.Severity)
	}
	if idea != nil || task != nil || story != nil || epic != nil {
		t.Error("other attrs should be nil for Bug type")
	}
}

func TestTaskForm_Save_BuildAttrs_EmptyBug_ReturnsNil(t *testing.T) {
	m := newBugTaskForm()
	// Leave all bug attrs at zero values (IsEmpty == true).
	bug, _, _, _, _ := m.buildDraftTaskAttrs()
	if bug != nil {
		t.Error("expected bugAttrs=nil when bug attrs are empty")
	}
}

// ── Session 6: happy-path create ─────────────────────────────────────────────

func TestTaskForm_Save_CreateHappyPath_EmitsTaskCreatedMsg(t *testing.T) {
	m, cleanup := newSaveableTestForm(t)
	defer cleanup()

	m.draft.title = "My New Task"
	m.titleInput.SetValue("My New Task")
	m.draft.priority = 2
	m.draft.tags = []string{"go", "tui"}
	m.draft.desc = "Task description"

	cmd := m.handleSave()
	if cmd == nil {
		t.Fatalf("expected non-nil Cmd on successful create; err=%v", m.err)
	}
	msg := cmd()
	created, ok := msg.(TaskCreatedMsg)
	if !ok {
		t.Fatalf("expected TaskCreatedMsg, got %T", msg)
	}
	if created.Task == nil {
		t.Fatal("TaskCreatedMsg.Task should be non-nil")
	}
	if created.Task.Title != "My New Task" {
		t.Errorf("expected Title='My New Task', got %q", created.Task.Title)
	}
	if created.Task.Priority != 2 {
		t.Errorf("expected Priority=2, got %d", created.Task.Priority)
	}
	if len(created.Task.Tags) != 2 {
		t.Errorf("expected 2 tags, got %v", created.Task.Tags)
	}
	if created.Task.Description != "Task description" {
		t.Errorf("expected Description='Task description', got %q", created.Task.Description)
	}
}

func TestTaskForm_Save_CreateHappyPath_AddsChecklistItems(t *testing.T) {
	m, cleanup := newSaveableTestForm(t)
	defer cleanup()

	m.draft.title = "Task with checklist"
	m.titleInput.SetValue("Task with checklist")
	m.draft.checklist = []models.ChecklistItem{
		{Text: "step one", Done: false},
		{Text: "  ", Done: false}, // blank — should be skipped
		{Text: "step two", Done: true},
	}

	cmd := m.handleSave()
	if cmd == nil {
		t.Fatalf("expected non-nil Cmd; err=%v", m.err)
	}
	created := cmd().(TaskCreatedMsg)
	if len(created.Task.Checklist) != 2 {
		t.Errorf("expected 2 checklist items (blank filtered), got %d", len(created.Task.Checklist))
	}
	if created.Task.Checklist[0].Text != "step one" {
		t.Errorf("expected first item 'step one', got %q", created.Task.Checklist[0].Text)
	}
	if !created.Task.Checklist[1].Done {
		t.Error("expected second checklist item Done=true")
	}
}

func TestTaskForm_Save_CreateHappyPath_AddsSubtasksForEpic(t *testing.T) {
	m, cleanup := newSaveableTestForm(t)
	defer cleanup()

	m.draft.taskType = models.TaskTypeEpic
	m.draft.title = "Parent Epic"
	m.titleInput.SetValue("Parent Epic")
	m.draft.subtasks = []formSubtaskDraft{
		{title: "child one", taskType: models.TaskTypeTask},
		{title: "", taskType: models.TaskTypeTask}, // empty — should be skipped
		{title: "child two", taskType: models.TaskTypeBug},
	}

	initialCount := len(m.project.Tasks)
	cmd := m.handleSave()
	if cmd == nil {
		t.Fatalf("expected non-nil Cmd; err=%v", m.err)
	}
	// 1 epic + 2 valid subtasks = 3 new tasks.
	got := len(m.project.Tasks) - initialCount
	if got != 3 {
		t.Errorf("expected 3 new tasks (1 epic + 2 subtasks), got %d", got)
	}
}

// ── Session 6: happy-path edit ────────────────────────────────────────────────

func TestTaskForm_Save_EditHappyPath_EmitsTaskUpdatedMsg(t *testing.T) {
	m, cleanup := newSaveableEditForm(t)
	defer cleanup()

	m.draft.title = "Updated Title"
	m.titleInput.SetValue("Updated Title")
	m.draft.priority = 1
	m.draft.desc = "Updated description"

	cmd := m.handleSave()
	if cmd == nil {
		t.Fatalf("expected non-nil Cmd on successful edit; err=%v", m.err)
	}
	msg := cmd()
	updated, ok := msg.(TaskUpdatedMsg)
	if !ok {
		t.Fatalf("expected TaskUpdatedMsg, got %T", msg)
	}
	if updated.Task == nil {
		t.Fatal("TaskUpdatedMsg.Task should be non-nil")
	}
}

func TestTaskForm_Save_EditHappyPath_MutatesTaskInProject(t *testing.T) {
	m, cleanup := newSaveableEditForm(t)
	defer cleanup()

	taskID := m.task.ID
	m.draft.title = "Mutated Title"
	m.titleInput.SetValue("Mutated Title")
	m.draft.priority = 5
	m.draft.tags = []string{"mutated"}

	cmd := m.handleSave()
	if cmd == nil {
		t.Fatalf("expected non-nil Cmd; err=%v", m.err)
	}
	_ = cmd()

	// Find the task in the project and verify mutation.
	var found *models.Task
	for _, t2 := range m.project.Tasks {
		if t2.ID == taskID {
			found = t2
			break
		}
	}
	if found == nil {
		t.Fatal("task not found in project after edit save")
	}
	if found.Title != "Mutated Title" {
		t.Errorf("expected Title='Mutated Title', got %q", found.Title)
	}
	if found.Priority != 5 {
		t.Errorf("expected Priority=5, got %d", found.Priority)
	}
}

// ── Session 1: Inline priority/type editing ───────────────────────────────────

func TestPriorityDirectSet(t *testing.T) {
	for _, want := range []int{1, 2, 3, 4, 5} {
		m := newTestTaskForm()
		m = navigateToField(m, 3)
		m = sendFormKey(m, syntheticKey(rune('0'+want)))
		if m.draft.priority != want {
			t.Errorf("key '%d': expected priority=%d, got %d", want, want, m.draft.priority)
		}
		if !m.hasChanges {
			t.Errorf("key '%d': expected hasChanges=true", want)
		}
		if m.activeOverlay != formOverlayNone {
			t.Errorf("key '%d': expected no overlay open", want)
		}
	}
}

func TestPriorityCycleUpDown(t *testing.T) {
	// k and up decrement, clamped at 1.
	m := newTestTaskForm()
	m = navigateToField(m, 3)
	m.draft.priority = 3
	m = sendFormKey(m, syntheticKey('k'))
	if m.draft.priority != 2 {
		t.Errorf("k from 3: expected 2, got %d", m.draft.priority)
	}
	m = sendFormKey(m, syntheticSpecialKey(tea.KeyUp))
	if m.draft.priority != 1 {
		t.Errorf("up from 2: expected 1, got %d", m.draft.priority)
	}
	// Clamp: k from 1 stays at 1.
	m = sendFormKey(m, syntheticKey('k'))
	if m.draft.priority != 1 {
		t.Errorf("k from 1 (clamp): expected 1, got %d", m.draft.priority)
	}

	// j and down increment, clamped at 5.
	m2 := newTestTaskForm()
	m2 = navigateToField(m2, 3)
	m2.draft.priority = 3
	m2 = sendFormKey(m2, syntheticKey('j'))
	if m2.draft.priority != 4 {
		t.Errorf("j from 3: expected 4, got %d", m2.draft.priority)
	}
	m2 = sendFormKey(m2, syntheticSpecialKey(tea.KeyDown))
	if m2.draft.priority != 5 {
		t.Errorf("down from 4: expected 5, got %d", m2.draft.priority)
	}
	m2 = sendFormKey(m2, syntheticKey('j'))
	if m2.draft.priority != 5 {
		t.Errorf("j from 5 (clamp): expected 5, got %d", m2.draft.priority)
	}
	if !m2.hasChanges {
		t.Error("expected hasChanges=true after cycling")
	}
}

func TestTypeCycleHL(t *testing.T) {
	// AllTaskTypes order: Epic(0), Story(1), Task(2), Bug(3), Idea(4).
	// Default type is Task.
	m := newTestTaskForm()
	m = navigateToField(m, 2)

	// l: Task → Bug.
	m = sendFormKey(m, syntheticKey('l'))
	if m.draft.taskType != models.TaskTypeBug {
		t.Errorf("l from Task: expected Bug, got %v", m.draft.taskType)
	}

	// l: Bug → Idea.
	m = sendFormKey(m, syntheticKey('l'))
	if m.draft.taskType != models.TaskTypeIdea {
		t.Errorf("l from Bug: expected Idea, got %v", m.draft.taskType)
	}

	// l: Idea → Epic (wrap around).
	m = sendFormKey(m, syntheticKey('l'))
	if m.draft.taskType != models.TaskTypeEpic {
		t.Errorf("l from Idea (wrap): expected Epic, got %v", m.draft.taskType)
	}

	// h: Epic → Idea (wrap around).
	m = sendFormKey(m, syntheticKey('h'))
	if m.draft.taskType != models.TaskTypeIdea {
		t.Errorf("h from Epic (wrap): expected Idea, got %v", m.draft.taskType)
	}

	// right arrow: Idea → Epic (wrap).
	m.draft.taskType = models.TaskTypeIdea
	m = sendFormKey(m, syntheticSpecialKey(tea.KeyRight))
	if m.draft.taskType != models.TaskTypeEpic {
		t.Errorf("right from Idea: expected Epic, got %v", m.draft.taskType)
	}

	// left arrow: Epic → Idea (wrap).
	m = sendFormKey(m, syntheticSpecialKey(tea.KeyLeft))
	if m.draft.taskType != models.TaskTypeIdea {
		t.Errorf("left from Epic: expected Idea, got %v", m.draft.taskType)
	}

	if !m.hasChanges {
		t.Error("expected hasChanges=true after type cycling")
	}
}

func TestTypeCycleSideEffects(t *testing.T) {
	project := testutil.NewTestProjectWithTasks("Test Project")
	cfg := testutil.NewTestConfig()

	// Find an Epic task to use as parent.
	var epicTask *models.Task
	for _, tsk := range project.Tasks {
		if tsk.Type == models.TaskTypeEpic {
			epicTask = tsk
			break
		}
	}
	if epicTask == nil {
		t.Skip("test project has no Epic tasks")
	}

	// Set up form: draft is Task, parent is Epic (Epic can hold Task children).
	m := NewTaskFormModelCreate(project, cfg, nil, nil)
	m.draft.taskType = models.TaskTypeTask
	epicID := epicTask.ID
	m.draft.parentID = &epicID
	m = navigateToField(m, 2)

	// l: Task → Bug — Epic can hold Bug children; parent should be preserved.
	m = sendFormKey(m, syntheticKey('l'))
	if m.draft.taskType != models.TaskTypeBug {
		t.Fatalf("expected Bug, got %v", m.draft.taskType)
	}
	if m.draft.parentID == nil {
		t.Error("parent should be preserved when cycling to compatible type Bug")
	}

	// l three more times: Bug → Idea → Epic (wrap) — Epic can't hold Epic.
	m = sendFormKey(m, syntheticKey('l')) // Bug → Idea
	m = sendFormKey(m, syntheticKey('l')) // Idea → Epic (wrap)
	if m.draft.taskType != models.TaskTypeEpic {
		t.Fatalf("expected Epic, got %v", m.draft.taskType)
	}
	if m.draft.parentID != nil {
		t.Error("parent should be cleared when cycling to Epic (Epic can't be child of Epic)")
	}
}

// ── checklist blank-item handling ─────────────────────────────────────────────

func TestTaskForm_Save_EditFiltersBlankChecklistItems(t *testing.T) {
	m, cleanup := newSaveableEditForm(t)
	defer cleanup()

	m.draft.title = "Edited task"
	m.titleInput.SetValue("Edited task")
	m.draft.checklist = []models.ChecklistItem{
		{Text: "keep me", Done: false},
		{Text: "   ", Done: false}, // whitespace-only — should be filtered
		{Text: "", Done: true},     // empty — should be filtered
		{Text: "also keep", Done: true},
	}

	cmd := m.handleSave()
	if cmd == nil {
		t.Fatalf("expected non-nil Cmd; err=%v", m.err)
	}
	msg := cmd()
	updated, ok := msg.(TaskUpdatedMsg)
	if !ok {
		t.Fatalf("expected TaskUpdatedMsg, got %T", msg)
	}
	if len(updated.Task.Checklist) != 2 {
		t.Errorf("expected 2 checklist items (blanks filtered), got %d", len(updated.Task.Checklist))
	}
	if updated.Task.Checklist[0].Text != "keep me" {
		t.Errorf("expected first item 'keep me', got %q", updated.Task.Checklist[0].Text)
	}
	if updated.Task.Checklist[1].Text != "also keep" {
		t.Errorf("expected second item 'also keep', got %q", updated.Task.Checklist[1].Text)
	}
}

func TestTaskForm_Children_ChecklistEdit_CommitBlank_RemovesItem(t *testing.T) {
	m := newTestTaskForm()
	m.draft.checklist = []models.ChecklistItem{{Text: "existing item"}}
	m.checklistExpanded = true
	m = navigateToChildrenTab(m)
	m.focusIndex = 1 // focus the checklist item

	// Open edit overlay for the existing item.
	m = sendFormKey(m, syntheticSpecialKey(tea.KeyEnter))
	if m.activeOverlay != formOverlayChecklistEdit {
		t.Fatalf("expected checklist edit overlay, got %v", m.activeOverlay)
	}

	// Clear the input text (select all + delete) then confirm with Enter.
	m.checklistItemInput.SetValue("")
	m = sendFormKey(m, syntheticSpecialKey(tea.KeyEnter))

	if m.activeOverlay != formOverlayNone {
		t.Error("overlay should be dismissed after Enter")
	}
	if len(m.draft.checklist) != 0 {
		t.Errorf("confirming blank text should remove the item; got %d items: %v",
			len(m.draft.checklist), m.draft.checklist)
	}
}
