package taskform

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"

	tea "charm.land/bubbletea/v2"
)

// ── Checklist overlay ───────────────────────────────────────────────────────

func TestTaskForm_Children_ChecklistEdit_CommitBlank_RemovesItem(t *testing.T) {
	m := newTestTaskForm()
	m.draft.checklist = []models.ChecklistItem{{Text: "existing item"}}
	m.checklistExpanded = true
	m = navigateToChildrenTab(m)
	m.focusIndex = 1

	m = sendFormKey(m, syntheticSpecialKey(tea.KeyEnter))
	if m.activeOverlay != formOverlayChecklistEdit {
		t.Fatalf("expected checklist edit overlay, got %v", m.activeOverlay)
	}

	m.checklistItemInput.SetValue("")
	m = sendFormKey(m, syntheticSpecialKey(tea.KeyEnter))

	if m.activeOverlay != formOverlayNone {
		t.Error("overlay should be dismissed after Enter")
	}
	if len(m.draft.checklist) != 0 {
		t.Errorf("confirming blank text should remove the item; got %d items: %v", len(m.draft.checklist), m.draft.checklist)
	}
}
