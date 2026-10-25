package notes

import (
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/storage"
)

// TestParseProjectNoteEditorResultLeavesModifiedToTheService keeps the edited
// note's modification time unset on both paths, so the service stamps it from
// its own clock at commit rather than keeping a wall-clock read from here.
func TestParseProjectNoteEditorResultLeavesModifiedToTheService(t *testing.T) {
	created := time.Date(2031, time.February, 3, 4, 5, 6, 0, time.UTC)
	original := &models.Note{
		Title:       "Garden",
		Filename:    models.GenerateNoteFilename("Garden", created),
		CreatedAt:   created,
		ModifiedAt:  created,
		RawRevision: "rev",
	}
	content, err := storage.RenderNoteContent(original)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	for _, create := range []bool{false, true} {
		updated, err := ParseProjectNoteEditorResult(original, content+"\nMore.\n", create)
		if err != nil {
			t.Fatalf("create=%v: %v", create, err)
		}
		if !updated.ModifiedAt.IsZero() {
			t.Errorf("create=%v: ModifiedAt = %s, want unset for the service to stamp", create, updated.ModifiedAt)
		}
		if !create && !updated.CreatedAt.Equal(created) {
			t.Errorf("edit changed CreatedAt to %s, want %s", updated.CreatedAt, created)
		}
	}
}
