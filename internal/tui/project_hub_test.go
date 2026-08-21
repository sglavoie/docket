package tui

import (
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
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

func openProjectHub(t *testing.T, app *AppModel) *ProjectHubModel {
	t.Helper()
	handled, cmd := app.handleProjectViewMessages(ShowProjectHubMsg{})
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

func focusProjectHubItem(t *testing.T, hub *ProjectHubModel, id ListItemID) {
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

func projectHubItem(t *testing.T, hub *ProjectHubModel, id ListItemID) ListItem {
	t.Helper()
	for _, item := range hub.surface.Items() {
		if item.ID == id {
			return item
		}
	}
	t.Fatalf("project hub omitted item %q", id)
	return ListItem{}
}

func projectHubPreviewTasks(t *testing.T, hub *ProjectHubModel, id ListItemID) string {
	t.Helper()
	for _, field := range projectHubItem(t, hub, id).Preview.Fields {
		if field.Label == "Tasks" {
			return field.Value
		}
	}
	t.Fatalf("project hub preview for %q carries no Tasks field", id)
	return ""
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

func projectHubHasItem(hub *ProjectHubModel, id ListItemID) bool {
	for _, item := range hub.surface.Items() {
		if item.ID == id {
			return true
		}
	}
	return false
}

func projectHubActionIDs(hub *ProjectHubModel) []OperationID {
	if hub.actions == nil || hub.actions.surface == nil {
		return nil
	}
	ids := make([]OperationID, 0)
	for _, item := range hub.actions.surface.Items() {
		if descriptor, ok := item.Data.(OperationDescriptor); ok {
			ids = append(ids, descriptor.ID)
		}
	}
	return ids
}

func TestProjectHubListsEveryProjectByLifecycle(t *testing.T) {
	app := projectHubApp(t)
	archived := scopeTestProject("archived", "Archived", "1")
	archived.Slug = "archived"
	archived.Archived = true
	archived.Path = t.TempDir()
	addTestAppProject(app, archived)

	hub := openProjectHub(t, app)
	wantCategories := []ListCategory{
		{ID: projectHubCategoryActive, Label: "Active Projects"},
		{ID: projectHubCategoryArchived, Label: "Archived Projects"},
	}
	if got := hub.surface.Categories(); !reflect.DeepEqual(got, wantCategories) {
		t.Fatalf("project categories = %#v, want %#v", got, wantCategories)
	}
	for _, id := range []ListItemID{
		"project-hub:project:alpha",
		"project-hub:project:beta",
		"project-hub:archived:archived",
	} {
		if !hub.surface.SetCursorToID(id) {
			t.Errorf("Project Hub omitted %q", id)
		}
	}
	for _, item := range hub.surface.Items() {
		if _, ok := item.Data.(projectHubEntry); !ok {
			t.Fatalf("Project Hub row carries %#v, want a project entry", item.Data)
		}
	}
}

func TestProjectHubCountsParkedAndArchivedProjects(t *testing.T) {
	app := projectHubDiskApp(t)
	parked := projectHubDiskProject(app, "parked", "Parked")
	parked.Tags = []string{"parked"}
	writeProjectHubTasks(t, parked, 3)
	addTestAppProject(app, parked)
	archived := projectHubDiskProject(app, "archived", "Archived")
	archived.Archived = true
	writeProjectHubTasks(t, archived, 2)
	addTestAppProject(app, archived)

	hub := openProjectHub(t, app)
	if got := projectHubItem(t, hub, "project-hub:project:parked").SecondaryText; got != "3 tasks · #parked" {
		t.Fatalf("parked row secondary = %q, want the real count", got)
	}
	if got := projectHubPreviewTasks(t, hub, "project-hub:archived:archived"); got != "2" {
		t.Fatalf("archived row task count = %q, want %q", got, "2")
	}
}

func TestProjectHubCountsProjectWithoutTasksFileAsZero(t *testing.T) {
	app := projectHubDiskApp(t)
	empty := projectHubDiskProject(app, "empty", "Empty")
	if err := os.MkdirAll(empty.Path, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", empty.Path, err)
	}
	addTestAppProject(app, empty)

	hub := openProjectHub(t, app)
	if got := projectHubItem(t, hub, "project-hub:project:empty").SecondaryText; got != "0 tasks" {
		t.Fatalf("empty project row secondary = %q, want %q", got, "0 tasks")
	}
}

func TestProjectHubKeepsUnknownCountForUnreadableProject(t *testing.T) {
	app := projectHubDiskApp(t)
	corrupt := projectHubDiskProject(app, "corrupt", "Corrupt")
	if err := os.MkdirAll(corrupt.Path, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", corrupt.Path, err)
	}
	tasksPath := storage.TasksFilePath(corrupt.Path)
	if err := os.WriteFile(tasksPath, []byte("this is not a task document\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(%q): %v", tasksPath, err)
	}
	addTestAppProject(app, corrupt)

	hub := openProjectHub(t, app)
	if got := projectHubItem(t, hub, "project-hub:project:corrupt").SecondaryText; got != "tasks not loaded" {
		t.Fatalf("unreadable project row secondary = %q, want %q", got, "tasks not loaded")
	}
	if hub.notice != "" {
		t.Fatalf("unreadable project raised notice %q, want the Hub silent", hub.notice)
	}
}

func TestProjectHubReusesOutlinerLoaderAcrossOpens(t *testing.T) {
	app := projectHubDiskApp(t)
	parked := projectHubDiskProject(app, "parked", "Parked")
	parked.Tags = []string{"parked"}
	writeProjectHubTasks(t, parked, 1)
	addTestAppProject(app, parked)

	loader := app.crossOutliner.projectService.workspaceLoader
	before := loader.ParseCount()
	openProjectHub(t, app)
	first := loader.ParseCount()
	if first == before {
		t.Fatalf("first Hub open parsed nothing, parse count stayed %d", first)
	}
	openProjectHub(t, app)
	if second := loader.ParseCount(); second != first {
		t.Fatalf("second Hub open parse count = %d, want %d", second, first)
	}
}

func TestProjectHubEscKeepsFilterThenCloses(t *testing.T) {
	app := projectHubApp(t)
	hub := openProjectHub(t, app)
	if hub.HasActiveSearch() || hub.HasTextInput() || hub.ShouldCaptureEsc() {
		t.Fatalf(
			"the Hub opened searching: active=%v input=%v capture=%v",
			hub.HasActiveSearch(),
			hub.HasTextInput(),
			hub.ShouldCaptureEsc(),
		)
	}
	hub.Update(syntheticKey('/'))
	if !hub.HasActiveSearch() || !hub.HasTextInput() || !hub.ShouldCaptureEsc() {
		t.Fatalf(
			"'/' did not open search: active=%v input=%v capture=%v",
			hub.HasActiveSearch(),
			hub.HasTextInput(),
			hub.ShouldCaptureEsc(),
		)
	}
	for _, r := range "bet" {
		hub.Update(syntheticKey(r))
	}
	if hub.SearchQuery() != "bet" || len(hub.surface.Matches()) != 1 {
		t.Fatalf("query=%q matches=%d, want beta only", hub.SearchQuery(), len(hub.surface.Matches()))
	}

	hub.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if hub.HasActiveSearch() || hub.SearchQuery() != "bet" {
		t.Fatalf("Esc left search active=%v query=%q", hub.HasActiveSearch(), hub.SearchQuery())
	}
	if app.CurrentRoute().ID() != SecondaryTool || app.currentView() != ViewProjectHub {
		t.Fatalf("first Esc closed Project Hub: %s/%s", app.CurrentRoute().ID(), app.currentView())
	}
	if len(hub.surface.Matches()) != 1 {
		t.Fatalf("Esc left %d matches, want the filter intact", len(hub.surface.Matches()))
	}

	handled, _ := app.handleGlobalKeyPress(tea.KeyPressMsg{Code: tea.KeyEscape})
	if !handled || app.CurrentRoute().ID() != TaskWorkspace {
		t.Fatalf("filtered browse Esc handled=%v route=%s, want Outliner", handled, app.CurrentRoute().ID())
	}
}

func TestProjectHubSearchEnterSelectsImmediately(t *testing.T) {
	app := projectHubApp(t)
	hub := openProjectHub(t, app)
	hub.Update(syntheticKey('/'))
	for _, r := range "bet" {
		hub.Update(syntheticKey(r))
	}
	_, cmd := hub.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("search Enter emitted no selection")
	}
	selected, ok := cmd().(ApplyProjectScopeMsg)
	if !ok || selected.ProjectID != "beta" {
		t.Fatalf("search Enter emitted %#v, want beta", cmd())
	}
	app.handleViewActionMessages(selected)
	if app.crossOutliner.scopedProjectID != "beta" || app.CurrentRoute().ID() != TaskWorkspace {
		t.Fatalf(
			"selection landed project=%q route=%s",
			app.crossOutliner.scopedProjectID,
			app.CurrentRoute().ID(),
		)
	}
}

func TestProjectHubSearchNavigationSelectsHighlightedProject(t *testing.T) {
	app := projectHubApp(t)
	hub := openProjectHub(t, app)
	hub.Update(syntheticKey('/'))
	hub.Update(syntheticKey('a'))

	assertCurrent := func(want ListItemID) {
		t.Helper()
		current, ok := hub.surface.CurrentItem()
		if !ok || current.ID != want {
			t.Fatalf("current project = %q (present=%v), want %q", current.ID, ok, want)
		}
		if query := hub.SearchQuery(); query != "a" {
			t.Fatalf("search navigation changed query to %q, want %q", query, "a")
		}
		if !hub.HasActiveSearch() {
			t.Fatal("search navigation deactivated search")
		}
	}

	assertCurrent("project-hub:project:alpha")
	hub.Update(syntheticCtrlKey('j'))
	assertCurrent("project-hub:project:beta")
	hub.Update(syntheticCtrlKey('k'))
	assertCurrent("project-hub:project:alpha")
	hub.Update(syntheticSpecialKey(tea.KeyDown))
	assertCurrent("project-hub:project:beta")
	hub.Update(syntheticSpecialKey(tea.KeyUp))
	assertCurrent("project-hub:project:alpha")
	hub.Update(syntheticSpecialKey(tea.KeyDown))

	_, cmd := hub.Update(syntheticSpecialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("Enter after search navigation emitted no selection")
	}
	selected, ok := cmd().(ApplyProjectScopeMsg)
	if !ok || selected.ProjectID != "beta" {
		t.Fatalf("Enter after search navigation emitted %#v, want beta", cmd())
	}
	spec, ok := hub.ContextualGuidance()
	if help := guidanceEntryText(spec); !ok || !strings.Contains(help, "↑/↓ or Ctrl+J/K navigate") {
		t.Fatalf("search guidance does not advertise navigation: %q", help)
	}
}

func TestProjectHubSearchPlainJAndKRemainQueryInput(t *testing.T) {
	hub := openProjectHub(t, projectHubApp(t))
	hub.Update(syntheticKey('/'))
	hub.Update(syntheticKey('j'))
	hub.Update(syntheticKey('k'))

	if query := hub.SearchQuery(); query != "jk" {
		t.Fatalf("plain j/k query = %q, want %q", query, "jk")
	}
	if !hub.HasActiveSearch() {
		t.Fatal("plain j/k deactivated search")
	}
}

func TestProjectHubArchivedEnterKeepsHubOpenWithNotice(t *testing.T) {
	app := projectHubApp(t)
	archived := scopeTestProject("archived", "Archived", "1")
	archived.Archived = true
	addTestAppProject(app, archived)
	hub := openProjectHub(t, app)
	focusProjectHubItem(t, hub, "project-hub:archived:archived")

	_, cmd := hub.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		t.Fatalf("archived Enter emitted %#v", cmd())
	}
	if !strings.Contains(hub.notice, "unarchived") || app.currentView() != ViewProjectHub {
		t.Fatalf("archived Enter notice=%q view=%s", hub.notice, app.currentView())
	}
}

func TestProjectHubBrowseACreatesProject(t *testing.T) {
	hub := openProjectHub(t, projectHubApp(t))

	_, cmd := hub.Update(syntheticKey('a'))
	if cmd == nil {
		t.Fatal("browse a emitted no project creation request")
	}
	if _, ok := cmd().(CreateProjectMsg); !ok {
		t.Fatalf("browse a emitted %T, want CreateProjectMsg", cmd())
	}
}

// TestProjectHubRetiredKeysAreSilent pins the keys the browse grammar dropped.
// n created a project, A cleared the selection, and a bare g jumped to the top;
// each now belongs to another surface or to nothing, so a press must leave the
// Hub untouched rather than act or complain.
func TestProjectHubRetiredKeysAreSilent(t *testing.T) {
	hub := openProjectHub(t, projectHubApp(t))
	focusProjectHubItem(t, hub, "project-hub:project:beta")

	for _, retired := range []rune{'n', 'A'} {
		_, cmd := hub.Update(syntheticKey(retired))
		if cmd != nil {
			t.Fatalf("retired key %q emitted %#v", string(retired), cmd())
		}
		if hub.notice != "" {
			t.Fatalf("retired key %q raised notice %q", string(retired), hub.notice)
		}
		if hub.selectionCount() != 0 {
			t.Fatalf("retired key %q changed the selection", string(retired))
		}
	}

	hub.Update(syntheticKey('g'))
	current, ok := hub.surface.CurrentItem()
	if !ok || current.ID != "project-hub:project:beta" {
		t.Fatalf("a lone g moved the cursor to %q", current.ID)
	}
	hub.Update(syntheticKey('g'))
	if current, _ = hub.surface.CurrentItem(); current.ID != "project-hub:project:alpha" {
		t.Fatalf("gg landed on %q, want the first project", current.ID)
	}
}

func TestProjectHubReopensInBrowseWithoutTheOldQuery(t *testing.T) {
	app := projectHubApp(t)
	hub := openProjectHub(t, app)
	hub.Update(syntheticKey('/'))
	hub.Update(syntheticKey('a'))
	hub.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if hub.HasActiveSearch() || hub.SearchQuery() != "a" {
		t.Fatalf("Esc left search active=%v query=%q", hub.HasActiveSearch(), hub.SearchQuery())
	}
	handled, _ := app.handleGlobalKeyPress(tea.KeyPressMsg{Code: tea.KeyEscape})
	if !handled {
		t.Fatal("browse Esc did not close Project Hub")
	}

	reopened := openProjectHub(t, app)
	if reopened.HasActiveSearch() || reopened.SearchQuery() != "" || reopened == hub {
		t.Fatalf(
			"reopened hub active=%v query=%q reused=%v",
			reopened.HasActiveSearch(),
			reopened.SearchQuery(),
			reopened == hub,
		)
	}
}

func TestProjectHubStaleProjectActionUsesProductionExecutorError(t *testing.T) {
	app := projectHubApp(t)
	hub := openProjectHub(t, app)
	if hub.registry != app.operationRegistry || hub.executor != app.operationExecutor {
		t.Fatal("Project Hub did not receive the production operation system")
	}

	focusProjectHubItem(t, hub, "project-hub:project:beta")
	hub.Update(syntheticKey(' '))
	if hub.actions == nil {
		t.Fatal("Space did not freeze the project action target")
	}
	if !hub.actions.surface.SetCursorToID(ListItemID("project-hub-action:" + string(operationScopeArchiveProject))) {
		t.Fatal("archive action was not selectable")
	}

	alpha := app.projectByID("alpha")
	setTestAppProjects(app, alpha)
	_, cmd := hub.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		t.Fatalf("stale project action still emitted %#v", cmd())
	}
	if hub.notice != "project beta is no longer available" {
		t.Fatalf("stale project action notice = %q, want production executor error", hub.notice)
	}
}

// TestProjectHubStaleDirectKeyUsesProductionExecutorError proves the direct
// grammar reaches the same executor as the menu: a key pressed on a row whose
// project has since left the workspace refuses with the executor's own sentence.
func TestProjectHubStaleDirectKeyUsesProductionExecutorError(t *testing.T) {
	app := projectHubApp(t)
	hub := openProjectHub(t, app)
	focusProjectHubItem(t, hub, "project-hub:project:beta")

	setTestAppProjects(app, app.projectByID("alpha"))
	_, cmd := hub.Update(syntheticKey('e'))
	if cmd == nil {
		t.Fatal("a stale direct key reported nothing")
	}
	failure, ok := cmd().(ErrorMsg)
	if !ok || failure.Err == nil || failure.Err.Error() != "project beta is no longer available" {
		t.Fatalf("stale direct key emitted %#v, want the production executor error", cmd())
	}
}

func TestProjectHubTaskFilterCyclesThreeStates(t *testing.T) {
	app := projectHubFilterApp(t)
	hub := openProjectHub(t, app)
	if !projectHubHasItem(hub, "project-hub:project:empty") {
		t.Fatal("fixture omitted the empty project before filtering")
	}

	// Routed as the shell routes it: nothing above the Hub may claim the key, so
	// the global pass declines it and delegation lands it on the Hub.
	if handled, _ := app.handleGlobalKeyPress(syntheticCtrlKey('t')); handled {
		t.Fatal("a shell handler claimed Ctrl+T before the Project Hub")
	}
	app.delegateToCurrentView(syntheticCtrlKey('t'))
	if hub.taskFilter != projectHubTaskFilterWithTasks {
		t.Fatalf("first Ctrl+T left filter=%v, want with tasks", hub.taskFilter)
	}
	if projectHubHasItem(hub, "project-hub:project:empty") {
		t.Fatal("the with-tasks filter still lists a project holding no tasks")
	}
	if !projectHubHasItem(hub, "project-hub:project:alpha") {
		t.Fatal("the with-tasks filter dropped a project holding tasks")
	}

	hub.Update(syntheticCtrlKey('t'))
	if hub.taskFilter != projectHubTaskFilterWithoutTasks {
		t.Fatalf("second Ctrl+T left filter=%v, want without tasks", hub.taskFilter)
	}
	if !projectHubHasItem(hub, "project-hub:project:empty") {
		t.Fatal("the without-tasks filter dropped a project holding no tasks")
	}
	if projectHubHasItem(hub, "project-hub:project:alpha") {
		t.Fatal("the without-tasks filter still lists a project holding tasks")
	}

	hub.Update(syntheticCtrlKey('t'))
	if hub.taskFilter != projectHubTaskFilterAll {
		t.Fatalf("third Ctrl+T left filter=%v, want every project", hub.taskFilter)
	}
	for _, id := range []ListItemID{"project-hub:project:empty", "project-hub:project:alpha"} {
		if !projectHubHasItem(hub, id) {
			t.Fatalf("the cleared axis omitted %q", id)
		}
	}
}

func TestProjectHubTaskFilterKeepsParkedAndArchivedProjectsWithTasks(t *testing.T) {
	app := projectHubDiskApp(t)
	parked := projectHubDiskProject(app, "parked", "Parked")
	parked.Tags = []string{"parked"}
	writeProjectHubTasks(t, parked, 3)
	addTestAppProject(app, parked)
	archived := projectHubDiskProject(app, "archived", "Archived")
	archived.Archived = true
	writeProjectHubTasks(t, archived, 2)
	addTestAppProject(app, archived)
	stale := projectHubDiskProject(app, "stale", "Stale")
	stale.Archived = true
	if err := os.MkdirAll(stale.Path, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", stale.Path, err)
	}
	addTestAppProject(app, stale)

	hub := openProjectHub(t, app)
	hub.Update(syntheticCtrlKey('t'))
	if !projectHubHasItem(hub, "project-hub:project:parked") {
		t.Error("filter hid a parked project holding tasks")
	}
	if !projectHubHasItem(hub, "project-hub:archived:archived") {
		t.Error("filter hid an archived project holding tasks")
	}
	if projectHubHasItem(hub, "project-hub:archived:stale") {
		t.Error("filter left an empty archived project listed")
	}

	hub.Update(syntheticCtrlKey('t'))
	if projectHubHasItem(hub, "project-hub:project:parked") {
		t.Error("the without-tasks filter left a parked project holding tasks listed")
	}
	if !projectHubHasItem(hub, "project-hub:archived:stale") {
		t.Error("the without-tasks filter hid an empty archived project")
	}
}

func TestProjectHubTaskFilterKeepsUnknownCounts(t *testing.T) {
	app := projectHubDiskApp(t)
	corrupt := projectHubDiskProject(app, "corrupt", "Corrupt")
	if err := os.MkdirAll(corrupt.Path, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", corrupt.Path, err)
	}
	tasksPath := storage.TasksFilePath(corrupt.Path)
	if err := os.WriteFile(tasksPath, []byte("this is not a task document\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(%q): %v", tasksPath, err)
	}
	addTestAppProject(app, corrupt)
	empty := projectHubDiskProject(app, "empty", "Empty")
	if err := os.MkdirAll(empty.Path, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", empty.Path, err)
	}
	addTestAppProject(app, empty)

	hub := openProjectHub(t, app)
	hub.Update(syntheticCtrlKey('t'))
	if !projectHubHasItem(hub, "project-hub:project:corrupt") {
		t.Error("the with-tasks filter hid a project whose count could not be read")
	}
	if projectHubHasItem(hub, "project-hub:project:empty") {
		t.Error("the with-tasks filter left a known-empty project listed")
	}

	hub.Update(syntheticCtrlKey('t'))
	if !projectHubHasItem(hub, "project-hub:project:corrupt") {
		t.Error("the without-tasks filter hid a project whose count could not be read")
	}
	if !projectHubHasItem(hub, "project-hub:project:empty") {
		t.Error("the without-tasks filter hid a known-empty project")
	}
}

func TestProjectHubTaskFilterTogglesUnderActiveSearch(t *testing.T) {
	hub := openProjectHub(t, projectHubFilterApp(t))
	hub.Update(syntheticKey('/'))
	for _, r := range "bet" {
		hub.Update(syntheticKey(r))
	}
	if hub.SearchQuery() != "bet" || len(hub.surface.Matches()) != 2 {
		t.Fatalf("query=%q matches=%d, want Beta and Beta Empty", hub.SearchQuery(), len(hub.surface.Matches()))
	}

	hub.Update(syntheticCtrlKey('t'))
	if !hub.HasActiveSearch() || hub.SearchQuery() != "bet" {
		t.Fatalf("toggle left search active=%v query=%q", hub.HasActiveSearch(), hub.SearchQuery())
	}
	matches := hub.surface.Matches()
	if len(matches) != 1 || matches[0].Item.ID != "project-hub:project:beta" {
		t.Fatalf("filtered matches = %d, want Beta alone", len(matches))
	}

	for _, r := range "zzz" {
		hub.Update(syntheticKey(r))
	}
	context, ok := hub.surface.EmptyContext()
	if !ok || context.Kind != ListEmptyNoMatches {
		t.Fatalf("no-match context = (%+v, %v)", context, ok)
	}
	if want := "Press Esc to clear the query, or Ctrl+T to show projects without tasks."; context.State.Detail != want {
		t.Fatalf("no-match detail = %q, want %q", context.State.Detail, want)
	}
}

func TestProjectHubTaskFilterIgnoredUnderActionSheet(t *testing.T) {
	hub := openProjectHub(t, projectHubFilterApp(t))
	focusProjectHubItem(t, hub, "project-hub:project:empty")
	hub.Update(syntheticKey(' '))
	if hub.actions == nil {
		t.Fatal("Space did not open the action sheet")
	}

	hub.Update(syntheticCtrlKey('t'))
	if hub.taskFilter != projectHubTaskFilterAll {
		t.Fatal("Ctrl+T reached the Hub while the action sheet owned the key stream")
	}
	if hub.actions == nil {
		t.Fatal("Ctrl+T dismissed the action sheet")
	}
	if !projectHubHasItem(hub, "project-hub:project:empty") {
		t.Fatal("Ctrl+T filtered the list from under the action sheet")
	}
}

func TestProjectHubTaskFilterDropsHiddenProjectsFromSelection(t *testing.T) {
	hub := openProjectHub(t, projectHubFilterApp(t))
	focusProjectHubItem(t, hub, "project-hub:project:beta")
	hub.Update(syntheticKey('v'))
	focusProjectHubItem(t, hub, "project-hub:project:empty")
	hub.Update(syntheticKey('v'))
	if got := hub.surface.SelectedIDs(); len(got) != 2 {
		t.Fatalf("selected IDs = %v, want both projects", got)
	}

	hub.Update(syntheticCtrlKey('t'))
	got := hub.surface.SelectedIDs()
	if len(got) != 1 || got[0] != "project-hub:project:beta" {
		t.Fatalf("selected IDs after filtering = %v, want the surviving project alone", got)
	}

	hub.Update(syntheticKey(' '))
	if hub.actions == nil {
		t.Fatal("Space did not open the bulk action sheet")
	}
	identities := hub.actions.context.Selection.EntityIdentities
	if len(identities) != 1 || identities[0].ProjectID != "beta" {
		t.Fatalf("bulk targets = %+v, want beta alone", identities)
	}
}

func TestProjectHubTaskFilterEmptyCopyNamesTheToggle(t *testing.T) {
	app := projectHubApp(t)
	app.crossOutliner.loadedProjects = []*models.Project{
		scopeTestProject("alpha", "Alpha"),
		scopeTestProject("beta", "Beta"),
	}
	hub := openProjectHub(t, app)

	hub.Update(syntheticCtrlKey('t'))
	if got := hub.surface.Items(); len(got) != 0 {
		t.Fatalf("filtered items = %d, want every project hidden", len(got))
	}
	context, ok := hub.surface.EmptyContext()
	if !ok || context.Kind != ListEmptySource {
		t.Fatalf("empty context = (%+v, %v), want the source-empty kind", context, ok)
	}
	if context.State.Title != "No projects hold tasks." ||
		context.State.Detail != "Press Ctrl+T to show projects without tasks." {
		t.Fatalf("filtered empty copy = %+v, want the filter-specific copy", context.State)
	}

	hub.Update(syntheticCtrlKey('t'))
	if _, ok := hub.surface.EmptyContext(); ok {
		t.Fatal("the without-tasks filter left the Hub empty")
	}
}

func TestProjectHubTaskFilterEmptyCopyNamesTheFullList(t *testing.T) {
	hub := openProjectHub(t, projectHubApp(t))
	hub.Update(syntheticCtrlKey('t'))
	hub.Update(syntheticCtrlKey('t'))
	if got := hub.surface.Items(); len(got) != 0 {
		t.Fatalf("without-tasks items = %d, want every project hidden", len(got))
	}
	context, ok := hub.surface.EmptyContext()
	if !ok || context.Kind != ListEmptySource {
		t.Fatalf("empty context = (%+v, %v), want the source-empty kind", context, ok)
	}
	if context.State.Title != "Every project holds tasks." ||
		context.State.Detail != "Press Ctrl+T to show every project." {
		t.Fatalf("without-tasks empty copy = %+v, want the way back to the full list", context.State)
	}
}

func TestProjectHubReopenClearsTaskFilter(t *testing.T) {
	app := projectHubFilterApp(t)
	hub := openProjectHub(t, app)
	hub.Update(syntheticCtrlKey('t'))
	if projectHubHasItem(hub, "project-hub:project:empty") {
		t.Fatal("filter did not hide the empty project")
	}

	reopened := openProjectHub(t, app)
	if reopened == hub {
		t.Fatal("reopening the Hub reused the filtered model")
	}
	if reopened.taskFilter != projectHubTaskFilterAll || !projectHubHasItem(reopened, "project-hub:project:empty") {
		t.Fatalf("reopened hub filter=%v, want it cleared", reopened.taskFilter)
	}
}

func TestProjectHubSubtitleNamesTheTaskFilter(t *testing.T) {
	hub := openProjectHub(t, projectHubFilterApp(t))

	counts := hub.subtitle()
	if strings.Contains(counts, "with tasks") {
		t.Fatalf("unfiltered subtitle = %q, want no filter marker", counts)
	}

	focusProjectHubItem(t, hub, "project-hub:project:beta")
	hub.Update(syntheticKey('v'))
	hub.Update(syntheticCtrlKey('t'))
	if !projectHubHasItem(hub, "project-hub:project:beta") {
		t.Fatal("the filter hid the selected project")
	}
	// The leading counts are asserted verbatim from the unfiltered read, so a
	// subtitle reporting filtered totals rather than workspace totals fails here.
	if want := counts + " · with tasks · 1 selected"; hub.subtitle() != want {
		t.Fatalf("filtered subtitle = %q, want %q", hub.subtitle(), want)
	}

	// The next press hides the selected project, so the selection it names is
	// dropped with it and only the marker is left to change.
	hub.Update(syntheticCtrlKey('t'))
	if want := counts + " · without tasks"; hub.subtitle() != want {
		t.Fatalf("without-tasks subtitle = %q, want %q", hub.subtitle(), want)
	}

	hub.Update(syntheticCtrlKey('t'))
	if hub.subtitle() != counts {
		t.Fatalf("cleared subtitle = %q, want %q", hub.subtitle(), counts)
	}
}

func TestProjectHubTaskFilterHintSurvivesTheEmptiedList(t *testing.T) {
	app := projectHubApp(t)
	app.crossOutliner.loadedProjects = []*models.Project{
		scopeTestProject("alpha", "Alpha"),
		scopeTestProject("beta", "Beta"),
	}
	hub := openProjectHub(t, app)

	spec, ok := hub.ContextualGuidance()
	if help := guidanceEntryText(spec); !ok || !strings.Contains(help, "Ctrl+T with tasks") {
		t.Fatalf("browse guidance omits the filter hint: %q", help)
	}

	hub.Update(syntheticCtrlKey('t'))
	if got := hub.surface.Items(); len(got) != 0 {
		t.Fatalf("filtered items = %d, want every project hidden", len(got))
	}
	spec, ok = hub.ContextualGuidance()
	help := guidanceEntryText(spec)
	if !ok || !strings.Contains(help, "Ctrl+T without tasks") {
		t.Fatalf("emptied guidance omits the next state: %q", help)
	}
	if strings.Contains(help, "/ search") {
		t.Fatalf("emptied guidance still offers search: %q", help)
	}

	hub.Update(syntheticCtrlKey('t'))
	spec, ok = hub.ContextualGuidance()
	if help := guidanceEntryText(spec); !ok || !strings.Contains(help, "Ctrl+T all projects") {
		t.Fatalf("without-tasks guidance omits the way back: %q", help)
	}
}

func TestProjectHubHelpNamesTheTaskFilterOnce(t *testing.T) {
	hub := openProjectHub(t, projectHubApp(t))
	found := 0
	for _, item := range hub.GetHelpItems() {
		if item.Key != "Ctrl+T" {
			continue
		}
		found++
		if item.Description != "Cycle every project, only those holding tasks, only those holding none" ||
			item.Category != "Discovery" {
			t.Fatalf("Ctrl+T help item = %+v", item)
		}
	}
	if found != 1 {
		t.Fatalf("Ctrl+T help entries = %d, want exactly 1", found)
	}
}

func TestShowProjectHubMsgOpensTheHubInBrowse(t *testing.T) {
	app := projectHubApp(t)
	handled, cmd := app.handleProjectViewMessages(ShowProjectHubMsg{})
	if !handled {
		t.Fatal("ShowProjectHubMsg was not handled")
	}
	if cmd != nil {
		cmd()
	}
	hub, ok := app.manageHub.ActiveModuleView().(*ProjectHubModel)
	if app.CurrentRoute().ID() != SecondaryTool || app.currentView() != ViewProjectHub || !ok {
		t.Fatalf("Project Hub landed on %s/%s (%T)", app.CurrentRoute().ID(), app.currentView(), app.manageHub.ActiveModuleView())
	}
	if hub.HasActiveSearch() {
		t.Fatal("the Project Hub opened into search instead of browse")
	}
	if _, ok := hub.selectedProjectHubEntry(); !ok {
		t.Fatal("the Project Hub opened without a live cursor")
	}
}
