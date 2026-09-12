package storage

import (
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

// These tests cover the one field group the three-way merge does not merge field
// by field. A check-in date, its repeat, and the review instant answer one
// question together, so the merge has to adopt or refuse them together; merged
// independently they compose schedules neither machine wrote.

// checkInMergeYear and checkInMergeTaskID are fixed because nothing in this file
// turns on either: the merge reads a date as an instant and a task by identity,
// so varying the year or the id would only restate what the fixtures already say.
const (
	checkInMergeYear   = 2026
	checkInMergeTaskID = "task-1"
)

// checkInMergeDay builds a calendar date the way the model does, so a merged
// value can be compared against what ValidateCheckInSchedule would accept.
func checkInMergeDay(t *testing.T, month time.Month, day int) *time.Time {
	t.Helper()
	value, err := util.StartOfCalendarDate(checkInMergeYear, month, day, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	return &value
}

func checkInMergeTask(t *testing.T, title string, next *time.Time, repeat int, reviewed *time.Time) *models.Task {
	t.Helper()
	task := mergeTestTask(checkInMergeTaskID, title)
	task.NextCheckIn = next
	task.CheckInRepeatDays = repeat
	task.LastReviewedAt = reviewed
	return task
}

// TestCheckInFieldsAreMergedAsOneUnit is the field-inventory half of the
// contract: the three fields stay in mergeableTaskFields, so tasksEqual and every
// other reader still sees a complete field set, and they are all marked as the
// unit mergeTaskFields skips.
func TestCheckInFieldsAreMergedAsOneUnit(t *testing.T) {
	want := map[string]bool{"next_check_in": false, "check_in_repeat_days": false, "last_reviewed_at": false}
	for _, field := range mergeableTaskFields {
		if _, named := want[field.jsonName]; !named {
			continue
		}
		if !field.checkInUnit {
			t.Fatalf("%s is merged field by field, which is what composes a hybrid schedule", field.jsonName)
		}
		want[field.jsonName] = true
	}
	for name, found := range want {
		if !found {
			t.Fatalf("merge field inventory does not include %s", name)
		}
	}
}

// TestMergeAdoptsAnUnopposedCheckInEdit keeps the ordinary case ordinary: one
// side schedules a check-in, the other edits something else, and the merge takes
// both.
func TestMergeAdoptsAnUnopposedCheckInEdit(t *testing.T) {
	next := checkInMergeDay(t, 9, 1)
	reviewed := time.Date(2026, 8, 20, 9, 0, 0, 0, time.UTC)

	base := mergeTestDoc("Sched", mergeTestTask("task-1", "Wait on vendor"))
	local := mergeTestDoc("Sched", checkInMergeTask(t, "Wait on vendor", next, 7, &reviewed))
	current := mergeTestDoc("Sched", mergeTestTask("task-1", "Wait on the vendor"))

	result, err := MergeProjectDocuments(base, local, current)
	if err != nil {
		t.Fatal(err)
	}
	if result.HasConflicts() {
		t.Fatalf("unopposed schedule conflicted: %+v", result.Conflicts)
	}
	merged := result.Merged.Tasks[0]
	if merged.NextCheckIn == nil || !merged.NextCheckIn.Equal(*next) {
		t.Fatalf("merged check-in = %v, want %v", merged.NextCheckIn, next)
	}
	if merged.CheckInRepeatDays != 7 {
		t.Fatalf("merged repeat = %d, want 7", merged.CheckInRepeatDays)
	}
	if merged.LastReviewedAt == nil || !merged.LastReviewedAt.Equal(reviewed) {
		t.Fatalf("merged review instant = %v, want %v", merged.LastReviewedAt, reviewed)
	}
	if merged.Title != "Wait on the vendor" {
		t.Fatalf("merged title = %q, want the unopposed current edit", merged.Title)
	}
	// The merged document becomes live state, so it must hold no pointer either
	// input still owns.
	if merged.NextCheckIn == next || merged.LastReviewedAt == &reviewed {
		t.Fatal("merged task aliases an input document's instants")
	}
}

// TestMergeCoalescesIdenticalCheckInEdits proves two machines that agreed do not
// conflict merely because they both wrote the schedule.
func TestMergeCoalescesIdenticalCheckInEdits(t *testing.T) {
	next := checkInMergeDay(t, 9, 1)
	other := checkInMergeDay(t, 9, 1)

	base := mergeTestDoc("Sched", mergeTestTask("task-1", "Wait"))
	local := mergeTestDoc("Sched", checkInMergeTask(t, "Wait", next, 14, nil))
	current := mergeTestDoc("Sched", checkInMergeTask(t, "Wait", other, 14, nil))

	result, err := MergeProjectDocuments(base, local, current)
	if err != nil {
		t.Fatal(err)
	}
	if result.HasConflicts() {
		t.Fatalf("identical schedules conflicted: %+v", result.Conflicts)
	}
	if merged := result.Merged.Tasks[0]; merged.CheckInRepeatDays != 14 {
		t.Fatalf("merged repeat = %d, want 14", merged.CheckInRepeatDays)
	}
}

// TestMergeRefusesAReviewVersusRescheduleHybrid is the case the unit exists for.
// One machine acknowledges a weekly check-in, which moves the date and stamps the
// review instant; the other reschedules it, which moves the date and stamps
// nothing. Field by field the two diverge only on the date, so a per-field merge
// would keep one side's date beside the other's review instant — a schedule
// neither machine wrote, claiming the task was answered on a day that produced a
// different next date.
func TestMergeRefusesAReviewVersusRescheduleHybrid(t *testing.T) {
	owed := checkInMergeDay(t, 8, 25)
	acknowledged := checkInMergeDay(t, 9, 1)
	rescheduled := checkInMergeDay(t, 10, 15)
	reviewedAt := time.Date(2026, 8, 25, 14, 0, 0, 0, time.UTC)

	base := mergeTestDoc("Sched", checkInMergeTask(t, "Wait", owed, 7, nil))
	local := mergeTestDoc("Sched", checkInMergeTask(t, "Wait", acknowledged, 7, &reviewedAt))
	current := mergeTestDoc("Sched", checkInMergeTask(t, "Wait", rescheduled, 7, nil))

	result, err := MergeProjectDocuments(base, local, current)
	if err != nil {
		t.Fatal(err)
	}
	if result.Merged != nil {
		t.Fatal("a conflicting merge produced a document")
	}
	if len(result.Conflicts) != 1 {
		t.Fatalf("conflicts = %+v, want exactly one", result.Conflicts)
	}
	conflict := result.Conflicts[0]
	if conflict.Kind != MergeConflictFieldEdit || conflict.Field != "check_in" {
		t.Fatalf("conflict = %+v, want one check_in field conflict", conflict)
	}
	// The bundle has to show the whole schedule on every side, or an operator
	// reading it would think the two agreed about the review instant.
	for _, want := range []string{"2026-09-01", "2026-10-15", "every 7d", reviewedAt.Format(time.RFC3339)} {
		if !strings.Contains(conflict.Local+conflict.Current, want) {
			t.Fatalf("conflict does not name %q: %+v", want, conflict)
		}
	}
	if !strings.Contains(conflict.Base, "2026-08-25") {
		t.Fatalf("conflict does not name the base schedule: %+v", conflict)
	}
}

// TestMergeRefusesCompetingScheduleEdits covers the plainer half of the same
// rule: two different reschedules are one conflict, not two.
func TestMergeRefusesCompetingScheduleEdits(t *testing.T) {
	base := mergeTestDoc("Sched", mergeTestTask("task-1", "Wait"))
	local := mergeTestDoc("Sched", checkInMergeTask(t, "Wait", checkInMergeDay(t, 9, 1), 7, nil))
	current := mergeTestDoc("Sched", checkInMergeTask(t, "Wait", checkInMergeDay(t, 9, 2), 30, nil))

	result, err := MergeProjectDocuments(base, local, current)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Conflicts) != 1 || result.Conflicts[0].Field != "check_in" {
		t.Fatalf("conflicts = %+v, want exactly one check_in conflict", result.Conflicts)
	}
}

// TestMergeRefusesAReminderOnTerminalWork is the invariant audit. Completion
// clears the schedule, so a terminal task carrying one can only be composed by a
// merge: one machine finishes the task while the other schedules it. Both edits
// are field-wise clean and land on different fields, so without the audit the
// merge would keep a reminder that fires forever against work nobody can do.
func TestMergeRefusesAReminderOnTerminalWork(t *testing.T) {
	completedAt := time.Date(2026, 8, 26, 9, 0, 0, 0, time.UTC)

	base := mergeTestDoc("Sched", mergeTestTask("task-1", "Wait"))

	finished := mergeTestTask("task-1", "Wait")
	finished.WorkflowState = models.StateDone
	finished.CompletedAt = &completedAt
	local := mergeTestDoc("Sched", finished)

	current := mergeTestDoc("Sched", checkInMergeTask(t, "Wait", checkInMergeDay(t, 9, 1), 7, nil))

	result, err := MergeProjectDocuments(base, local, current)
	if err != nil {
		t.Fatal(err)
	}
	if result.Merged != nil {
		t.Fatal("the merge published a terminal task that still owes a check-in")
	}
	found := false
	for _, conflict := range result.Conflicts {
		if conflict.Kind == MergeConflictInvariant && conflict.Field == "check_in" {
			found = true
			if !strings.Contains(conflict.Detail, "2026-09-01") {
				t.Fatalf("invariant conflict does not name the retained schedule: %+v", conflict)
			}
		}
	}
	if !found {
		t.Fatalf("conflicts = %+v, want a retained-check-in invariant", result.Conflicts)
	}
}

// TestMergeKeepsAReviewInstantOnTerminalWork is the deliberate other half: the
// review instant is history rather than owed work, so finishing a task that was
// reviewed keeps the record of having reviewed it.
func TestMergeKeepsAReviewInstantOnTerminalWork(t *testing.T) {
	completedAt := time.Date(2026, 8, 26, 9, 0, 0, 0, time.UTC)
	reviewedAt := time.Date(2026, 8, 25, 14, 0, 0, 0, time.UTC)

	reviewed := checkInMergeTask(t, "Wait", nil, models.CheckInRepeatOff, &reviewedAt)
	base := mergeTestDoc("Sched", reviewed)

	finished := checkInMergeTask(t, "Wait", nil, models.CheckInRepeatOff, &reviewedAt)
	finished.WorkflowState = models.StateDone
	finished.CompletedAt = &completedAt
	local := mergeTestDoc("Sched", finished)

	current := mergeTestDoc("Sched", checkInMergeTask(t, "Waiting on vendor", nil, models.CheckInRepeatOff, &reviewedAt))

	result, err := MergeProjectDocuments(base, local, current)
	if err != nil {
		t.Fatal(err)
	}
	if result.HasConflicts() {
		t.Fatalf("a finished, reviewed task conflicted: %+v", result.Conflicts)
	}
	if merged := result.Merged.Tasks[0]; merged.LastReviewedAt == nil || !merged.LastReviewedAt.Equal(reviewedAt) {
		t.Fatalf("merged review instant = %v, want it kept", result.Merged.Tasks[0].LastReviewedAt)
	}
}
