package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestTaskForm_EnterOnPriority_OpensPriorityPicker(t *testing.T) {
	m := newTestTaskForm()
	m = navigateToField(m, 2)
	m = sendFormKey(m, syntheticSpecialKey(tea.KeyEnter))
	if m.activeOverlay != formOverlayPriority {
		t.Errorf("expected formOverlayPriority, got %v", m.activeOverlay)
	}
}

func TestTaskForm_PriorityPicker_DirectKey_UpdatesDraft(t *testing.T) {
	m := newTestTaskForm()
	m = navigateToField(m, 2)
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
	m = navigateToField(m, 2)
	original := m.draft.priority
	m = sendFormKey(m, syntheticSpecialKey(tea.KeyEnter))
	m = sendFormKey(m, syntheticSpecialKey(tea.KeyEscape))
	if m.draft.priority != original {
		t.Errorf("cancel should preserve priority; got %d", m.draft.priority)
	}
}

func TestTaskForm_EnterOnFlags_OpensFlagsPicker(t *testing.T) {
	m := newTestTaskForm()
	m = navigateToField(m, 5)
	m = sendFormKey(m, syntheticSpecialKey(tea.KeyEnter))
	if m.activeOverlay != formOverlayFlags {
		t.Errorf("expected formOverlayFlags, got %v", m.activeOverlay)
	}
}

func TestTaskForm_FlagsPicker_Toggle_UpdatesDraft(t *testing.T) {
	m := newTestTaskForm()
	m = navigateToField(m, 5)
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
	m = navigateToField(m, 5)
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
	m = sendFormMsg(m, ParentSelectedMsg{ParentID: &parentID, Source: ViewTaskComposer})
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
	m = sendFormMsg(m, ParentSelectedMsg{ParentID: &parentID, Source: ViewUnknown})
	if m.draft.parentID != nil {
		t.Error("wrong-source ParentSelectedMsg should not update draft")
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
	for _, label := range []string{"Title:", "Description:", "Priority:", "Parent:", "State:", "Flags:", "Tags:"} {
		if !strings.Contains(view, label) {
			t.Errorf("Basics tab should contain label %q; got:\n%s", label, view)
		}
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
	for _, want := range []int{0, 1, 2, 3} {
		m := newTestTaskForm()
		m = navigateToField(m, 2)
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
	m = navigateToField(m, 2)
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
	if m.draft.priority != 0 {
		t.Errorf("k from 1: expected unset (0), got %d", m.draft.priority)
	}

	m2 := newTestTaskForm()
	m2 = navigateToField(m2, 2)
	m2.draft.priority = 2
	m2 = sendFormKey(m2, syntheticKey('j'))
	if m2.draft.priority != 3 {
		t.Errorf("j from 2: expected 3, got %d", m2.draft.priority)
	}
	m2 = sendFormKey(m2, syntheticSpecialKey(tea.KeyDown))
	if m2.draft.priority != 3 {
		t.Errorf("down from 3 (clamp): expected 3, got %d", m2.draft.priority)
	}
	m2 = sendFormKey(m2, syntheticKey('j'))
	if m2.draft.priority != 3 {
		t.Errorf("j from 3 (clamp): expected 3, got %d", m2.draft.priority)
	}
	if !m2.hasChanges {
		t.Error("expected hasChanges=true after cycling")
	}
}
