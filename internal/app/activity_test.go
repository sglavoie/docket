package app_test

import (
	"errors"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

func TestCompletionActivityEmptyWorkspace(t *testing.T) {
	repo := newFakeRepo()
	now := time.Date(2026, time.August, 19, 14, 30, 0, 0, time.UTC)

	activity, err := newTestServiceWithClock(repo, fixedClock{t: now}).CompletionActivity()
	if err != nil {
		t.Fatalf("CompletionActivity: %v", err)
	}
	if activity.HasHistory() {
		t.Fatalf("HasHistory() = true, want false; activity = %+v", activity)
	}
	if activity.MedianPerDay() != 0 || activity.CompletedToday() != 0 || len(activity.DailyCompletions) != 0 {
		t.Fatalf("empty activity = %+v, want zero values", activity)
	}
	if repo.snapshots != 1 {
		t.Fatalf("Snapshot calls = %d, want exactly 1", repo.snapshots)
	}
	if len(repo.commits) != 0 {
		t.Fatalf("Commit calls = %d, want 0 for read-only query", len(repo.commits))
	}
}

func TestCompletionActivitySingleDayKeepsZeroDaysInMedian(t *testing.T) {
	now := time.Date(2026, time.August, 19, 18, 0, 0, 0, time.UTC)
	completed := time.Date(2026, time.August, 18, 9, 15, 0, 0, time.UTC)
	project := newProject("p1", "One", completedTask("done-1", 1, completed))

	activity, err := newTestServiceWithClock(newFakeRepo(project), fixedClock{t: now}).CompletionActivity()
	if err != nil {
		t.Fatalf("CompletionActivity: %v", err)
	}
	day := time.Date(2026, time.August, 18, 0, 0, 0, 0, time.UTC)
	if got := activity.DailyCompletions[day]; got != 1 {
		t.Fatalf("DailyCompletions[%s] = %d, want 1", day, got)
	}
	if !activity.HasHistory() {
		t.Fatal("HasHistory() = false, want true")
	}
	if activity.MedianPerDay() != 0 {
		t.Fatalf("MedianPerDay = %v, want 0 with 29 zero-count days", activity.MedianPerDay())
	}
	if activity.CompletedToday() != 0 {
		t.Fatalf("CompletedToday = %d, want 0", activity.CompletedToday())
	}
}

func TestCompletionActivityMedianUsesAllThirtyCalendarDays(t *testing.T) {
	now := time.Date(2026, time.August, 19, 12, 0, 0, 0, time.UTC)
	today := time.Date(2026, time.August, 19, 0, 0, 0, 0, time.UTC)
	tasks := make([]*models.Task, 0, 45)
	shortID := 1
	for dayOffset := 0; dayOffset < 30; dayOffset++ {
		count := 1
		if dayOffset < 15 {
			count = 2
		}
		for n := 0; n < count; n++ {
			completed := today.AddDate(0, 0, -dayOffset).Add(time.Duration(9+n) * time.Hour)
			tasks = append(tasks, completedTask("done-"+completed.Format("20060102-15"), shortID, completed))
			shortID++
		}
	}

	activity, err := newTestServiceWithClock(newFakeRepo(newProject("p1", "Thirty days", tasks...)), fixedClock{t: now}).CompletionActivity()
	if err != nil {
		t.Fatalf("CompletionActivity: %v", err)
	}
	if activity.MedianPerDay() != 1.5 {
		t.Fatalf("MedianPerDay = %v, want 1.5", activity.MedianPerDay())
	}
	if activity.CompletedToday() != 2 {
		t.Fatalf("CompletedToday = %d, want 2", activity.CompletedToday())
	}
}

func TestCompletionActivityBucketsAndBoundsByLocalCalendarDayAcrossDST(t *testing.T) {
	location, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatalf("load location: %v", err)
	}
	now := time.Date(2025, time.March, 10, 12, 0, 0, 0, location)
	firstWindowDay := time.Date(2025, time.February, 9, 12, 0, 0, 0, location)
	beforeWindow := time.Date(2025, time.February, 8, 23, 59, 0, 0, location)
	// UTC inputs straddle the skipped local hour; both must normalize into the
	// same March 9 bucket in the user's zone.
	beforeSpringForward := time.Date(2025, time.March, 9, 6, 30, 0, 0, time.UTC)
	afterSpringForward := time.Date(2025, time.March, 9, 7, 30, 0, 0, time.UTC)
	project := newProject("p1", "DST",
		completedTask("first", 1, firstWindowDay),
		completedTask("outside", 2, beforeWindow),
		completedTask("before-jump", 3, beforeSpringForward),
		completedTask("after-jump", 4, afterSpringForward),
	)

	activity, err := newTestServiceWithClock(newFakeRepo(project), fixedClock{t: now}).CompletionActivity()
	if err != nil {
		t.Fatalf("CompletionActivity: %v", err)
	}
	windowDay := time.Date(2025, time.February, 9, 0, 0, 0, 0, location)
	dstDay := time.Date(2025, time.March, 9, 0, 0, 0, 0, location)
	outsideDay := time.Date(2025, time.February, 8, 0, 0, 0, 0, location)
	if got := activity.DailyCompletions[windowDay]; got != 1 {
		t.Fatalf("first window calendar day count = %d, want 1", got)
	}
	if got := activity.DailyCompletions[dstDay]; got != 2 {
		t.Fatalf("spring-forward calendar day count = %d, want 2", got)
	}
	if _, ok := activity.DailyCompletions[outsideDay]; ok {
		t.Fatalf("day before calendar window unexpectedly included: %s", outsideDay)
	}
}

func TestCompletionActivityCountsOnlyDoneTasksWithCompletedAt(t *testing.T) {
	now := time.Date(2026, time.August, 19, 12, 0, 0, 0, time.UTC)
	completed := now.Add(-time.Hour)
	done := completedTask("done", 1, completed)
	doneWithoutTimestamp := newTask("done-no-time", 2, "done without timestamp")
	doneWithoutTimestamp.WorkflowState = models.StateDone
	canceled := completedTask("canceled", 3, completed)
	canceled.WorkflowState = models.StateCanceled
	openWithTimestamp := completedTask("open", 4, completed)
	openWithTimestamp.WorkflowState = models.StateBacklog
	project := newProject("p1", "States", done, doneWithoutTimestamp, canceled, openWithTimestamp)

	activity, err := newTestServiceWithClock(newFakeRepo(project), fixedClock{t: now}).CompletionActivity()
	if err != nil {
		t.Fatalf("CompletionActivity: %v", err)
	}
	if activity.CompletedToday() != 1 {
		t.Fatalf("CompletedToday = %d, want only the Done task with CompletedAt", activity.CompletedToday())
	}
}

func TestCompletionActivityUsesTaskWorkspaceProjectPopulation(t *testing.T) {
	now := time.Date(2026, time.August, 19, 12, 0, 0, 0, time.UTC)
	completed := now.Add(-time.Hour)
	active := newProject("active", "Active", completedTask("active-task", 1, completed))
	untagged := newProject("untagged", "Untagged", completedTask("untagged-task", 1, completed))
	archived := newProject("archived", "Archived", completedTask("archived-task", 1, completed))
	archived.Archived = true
	parked := newProject("parked", "Parked", completedTask("parked-task", 1, completed))
	parked.Tags = []string{models.ParkedProjectTag}

	activity, err := newTestServiceWithClock(newFakeRepo(active, untagged, archived, parked), fixedClock{t: now}).CompletionActivity()
	if err != nil {
		t.Fatalf("CompletionActivity: %v", err)
	}
	if activity.CompletedToday() != 2 {
		t.Fatalf("CompletedToday = %d, want active and untagged projects only", activity.CompletedToday())
	}
}

func TestCompletionActivityReturnsSnapshotError(t *testing.T) {
	repo := newFakeRepo()
	wantErr := errors.New("snapshot unavailable")
	repo.snapErr = wantErr

	_, err := newTestService(repo).CompletionActivity()
	if !errors.Is(err, wantErr) {
		t.Fatalf("CompletionActivity error = %v, want %v", err, wantErr)
	}
	if repo.snapshots != 1 {
		t.Fatalf("Snapshot calls = %d, want exactly 1", repo.snapshots)
	}
}

func completedTask(id string, shortID int, completed time.Time) *models.Task {
	task := newTask(id, shortID, id)
	task.WorkflowState = models.StateDone
	task.CompletedAt = &completed
	return task
}
