package engine

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// createHeadlessProject adds one project to the catalog and writes its empty
// tasks.md in one bootstrap transaction, as project creation does.
func createHeadlessProject(t *testing.T, runtime *Runtime, name string, tags ...string) *models.Project {
	t.Helper()
	project, err := models.NewProject("project-"+models.Slugify(name), name, "")
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}
	project.Path = runtime.State().ProjectPath(project.Slug)
	if project.Tags, err = models.NormalizeProjectTags(tags); err != nil {
		t.Fatalf("NormalizeProjectTags: %v", err)
	}
	project.CreatedAt, project.UpdatedAt = runtime.Now(), runtime.Now()
	domains, err := runtime.State().CloneDomains()
	if err != nil {
		t.Fatalf("CloneDomains: %v", err)
	}
	if upsertErr := domains.Catalog.UpsertProject(project); upsertErr != nil {
		t.Fatalf("UpsertProject: %v", upsertErr)
	}
	catalog, err := runtime.State().CheckedCatalogArtifact(domains.Catalog)
	if err != nil {
		t.Fatalf("CheckedCatalogArtifact: %v", err)
	}
	doc, err := storage.CanonicalProjectDocument(project, nil)
	if err != nil {
		t.Fatalf("CanonicalProjectDocument: %v", err)
	}
	content, err := doc.WriteCurrent()
	if err != nil {
		t.Fatalf("WriteCurrent: %v", err)
	}
	absent := ""
	if err := runtime.Service().CommitWorkspaceBootstrap(nil,
		catalog.StorageChange(),
		storage.ArtifactChange{Path: storage.TasksFilePath(project.Path), Data: []byte(content), ExpectedRevision: &absent},
	); err != nil {
		t.Fatalf("create project %s: %v", name, err)
	}
	if err := runtime.State().RefreshAfterCommit(); err != nil {
		t.Fatalf("RefreshAfterCommit: %v", err)
	}
	return project
}

func archiveHeadlessProject(t *testing.T, runtime *Runtime, projectID string) {
	t.Helper()
	domains, err := runtime.State().CloneDomains()
	if err != nil {
		t.Fatalf("CloneDomains: %v", err)
	}
	for index := range domains.Catalog.Projects {
		if entry := &domains.Catalog.Projects[index]; entry.ID == projectID {
			at := runtime.Now()
			entry.Archived, entry.ArchivedAt = true, &at
		}
	}
	artifact, err := runtime.State().CheckedCatalogArtifact(domains.Catalog)
	if err != nil {
		t.Fatalf("CheckedCatalogArtifact: %v", err)
	}
	if err := runtime.Service().CommitConfig(artifact.StorageChange()); err != nil {
		t.Fatalf("archive project: %v", err)
	}
	if err := runtime.Refresh(); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
}

func captureHeadless(t *testing.T, runtime *Runtime, req app.CaptureRequest) app.TaskLocator {
	t.Helper()
	result, err := runtime.Service().Capture(req)
	if err != nil {
		t.Fatalf("Capture(%q): %v", req.Title, err)
	}
	return app.TaskLocator{ProjectID: result.ProjectID, TaskID: result.TaskID}
}

// TestRuntimeQueryAnswersPopulationCensusAndModes evaluates every half of the
// canonical query through the shared runtime: the population a clause obliges,
// the census fields no single task can answer, the lifecycle population rules,
// each mode's membership and order, Scope, and a named Sort.
func TestRuntimeQueryAnswersPopulationCensusAndModes(t *testing.T) {
	env := newHeadlessEnvironment(t)
	runtime := openHeadless(t, env.config(true))
	service := runtime.Service()

	alpha := createHeadlessProject(t, runtime, "Alpha")
	parked := createHeadlessProject(t, runtime, "Shelved", models.ParkedProjectTag)
	archived := createHeadlessProject(t, runtime, "Old")

	parent := captureHeadless(t, runtime, app.CaptureRequest{ProjectID: alpha.ID, Title: "Parent", Priority: 3})
	captureHeadless(t, runtime, app.CaptureRequest{ProjectID: alpha.ID, Title: "Child", ParentID: &parent.TaskID})
	blocker := captureHeadless(t, runtime, app.CaptureRequest{ProjectID: alpha.ID, Title: "Blocker", Priority: 1})
	dependent := captureHeadless(t, runtime, app.CaptureRequest{
		ProjectID: alpha.ID, Title: "Dependent", Dependencies: []app.TaskLocator{blocker},
	})
	tomorrow := time.Date(2026, time.October, 7, 0, 0, 0, 0, time.Local)
	later := tomorrow.AddDate(0, 0, 4)
	captureHeadless(t, runtime, app.CaptureRequest{ProjectID: alpha.ID, Title: "Dated", Priority: 2, Due: &later})
	captureHeadless(t, runtime, app.CaptureRequest{ProjectID: alpha.ID, Title: "Soon", Priority: 3, Due: &tomorrow})
	finished := captureHeadless(t, runtime, app.CaptureRequest{ProjectID: alpha.ID, Title: "Finished"})
	if _, err := service.Transition(app.StateRequest{Locator: finished, Target: models.StateDone}); err != nil {
		t.Fatalf("complete: %v", err)
	}
	deferred := captureHeadless(t, runtime, app.CaptureRequest{ProjectID: alpha.ID, Title: "Deferred"})
	wake := env.clock.Now().Add(48 * time.Hour)
	if _, err := service.Defer(deferred, &wake); err != nil {
		t.Fatalf("Defer: %v", err)
	}
	captureHeadless(t, runtime, app.CaptureRequest{ProjectID: parked.ID, Title: "Parked work"})
	captureHeadless(t, runtime, app.CaptureRequest{ProjectID: archived.ID, Title: "Archived work"})
	archiveHeadlessProject(t, runtime, archived.ID)

	positive := func(field models.QueryField, operator models.QueryOperator, value string) models.QueryClause {
		return clauseOf(t, field, models.QueryPolarityPositive, operator, value)
	}
	present := func(field models.QueryField) models.QueryClause {
		return positive(field, models.QueryOperatorPresent, "")
	}

	t.Run("all", func(t *testing.T) {
		requireTitles(t, "all", runQuery(t, runtime, queryOf(t, models.QueryModeAll)),
			"Blocker", "Dated", "Parent", "Soon", "Child", "Dependent")
	})
	t.Run("census", func(t *testing.T) {
		requireTitles(t, "parent", runQuery(t, runtime, queryOf(t, models.QueryModeAll, present(models.QueryFieldParent))), "Parent")
		requireTitles(t, "child", runQuery(t, runtime, queryOf(t, models.QueryModeAll, present(models.QueryFieldChild))), "Child")
		requireTitles(t, "blocked", runQuery(t, runtime, queryOf(t, models.QueryModeAll, present(models.QueryFieldBlocked))), "Dependent")
		requireTitles(t, "blocking", runQuery(t, runtime, queryOf(t, models.QueryModeAll, present(models.QueryFieldBlocking))), "Blocker")
		notLeaf := clauseOf(t, models.QueryFieldLeaf, models.QueryPolarityNegative, models.QueryOperatorPresent, "")
		requireTitles(t, "-leaf", runQuery(t, runtime, queryOf(t, models.QueryModeAll, notLeaf)), "Parent")
	})
	t.Run("population", func(t *testing.T) {
		parkedClause := positive(models.QueryFieldParked, models.QueryOperatorIs, models.QueryPopulationOnly)
		requireTitles(t, "parked only", runQuery(t, runtime, queryOf(t, models.QueryModeAll, parkedClause)), "Parked work")
		archivedClause := positive(models.QueryFieldArchived, models.QueryOperatorIs, models.QueryPopulationOnly)
		requireTitles(t, "archived only", runQuery(t, runtime, queryOf(t, models.QueryModeAll, archivedClause)), "Archived work")
		completed := positive(models.QueryFieldState, models.QueryOperatorIs, string(models.StateDone))
		requireTitles(t, "done", runQuery(t, runtime, queryOf(t, models.QueryModeAll, completed)), "Finished")
		requireTitles(t, "deferred", runQuery(t, runtime, queryOf(t, models.QueryModeAll, present(models.QueryFieldDeferred))), "Deferred")
	})
	t.Run("modes", func(t *testing.T) {
		requireTitles(t, "due", runQuery(t, runtime, queryOf(t, models.QueryModeDue)), "Soon", "Dated")
		requireTitles(t, "inbox", runQuery(t, runtime, queryOf(t, models.QueryModeInbox)),
			"Blocker", "Dated", "Parent", "Soon", "Dependent")
		requireTitles(t, "backlog", runQuery(t, runtime, queryOf(t, models.QueryModeBacklog)), "Child")

		for _, locator := range []app.TaskLocator{dependent, parent} {
			if _, err := service.ApplyPlanning(app.PlanningRequest{Locator: locator, Action: app.PlanningAddNext}); err != nil {
				t.Fatalf("add next: %v", err)
			}
		}
		next := runQuery(t, runtime, queryOf(t, models.QueryModeNext))
		requireTitles(t, "next", next, "Parent", "Dependent")
		if !next[0].Next || next[0].Today {
			t.Fatalf("next item carries the wrong membership: %+v", next[0])
		}

		env.clock.advance(time.Hour)
		if _, err := service.SetPriority(blocker, 2); err != nil {
			t.Fatalf("SetPriority: %v", err)
		}
		recent := runQuery(t, runtime, queryOf(t, models.QueryModeRecent))
		titles := strings.Join(titlesOf(recent), "|")
		if len(recent) != 8 || recent[0].Task.Title != "Blocker" ||
			!strings.Contains(titles, "Finished") || !strings.Contains(titles, "Deferred") {
			t.Fatalf("recent should list every touched task newest first, terminal and deferred included: %q", titlesOf(recent))
		}
	})
	t.Run("scope and sort", func(t *testing.T) {
		scoped, err := models.NewQuery(models.QueryModeAll, alpha.ID, models.QuerySort{Field: models.QuerySortTitle, Ascending: true})
		if err != nil {
			t.Fatalf("NewQuery: %v", err)
		}
		requireTitles(t, "title sort", runQuery(t, runtime, scoped), "Blocker", "Child", "Dated", "Dependent", "Parent", "Soon")

		parkedScope, err := models.NewQuery(models.QueryModeAll, parked.ID, models.QuerySort{})
		if err != nil {
			t.Fatalf("NewQuery: %v", err)
		}
		if _, err := runtime.Query(parkedScope); !errors.Is(err, app.ErrProjectNotFound) {
			t.Fatalf("a scope outside the population answered: %v", err)
		}
	})
}
