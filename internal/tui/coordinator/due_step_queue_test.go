package coordinator

import (
	"errors"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/app"
)

func dueStepGesture(direction app.DueStepDirection, taskID string) DueStepGesture {
	return DueStepGesture{
		Direction: direction,
		Targets:   []app.TaskLocator{{ProjectID: "project", TaskID: taskID}},
	}
}

// TestDueStepQueueRunsOneGestureAtATime is the serialization rule: the first
// keypress dispatches, every one behind it waits, and a successor is handed back
// only when the one in flight settles.
func TestDueStepQueueRunsOneGestureAtATime(t *testing.T) {
	var queue DueStepQueue

	first, dispatch := queue.Enqueue(dueStepGesture(app.DueStepForward, "one"))
	if !dispatch {
		t.Fatal("the first gesture did not dispatch against an idle queue")
	}

	if _, dispatch := queue.Enqueue(dueStepGesture(app.DueStepForward, "two")); dispatch {
		t.Fatal("a second gesture dispatched while the first was in flight")
	}
	if _, dispatch := queue.Enqueue(dueStepGesture(app.DueStepBackward, "three")); dispatch {
		t.Fatal("a third gesture dispatched while the first was in flight")
	}
	if got := queue.Pending(); got != 2 {
		t.Fatalf("pending = %d, want 2", got)
	}

	next, hasNext, current := queue.Settle(first)
	if !current || !hasNext {
		t.Fatalf("settling the in-flight gesture reported current=%v hasNext=%v", current, hasNext)
	}
	if next.Targets[0].TaskID != "two" {
		t.Fatalf("the successor names %q, want the gesture typed second", next.Targets[0].TaskID)
	}

	// Arrival order is the whole contract: the third gesture is still waiting for
	// the second, which has only just been armed.
	second := queue.Arm()
	if second == first {
		t.Fatal("the successor reused the sequence of the gesture it followed")
	}
	next, hasNext, current = queue.Settle(second)
	if !current || !hasNext || next.Targets[0].TaskID != "three" {
		t.Fatalf("settling the second gesture yielded %+v (hasNext=%v current=%v)", next, hasNext, current)
	}

	third := queue.Arm()
	if _, hasNext, current = queue.Settle(third); !current || hasNext {
		t.Fatalf("the last gesture reported hasNext=%v current=%v", hasNext, current)
	}
	if got := queue.Pending(); got != 0 {
		t.Fatalf("pending = %d after draining, want 0", got)
	}
}

// TestDueStepQueueIgnoresStaleDeliveries holds the stamp's whole purpose: a
// result belonging to a superseded gesture must move nothing, and must not
// revive a queue some other act already emptied.
func TestDueStepQueueIgnoresStaleDeliveries(t *testing.T) {
	var queue DueStepQueue

	sequence, _ := queue.Enqueue(dueStepGesture(app.DueStepForward, "one"))
	queue.Enqueue(dueStepGesture(app.DueStepForward, "two"))
	queue.Supersede()
	if got := queue.Pending(); got != 0 {
		t.Fatalf("pending = %d after superseding, want the tail dropped", got)
	}

	if _, hasNext, current := queue.Settle(sequence); current || hasNext {
		t.Fatalf("a superseded delivery settled as current=%v hasNext=%v", current, hasNext)
	}
	if dropped, current := queue.Abandon(sequence); current || dropped != 0 {
		t.Fatalf("a superseded refusal abandoned %d gestures as current=%v", dropped, current)
	}

	// The queue still takes new work, and the fresh gesture dispatches rather
	// than waiting behind a gesture nothing is going to settle.
	if _, dispatch := queue.Enqueue(dueStepGesture(app.DueStepBackward, "three")); !dispatch {
		t.Fatal("the queue stayed busy after being superseded")
	}
}

// TestDueStepQueueSupersedesPendingWorkApartFromCommittedEvidence holds the
// separation the two supersede forms exist for. Either form drops the whole
// pending tail, because a queued step is computed from a committed state that
// has moved. What differs is what the detached gesture's own commit may still
// say: an edit names the tasks it overrode and no others, while a restore names
// none and so overrides everything.
func TestDueStepQueueSupersedesPendingWorkApartFromCommittedEvidence(t *testing.T) {
	stepped := app.TaskLocator{ProjectID: "project", TaskID: "stepped"}
	edited := app.TaskLocator{ProjectID: "project", TaskID: "edited"}
	cleared := time.Date(2026, time.September, 11, 14, 30, 0, 0, time.Local)
	effect := CommittedDueEffect{Steps: []app.DueStepOutcome{{Target: stepped, Before: &cleared}}}

	var queue DueStepQueue
	sequence, _ := queue.Enqueue(dueStepGesture(app.DueStepBackward, "stepped"))
	queue.Enqueue(dueStepGesture(app.DueStepBackward, "stepped"))
	queue.SupersedeTargets([]app.TaskLocator{edited})
	if got := queue.Pending(); got != 0 {
		t.Fatalf("pending = %d after an edit superseded the queue, want the tail dropped", got)
	}
	retained := queue.Superseded(sequence).Retain(effect)
	if len(retained.Steps) != 1 || retained.Steps[0].Target != stepped {
		t.Fatalf("an edit to a different task discarded the step's own evidence: %+v", retained.Steps)
	}

	// The same edit naming the stepped task is the override it always was.
	var owned DueStepQueue
	sequence, _ = owned.Enqueue(dueStepGesture(app.DueStepBackward, "stepped"))
	owned.SupersedeTargets([]app.TaskLocator{stepped})
	if retained := owned.Superseded(sequence).Retain(effect); len(retained.Steps) != 0 {
		t.Fatalf("an edit to the stepped task kept its evidence: %+v", retained.Steps)
	}

	// A ledger restore names nothing, so it keeps blanking the whole effect.
	var restored DueStepQueue
	sequence, _ = restored.Enqueue(dueStepGesture(app.DueStepBackward, "stepped"))
	restored.Supersede()
	if retained := restored.Superseded(sequence).Retain(effect); len(retained.Steps) != 0 {
		t.Fatalf("a ledger restore kept committed step evidence: %+v", retained.Steps)
	}
}

// TestDueStepQueueAccumulatesEverySupersessionADeliveryOutlived keeps the record
// honest across more than one supersede. A detached result can sit in flight
// while several edits commit, and each of them names evidence the ones before it
// did not; a delivery the queue has no record of at all is trusted with none.
func TestDueStepQueueAccumulatesEverySupersessionADeliveryOutlived(t *testing.T) {
	first := app.TaskLocator{ProjectID: "project", TaskID: "first"}
	second := app.TaskLocator{ProjectID: "project", TaskID: "second"}
	spared := app.TaskLocator{ProjectID: "project", TaskID: "spared"}
	effect := CommittedDueEffect{Steps: []app.DueStepOutcome{
		{Target: first}, {Target: second}, {Target: spared},
	}}

	var queue DueStepQueue
	sequence, _ := queue.Enqueue(dueStepGesture(app.DueStepBackward, "bulk"))
	queue.SupersedeTargets([]app.TaskLocator{first})
	queue.SupersedeTargets([]app.TaskLocator{second})

	retained := queue.Superseded(sequence).Retain(effect)
	if len(retained.Steps) != 1 || retained.Steps[0].Target != spared {
		t.Fatalf("retained %+v, want only the member no edit named", retained.Steps)
	}

	// The record is spent by that delivery, and an unaccounted sequence is
	// wholly superseded rather than trusted.
	if retained := queue.Superseded(sequence).Retain(effect); len(retained.Steps) != 0 {
		t.Fatalf("a second delivery of one sequence kept evidence: %+v", retained.Steps)
	}
	if retained := queue.Superseded(0).Retain(effect); len(retained.Steps) != 0 {
		t.Fatalf("an unrecognized delivery kept evidence: %+v", retained.Steps)
	}
}

// TestDueStepSupersessionNarrowsStepsAlone pins which evidence a supersession
// may touch. Only a step writes clock memory back; an edit, a removal, and an
// address change forget or re-key one, so narrowing them would lose bookkeeping
// without protecting anything.
func TestDueStepSupersessionNarrowsStepsAlone(t *testing.T) {
	stepped := app.TaskLocator{ProjectID: "project", TaskID: "stepped"}
	moved := app.TaskLocator{ProjectID: "other", TaskID: "stepped"}
	effect := CommittedDueEffect{
		Steps:   []app.DueStepOutcome{{Target: stepped}},
		Edits:   []app.TaskLocator{stepped},
		Moves:   []app.TaskAddressChange{{From: stepped, To: moved}},
		Removed: []app.TaskLocator{stepped},
	}

	var queue DueStepQueue
	sequence, _ := queue.Enqueue(dueStepGesture(app.DueStepBackward, "stepped"))
	queue.SupersedeTargets([]app.TaskLocator{stepped})

	retained := queue.Superseded(sequence).Retain(effect)
	if len(retained.Steps) != 0 {
		t.Fatalf("the superseded step survived: %+v", retained.Steps)
	}
	if len(retained.Edits) != 1 || len(retained.Moves) != 1 || len(retained.Removed) != 1 {
		t.Fatalf("narrowing dropped evidence that only forgets or re-keys: %+v", retained)
	}
}

// TestDueStepQueueAbandonsTheTailOnRefusal is the failure rule: the pending
// sequence was frozen against a premise the refusal disproved, so it is dropped
// and counted rather than retried against whatever the view shows now.
func TestDueStepQueueAbandonsTheTailOnRefusal(t *testing.T) {
	var queue DueStepQueue

	sequence, _ := queue.Enqueue(dueStepGesture(app.DueStepForward, "one"))
	queue.Enqueue(dueStepGesture(app.DueStepForward, "two"))
	queue.Enqueue(dueStepGesture(app.DueStepForward, "three"))

	dropped, current := queue.Abandon(sequence)
	if !current || dropped != 2 {
		t.Fatalf("abandon reported %d dropped (current=%v), want 2", dropped, current)
	}
	if got := queue.Pending(); got != 0 {
		t.Fatalf("pending = %d after a refusal, want 0", got)
	}
	if _, dispatch := queue.Enqueue(dueStepGesture(app.DueStepForward, "four")); !dispatch {
		t.Fatal("the queue stayed busy after a refusal")
	}
}

// TestDueStepQueueToleratesNoReceiver keeps every method safe on the nil pointer
// a partially wired shell can hand it.
func TestDueStepQueueToleratesNoReceiver(t *testing.T) {
	var queue *DueStepQueue
	if _, dispatch := queue.Enqueue(dueStepGesture(app.DueStepForward, "one")); dispatch {
		t.Fatal("a nil queue dispatched a gesture")
	}
	if got := queue.Arm(); got != 0 {
		t.Fatalf("a nil queue armed sequence %d", got)
	}
	if _, hasNext, current := queue.Settle(1); hasNext || current {
		t.Fatal("a nil queue settled a gesture")
	}
	if dropped, current := queue.Abandon(1); dropped != 0 || current {
		t.Fatal("a nil queue abandoned a gesture")
	}
	queue.Supersede()
	queue.SupersedeTargets([]app.TaskLocator{{ProjectID: "project", TaskID: "one"}})
	if retained := queue.Superseded(1).Retain(CommittedDueEffect{
		Steps: []app.DueStepOutcome{{Target: app.TaskLocator{ProjectID: "project", TaskID: "one"}}},
	}); len(retained.Steps) != 0 {
		t.Fatalf("a nil queue vouched for a delivery it never armed: %+v", retained.Steps)
	}
	if got := queue.Pending(); got != 0 {
		t.Fatalf("a nil queue reports %d pending", got)
	}
}

// TestDueStepSummaryNamesWhatLanded pins the reporting grammar: one task reads
// back its own deadline, a selection reads back counts, and the members the
// projection hid are named rather than silently included.
func TestDueStepSummaryNamesWhatLanded(t *testing.T) {
	day := time.Date(2026, time.September, 12, 0, 0, 0, 0, time.Local)
	timed := time.Date(2026, time.September, 12, 14, 30, 0, 0, time.Local)

	cases := []struct {
		name     string
		outcomes []app.DueStepOutcome
		hidden   int
		want     string
	}{
		{name: "nothing stepped", want: "Step Due Date"},
		{
			name:     "one date only deadline",
			outcomes: []app.DueStepOutcome{{After: &day}},
			want:     "Due 2026-09-12",
		},
		{
			name:     "one deadline carrying a clock",
			outcomes: []app.DueStepOutcome{{After: &timed}},
			want:     "Due 2026-09-12 14:30",
		},
		{
			name:     "one deadline cleared",
			outcomes: []app.DueStepOutcome{{}},
			want:     "Due date cleared",
		},
		{
			name:     "a selection with a hidden member",
			outcomes: []app.DueStepOutcome{{After: &day}, {After: &timed}, {}},
			hidden:   1,
			want:     "3 stepped · 2 set · 1 cleared · 1 hidden",
		},
		{
			name:     "a selection the projection showed whole",
			outcomes: []app.DueStepOutcome{{After: &day}, {After: &day}},
			want:     "2 stepped · 2 set · 0 cleared",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := DueStepSummary(tc.outcomes, tc.hidden); got != tc.want {
				t.Fatalf("DueStepSummary = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestDueStepRefusalCountsOnlyUnexecutedWork keeps the refusal honest: it says
// why the application layer refused, and adds a count only when a press the user
// made never ran.
func TestDueStepRefusalCountsOnlyUnexecutedWork(t *testing.T) {
	refused := errors.New("terminal tasks cannot be added to Today")

	if got := DueStepRefusal(nil, 3); got != nil {
		t.Fatalf("DueStepRefusal wrapped a missing error as %v", got)
	}
	if got := DueStepRefusal(refused, 0); !errors.Is(got, refused) || got.Error() != refused.Error() {
		t.Fatalf("DueStepRefusal rewrote a refusal with an empty queue as %v", got)
	}

	wrapped := DueStepRefusal(refused, 2)
	if !errors.Is(wrapped, refused) {
		t.Fatalf("the refusal %v no longer carries the application layer's reason", wrapped)
	}
	if got, want := wrapped.Error(), refused.Error()+" · 2 queued deadline step(s) not applied"; got != want {
		t.Fatalf("refusal = %q, want %q", got, want)
	}
}
