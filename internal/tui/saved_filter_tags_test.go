package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

func TestApplySavedFilterToProjectViewsKeepsZeroUseCatalogTag(t *testing.T) {
	app, project := newTestAppModelWithProject()
	project.Tags = []string{"catalog-only"}

	filter := config.SavedFilter{
		Name:       "Catalog Tags",
		Mode:       "snapshot",
		FilterTags: []string{"catalog-only", "missing"},
	}

	app.crossOutliner = NewOutlinerModel(project, app.configManager, app.runtime)
	app.applySavedFilterToOutliner(filter)
	if !app.crossOutliner.filter.Tags["catalog-only"] {
		t.Fatalf("outliner tags = %v, want catalog-only restored", app.crossOutliner.filter.Tags)
	}
	if app.crossOutliner.filter.Tags["missing"] {
		t.Fatalf("outliner tags = %v, want missing tag rejected", app.crossOutliner.filter.Tags)
	}
}

func TestLoadSavedFilterAppliesCrossProjectCatalogTags(t *testing.T) {
	app := newTestAppModel()
	project := &models.Project{
		ID:   "alpha",
		Name: "Alpha",
		Tags: []string{"catalog-only"},
	}

	app.crossOutliner = NewOutlinerModel(nil, app.configManager, app.runtime)
	app.crossOutliner.loadedProjects = []*models.Project{project}
	app.pushView(ViewCrossProjectOutliner)

	handled, cmd := app.handleSavedFilterMessages(LoadSavedFilterMsg{
		Filter: config.SavedFilter{
			Name:       "Cross Catalog",
			Mode:       "snapshot",
			FilterTags: []string{"catalog-only", "missing"},
		},
	})
	if !handled {
		t.Fatal("LoadSavedFilterMsg should be handled")
	}
	if cmd != nil {
		t.Fatalf("LoadSavedFilterMsg command = %v, want nil", cmd)
	}
	if !app.crossOutliner.filter.Tags["catalog-only"] {
		t.Fatalf("cross-project tags = %v, want catalog-only restored", app.crossOutliner.filter.Tags)
	}
	if app.crossOutliner.filter.Tags["missing"] {
		t.Fatalf("cross-project tags = %v, want missing tag rejected", app.crossOutliner.filter.Tags)
	}
	if app.crossOutliner.activeSavedFilter != "Cross Catalog" {
		t.Fatalf("activeSavedFilter = %q, want Cross Catalog", app.crossOutliner.activeSavedFilter)
	}
}

func TestLoadSavedFilterAppliesCrossProjectStates(t *testing.T) {
	app := newTestAppModel()
	project := &models.Project{
		ID:   "alpha",
		Name: "Alpha",
	}

	app.crossOutliner = NewOutlinerModel(nil, app.configManager, app.runtime)
	app.crossOutliner.loadedProjects = []*models.Project{project}
	app.pushView(ViewCrossProjectOutliner)

	filter := config.SavedFilter{
		Name: "Cross States",
		Mode: "snapshot",
	}
	filter.SetFilterStates(testWorkflowStateSet(models.StateNext, models.StateWaiting), true)

	handled, cmd := app.handleSavedFilterMessages(LoadSavedFilterMsg{
		Filter: filter,
	})
	if !handled {
		t.Fatal("LoadSavedFilterMsg should be handled")
	}
	if cmd != nil {
		t.Fatalf("LoadSavedFilterMsg command = %v, want nil", cmd)
	}
	if !app.crossOutliner.filter.States.Contains(models.StateNext) {
		t.Fatalf("cross-project states = %v, want Next restored", app.crossOutliner.filter.States.Values())
	}
	if !app.crossOutliner.filter.States.Contains(models.StateWaiting) {
		t.Fatalf("cross-project states = %v, want Waiting restored", app.crossOutliner.filter.States.Values())
	}
	if got := app.crossOutliner.filter.States.Len(); got != 2 {
		t.Fatalf("cross-project states = %v, want exactly two canonical states", app.crossOutliner.filter.States.Values())
	}
}

func TestCaptureAndApplySavedFilterPreservesCanonicalStateSet(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.crossOutliner = NewOutlinerModel(project, app.configManager, app.runtime)
	app.crossOutliner.filter.setStateFilter(testWorkflowStateSet(models.StateInbox, models.StateNext), true)

	captured := app.captureOutlinerFilter("Planning", "planning")
	if !captured.HasStateFilter() || !captured.FilterStates.Equal(app.crossOutliner.filter.States) {
		t.Fatalf("captured states = %v active=%v, want %v", captured.FilterStates.Values(), captured.HasStateFilter(), app.crossOutliner.filter.States.Values())
	}

	app.crossOutliner.filter.setStateFilter(testWorkflowStateSet(models.StateDone), true)
	app.applySavedFilterToOutliner(captured)
	if !app.crossOutliner.filter.States.Equal(captured.FilterStates) {
		t.Fatalf("applied states = %v, want %v", app.crossOutliner.filter.States.Values(), captured.FilterStates.Values())
	}
}
