package app

import (
	"fmt"

	"github.com/sglavoie/tt/internal/models"
)

// ResumeNoteResult reports what SetResumeNote did. Changed is false when the
// requested note already equalled the stored one, in which case Task carries the
// resolved identity and nothing was committed: a no-op edit must not advance
// UpdatedAt, mint a transaction, or push an undo entry a later `u` would spend
// on nothing.
type ResumeNoteResult struct {
	Task    TaskResult
	Changed bool
}

// SetResumeNote writes, replaces, or clears one task's resume note. The empty
// note is the only spelling of "clear", matching the model's absence rule, so
// this is a single entry point rather than a set/clear pair.
//
// The note is validated, never repaired: models.ValidateResumeNote is the one
// gate, and a caller collecting text from a human trims before calling. Every
// lifecycle state is writable and no lifecycle clock, planning membership, or
// other field is touched, because a resume note is orientation beside the work
// rather than a step in it.
func (s *Service) SetResumeNote(locator TaskLocator, note string) (ResumeNoteResult, error) {
	if err := models.ValidateResumeNote(note); err != nil {
		return ResumeNoteResult{}, invalidInput("resume note", err.Error())
	}
	ref, _, revision, err := s.loadTaskSelection("", locator)
	if err != nil {
		return ResumeNoteResult{}, err
	}
	if ref.Task.ResumeNote == note {
		return ResumeNoteResult{Task: newTaskResult(ref, revision, "")}, nil
	}

	action := "Set Resume Note"
	if note == "" {
		action = "Clear Resume Note"
	}
	undoContext := fmt.Sprintf("#%d %s", ref.Task.ShortID, ref.Task.Title)
	ref.Task.ResumeNote = note
	ref.Task.UpdatedAt = s.clock.Now()
	result, err := s.commitProject(ref, revision, action, undoContext)
	if err != nil {
		return ResumeNoteResult{}, err
	}
	return ResumeNoteResult{Task: result, Changed: true}, nil
}
