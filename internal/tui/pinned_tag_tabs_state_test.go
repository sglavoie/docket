package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

func TestSaveProjectTreeStatePersistsPinnedTagTab(t *testing.T) {
	app, project := newTestAppModelWithProject()
	project.PinnedTags = []string{"backend"}
	app.projectTree = NewProjectTreeModel(project, app.config, app.configManager)
	app.projectTree.pinnedTagTab = 1

	app.saveProjectTreeState()

	state := app.config.ProjectViewStates[project.ID]
	if got := state.PinnedTagTab; got != "backend" {
		t.Fatalf("PinnedTagTab = %q, want backend", got)
	}
}

func TestRestoreProjectTreeStateRestoresPinnedTagTab(t *testing.T) {
	app, project := newTestAppModelWithProject()
	project.PinnedTags = []string{"backend"}
	app.config.SetProjectViewState(project.ID, config.ProjectViewState{PinnedTagTab: "backend"})
	app.projectTree = NewProjectTreeModel(project, app.config, app.configManager)

	app.restoreProjectTreeState()

	if got := app.projectTree.activePinnedTagFilter(); got != "backend" {
		t.Fatalf("activePinnedTagFilter() = %q, want backend", got)
	}
}

func TestSaveOutlinerFilterStatePersistsPinnedTagTab(t *testing.T) {
	app, project := newTestAppModelWithProject()
	project.PinnedTags = []string{"backend"}
	app.outliner = NewOutlinerModel(project, app.config, app.configManager)
	app.outliner.setPinnedTagTabIndex(1)

	app.saveOutlinerFilterState()

	state := app.config.ProjectViewStates[project.ID]
	if got := state.PinnedTagTab; got != "backend" {
		t.Fatalf("PinnedTagTab = %q, want backend", got)
	}
}

func TestRestoreOutlinerFilterStateRestoresPinnedTagTab(t *testing.T) {
	app, project := newTestAppModelWithProject()
	project.PinnedTags = []string{"backend"}
	app.config.SetProjectViewState(project.ID, config.ProjectViewState{PinnedTagTab: "backend"})
	app.outliner = NewOutlinerModel(project, app.config, app.configManager)

	app.restoreOutlinerFilterState()

	if got := app.outliner.activePinnedTagFilter(); got != "backend" {
		t.Fatalf("activePinnedTagFilter() = %q, want backend", got)
	}
}

func TestSaveCrossOutlinerStatePersistsPinnedTagTab(t *testing.T) {
	app := newTestAppModel()
	app.config.GlobalPinnedTags = []string{"backend"}
	app.crossOutliner = NewOutlinerModel(nil, app.config, app.configManager)
	task := &models.Task{ID: "task", Title: "Backend task", WorkflowState: models.StateReady, Tags: []string{"backend"}}
	project := &models.Project{ID: "project", Name: "Project", Tasks: []*models.Task{task}}
	project.BuildTaskTree()
	app.crossOutliner.loadedProjects = []*models.Project{project}
	app.crossOutliner.setPinnedTagTabIndex(1)

	app.saveCrossOutlinerState()

	if got := app.config.CrossOutlinerPinnedTagTab; got != "backend" {
		t.Fatalf("CrossOutlinerPinnedTagTab = %q, want backend", got)
	}
}

func TestHandleCrossProjectsLoadedRestoresPinnedTagTab(t *testing.T) {
	app := newTestAppModel()
	app.config.GlobalPinnedTags = []string{"backend"}
	app.config.CrossOutlinerPinnedTagTab = "backend"
	task := &models.Task{ID: "task", Title: "Backend task", WorkflowState: models.StateReady, Tags: []string{"backend"}}
	project := &models.Project{ID: "project", Name: "Project", Tasks: []*models.Task{task}}
	project.BuildTaskTree()
	m := NewOutlinerModel(nil, app.config, app.configManager)

	m.handleCrossProjectsLoaded(crossProjectsLoadedMsg{
		projects:   []*models.Project{project},
		generation: 0,
	})

	if got := m.activePinnedTagFilter(); got != "backend" {
		t.Fatalf("activePinnedTagFilter() = %q, want backend", got)
	}
}
