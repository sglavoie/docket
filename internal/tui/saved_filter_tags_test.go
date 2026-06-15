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
