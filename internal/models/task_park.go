package models

import (
	"fmt"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/util"
)

// Park stores a resume note for an interrupted task. Re-parking a task preserves
// the previous note as a timestamped comment before replacing it.
func (t *Task) Park(note string) {
	if t.IsParked() {
		t.rollParkedNoteToComment()
	}
	now := time.Now()
	t.ParkedNote = strings.TrimSpace(note)
	t.ParkedAt = &now
	t.UpdatedAt = now
}

// Unpark clears the parked state. When rollToComment is true, the current note
// is preserved as a timestamped comment before it is cleared.
func (t *Task) Unpark(rollToComment bool) {
	if !t.IsParked() {
		return
	}
	if rollToComment {
		t.rollParkedNoteToComment()
	}
	t.ParkedNote = ""
	t.ParkedAt = nil
	t.UpdatedAt = time.Now()
}

// IsParked reports whether the task carries an active parking marker.
func (t *Task) IsParked() bool {
	return t.ParkedAt != nil || strings.TrimSpace(t.ParkedNote) != ""
}

func (t *Task) rollParkedNoteToComment() {
	note := strings.TrimSpace(t.ParkedNote)
	if note == "" {
		return
	}
	parkedAt := time.Now()
	if t.ParkedAt != nil {
		parkedAt = *t.ParkedAt
	}
	t.AddComment(fmt.Sprintf("[parked %s] %s", parkedAt.Format(util.DateTimeFormat), note))
}
