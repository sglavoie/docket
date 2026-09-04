package tui

import (
	"fmt"
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/route"
)

// projectHubApp builds a two-project shell whose cross-project Outliner is
// already populated, so hub rows carry real task counts.
func projectHubApp(t *testing.T) *AppModel {
	t.Helper()
	app := typedTestApp(t)
	alpha := scopeTestProject("alpha", "Alpha", "1", "2")
	alpha.Slug = "alpha"
	alpha.Path = t.TempDir()
	beta := scopeTestProject("beta", "Beta", "1")
	beta.Slug = "beta"
	beta.Path = t.TempDir()
	setTestAppProjects(app, alpha, beta)
	app.currentProject = nil
	app.crossOutliner = NewOutlinerModel(nil, app.configManager, app.runtime)
	app.crossOutliner.SetSize(120, 40)
	app.crossOutliner.loadedProjects = []*models.Project{alpha, beta}
	app.crossOutliner.crossForceRebuild = true
	app.crossOutliner.rebuildRows()
	return app
}

func openProjectHub(t *testing.T, app *AppModel, modes ...route.ProjectHubMode) *ProjectHubModel {
	t.Helper()
	mode := route.ProjectHubModeAll
	if len(modes) > 0 {
		mode = modes[0]
	}
	handled, cmd := app.handleProjectViewMessages(ShowProjectHubMsg{Mode: mode})
	if !handled {
		t.Fatal("ShowProjectHubMsg was not handled")
	}
	if cmd != nil {
		cmd()
	}
	hub, ok := app.manageHub.ActiveModuleView().(*ProjectHubModel)
	if !ok {
		t.Fatalf("active Manage module view = %T, want *ProjectHubModel", app.manageHub.ActiveModuleView())
	}
	hub.SetSize(120, 30)
	return hub
}

func focusProjectHubItem(t *testing.T, hub *ProjectHubModel, id listsurface.ListItemID) {
	t.Helper()
	if hub.surface == nil || !hub.surface.SetCursorToID(id) {
		t.Fatalf("project hub omitted item %q", id)
	}
}

// projectHubDiskApp roots the shell at a real data directory, so a cataloged
// project the Outliner never loads still resolves to an on-disk tasks.md.
func projectHubDiskApp(t *testing.T) *AppModel {
	t.Helper()
	app := typedTestApp(t)
	app.runtime = testRuntimeAt(t.TempDir(), newTestConfig())
	alpha := scopeTestProject("alpha", "Alpha", "1", "2")
	beta := scopeTestProject("beta", "Beta", "1")
	setTestAppProjects(app, alpha, beta)
	app.currentProject = nil
	app.crossOutliner = NewOutlinerModel(nil, app.configManager, app.runtime)
	app.crossOutliner.SetSize(120, 40)
	app.crossOutliner.loadedProjects = []*models.Project{alpha, beta}
	app.crossOutliner.crossForceRebuild = true
	app.crossOutliner.rebuildRows()
	return app
}

// projectHubDiskProject builds a project at the path the runtime derives for its
// slug, without cataloging it. The caller decides what reaches disk there.
func projectHubDiskProject(app *AppModel, id, name string) *models.Project {
	project := scopeTestProject(id, name)
	project.Path = app.runtime.ProjectPath(project.Slug)
	return project
}

func writeProjectHubTasks(t *testing.T, project *models.Project, count int) {
	t.Helper()
	for i := 1; i <= count; i++ {
		task, err := models.NewGenericTask(fmt.Sprintf("%s-%d", project.ID, i), i, fmt.Sprintf("Task %d", i))
		if err != nil {
			t.Fatalf("NewGenericTask: %v", err)
		}
		task.Project = project
		project.Tasks = append(project.Tasks, task)
	}
	project.NextShortID = count + 1
	project.BuildTaskTree()
	if err := storage.SaveProject(project); err != nil {
		t.Fatalf("SaveProject(%q): %v", project.ID, err)
	}
}

// finishProjectHubTasks completes the first n tasks a project holds and rewrites
// its tasks file, so the Hub reads a total that outruns the open count.
func finishProjectHubTasks(t *testing.T, project *models.Project, n int) {
	t.Helper()
	if n > len(project.Tasks) {
		t.Fatalf("finishProjectHubTasks(%q, %d): project holds %d tasks", project.ID, n, len(project.Tasks))
	}
	for _, task := range project.Tasks[:n] {
		markDone(task)
	}
	project.BuildTaskTree()
	if err := storage.SaveProject(project); err != nil {
		t.Fatalf("SaveProject(%q): %v", project.ID, err)
	}
}

func projectHubItem(t *testing.T, hub *ProjectHubModel, id listsurface.ListItemID) listsurface.ListItem {
	t.Helper()
	for _, item := range hub.surface.Items() {
		if item.ID == id {
			return item
		}
	}
	t.Fatalf("project hub omitted item %q", id)
	return listsurface.ListItem{}
}

// projectHubPreviewSummary reads the preview header's one muted meta line, which
// carries the lifecycle, the tally, the slug, and a pin when there is one. The
// five labeled fields it replaced spent six rows saying the same thing and left
// the task pane below them nowhere to go.
func projectHubPreviewSummary(t *testing.T, hub *ProjectHubModel, id listsurface.ListItemID) string {
	t.Helper()
	preview := projectHubItem(t, hub, id).Preview
	if len(preview.Fields) > 0 {
		t.Fatalf("project hub preview for %q carries %d labeled fields, want the meta line alone", id, len(preview.Fields))
	}
	return preview.Summary
}

// projectHubFilterApp catalogs a project the Outliner loaded with no tasks, so
// the Hub holds a known zero for the Ctrl+T filter to hide. Its name shares the
// "bet" prefix with Beta, so one query can match a counted and an empty row.
func projectHubFilterApp(t *testing.T) *AppModel {
	t.Helper()
	app := projectHubApp(t)
	empty := scopeTestProject("empty", "Beta Empty")
	empty.Path = t.TempDir()
	addTestAppProject(app, empty)
	app.crossOutliner.loadedProjects = append(app.crossOutliner.loadedProjects, empty)
	return app
}

func projectHubHasItem(hub *ProjectHubModel, id listsurface.ListItemID) bool {
	for _, item := range hub.surface.Items() {
		if item.ID == id {
			return true
		}
	}
	return false
}

func projectHubActionIDs(hub *ProjectHubModel) []operation.OperationID {
	if hub.actions == nil || hub.actions.Surface == nil {
		return nil
	}
	ids := make([]operation.OperationID, 0)
	for _, item := range hub.actions.Surface.Items() {
		if descriptor, ok := item.Data.(operation.OperationDescriptor); ok {
			ids = append(ids, descriptor.ID)
		}
	}
	return ids
}
