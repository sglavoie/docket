package cmd

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

func TestFilterTasks_CompletedTasksExcludedByDefault(t *testing.T) {
	defer resetFilterVars()

	tasks := []*models.Task{
		testutil.NewTestTaskWithStatus("Todo", models.StatusTodo),
		testutil.NewTestTaskWithStatus("InProgress", models.StatusInProgress),
		testutil.NewTestTaskWithStatus("Review", models.StatusReview),
		testutil.NewTestTaskWithStatus("Standby", models.StatusStandby),
		testutil.NewTestTaskWithStatus("Blocked", models.StatusBlocked),
		testutil.NewTestTaskWithStatus("Done", models.StatusDone),
		testutil.NewTestTaskWithStatus("Canceled", models.StatusCanceled),
	}

	result := filterTasks(tasks)

	if len(result) != 5 {
		t.Errorf("expected 5 active tasks, got %d", len(result))
	}

	for _, task := range result {
		if task.IsComplete() {
			t.Errorf("completed task %s should not be in results", task.Title)
		}
	}
}

func TestTaskMatchesFilters_SnoozedHiddenByDefault(t *testing.T) {
	defer resetFilterVars()

	snoozed := testutil.NewTestTask("Snoozed", models.TaskTypeTask)
	future := time.Now().Add(24 * time.Hour)
	snoozed.SnoozedUntil = &future

	normal := testutil.NewTestTask("Normal", models.TaskTypeTask)

	if taskMatchesFilters(snoozed) {
		t.Error("expected snoozed task to be hidden by default")
	}
	if !taskMatchesFilters(normal) {
		t.Error("expected normal task to be visible")
	}

	listSnoozed = true
	if !taskMatchesFilters(snoozed) {
		t.Error("expected snoozed task to be visible with --snoozed flag")
	}
}

func TestTaskMatchesFilters_CompletedHiddenByDefault(t *testing.T) {
	defer resetFilterVars()

	done := testutil.NewTestTaskWithStatus("Done", models.StatusDone)
	todo := testutil.NewTestTaskWithStatus("Todo", models.StatusTodo)

	if taskMatchesFilters(done) {
		t.Error("expected completed task to be hidden by default")
	}
	if !taskMatchesFilters(todo) {
		t.Error("expected todo task to be visible")
	}

	listShowAll = true
	if !taskMatchesFilters(done) {
		t.Error("expected completed task to be visible with --all flag")
	}
}

func TestTaskMatchesFilters_ParkedOnly(t *testing.T) {
	defer resetFilterVars()
	listParked = true

	parkedByNote := testutil.NewTestTask("Parked by note", models.TaskTypeTask)
	parkedByNote.ParkedNote = "resume after review"
	parkedByDate := testutil.NewTestTask("Parked by date", models.TaskTypeTask)
	parkedAt := time.Now()
	parkedByDate.ParkedAt = &parkedAt
	plain := testutil.NewTestTask("Plain", models.TaskTypeTask)

	if !taskMatchesFilters(parkedByNote) {
		t.Error("expected task with parked note to match --parked")
	}
	if !taskMatchesFilters(parkedByDate) {
		t.Error("expected task with parked_at to match --parked")
	}
	if taskMatchesFilters(plain) {
		t.Error("expected non-parked task to be excluded by --parked")
	}
}

func TestTaskMatchesFilters_ParkedRespectsCompletedVisibility(t *testing.T) {
	defer resetFilterVars()
	listParked = true

	done := testutil.NewTestTaskWithStatus("Done parked", models.StatusDone)
	done.ParkedNote = "resume archived state"

	if taskMatchesFilters(done) {
		t.Error("expected completed parked task to be hidden without --all")
	}

	listShowAll = true
	if !taskMatchesFilters(done) {
		t.Error("expected completed parked task to be visible with --parked and --all")
	}
}

func TestTaskMatchesFilters_ParkedRespectsSnoozedVisibility(t *testing.T) {
	defer resetFilterVars()
	listParked = true

	snoozed := testutil.NewTestTask("Snoozed parked", models.TaskTypeTask)
	snoozed.ParkedNote = "resume later"
	future := time.Now().Add(24 * time.Hour)
	snoozed.SnoozedUntil = &future

	if taskMatchesFilters(snoozed) {
		t.Error("expected snoozed parked task to be hidden without --snoozed")
	}

	listSnoozed = true
	if !taskMatchesFilters(snoozed) {
		t.Error("expected snoozed parked task to be visible with --parked and --snoozed")
	}
}
