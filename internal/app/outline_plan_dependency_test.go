package app_test

import (
	"strings"
	"testing"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/models"
)

func dependencyLocatorsOf(task *models.Task) []string {
	locators := make([]string, 0, len(task.Dependencies))
	for _, reference := range task.Dependencies {
		locators = append(locators, reference.Locator())
	}
	return locators
}

func TestOutlinePlanDependenciesAreTheDeclaredFinalSet(t *testing.T) {
	alpha := newTask("t1", 1, "Alpha")
	beta := newTask("t2", 2, "Beta")
	gamma := newTask("t3", 3, "Gamma")
	alpha.Dependencies = []models.TaskReference{{ProjectID: "p1", TaskID: beta.ID}}
	project := newProject("p1", "Alpha Project", alpha, beta, gamma)
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	if _, err := svc.ApplyOutlinePlan(app.OutlinePlanRequest{
		ProjectID:        project.ID,
		ExpectedRevision: repo.revisions[project.ID],
		ScopeTaskIDs:     []string{alpha.ID, beta.ID, gamma.ID},
		Lines: []app.OutlinePlanLine{
			// The line drops beta and adds gamma: the buffer is declarative, so the
			// set it prints is the set that is stored.
			{SourceLine: 1, TaskID: alpha.ID, Title: "Alpha", Dependencies: []app.TaskLocator{{ProjectID: "p1", TaskID: gamma.ID}}},
			{SourceLine: 2, TaskID: beta.ID, Title: "Beta"},
			{SourceLine: 3, TaskID: gamma.ID, Title: "Gamma"},
		},
	}); err != nil {
		t.Fatalf("ApplyOutlinePlan: %v", err)
	}

	if got, want := strings.Join(dependencyLocatorsOf(alpha), ","), "p1:t3"; got != want {
		t.Errorf("alpha dependencies = %q, want %q", got, want)
	}
}

func TestOutlinePlanOmittedDependencyClearsTheStoredSet(t *testing.T) {
	alpha := newTask("t1", 1, "Alpha")
	beta := newTask("t2", 2, "Beta")
	alpha.Dependencies = []models.TaskReference{{ProjectID: "p1", TaskID: beta.ID}}
	project := newProject("p1", "Alpha Project", alpha, beta)
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	result, err := svc.ApplyOutlinePlan(app.OutlinePlanRequest{
		ProjectID:        project.ID,
		ExpectedRevision: repo.revisions[project.ID],
		ScopeTaskIDs:     []string{alpha.ID, beta.ID},
		Lines: []app.OutlinePlanLine{
			{SourceLine: 1, TaskID: alpha.ID, Title: "Alpha"},
			{SourceLine: 2, TaskID: beta.ID, Title: "Beta"},
		},
	})
	if err != nil {
		t.Fatalf("ApplyOutlinePlan: %v", err)
	}
	if len(alpha.Dependencies) != 0 {
		t.Errorf("alpha dependencies = %v, want the omitted set cleared", dependencyLocatorsOf(alpha))
	}
	// Clearing a set is a change, so the line must be counted as an edit rather
	// than reported as an untouched restatement.
	if len(result.Updated) != 1 || result.Updated[0].TaskID != alpha.ID {
		t.Errorf("updated = %+v, want only alpha", result.Updated)
	}
}

func TestOutlinePlanRoundTripsAnExistingUnavailableReference(t *testing.T) {
	alpha := newTask("t1", 1, "Alpha")
	stored := models.TaskReference{ProjectID: "archived-project", TaskID: "gone"}
	alpha.Dependencies = []models.TaskReference{stored}
	project := newProject("p1", "Alpha Project", alpha)
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	if _, err := svc.ApplyOutlinePlan(app.OutlinePlanRequest{
		ProjectID:        project.ID,
		ExpectedRevision: repo.revisions[project.ID],
		ScopeTaskIDs:     []string{alpha.ID},
		Lines: []app.OutlinePlanLine{
			{SourceLine: 1, TaskID: alpha.ID, Title: "Alpha", Dependencies: []app.TaskLocator{app.LocatorForReference(stored)}},
		},
	}); err != nil {
		t.Fatalf("ApplyOutlinePlan: %v", err)
	}
	if got, want := strings.Join(dependencyLocatorsOf(alpha), ","), stored.Locator(); got != want {
		t.Errorf("alpha dependencies = %q, want the unavailable reference preserved as %q", got, want)
	}
}

func TestOutlinePlanRoundTripsAnExistingTerminalReference(t *testing.T) {
	alpha := newTask("t1", 1, "Alpha")
	closed := newTask("t2", 2, "Closed")
	closed.WorkflowState = models.StateDone
	reference := models.TaskReference{ProjectID: "p1", TaskID: closed.ID}
	alpha.Dependencies = []models.TaskReference{reference}
	project := newProject("p1", "Alpha Project", alpha, closed)
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	if _, err := svc.ApplyOutlinePlan(app.OutlinePlanRequest{
		ProjectID:        project.ID,
		ExpectedRevision: repo.revisions[project.ID],
		ScopeTaskIDs:     []string{alpha.ID, closed.ID},
		Lines: []app.OutlinePlanLine{
			{SourceLine: 1, TaskID: alpha.ID, Title: "Alpha", Dependencies: []app.TaskLocator{app.LocatorForReference(reference)}},
			{SourceLine: 2, TaskID: closed.ID, Title: "Closed"},
		},
	}); err != nil {
		t.Fatalf("ApplyOutlinePlan: %v", err)
	}
	if got, want := strings.Join(dependencyLocatorsOf(alpha), ","), reference.Locator(); got != want {
		t.Errorf("alpha dependencies = %q, want the closed target preserved as %q", got, want)
	}
}

func TestOutlinePlanRefusesANewlyIntroducedUnusableTarget(t *testing.T) {
	closed := newTask("t2", 2, "Closed")
	closed.WorkflowState = models.StateDone

	cases := []struct {
		name    string
		target  app.TaskLocator
		message string
	}{
		{"terminal", app.TaskLocator{ProjectID: "p1", TaskID: "t2"}, "already closed"},
		{"missing", app.TaskLocator{ProjectID: "p1", TaskID: "nope"}, "does not exist"},
		{"unloaded", app.TaskLocator{ProjectID: "other", TaskID: "t9"}, "is not available"},
		{"self", app.TaskLocator{ProjectID: "p1", TaskID: "t1"}, "cannot depend on itself"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			alpha := newTask("t1", 1, "Alpha")
			project := newProject("p1", "Alpha Project", alpha, closed)
			repo := newFakeRepo(project)
			svc := newTestService(repo)

			_, err := svc.ApplyOutlinePlan(app.OutlinePlanRequest{
				ProjectID:        project.ID,
				ExpectedRevision: repo.revisions[project.ID],
				ScopeTaskIDs:     []string{alpha.ID, closed.ID},
				Lines: []app.OutlinePlanLine{
					{SourceLine: 1, TaskID: alpha.ID, Title: "Alpha renamed", Dependencies: []app.TaskLocator{tt.target}},
					{SourceLine: 2, TaskID: closed.ID, Title: "Closed"},
				},
			})
			if err == nil {
				t.Fatal("ApplyOutlinePlan accepted an unusable new dependency target")
			}
			if !strings.Contains(err.Error(), tt.message) {
				t.Errorf("error = %v, want it to name %q", err, tt.message)
			}
			if !strings.HasPrefix(err.Error(), "line 1: ") {
				t.Errorf("error = %v, want the offending line named", err)
			}
			// Validation runs before anything is mutated, so the rejected buffer's
			// other edits never reach the project either.
			if alpha.Title != "Alpha" {
				t.Errorf("alpha title = %q, want the rejected buffer to have changed nothing", alpha.Title)
			}
			if len(repo.commits) != 0 {
				t.Errorf("commits = %d, want none", len(repo.commits))
			}
		})
	}
}

func TestOutlinePlanDeletionRepairsIncomingReferences(t *testing.T) {
	alpha := newTask("t1", 1, "Alpha")
	doomed := newTask("t2", 2, "Doomed")
	holder := newTask("t3", 3, "Holder")
	removed := models.TaskReference{ProjectID: "p1", TaskID: doomed.ID}
	alpha.Dependencies = []models.TaskReference{removed}
	holder.Dependencies = []models.TaskReference{removed, {ProjectID: "p1", TaskID: alpha.ID}}
	project := newProject("p1", "Alpha Project", alpha, doomed, holder)
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	// The holder restates the set it already stores, so the buffer makes no edit to
	// it: everything that happens to its document is reference maintenance.
	touched := holder.UpdatedAt
	if _, err := svc.ApplyOutlinePlan(app.OutlinePlanRequest{
		ProjectID:        project.ID,
		ExpectedRevision: repo.revisions[project.ID],
		ScopeTaskIDs:     []string{alpha.ID, doomed.ID, holder.ID},
		DeleteTaskIDs:    []string{doomed.ID},
		ConfirmDeletions: true,
		Lines: []app.OutlinePlanLine{
			{SourceLine: 1, TaskID: alpha.ID, Title: "Alpha"},
			{SourceLine: 2, TaskID: holder.ID, Title: "Holder", Dependencies: []app.TaskLocator{app.LocatorForReference(removed), {ProjectID: "p1", TaskID: alpha.ID}}},
		},
	}); err != nil {
		t.Fatalf("ApplyOutlinePlan: %v", err)
	}

	if len(alpha.Dependencies) != 0 {
		t.Errorf("alpha dependencies = %v, want the deleted target dropped", dependencyLocatorsOf(alpha))
	}
	if got, want := strings.Join(dependencyLocatorsOf(holder), ","), "p1:t1"; got != want {
		t.Errorf("holder dependencies = %q, want %q", got, want)
	}
	// Repairing someone else's removed locator is maintenance, not an edit, so it
	// must not move the holder's touch time.
	if !holder.UpdatedAt.Equal(touched) {
		t.Errorf("holder UpdatedAt moved to %v, want the maintenance rewrite to leave it at %v", holder.UpdatedAt, touched)
	}
	if len(repo.commits) != 1 {
		t.Errorf("commits = %d, want one atomic transaction", len(repo.commits))
	}
}
