package models

import (
	"testing"
	"time"
)

func TestQueryAdmitsTheDerivedTaskFlags(t *testing.T) {
	now := queryTestNow()
	project := queryTestProject()
	parentID := "task-parent"
	openChild := &Task{ID: "task-child", WorkflowState: StateBacklog}
	doneChild := &Task{ID: "task-child", WorkflowState: StateDone}

	cases := []struct {
		name   string
		field  QueryField
		mutate func(*Task)
		admits bool
	}{
		{"pinned matches a pinned task", QueryFieldPinned, func(task *Task) { task.Pinned = true }, true},
		{"pinned refuses an unpinned task", QueryFieldPinned, func(*Task) {}, false},
		{"stale matches untouched open work", QueryFieldStale, func(task *Task) {
			task.UpdatedAt = now.AddDate(0, 0, -StaleTaskThresholdDays)
		}, true},
		{"stale refuses work touched inside the window", QueryFieldStale, func(task *Task) {
			task.UpdatedAt = now.AddDate(0, 0, -(StaleTaskThresholdDays - 1))
		}, false},
		{"stale refuses terminal work", QueryFieldStale, func(task *Task) {
			task.UpdatedAt = now.AddDate(0, 0, -365)
			task.WorkflowState = StateDone
		}, false},
		{"stale refuses deferred work", QueryFieldStale, func(task *Task) {
			task.UpdatedAt = now.AddDate(0, 0, -365)
			task.DeferredUntil = ptr(now.AddDate(0, 0, 3))
		}, false},
		{"deferred matches an unexpired deferral", QueryFieldDeferred, func(task *Task) {
			task.DeferredUntil = ptr(now.Add(time.Hour))
		}, true},
		{"deferred refuses an expired deferral", QueryFieldDeferred, func(task *Task) {
			task.DeferredUntil = ptr(now.Add(-time.Hour))
		}, false},
		{"parent matches an open direct child", QueryFieldParent, func(task *Task) {
			task.Children = []*Task{openChild}
		}, true},
		{"parent refuses a wholly finished subtree", QueryFieldParent, func(task *Task) {
			task.Children = []*Task{nil, doneChild}
		}, false},
		{"leaf matches no open child", QueryFieldLeaf, func(task *Task) {
			task.Children = []*Task{doneChild}
		}, true},
		{"leaf refuses an open child", QueryFieldLeaf, func(task *Task) {
			task.Children = []*Task{openChild}
		}, false},
		{"child matches a parented task", QueryFieldChild, func(task *Task) { task.ParentID = &parentID }, true},
		{"child refuses a root task", QueryFieldChild, func(*Task) {}, false},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			task := queryTestTask()
			testCase.mutate(task)
			query := Query{Clauses: []QueryClause{
				positiveClause(t, testCase.field, QueryOperatorPresent, ""),
			}}
			if got := query.Admits(task, project, now); got != testCase.admits {
				t.Fatalf("Admits = %t, want %t", got, testCase.admits)
			}
		})
	}
}

// TestQueryReviewAttentionIsDistinctFromTheStoredFlag pins the compatibility
// boundary between =review and ?review. The former reads one persisted boolean;
// the latter delegates to TaskNeedsAttention, including an arrived check-in and
// excluding the retired stale and Waiting routes.
func TestQueryReviewAttentionIsDistinctFromTheStoredFlag(t *testing.T) {
	now := queryTestNow()
	project := queryTestProject()
	due, future := queryTestDay(29), queryTestDay(30)
	deferredUntil := queryTestDay(31)

	stored, err := ParseQuery("=review")
	if err != nil {
		t.Fatalf("ParseQuery(=review): %v", err)
	}
	attention, err := ParseQuery("?review")
	if err != nil {
		t.Fatalf("ParseQuery(?review): %v", err)
	}
	notAttention, err := ParseQuery("-?review")
	if err != nil {
		t.Fatalf("ParseQuery(-?review): %v", err)
	}

	build := func(mutate func(*Task)) *Task {
		task := queryTestTask()
		mutate(task)
		return task
	}
	cases := []struct {
		name      string
		task      *Task
		isStored  bool
		needsLook bool
	}{
		{"plain", build(func(*Task) {}), false, false},
		{"stored flag", build(func(task *Task) { task.Review = true }), true, true},
		{"arrived check-in", build(func(task *Task) { task.NextCheckIn = &due }), false, true},
		{"future check-in", build(func(task *Task) { task.NextCheckIn = &future }), false, false},
		{"merely stale", build(func(task *Task) {
			task.UpdatedAt = now.AddDate(0, 0, -StaleTaskThresholdDays)
		}), false, false},
		{"merely waiting", build(func(task *Task) { task.WorkflowState = StateWaiting }), false, false},
		{"deferred arrived check-in", build(func(task *Task) {
			task.NextCheckIn = &due
			task.DeferredUntil = &deferredUntil
		}), false, false},
		{"deferred stored flag", build(func(task *Task) {
			task.Review = true
			task.DeferredUntil = &deferredUntil
		}), true, true},
		{"terminal stored flag", build(func(task *Task) {
			task.Review = true
			task.WorkflowState = StateDone
		}), true, false},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := stored.Admits(testCase.task, project, now); got != testCase.isStored {
				t.Errorf("=review admits = %t, want %t", got, testCase.isStored)
			}
			if got := attention.Admits(testCase.task, project, now); got != testCase.needsLook {
				t.Errorf("?review admits = %t, want %t", got, testCase.needsLook)
			}
			if got := notAttention.Admits(testCase.task, project, now); got == testCase.needsLook {
				t.Errorf("-?review admits = %t, want the inverse of %t", got, testCase.needsLook)
			}
		})
	}

	if stored.OpensDeferredPopulation() {
		t.Fatal("=review changed its population admission")
	}
	if !attention.OpensDeferredPopulation() {
		t.Fatal("?review did not open the deferred population its flagged branch can match")
	}
	if notAttention.OpensDeferredPopulation() {
		t.Fatal("-?review opened the deferred population")
	}
}

func TestQueryAdmitsCompletionOnlyWithAUsableTimestamp(t *testing.T) {
	now := queryTestNow()
	project := queryTestProject()
	cases := []struct {
		name        string
		operator    QueryOperator
		value       string
		completedAt *time.Time
		admits      bool
	}{
		{"present matches any completion", QueryOperatorPresent, "", ptr(now.AddDate(0, 0, -1)), true},
		{"present refuses a missing timestamp", QueryOperatorPresent, "", nil, false},
		{"present refuses a zero timestamp", QueryOperatorPresent, "", ptr(time.Time{}), false},
		{"age matches at the threshold", QueryOperatorAtLeast, "30d", ptr(now.AddDate(0, 0, -30)), true},
		{"age refuses inside the threshold", QueryOperatorAtLeast, "30d", ptr(now.AddDate(0, 0, -29)), false},
		{"age refuses a missing timestamp", QueryOperatorAtLeast, "1d", nil, false},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			task := queryTestTask()
			task.WorkflowState = StateDone
			task.CompletedAt = testCase.completedAt
			query := Query{Clauses: []QueryClause{
				positiveClause(t, QueryFieldCompleted, testCase.operator, testCase.value),
			}}
			if got := query.Admits(task, project, now); got != testCase.admits {
				t.Fatalf("Admits = %t, want %t", got, testCase.admits)
			}
		})
	}
}

func TestQueryTaskIsStaleReadsTheTouchClockAlone(t *testing.T) {
	now := queryTestNow()
	if QueryTaskIsStale(nil, now) {
		t.Fatal("a nil task reported stale")
	}
	task := queryTestTask()
	task.UpdatedAt = now.AddDate(0, 0, -StaleTaskThresholdDays)
	task.StateChangedAt = now
	if !QueryTaskIsStale(task, now) {
		t.Fatal("an untouched task was not stale even though its state clock is recent")
	}
	task.UpdatedAt = now
	task.StateChangedAt = now.AddDate(0, 0, -365)
	if QueryTaskIsStale(task, now) {
		t.Fatal("a recently touched task was stale because of an old state clock")
	}
}
