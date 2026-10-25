package app_test

import (
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/models"
)

// TestOutlinePlanUneditedPinSurvivesWithItsOriginalStamp is the guarantee the
// manual J/K pinned order rests on: re-declaring an unchanged pin must not
// restamp PinnedAt, or every apply would reshuffle the pinned section.
func TestOutlinePlanUneditedPinSurvivesWithItsOriginalStamp(t *testing.T) {
	pinned := newTask("t1", 1, "Ship release")
	pinnedAt := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	pinned.Pinned = true
	pinned.PinnedAt = &pinnedAt
	project := newProject("p1", "Alpha Project", pinned)
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	result, err := svc.ApplyOutlinePlan(app.OutlinePlanRequest{
		ProjectID:        project.ID,
		ExpectedRevision: repo.revisions[project.ID],
		ScopeTaskIDs:     []string{pinned.ID},
		Lines: []app.OutlinePlanLine{
			{SourceLine: 1, TaskID: pinned.ID, Depth: 0, Title: "Ship release", Pinned: true},
		},
	})
	if err != nil {
		t.Fatalf("ApplyOutlinePlan: %v", err)
	}
	if !pinned.Pinned {
		t.Error("Pinned = false, want the declared pin preserved")
	}
	if pinned.PinnedAt == nil || *pinned.PinnedAt != pinnedAt {
		t.Errorf("PinnedAt = %v, want the original stamp %v", pinned.PinnedAt, pinnedAt)
	}
	if len(result.Updated) != 0 {
		t.Errorf("Updated = %+v, want no reported change", result.Updated)
	}
}

func TestOutlinePlanOmittedPinUnpinsAndReportsTheChange(t *testing.T) {
	pinned := newTask("t1", 1, "Ship release")
	pinnedAt := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	pinned.Pinned = true
	pinned.PinnedAt = &pinnedAt
	project := newProject("p1", "Alpha Project", pinned)
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	result, err := svc.ApplyOutlinePlan(app.OutlinePlanRequest{
		ProjectID:        project.ID,
		ExpectedRevision: repo.revisions[project.ID],
		ScopeTaskIDs:     []string{pinned.ID},
		Lines: []app.OutlinePlanLine{
			{SourceLine: 1, TaskID: pinned.ID, Depth: 0, Title: "Ship release"},
		},
	})
	if err != nil {
		t.Fatalf("ApplyOutlinePlan: %v", err)
	}
	if pinned.Pinned || pinned.PinnedAt != nil {
		t.Errorf("Pinned/PinnedAt = %v/%v, want the omitted sigil to unpin", pinned.Pinned, pinned.PinnedAt)
	}
	if len(result.Updated) != 1 || result.Updated[0].TaskID != pinned.ID {
		t.Fatalf("Updated = %+v, want the unpinned task reported", result.Updated)
	}
}

func TestOutlinePlanAddedPinStampsTheServiceClock(t *testing.T) {
	task := newTask("t1", 1, "Ship release")
	project := newProject("p1", "Alpha Project", task)
	repo := newFakeRepo(project)
	svc := newTestService(repo)
	wantNow := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)

	result, err := svc.ApplyOutlinePlan(app.OutlinePlanRequest{
		ProjectID:        project.ID,
		ExpectedRevision: repo.revisions[project.ID],
		ScopeTaskIDs:     []string{task.ID},
		Lines: []app.OutlinePlanLine{
			{SourceLine: 1, TaskID: task.ID, Depth: 0, Title: "Ship release", Pinned: true},
		},
	})
	if err != nil {
		t.Fatalf("ApplyOutlinePlan: %v", err)
	}
	if !task.Pinned {
		t.Fatal("Pinned = false, want the declared pin applied")
	}
	if task.PinnedAt == nil || *task.PinnedAt != wantNow {
		t.Errorf("PinnedAt = %v, want the injected clock %v", task.PinnedAt, wantNow)
	}
	if len(result.Updated) != 1 || result.Updated[0].TaskID != task.ID {
		t.Errorf("Updated = %+v, want the newly pinned task reported", result.Updated)
	}
}

// TestOutlinePlanPinKeepsTheManualPinnedOrder is the whole reason ApplyPinned is
// idempotent: a buffer that edits one line must not restamp the pins it merely
// restates, or the J/K order swaps under the user.
func TestOutlinePlanPinKeepsTheManualPinnedOrder(t *testing.T) {
	first := newTask("t1", 1, "First pinned")
	second := newTask("t2", 2, "Second pinned")
	firstAt := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	secondAt := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	first.Pinned, first.PinnedAt = true, &firstAt
	second.Pinned, second.PinnedAt = true, &secondAt
	project := newProject("p1", "Alpha Project", first, second)
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	if _, err := svc.ApplyOutlinePlan(app.OutlinePlanRequest{
		ProjectID:        project.ID,
		ExpectedRevision: repo.revisions[project.ID],
		ScopeTaskIDs:     []string{first.ID, second.ID},
		Lines: []app.OutlinePlanLine{
			{SourceLine: 1, TaskID: first.ID, Depth: 0, Title: "First pinned, renamed", Pinned: true},
			{SourceLine: 2, TaskID: second.ID, Depth: 0, Title: "Second pinned", Pinned: true},
		},
	}); err != nil {
		t.Fatalf("ApplyOutlinePlan: %v", err)
	}
	if *first.PinnedAt != firstAt || *second.PinnedAt != secondAt {
		t.Errorf("PinnedAt = %v/%v, want the manual order %v/%v", first.PinnedAt, second.PinnedAt, firstAt, secondAt)
	}
}

func TestOutlinePlanPinnedNewTaskIsPinnedOnCreation(t *testing.T) {
	project := newProject("p1", "Alpha Project")
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	result, err := svc.ApplyOutlinePlan(app.OutlinePlanRequest{
		ProjectID:        project.ID,
		ExpectedRevision: repo.revisions[project.ID],
		Lines: []app.OutlinePlanLine{
			{SourceLine: 1, Depth: 0, Title: "Brand new", Pinned: true},
		},
	})
	if err != nil {
		t.Fatalf("ApplyOutlinePlan: %v", err)
	}
	if len(result.Created) != 1 {
		t.Fatalf("Created = %+v, want the one new line", result.Created)
	}
	created := project.GetTaskByID(result.Created[0].TaskID)
	if created == nil || !created.Pinned || created.PinnedAt == nil {
		t.Fatalf("created task = %+v, want a pinned task carrying a stamp", created)
	}
}

func TestOutlinePlanUnpinnedTerminalTaskIsUntouchedByThePinLoop(t *testing.T) {
	done := newTask("t1", 1, "Shipped")
	done.ApplyWorkflowState(models.StateDone)
	project := newProject("p1", "Alpha Project", done)
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	result, err := svc.ApplyOutlinePlan(app.OutlinePlanRequest{
		ProjectID:        project.ID,
		ExpectedRevision: repo.revisions[project.ID],
		ScopeTaskIDs:     []string{done.ID},
		Lines: []app.OutlinePlanLine{
			{SourceLine: 1, TaskID: done.ID, Depth: 0, Title: "Shipped"},
		},
	})
	if err != nil {
		t.Fatalf("ApplyOutlinePlan: %v", err)
	}
	if done.Pinned || done.PinnedAt != nil {
		t.Errorf("Pinned/PinnedAt = %v/%v, want an untouched unpinned task", done.Pinned, done.PinnedAt)
	}
	if len(result.Updated) != 0 {
		t.Errorf("Updated = %+v, want no reported change", result.Updated)
	}
	if strings.Contains(result.Summary(), "1 change") {
		t.Errorf("Summary() = %q, want nothing reported", result.Summary())
	}
}
