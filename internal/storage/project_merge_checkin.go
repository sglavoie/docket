package storage

import (
	"fmt"
	"time"

	"github.com/sglavoie/docket/internal/models"
)

// taskCheckInUnit is the check-in trio read off one task as a single value: the
// date the task is next owed, the cadence that moves it, and the instant it was
// last answered. It exists only for the merge, which is the one place the three
// have to be compared and adopted together.
type taskCheckInUnit struct {
	next           *time.Time
	repeatDays     int
	lastReviewedAt *time.Time
}

func checkInUnitOf(task *models.Task) taskCheckInUnit {
	return taskCheckInUnit{
		next:           task.NextCheckIn,
		repeatDays:     task.CheckInRepeatDays,
		lastReviewedAt: task.LastReviewedAt,
	}
}

// equals compares two units by what they mean rather than by their in-memory
// shape. Two pointers holding the same instant in different locations are the
// same schedule, so they are compared with Equal; reflect.DeepEqual, which the
// per-field path uses, would call that a divergent edit and conflict two machines
// that agreed.
func (u taskCheckInUnit) equals(other taskCheckInUnit) bool {
	return u.repeatDays == other.repeatDays &&
		sameOptionalInstant(u.next, other.next) &&
		sameOptionalInstant(u.lastReviewedAt, other.lastReviewedAt)
}

func sameOptionalInstant(left, right *time.Time) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return left.Equal(*right)
}

// String renders the unit for a conflict bundle a person reads. All three values
// appear even when only one diverged, because the conflict is about the schedule
// as a whole and showing one line of it would suggest the other two were agreed.
func (u taskCheckInUnit) String() string {
	next := "none"
	if u.next != nil {
		next = u.next.Format("2006-01-02")
	}
	repeat := "off"
	if u.repeatDays > models.CheckInRepeatOff {
		repeat = fmt.Sprintf("every %dd", u.repeatDays)
	}
	reviewed := "never"
	if u.lastReviewedAt != nil {
		reviewed = u.lastReviewedAt.Format(time.RFC3339)
	}
	return fmt.Sprintf("check-in %s, repeat %s, reviewed %s", next, repeat, reviewed)
}

// mergeCheckInUnit merges the check-in date, its repeat, and the review instant
// as one value, and is why a concurrent acknowledgement and a concurrent
// reschedule cannot compose a schedule neither machine wrote.
//
// Merging the three independently would do exactly that: acknowledging a weekly
// check-in moves the date forward and stamps the review instant, while
// rescheduling moves the date and leaves the instant alone. Field by field, the
// two sides diverge only on the date, so one side's date would be adopted beside
// the other side's review instant — a hybrid claiming the task was answered on a
// day that produced a different next date. Compared as a unit, the same pair is
// one conflict a person resolves by choosing a schedule.
//
// Both sides making the same change still coalesces, and a side that left the
// unit at base still loses to the side that changed it, so an ordinary edit to
// an unrelated field never drags the schedule into a conflict.
//
// The adopted instants are cloned. The merged document becomes live state, and
// SetCheckIn replaces these pointers rather than writing through them, but a
// caller reaching into the merged task directly must still not be able to rewrite
// the base or current document the merge read from.
func mergeCheckInUnit(merged, base, local, current *models.Task) *MergeConflict {
	baseUnit := checkInUnitOf(base)
	localUnit := checkInUnitOf(local)
	currentUnit := checkInUnitOf(current)

	localChanged := !localUnit.equals(baseUnit)
	currentChanged := !currentUnit.equals(baseUnit)

	adopt := baseUnit
	switch {
	case localChanged && currentChanged:
		if !localUnit.equals(currentUnit) {
			conflict := fieldEditConflict(
				mergedTitle(local, current), "check_in", baseUnit, localUnit, currentUnit,
			)
			return &conflict
		}
		adopt = localUnit
	case localChanged:
		adopt = localUnit
	case currentChanged:
		adopt = currentUnit
	}

	merged.NextCheckIn = models.CloneInstant(adopt.next)
	merged.CheckInRepeatDays = adopt.repeatDays
	merged.LastReviewedAt = models.CloneInstant(adopt.lastReviewedAt)
	return nil
}

// auditMergedDocument re-runs the structural audit and state
// invariants on a merged document, returning conflicts for any violation. It
// never mutates the merged tasks (unlike BuildTaskTree, which silently repairs),
// so a genuine invariant break surfaces instead of being papered over.
func auditMergedDocument(doc *ProjectDocument) []MergeConflict {
	project := &models.Project{
		Name:     doc.ProjectName,
		Tasks:    doc.Tasks,
		Comments: doc.Comments,
	}

	var conflicts []MergeConflict
	if err := models.AuditProject(project); err != nil {
		conflicts = append(conflicts, MergeConflict{
			Kind:   MergeConflictInvariant,
			Field:  "audit",
			Detail: err.Error(),
		})
	}

	// ShortID uniqueness is the one project-local invariant the canonical writer
	// enforces that models.AuditProject does not, and it is precisely the one two
	// concurrent creations break: each machine mints the next free short id
	// against a document that has not seen the other's task, so a field-wise clean
	// merge keeping both tasks keeps two copies of one number. Auditing it here is
	// what makes that merge a recorded conflict the loser can retry, rather than a
	// render failure raised at the commit boundary — which named no bundle, so the
	// losing machine had nothing to resolve and nothing to show.
	shortIDs := make(map[int]*models.Task, len(doc.Tasks))
	for _, task := range doc.Tasks {
		if existing, clash := shortIDs[task.ShortID]; clash {
			conflicts = append(conflicts, MergeConflict{
				Kind:      MergeConflictInvariant,
				TaskTitle: task.Title,
				Field:     "short_id",
				Detail:    fmt.Sprintf("short id %d is already used by %q", task.ShortID, existing.Title),
			})
			continue
		}
		shortIDs[task.ShortID] = task
	}

	byID := indexTasks(doc.Tasks)
	for _, task := range doc.Tasks {
		terminal := task.IsTerminalState()
		// A concurrent completion and child creation/reparenting can each be
		// valid in isolation but leave unfinished work under a terminal parent.
		// Judge the stored edges, not computed Children links from an input tree,
		// and record a conflict rather than silently undoing either edit.
		if !terminal && task.ParentID != nil {
			if parent := byID[*task.ParentID]; parent != nil && parent.IsTerminalState() {
				conflicts = append(conflicts, MergeConflict{
					Kind:      MergeConflictInvariant,
					TaskTitle: task.Title,
					Field:     "parent_id",
					Detail:    fmt.Sprintf("non-terminal task has %s parent %q", parent.State(), parent.Title),
				})
			}
		}
		// State invariant: a completion timestamp exists exactly for terminal
		// states. A field-wise clean merge can still combine a terminal state from
		// one side with a nil (or lingering) timestamp from the other.
		if terminal && task.CompletedAt == nil {
			conflicts = append(conflicts, MergeConflict{
				Kind:      MergeConflictInvariant,
				TaskTitle: task.Title,
				Field:     "completed_at",
				Detail:    fmt.Sprintf("terminal state %q has no completion timestamp", task.State()),
			})
		}
		if !terminal && task.CompletedAt != nil {
			conflicts = append(conflicts, MergeConflict{
				Kind:      MergeConflictInvariant,
				TaskTitle: task.Title,
				Field:     "completed_at",
				Detail:    fmt.Sprintf("non-terminal state %q carries a completion timestamp", task.State()),
			})
		}
		// State invariant: terminal work owes no check-in. The lifecycle clears the
		// schedule the moment a task finishes, so a terminal task carrying one can
		// only be composed here — one machine finishing the task while the other
		// schedules or reschedules it. Field-wise both edits are clean and land on
		// different fields, so without this the merge would keep a reminder that
		// would fire forever against work nobody can do.
		//
		// The review instant is deliberately not audited: it is history rather than
		// owed work, and a task finished after being reviewed keeps it on purpose.
		if terminal && !task.CheckIn().IsEmpty() {
			conflicts = append(conflicts, MergeConflict{
				Kind:      MergeConflictInvariant,
				TaskTitle: task.Title,
				Field:     "check_in",
				Detail: fmt.Sprintf("terminal state %q retains a check-in schedule (%s)",
					task.State(), checkInUnitOf(task)),
			})
		}
	}

	return conflicts
}

// mergeScalarMetadata three-way merges a single string project-metadata field,
// returning the merged value and a conflict when both sides diverged.
func mergeScalarMetadata(field, base, local, current string) (string, *MergeConflict) {
	value, conflicted := scalarThreeWay(base, local, current)
	if !conflicted {
		return value, nil
	}
	return "", &MergeConflict{
		Kind:    MergeConflictProjectMetadata,
		Field:   field,
		Base:    base,
		Local:   local,
		Current: current,
	}
}

// --- small helpers -------------------------------------------------------
