package app_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/storage"
)

func (r *fakeRepo) CheckInSetupCensus() (app.WorkspaceCensus, error) { return r.WorkspaceCensus() }

func TestCheckInSetupDeterministicActivePopulation(t *testing.T) {
	active := func(id string, short int, state models.WorkflowState) *models.Task {
		task := newTask(id, short, id)
		task.ApplyWorkflowState(state)
		return task
	}
	first, second := active("a", 1, models.StateDoing), active("b", 2, models.StateWaiting)
	scheduled := active("scheduled", 3, models.StateDoing)
	scheduled.NextCheckIn = checkInSchedule(2, 0).Next
	// Hidden work stays out, whether it stores the flag or inherits it.
	hiddenParent := active("hidden", 5, models.StateDoing)
	hiddenParent.Hidden = true
	hiddenChild := childOf("hidden-child", 6, "hidden-child", hiddenParent)
	hiddenChild.ApplyWorkflowState(models.StateWaiting)
	p := newProject("p", "Alpha", second, scheduled, first, newTask("backlog", 4, "backlog"), hiddenParent, hiddenChild)
	p.Slug = "alpha"
	q := newProject("q", "Beta", active("q1", 1, models.StateDoing))
	q.Slug = "beta"
	parked := newProject("parked", "Parked", active("p1", 1, models.StateDoing))
	parked.Tags = []string{models.ParkedProjectTag}
	archived := newProject("archived", "Archived", active("r1", 1, models.StateWaiting))
	archived.Archived = true
	repo := newFakeRepo(q, parked, p, archived)
	svc := newCheckInService(repo)
	candidates, err := svc.CheckInSetupCandidates()
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, candidate := range candidates {
		got = append(got, candidate.Locator.TaskID)
	}
	if !reflect.DeepEqual(got, []string{"a", "b", "q1"}) {
		t.Fatalf("population = %v", got)
	}
	// No census or skip creates an undo record.
	if len(repo.commits) != 0 {
		t.Fatal("enumeration wrote changes")
	}
}

func TestCheckInSetupFreshEligibilityAndIndependentTransactions(t *testing.T) {
	for _, change := range []string{"deleted", "moved", "done", "backlog", "scheduled", "parked", "archived", "unchanged"} {
		t.Run(change, func(t *testing.T) {
			task := newTask("one", 1, "one")
			task.ApplyWorkflowState(models.StateDoing)
			p := newProject("p", "Project", task)
			repo := newFakeRepo(p)
			svc := newCheckInService(repo)
			candidates, err := svc.CheckInSetupCandidates()
			if err != nil {
				t.Fatal(err)
			}
			switch change {
			case "deleted":
				p.Tasks = nil
			case "moved":
				p.Tasks = nil
				q := newProject("q", "Other", task)
				repo.projects = append(repo.projects, q)
			case "done":
				task.ApplyWorkflowState(models.StateDone)
			case "backlog":
				task.ApplyWorkflowState(models.StateBacklog)
			case "scheduled":
				task.NextCheckIn = checkInSchedule(8, 0).Next
			case "parked":
				p.Tags = []string{models.ParkedProjectTag}
			case "archived":
				p.Archived = true
			}
			_, eligible, err := svc.ResolveCheckInSetup(candidates[0].Locator)
			if err != nil || eligible != (change == "unchanged") {
				t.Fatalf("resolve = %v %v", eligible, err)
			}
			result, skipped, err := svc.SaveCheckInSetup(candidates[0].Locator, checkInSchedule(2, 7))
			if err != nil || skipped != (change != "unchanged") {
				t.Fatalf("save = %+v %v %v", result, skipped, err)
			}
			want := 0
			if change == "unchanged" {
				want = 1
			}
			if len(repo.commits) != want {
				t.Fatalf("commits = %d want %d", len(repo.commits), want)
			}
		})
	}
}

func TestCheckInSetupRejectsReadAndCommitFailures(t *testing.T) {
	for _, failure := range []string{"read", "write", "task-race", "catalog-race"} {
		t.Run(failure, func(t *testing.T) {
			task := newTask("one", 1, "one")
			task.ApplyWorkflowState(models.StateDoing)
			repo := newFakeRepo(newProject("p", "Project", task))
			svc := newCheckInService(repo)
			target := repo.loc(task.ID)
			switch failure {
			case "read":
				repo.censusErr = errors.New("unreadable active owner")
			case "write":
				repo.commitErr = errors.New("disk full")
			case "task-race":
				repo.preCommit = func() { repo.revisions["p"] = "concurrent" }
			case "catalog-race":
				repo.preCommit = func() { repo.files[fakeCatalogPath] = []byte("parked concurrently") }
			}
			_, skipped, err := svc.SaveCheckInSetup(target, checkInSchedule(2, 7))
			if err == nil || skipped {
				t.Fatalf("failure became success/skip: %v %v", skipped, err)
			}
			if !repo.projects[0].Tasks[0].CheckIn().IsEmpty() || len(repo.commits) != 0 {
				t.Fatal("failed save mutated task or ledger")
			}
			if failure == "task-race" && !errors.Is(err, storage.ErrStaleRevision) {
				t.Fatalf("task guard: %v", err)
			}
			if failure == "catalog-race" && !errors.Is(err, storage.ErrStaleArtifact) {
				t.Fatalf("catalog guard: %v", err)
			}
			if failure == "read" {
				if candidates, err := svc.CheckInSetupCandidates(); err == nil || len(candidates) != 0 {
					t.Fatal("partial census reported success")
				}
			}
		})
	}
}
