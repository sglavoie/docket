package app_test

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/textgrammar"
)

func TestCapturePinnedStampsInjectedClock(t *testing.T) {
	wantNow := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	project := newProject("p1", "Alpha")
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	result, err := svc.Capture(app.CaptureRequest{
		ProjectID: project.ID, Title: "Ship release", Pinned: true,
	})
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	task := project.GetTaskByID(result.TaskID)
	if task == nil {
		t.Fatal("captured task not found")
	}
	if !task.Pinned {
		t.Error("Pinned = false, want true")
	}
	if task.PinnedAt == nil {
		t.Fatal("PinnedAt = nil, want the injected clock time")
	}
	if *task.PinnedAt != wantNow {
		t.Errorf("PinnedAt = %v, want %v", *task.PinnedAt, wantNow)
	}
}

func TestCaptureWithoutPinnedLeavesTaskUnpinned(t *testing.T) {
	project := newProject("p1", "Alpha")
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	result, err := svc.Capture(app.CaptureRequest{ProjectID: project.ID, Title: "Ship release"})
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	task := project.GetTaskByID(result.TaskID)
	if task == nil {
		t.Fatal("captured task not found")
	}
	if task.Pinned || task.PinnedAt != nil {
		t.Errorf("Pinned/PinnedAt = %v/%v, want false/nil", task.Pinned, task.PinnedAt)
	}
}

// The clock advances on every read, so a batch that stamped each pinned line
// separately would give the later lines newer PinnedAt values and the pinned
// section would list them before the earlier ones.
func TestCaptureManyPinsEveryLineInOneTransaction(t *testing.T) {
	project := newProject("p1", "Alpha")
	repo := newFakeRepo(project)
	clock := &advancingClock{t: time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC), step: time.Second}
	svc := newTestServiceWithClock(repo, clock)

	results, err := svc.CaptureMany([]app.CaptureRequest{
		{SourceLine: 1, ProjectID: project.ID, Title: "First", Pinned: true, Prepend: true},
		{SourceLine: 2, ProjectID: project.ID, Title: "Second", Pinned: true, Prepend: true},
	})
	if err != nil {
		t.Fatalf("CaptureMany: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("results = %d, want 2", len(results))
	}
	if len(repo.commits) != 1 {
		t.Fatalf("commits = %d, want one atomic commit", len(repo.commits))
	}
	var stamps []time.Time
	for _, result := range results {
		task := project.GetTaskByID(result.TaskID)
		if task == nil {
			t.Fatalf("captured task %s not found", result.TaskID)
		}
		if !task.Pinned {
			t.Errorf("%q Pinned = false, want true", task.Title)
		}
		if task.PinnedAt == nil {
			t.Fatalf("%q PinnedAt = nil, want the batch timestamp", task.Title)
		}
		stamps = append(stamps, *task.PinnedAt)
	}
	if !stamps[0].Equal(stamps[1]) {
		t.Errorf("PinnedAt = %v and %v, want one shared batch timestamp", stamps[0], stamps[1])
	}
	if got := []string{project.Tasks[0].Title, project.Tasks[1].Title}; got[0] != "First" || got[1] != "Second" {
		t.Errorf("prepended order = %v, want request order [First Second]", got)
	}
}

func TestCaptureTerminalDestinationAcceptsPinned(t *testing.T) {
	project := newProject("p1", "Alpha")
	repo := newFakeRepo(project)
	planning := &config.PlanningState{}
	svc := newTestService(repo).WithPlanningArtifacts(memoryPlanningBuilder(planning))

	result, err := svc.Capture(app.CaptureRequest{
		ProjectID:   project.ID,
		Title:       "Already shipped",
		Destination: textgrammar.DestinationDone,
		Pinned:      true,
	})
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	task := project.GetTaskByID(result.TaskID)
	if task == nil {
		t.Fatal("captured task not found")
	}
	if task.State() != models.StateDone {
		t.Errorf("state = %s, want %s", task.State(), models.StateDone)
	}
	if !task.Pinned || task.PinnedAt == nil {
		t.Errorf("Pinned/PinnedAt = %v/%v, want a pinned terminal task", task.Pinned, task.PinnedAt)
	}
}
