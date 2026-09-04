package tui

import (
	"fmt"
	"testing"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/notes"
)

func TestNotesViewPersistentScrollKeepsWindowOnMoveUp(t *testing.T) {
	manager := make([]*models.Note, 0, 8)
	for i := range 8 {
		manager = append(manager, &models.Note{
			Title:    fmt.Sprintf("Note %02d", i),
			Filename: fmt.Sprintf("note-%02d.md", i),
		})
	}

	rows := make([]notes.LocatedNoteRow, len(manager))
	for i := range manager {
		rows[i] = notes.LocatedNoteRow{Note: manager[i], Location: app.GlobalNoteLocation(), OwnerLabel: "Global"}
	}
	m := &notes.NotesViewModel{VisibleRows: rows}
	m.SetSize(80, 10)

	m.Cursor = len(rows) - 1
	_ = m.View()
	start := m.ScrollOffset
	if start == 0 {
		t.Fatalf("expected manager list to scroll down, offset stayed at 0")
	}

	m.Cursor--
	_ = m.View()
	if m.ScrollOffset != start {
		t.Fatalf("moving up inside viewport changed first rendered row: offset=%d want %d", m.ScrollOffset, start)
	}
}
