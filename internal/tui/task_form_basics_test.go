package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

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
	m = sendFormKey(m, syntheticKey('1'))
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
	m = sendFormKey(m, syntheticSpecialKey(tea.KeyEnter))
	m = sendFormKey(m, syntheticSpecialKey(tea.KeySpace))
	m = sendFormKey(m, syntheticSpecialKey(tea.KeyEnter))
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
	m = sendFormKey(m, syntheticSpecialKey(tea.KeySpace))
	m = sendFormKey(m, syntheticSpecialKey(tea.KeyEscape))
	if m.draft.pinned {
		t.Error("cancel should not update draft.pinned")
	}
}

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

func TestTaskForm_TypingTitle_SetsHasChanges(t *testing.T) {
	m := newTestTaskForm()
	if m.activeTab != formTabBasics || m.focusIndex != 0 {
		t.Skip("test requires focus on title field")
	}
	m = sendFormKey(m, syntheticKey('H'))
	if !m.hasChanges {
		t.Error("typing in title should set hasChanges=true")
	}
}

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
	m = sendFormKey(m, syntheticKey('k'))
	if m.draft.priority != 1 {
		t.Errorf("k from 1 (clamp): expected 1, got %d", m.draft.priority)
	}

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
	m := newTestTaskForm()
	m = navigateToField(m, 2)

	m = sendFormKey(m, syntheticKey('l'))
	if m.draft.taskType != models.TaskTypeBug {
		t.Errorf("l from Task: expected Bug, got %v", m.draft.taskType)
	}

	m = sendFormKey(m, syntheticKey('l'))
	if m.draft.taskType != models.TaskTypeIdea {
		t.Errorf("l from Bug: expected Idea, got %v", m.draft.taskType)
	}

	m = sendFormKey(m, syntheticKey('l'))
	if m.draft.taskType != models.TaskTypeEpic {
		t.Errorf("l from Idea (wrap): expected Epic, got %v", m.draft.taskType)
	}

	m = sendFormKey(m, syntheticKey('h'))
	if m.draft.taskType != models.TaskTypeIdea {
		t.Errorf("h from Epic (wrap): expected Idea, got %v", m.draft.taskType)
	}

	m.draft.taskType = models.TaskTypeIdea
	m = sendFormKey(m, syntheticSpecialKey(tea.KeyRight))
	if m.draft.taskType != models.TaskTypeEpic {
		t.Errorf("right from Idea: expected Epic, got %v", m.draft.taskType)
	}

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

	m := NewTaskFormModelCreate(project, cfg, nil, nil)
	m.draft.taskType = models.TaskTypeTask
	epicID := epicTask.ID
	m.draft.parentID = &epicID
	m = navigateToField(m, 2)

	m = sendFormKey(m, syntheticKey('l'))
	if m.draft.taskType != models.TaskTypeBug {
		t.Fatalf("expected Bug, got %v", m.draft.taskType)
	}
	if m.draft.parentID == nil {
		t.Error("parent should be preserved when cycling to compatible type Bug")
	}

	m = sendFormKey(m, syntheticKey('l'))
	m = sendFormKey(m, syntheticKey('l'))
	if m.draft.taskType != models.TaskTypeEpic {
		t.Fatalf("expected Epic, got %v", m.draft.taskType)
	}
	if m.draft.parentID != nil {
		t.Error("parent should be cleared when cycling to Epic (Epic can't be child of Epic)")
	}
}
