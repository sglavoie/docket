package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

func TestSaveOutlinerFilterStatePersistsPinnedTagTab(t *testing.T) {
	app, project := newTestAppModelWithProject()
	project.PinnedTags = []string{"backend"}
	app.outliner = NewOutlinerModel(project, app.configManager, app.runtime)
	app.outliner.setPinnedTagTabIndex(1)

	app.saveOutlinerFilterState()

	state := app.runtime.Machine().ProjectViewStates[project.ID]
	if got := state.PinnedTagTab; got != "backend" {
		t.Fatalf("PinnedTagTab = %q, want backend", got)
	}
}

func TestRestoreOutlinerFilterStateRestoresPinnedTagTab(t *testing.T) {
	app, project := newTestAppModelWithProject()
	project.PinnedTags = []string{"backend"}
	app.runtime.Machine().SetProjectViewState(project.ID, config.ProjectViewState{PinnedTagTab: "backend"})
	app.outliner = NewOutlinerModel(project, app.configManager, app.runtime)

	app.restoreOutlinerFilterState()

	if got := app.outliner.activePinnedTagFilter(); got != "backend" {
		t.Fatalf("activePinnedTagFilter() = %q, want backend", got)
	}
}

func TestSaveCrossOutlinerStatePersistsPinnedTagTab(t *testing.T) {
	app := newTestAppModel()
	app.runtime.Settings().GlobalPinnedTags = []string{"backend"}
	syncTestRuntime(app)
	app.crossOutliner = NewOutlinerModel(nil, app.configManager, app.runtime)
	task := &models.Task{ID: "task", Title: "Backend task", WorkflowState: models.StateReady, Tags: []string{"backend"}}
	project := &models.Project{ID: "project", Name: "Project", Tasks: []*models.Task{task}}
	project.BuildTaskTree()
	app.crossOutliner.loadedProjects = []*models.Project{project}
	app.crossOutliner.setPinnedTagTabIndex(1)

	app.saveCrossOutlinerState()

	if got := app.runtime.Machine().CrossOutlinerPinnedTagTab; got != "backend" {
		t.Fatalf("CrossOutlinerPinnedTagTab = %q, want backend", got)
	}
}

func TestHandleCrossProjectsLoadedRestoresPinnedTagTab(t *testing.T) {
	app := newTestAppModel()
	app.runtime.Settings().GlobalPinnedTags = []string{"backend"}
	syncTestRuntime(app)
	app.runtime.Machine().CrossOutlinerPinnedTagTab = "backend"
	task := &models.Task{ID: "task", Title: "Backend task", WorkflowState: models.StateReady, Tags: []string{"backend"}}
	project := &models.Project{ID: "project", Name: "Project", Tasks: []*models.Task{task}}
	project.BuildTaskTree()
	m := NewOutlinerModel(nil, app.configManager, app.runtime)

	m.handleCrossProjectsLoaded(crossProjectsLoadedMsg{
		projects:   []*models.Project{project},
		generation: 0,
	})

	if got := m.activePinnedTagFilter(); got != "backend" {
		t.Fatalf("activePinnedTagFilter() = %q, want backend", got)
	}
}
