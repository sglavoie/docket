package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/models"
)

func newTestNotesView(notes []*models.Note) *NotesViewModel {
	project := &models.Project{Name: "test"}
	m := &NotesViewModel{
		project:       project,
		notes:         notes,
		filterTags:    make(map[string]bool),
		selectedNotes: make(map[string]bool),
	}
	m.SetSize(80, 40)
	return m
}

func sampleNotes() []*models.Note {
	return []*models.Note{
		{Title: "Note A", Filename: "a.md"},
		{Title: "Note B", Filename: "b.md"},
		{Title: "Note C", Filename: "c.md"},
	}
}

func TestCursorMovement(t *testing.T) {
	m := newTestNotesView(sampleNotes())

	// j moves down
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if m.cursor != 1 {
		t.Errorf("expected cursor=1 after j, got %d", m.cursor)
	}

	// k moves up
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	if m.cursor != 0 {
		t.Errorf("expected cursor=0 after k, got %d", m.cursor)
	}

	// k at top stays at 0
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	if m.cursor != 0 {
		t.Errorf("expected cursor=0 at top, got %d", m.cursor)
	}

	// G goes to bottom
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("G")})
	if m.cursor != 2 {
		t.Errorf("expected cursor=2 after G, got %d", m.cursor)
	}

	// j at bottom stays
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if m.cursor != 2 {
		t.Errorf("expected cursor=2 at bottom, got %d", m.cursor)
	}
}

func TestMultiSelectToggle(t *testing.T) {
	m := newTestNotesView(sampleNotes())

	// V enters multi-select
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("V")})
	if !m.multiSelectMode {
		t.Error("expected multi-select mode after V")
	}

	// space toggles selection and moves cursor down
	m.Update(tea.KeyMsg{Type: tea.KeySpace})
	if !m.selectedNotes["a.md"] {
		t.Error("expected a.md selected after space")
	}
	if m.cursor != 1 {
		t.Errorf("expected cursor=1 after space, got %d", m.cursor)
	}

	// 'a' selects all
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	if len(m.selectedNotes) != 3 {
		t.Errorf("expected 3 selected after 'a', got %d", len(m.selectedNotes))
	}

	// V exits multi-select
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("V")})
	if m.multiSelectMode {
		t.Error("expected multi-select off after second V")
	}
}

func TestSortCycling(t *testing.T) {
	m := newTestNotesView(sampleNotes())

	if m.sortMode != noteSortModified {
		t.Errorf("expected initial sort=modified, got %v", m.sortMode)
	}

	m.cycleSortMode()
	if m.sortMode != noteSortCreated {
		t.Errorf("expected sort=created after first cycle, got %v", m.sortMode)
	}

	m.cycleSortMode()
	if m.sortMode != noteSortTitleAZ {
		t.Errorf("expected sort=titleAZ after second cycle, got %v", m.sortMode)
	}

	m.cycleSortMode()
	if m.sortMode != noteSortTitleZA {
		t.Errorf("expected sort=titleZA after third cycle, got %v", m.sortMode)
	}

	m.cycleSortMode()
	if m.sortMode != noteSortModified {
		t.Errorf("expected sort=modified after fourth cycle, got %v", m.sortMode)
	}
}

func TestRenameFlowCancel(t *testing.T) {
	m := newTestNotesView(sampleNotes())

	m.startRename()
	if !m.renaming {
		t.Error("expected renaming=true after startRename")
	}
	if m.renameInput.Value() != "Note A" {
		t.Errorf("expected rename input value='Note A', got %q", m.renameInput.Value())
	}

	// Esc cancels
	m.handleRenameKey(tea.KeyMsg{Type: tea.KeyEscape})
	if m.renaming {
		t.Error("expected renaming=false after Esc")
	}
	if m.statusMessage != "Rename canceled" {
		t.Errorf("expected 'Rename canceled' status, got %q", m.statusMessage)
	}
}

func TestPendingGKeyConsumed(t *testing.T) {
	m := newTestNotesView(sampleNotes())

	// Move cursor down first
	m.cursor = 2

	// Press g then d — should jump to top, NOT trigger delete
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	if !m.pendingGKey {
		t.Error("expected pendingGKey after g")
	}

	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	if m.pendingGKey {
		t.Error("expected pendingGKey=false after second key")
	}
	if m.cursor != 0 {
		t.Errorf("expected cursor=0 after gd (jump to top), got %d", m.cursor)
	}
	if m.confirmDelete {
		t.Error("expected no delete confirmation after gd — second key should be consumed")
	}
}

func TestPendingGKeyDeselectAll(t *testing.T) {
	m := newTestNotesView(sampleNotes())
	m.multiSelectMode = true
	m.selectedNotes = map[string]bool{"a.md": true, "b.md": true}

	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("X")})

	if len(m.selectedNotes) != 0 {
		t.Error("expected all deselected after gX")
	}
}
