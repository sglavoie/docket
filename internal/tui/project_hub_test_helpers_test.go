package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/storage"
	"github.com/sglavoie/docket/internal/tui/outliner"
	"github.com/sglavoie/docket/internal/tui/projecthub"
	"github.com/sglavoie/docket/internal/tui/route"
)

// The Project Hub's own coverage lives in internal/tui/projecthub, against a
// fixture that reads the surface directly. What these helpers mount is the shell:
// a real AppModel whose registry, executor, count pass and preview painter the
// Hub is wired to. The Hub's cursor is its own, so a shell test steers it through
// the browse grammar rather than reaching across the package boundary.

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
	app.crossOutliner = outliner.NewOutlinerModel(nil, app.domain.manager, app.domain.runtime)
	app.crossOutliner.SetSize(120, 40)
	app.crossOutliner.Population.Projects = []*models.Project{alpha, beta}
	app.crossOutliner.Population.Invalidate()
	app.crossOutliner.RebuildRows()
	return app
}

func openProjectHub(t *testing.T, app *AppModel, modes ...route.ProjectHubMode) *projecthub.ProjectHubModel {
	t.Helper()
	mode := route.ProjectHubModeAll
	if len(modes) > 0 {
		mode = modes[0]
	}
	handled, cmd := app.handleProjectViewMessages(projecthub.ShowProjectHubMsg{Mode: mode})
	if !handled {
		t.Fatal("ShowProjectHubMsg was not handled")
	}
	if cmd != nil {
		cmd()
	}
	hub, ok := app.manageHub.ActiveModuleView().(*projecthub.ProjectHubModel)
	if !ok {
		t.Fatalf("active Manage module view = %T, want *ProjectHubModel", app.manageHub.ActiveModuleView())
	}
	hub.SetSize(120, 30)
	return hub
}

// focusProjectHubRow puts the Hub's cursor on the row naming one project the way
// a user does: it searches for the name and browses the matches the query left.
// The fixtures below name their projects distinctly enough for the query to land
// on one row, and the assertion holds it to that.
func focusProjectHubRow(t *testing.T, hub *projecthub.ProjectHubModel, name string) {
	t.Helper()
	hub.ClearSearch()
	hub.Update(syntheticKey('/'))
	for _, r := range strings.ToLower(name) {
		hub.Update(syntheticKey(r))
	}
	hub.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if got := projectHubCursorRow(t, hub); !strings.Contains(got, name) {
		t.Fatalf("searching %q left the Project Hub cursor on %q", name, got)
	}
}

// projectHubCursorRow reads the row the Hub draws its cursor marker on, which is
// the only place the rendered frame says where the cursor is.
func projectHubCursorRow(t *testing.T, hub *projecthub.ProjectHubModel) string {
	t.Helper()
	rendered := hub.View().Content
	for _, line := range strings.Split(rendered, "\n") {
		if row, marked := strings.CutPrefix(ansi.Strip(line), "› "); marked {
			return row
		}
	}
	t.Fatalf("the Project Hub frame carries no cursor row:\n%s", rendered)
	return ""
}

// projectHubSubtitleLine reads the Hub's header line, where the subtitle names
// the narrowing the list is under. The footer hint names a filter too -- as the
// state the next press produces -- so a whole-frame search cannot tell the two
// apart.
func projectHubSubtitleLine(hub *projecthub.ProjectHubModel) string {
	return ansi.Strip(strings.SplitN(hub.View().Content, "\n", 2)[0])
}

// projectHubDiskApp roots the shell at a real data directory, so a cataloged
// project the Outliner never loads still resolves to an on-disk tasks.md.
func projectHubDiskApp(t *testing.T) *AppModel {
	t.Helper()
	app := typedTestApp(t)
	app.domain.runtime = testRuntimeAt(t.TempDir(), newTestConfig())
	alpha := scopeTestProject("alpha", "Alpha", "1", "2")
	beta := scopeTestProject("beta", "Beta", "1")
	setTestAppProjects(app, alpha, beta)
	app.currentProject = nil
	app.crossOutliner = outliner.NewOutlinerModel(nil, app.domain.manager, app.domain.runtime)
	app.crossOutliner.SetSize(120, 40)
	app.crossOutliner.Population.Projects = []*models.Project{alpha, beta}
	app.crossOutliner.Population.Invalidate()
	app.crossOutliner.RebuildRows()
	return app
}

// projectHubDiskProject builds a project at the path the runtime derives for its
// slug, without cataloging it. The caller decides what reaches disk there.
func projectHubDiskProject(app *AppModel, id, name string) *models.Project {
	project := scopeTestProject(id, name)
	project.Path = app.domain.runtime.ProjectPath(project.Slug)
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
