package tui

import (
	"slices"
	"strings"
	"testing"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/listsurface"
	"github.com/sglavoie/docket/internal/tui/operation"
	"github.com/sglavoie/docket/internal/tui/outliner"
	"github.com/sglavoie/docket/internal/tui/query"
)

// newProjectFilterTestApp loads three projects into the cross-project
// Outliner. "alphabet" shares a prefix with "alpha" so the facet's exact slug
// match can be told apart from search's prefix reading.
func newProjectFilterTestApp(t *testing.T) *AppModel {
	t.Helper()
	app := newUITestApp()
	project := func(id, name, slug string, tasks ...*models.Task) *models.Project {
		p := &models.Project{ID: id, Name: name, Slug: slug, Tasks: tasks}
		p.BuildTaskTree()
		return p
	}
	alpha := project("project-alpha", "Alpha", "alpha",
		&models.Task{ID: "alpha-one", ShortID: 1, Title: "Alpha one", WorkflowState: models.StateBacklog},
		&models.Task{ID: "alpha-two", ShortID: 2, Title: "Alpha two", WorkflowState: models.StateBacklog})
	alphabet := project("project-alphabet", "Alphabet", "alphabet",
		&models.Task{ID: "alphabet-one", ShortID: 1, Title: "Alphabet one", WorkflowState: models.StateBacklog})
	beta := project("project-beta", "Beta", "beta",
		&models.Task{ID: "beta-one", ShortID: 1, Title: "Beta one", WorkflowState: models.StateBacklog})
	setTestAppProjects(app, alpha, alphabet, beta)
	app.crossOutliner.Population.Projects = []*models.Project{alpha, alphabet, beta}
	app.crossOutliner.Population.Invalidate()
	app.crossOutliner.RebuildRows()
	return app
}

func TestFilterProjectsChordPickerListsLoadedProjectsSeededFromTheFacet(t *testing.T) {
	app := newProjectFilterTestApp(t)
	app.crossOutliner.Filter.Projects = []string{"beta", "retired"}

	cmd := pressFilterChord(t, app, "f%")
	if cmd == nil {
		t.Fatal("f% produced no value-request command")
	}
	app.handleOperationOverlayMessage(cmd())
	picker := app.operations.ValuePicker
	if picker == nil || picker.Provider() != operation.ValueProviderFilterProjects {
		t.Fatalf("f%% did not open the project picker: %#v", picker)
	}
	if title := picker.Title(); title != "Filter by Projects" {
		t.Errorf("picker title = %q, want %q", title, "Filter by Projects")
	}

	items := picker.Surface().Items()
	wantIDs := []listsurface.ListItemID{"filter-project:alpha", "filter-project:alphabet", "filter-project:beta"}
	gotIDs := make([]listsurface.ListItemID, 0, len(items))
	for _, item := range items {
		gotIDs = append(gotIDs, item.ID)
	}
	if !slices.Equal(gotIDs, wantIDs) {
		t.Fatalf("project picker items = %v, want %v", gotIDs, wantIDs)
	}
	if items[0].PrimaryText != "Alpha" || items[0].SecondaryText != "[alpha] · 2 tasks" {
		t.Errorf("alpha row = %q / %q, want the name then [slug] and its count",
			items[0].PrimaryText, items[0].SecondaryText)
	}

	// A seeded slug no row carries is dropped rather than kept invisibly checked.
	selected := picker.Surface().SelectedIDs()
	if !slices.Equal(selected, []listsurface.ListItemID{"filter-project:beta"}) {
		t.Fatalf("seeded selection = %v, want only the known beta row", selected)
	}
}

// The slugs compose with OR because a task belongs to exactly one project, and
// each matches exactly, so naming alpha leaves alphabet out.
func TestFilterProjectsOperationUnionsExactSlugsAndClearsWithFx(t *testing.T) {
	app := newProjectFilterTestApp(t)
	ctx := simplifiedOperationContext()
	out := app.crossOutliner

	result := app.operations.Executor.Execute(operation.OperationExecutionRequest{
		ID:       operation.OperationFilterProjects,
		Context:  ctx,
		Value:    []string{"alpha", "beta"},
		HasValue: true,
	})
	if result.Err != nil {
		t.Fatalf("project filter error: %v", result.Err)
	}
	if result.Status != "Filtering 2 projects" {
		t.Errorf("Status = %q, want %q", result.Status, "Filtering 2 projects")
	}
	visible := visibleOutlinerTaskIDs(out)
	for _, id := range []string{"alpha-one", "alpha-two", "beta-one"} {
		if !visible[id] {
			t.Errorf("a task in a named project is hidden: %s in %v", id, visible)
		}
	}
	if visible["alphabet-one"] {
		t.Errorf("a project sharing only a prefix was admitted: %v", visible)
	}
	if got := query.CommonFilterParts(&out.Filter.CommonFilterState); !slices.Equal(got, []string{"%alpha", "%beta"}) {
		t.Errorf("chips = %v, want the %%slug spelling", got)
	}
	if got := out.ActiveFilterCount(); got != 1 {
		t.Errorf("filter count = %d, want the project facet counted once", got)
	}

	cleared := app.operations.Executor.Execute(operation.OperationExecutionRequest{
		ID:      operation.OperationFilterClear,
		Context: ctx,
	})
	if cleared.Err != nil {
		t.Fatalf("fx error: %v", cleared.Err)
	}
	if len(out.Filter.Projects) != 0 || len(query.ClausesForField(out.FilterQuery, models.QueryFieldProject)) != 0 {
		t.Fatalf("fx left the project facet standing: %v / %s", out.Filter.Projects, out.FilterQuery.String())
	}
	if !visibleOutlinerTaskIDs(out)["alphabet-one"] {
		t.Error("clearing the facet did not bring every project back")
	}
}

// A lens saves the facet as its %slug clauses and reinstalls it whole.
func TestLensStateRoundTripsProjectFacet(t *testing.T) {
	want := lensStateWithFacets(t, lensRoundTripBase(t), "%alpha %beta")
	stored := want.NamedLens(lensRoundTripID, "Two projects")
	if err := stored.Validate(); err != nil {
		t.Fatalf("stored definition invalid: %v", err)
	}
	got, err := outliner.LensStateFromNamedLens(stored)
	if err != nil {
		t.Fatalf("LensStateFromNamedLens: %v", err)
	}
	if got.Fingerprint() != want.Fingerprint() {
		t.Fatalf("round trip = %q, want %q", got.Fingerprint(), want.Fingerprint())
	}
	if strings.Contains(got.Query, "%") {
		t.Fatalf("project clauses leaked into the remainder: %q", got.Query)
	}

	app := newProjectFilterTestApp(t)
	got.ScopeProjectID = ""
	app.crossOutliner.ApplyLensState(got)
	if !slices.Equal(app.crossOutliner.Filter.Projects, []string{"alpha", "beta"}) {
		t.Fatalf("applied facet = %v, want [alpha beta]", app.crossOutliner.Filter.Projects)
	}
}
