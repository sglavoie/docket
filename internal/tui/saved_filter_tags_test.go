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

	app.projectTree = NewProjectTreeModel(project, app.config, app.configManager)
	app.applySavedFilterToTree(filter)
	if !app.projectTree.filter.Tags["catalog-only"] {
		t.Fatalf("project tree tags = %v, want catalog-only restored", app.projectTree.filter.Tags)
	}
	if app.projectTree.filter.Tags["missing"] {
		t.Fatalf("project tree tags = %v, want missing tag rejected", app.projectTree.filter.Tags)
	}

	app.outliner = NewOutlinerModel(project, app.config, app.configManager)
	app.applySavedFilterToOutliner(filter)
	if !app.outliner.filter.Tags["catalog-only"] {
		t.Fatalf("outliner tags = %v, want catalog-only restored", app.outliner.filter.Tags)
	}
	if app.outliner.filter.Tags["missing"] {
		t.Fatalf("outliner tags = %v, want missing tag rejected", app.outliner.filter.Tags)
	}
}

func TestLoadSavedFilterAppliesCrossProjectCatalogTags(t *testing.T) {
	app := newTestAppModel()
	project := &models.Project{
		ID:   "alpha",
		Name: "Alpha",
		Tags: []string{"catalog-only"},
	}

	app.crossOutliner = NewOutlinerModel(nil, app.config, app.configManager)
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

func TestLoadSavedFilterAppliesCrossProjectSections(t *testing.T) {
	app := newTestAppModel()
	project := &models.Project{
		ID:       "alpha",
		Name:     "Alpha",
		Sections: []string{models.SectionNext, "Sprint-1"},
	}

	app.crossOutliner = NewOutlinerModel(nil, app.config, app.configManager)
	app.crossOutliner.loadedProjects = []*models.Project{project}
	app.pushView(ViewCrossProjectOutliner)

	handled, cmd := app.handleSavedFilterMessages(LoadSavedFilterMsg{
		Filter: config.SavedFilter{
			Name:           "Cross Sections",
			Mode:           "snapshot",
			FilterSections: []string{models.SectionNext, "Sprint-1", "missing"},
		},
	})
	if !handled {
		t.Fatal("LoadSavedFilterMsg should be handled")
	}
	if cmd != nil {
		t.Fatalf("LoadSavedFilterMsg command = %v, want nil", cmd)
	}
	if !app.crossOutliner.filter.Sections[models.SectionNext] {
		t.Fatalf("cross-project sections = %v, want Next restored", app.crossOutliner.filter.Sections)
	}
	if app.crossOutliner.filter.Sections["Sprint-1"] {
		t.Fatalf("cross-project sections = %v, want Sprint-1 rejected", app.crossOutliner.filter.Sections)
	}
	if app.crossOutliner.filter.Sections["missing"] {
		t.Fatalf("cross-project sections = %v, want missing section rejected", app.crossOutliner.filter.Sections)
	}
}

func TestCaptureAndLoadSavedFilterRestoresSingleProjectPinnedTagTab(t *testing.T) {
	app, project := newTestAppModelWithProject()
	project.PinnedTags = []string{"backend"}
	app.outliner = NewOutlinerModel(project, app.config, app.configManager)
	app.pushView(ViewOutliner)
	app.outliner.setPinnedTagTabIndex(1)

	filter := app.captureCurrentFilter("Single Backend")

	if filter.PinnedTagTab != "backend" {
		t.Fatalf("PinnedTagTab = %q, want backend", filter.PinnedTagTab)
	}

	app.outliner.setPinnedTagTabIndex(0)
	handled, cmd := app.handleSavedFilterMessages(LoadSavedFilterMsg{Filter: filter})
	if !handled {
		t.Fatal("LoadSavedFilterMsg should be handled")
	}
	if cmd != nil {
		t.Fatalf("LoadSavedFilterMsg command = %v, want nil", cmd)
	}
	if got := app.outliner.activePinnedTagFilter(); got != "backend" {
		t.Fatalf("activePinnedTagFilter() = %q, want backend", got)
	}
}

func TestCaptureAndLoadSavedFilterRestoresCrossProjectPinnedTagTab(t *testing.T) {
	app := newTestAppModel()
	app.config.GlobalPinnedTags = []string{"backend"}
	task := &models.Task{ID: "task", Title: "Backend", WorkflowState: models.StateReady, Tags: []string{"backend"}}
	project := &models.Project{
		ID:    "alpha",
		Name:  "Alpha",
		Tasks: []*models.Task{task},
	}
	project.BuildTaskTree()
	app.crossOutliner = NewOutlinerModel(nil, app.config, app.configManager)
	app.crossOutliner.loadedProjects = []*models.Project{project}
	app.pushView(ViewCrossProjectOutliner)
	app.crossOutliner.setPinnedTagTabIndex(1)

	filter := app.captureCurrentFilter("Cross Backend")

	if filter.PinnedTagTab != "backend" {
		t.Fatalf("PinnedTagTab = %q, want backend", filter.PinnedTagTab)
	}

	app.crossOutliner.setPinnedTagTabIndex(0)
	handled, cmd := app.handleSavedFilterMessages(LoadSavedFilterMsg{Filter: filter})
	if !handled {
		t.Fatal("LoadSavedFilterMsg should be handled")
	}
	if cmd != nil {
		t.Fatalf("LoadSavedFilterMsg command = %v, want nil", cmd)
	}
	if got := app.crossOutliner.activePinnedTagFilter(); got != "backend" {
		t.Fatalf("activePinnedTagFilter() = %q, want backend", got)
	}
}
