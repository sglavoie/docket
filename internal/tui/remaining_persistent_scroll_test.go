package tui

import (
	"fmt"
	"testing"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
)

func TestNotesViewPersistentScrollKeepsWindowOnMoveUp(t *testing.T) {
	notes := make([]*models.Note, 0, 8)
	for i := range 8 {
		notes = append(notes, &models.Note{
			Title:    fmt.Sprintf("Note %02d", i),
			Filename: fmt.Sprintf("note-%02d.md", i),
		})
	}

	rows := make([]LocatedNoteRow, len(notes))
	for i := range notes {
		rows[i] = LocatedNoteRow{Note: notes[i], Location: app.GlobalNoteLocation(), OwnerLabel: "Global"}
	}
	m := &NotesViewModel{visibleRows: rows}
	m.SetSize(80, 10)

	m.cursor = 5
	_ = m.renderListPane()
	start := m.scrollOffset
	if start == 0 {
		t.Fatalf("expected notes list to scroll down, offset stayed at 0")
	}

	m.cursor = 4
	_ = m.renderListPane()
	if m.scrollOffset != start {
		t.Fatalf("moving up inside viewport changed first rendered row: offset=%d want %d", m.scrollOffset, start)
	}
}
