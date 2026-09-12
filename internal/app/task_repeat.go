package app

import (
	"github.com/sglavoie/tt/internal/models"
)

// TaskRepeatDraft is the read-only starting point for repeating a task: the
// fields a repetition retains, plus the destination the new task starts in. It
// is a value, not a view of the source task — every slice is copied on
// preparation — so a draft handed to a surface stays what it was even if the
// source is edited or deleted afterwards.
//
// What is absent is the contract as much as what is present. There is no
// identity, no timestamp, no parent, no checklist origin, no due date, no deferral, no planning
// membership, no pin or review flag, no resume note, no comment, no attachment,
// and no dependency: a repetition is fresh work carrying the same shape, and
// every one of those fields describes the particular occurrence that is being
// repeated rather than the shape of it. ChecklistItems carries text alone for
// the same reason — a repeated checklist starts unticked, and a struct that
// cannot spell a tick cannot leak one.
//
// The source project is absent too, and for a different reason: the caller
// named it in the locator it passed, so echoing it back would be a value the
// surface already holds rather than one the draft decides. A repetition never
// crosses a project boundary on its own, and that is enforced where the
// destination project is chosen, not by a field nothing reads.
type TaskRepeatDraft struct {
	// Destination is always CaptureDestinationBacklog. It is carried rather
	// than assumed so a surface seeds one value instead of restating the rule.
	Destination CaptureDestination

	Title       string
	Description string
	Priority    int
	Tags        []string
	Context     string
	// ChecklistItems is the source checklist's text in its stored order.
	ChecklistItems []string
}

// PrepareTaskRepeatDraft reads one task and returns the draft a repetition of it
// starts from. It writes nothing: no transaction, no undo entry, no touch time.
// Opening a repeat draft that is then abandoned must leave the workspace byte
// for byte as it was, so the preparation is a pure read and the creation
// transaction stays the one and only write.
//
// Every lifecycle state repeats, terminal ones included: finishing a task is the
// usual reason to want another like it. The project must be writable — an
// archived owner refuses here as it would for any mutation — but a parked one
// does not, since parking hides a project from the default population rather
// than sealing it.
func (s *Service) PrepareTaskRepeatDraft(locator TaskLocator) (TaskRepeatDraft, error) {
	ref, _, _, err := s.loadTaskSelection("", locator)
	if err != nil {
		return TaskRepeatDraft{}, err
	}
	if ref.Project.Archived {
		return TaskRepeatDraft{}, invalidInput("project", "project is archived; restore it to add tasks")
	}
	return newTaskRepeatDraft(ref.Task), nil
}

// newTaskRepeatDraft copies the retained fields off a source task. It is split
// out so the copying rule has one home and the tests can state it directly.
func newTaskRepeatDraft(task *models.Task) TaskRepeatDraft {
	draft := TaskRepeatDraft{
		Destination: CaptureDestinationBacklog,
		Title:       task.Title,
		Description: task.Description,
		Priority:    task.Priority,
		Context:     task.Context,
	}
	if len(task.Tags) > 0 {
		draft.Tags = append([]string(nil), task.Tags...)
	}
	if len(task.Checklist) > 0 {
		draft.ChecklistItems = make([]string, 0, len(task.Checklist))
		for _, item := range task.Checklist {
			draft.ChecklistItems = append(draft.ChecklistItems, item.Text)
		}
	}
	return draft
}
