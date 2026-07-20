package tui

import (
	"regexp"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
)

// syncTestRuntime reconciles the shell project cache after a test mutates the
// in-memory runtime domains.
func syncTestRuntime(app *AppModel) {
	app.syncProjectCache()
}

func TestRenderProjectCyclePopupKeepsCountsOnSameLine(t *testing.T) {
	app := newTestAppModel()
	app.width = 80
	app.height = 24
	app.showProjectCycle = true
	app.runtime.Machine().FilterCustomContext = "Work"
	app.runtime.Machine().SetCycleProjectsForContext("+Work", []string{"p1", "p2", "p3", "p4"})
	syncTestRuntime(app)

	app.projectCycleItems = []ProjectInfo{
		{Project: &models.Project{ID: "p1", Name: "Excel Add-in"}, TaskCount: 2},
		{Project: &models.Project{ID: "p2", Name: "SIS Shield"}, TaskCount: 21},
		{Project: &models.Project{ID: "p3", Name: "muse"}, TaskCount: 2},
		{Project: &models.Project{ID: "p4", Name: "Next Unicorn"}, TaskCount: 0},
		{Project: &models.Project{ID: "p5", Name: "LogScanner"}, TaskCount: 3},
		{Project: &models.Project{ID: "p6", Name: "ocm"}, TaskCount: 0},
		{Project: &models.Project{ID: "p7", Name: "spec"}, TaskCount: 6},
		{Project: &models.Project{ID: "p8", Name: "tt"}, TaskCount: 1},
	}

	rendered := ansi.Strip(app.renderProjectCyclePopup())
	for _, want := range []string{
		`1 \[x\] Excel Add-in\s+2`,
		`2 \[x\] SIS Shield\s+21`,
		`3 \[x\] muse\s+2`,
		`4 \[x\] Next Unicorn\s+0`,
		`5 \[ \] LogScanner\s+3`,
		`6 \[ \] ocm\s+0`,
		`7 \[ \] spec\s+6`,
		`8 \[ \] tt\s+1`,
	} {
		if !regexp.MustCompile(want).MatchString(rendered) {
			t.Fatalf("rendered popup is missing expected row %q\n\n%s", want, rendered)
		}
	}

	for _, bad := range []string{"│ 2 │", "│ 21 │", "│ 0 │", "│ 3 │", "│ 6 │", "│ 1 │"} {
		if strings.Contains(rendered, bad) {
			t.Fatalf("rendered popup still wraps counts onto standalone lines: %q\n\n%s", bad, rendered)
		}
	}
}

func TestHandleCycleProjectReportsArchivedOnlyCycle(t *testing.T) {
	app := newTestAppModel()
	addTestAppProject(app, &models.Project{ID: "p1", Slug: "archived-project", Name: "Archived Project", Archived: true})
	app.runtime.Machine().SetCycleProjects([]string{"p1"})
	syncTestRuntime(app)

	handled, _ := app.handleCycleProject(true)

	if !handled {
		t.Fatal("expected cycle project message to be handled")
	}
	for _, want := range []string{
		"No active projects in cycle",
		"1 archived project hidden",
		"Unarchive from Dashboard archived mode",
	} {
		if !strings.Contains(app.statusMessage, want) {
			t.Fatalf("statusMessage = %q, want substring %q", app.statusMessage, want)
		}
	}
}

func TestHandleCycleProjectReportsArchivedOnlyCustomContextCycle(t *testing.T) {
	app := newTestAppModel()
	app.runtime.Machine().FilterCustomContext = "Work"
	addTestAppProject(app, &models.Project{ID: "p1", Slug: "archived-one", Name: "Archived One", Archived: true})
	addTestAppProject(app, &models.Project{ID: "p2", Slug: "archived-two", Name: "Archived Two", Archived: true})
	app.runtime.Machine().SetCycleProjectsForContext("+Work", []string{"p1", "p2"})
	syncTestRuntime(app)

	handled, _ := app.handleCycleProject(true)

	if !handled {
		t.Fatal("expected cycle project message to be handled")
	}
	for _, want := range []string{
		"No active projects in cycle for +Work",
		"2 archived projects hidden",
		"Unarchive from Dashboard archived mode",
	} {
		if !strings.Contains(app.statusMessage, want) {
			t.Fatalf("statusMessage = %q, want substring %q", app.statusMessage, want)
		}
	}
}

func TestRenderProjectCyclePopupShowsArchivedHiddenCount(t *testing.T) {
	app := newTestAppModel()
	app.width = 100
	app.height = 24
	app.showProjectCycle = true
	active := &models.Project{ID: "p1", Name: "Active Project"}
	archived := &models.Project{ID: "p2", Name: "Archived Project", Archived: true}
	setTestAppProjects(app, active, archived)
	app.runtime.Machine().SetCycleProjects([]string{"p1", "p2"})
	syncTestRuntime(app)
	app.projectCycleAllItems = []ProjectInfo{{Project: active, TaskCount: 3}}
	app.projectCycleItems = app.projectCycleAllItems

	rendered := ansi.Strip(app.renderProjectCyclePopup())
	normalized := strings.Join(strings.Fields(rendered), " ")
	for _, want := range []string{
		"2 projects selected",
		"1 archived",
		"project hidden",
	} {
		if !strings.Contains(normalized, want) {
			t.Fatalf("rendered popup missing %q\n\n%s", want, rendered)
		}
	}
}

func TestRenderProjectCyclePopupExplainsAllArchivedCycle(t *testing.T) {
	app := newTestAppModel()
	app.width = 100
	app.height = 24
	app.showProjectCycle = true
	addTestAppProject(app, &models.Project{ID: "p1", Slug: "archived-project", Name: "Archived Project", Archived: true})
	app.runtime.Machine().SetCycleProjects([]string{"p1"})
	syncTestRuntime(app)

	rendered := ansi.Strip(app.renderProjectCyclePopup())
	normalized := strings.Join(strings.Fields(rendered), " ")
	for _, want := range []string{
		"No active projects in this cycle",
		"1 archived",
		"project hidden",
		"Unarchive from Dashboard archived mode",
	} {
		if !strings.Contains(normalized, want) {
			t.Fatalf("rendered popup missing %q\n\n%s", want, rendered)
		}
	}
}

func TestRenderProjectCyclePopupMatchesSplitOnWideTerminal(t *testing.T) {
	app := newTestAppModel()
	app.width = 160
	app.height = 24
	app.showProjectCycle = true

	longName := "Very Long Project Name That Should Not Be Cut To Footer Width"
	app.projectCycleAllItems = []ProjectInfo{
		{Project: &models.Project{ID: "p1", Name: longName}, TaskCount: 12},
	}
	app.projectCycleItems = app.projectCycleAllItems

	rendered := ansi.Strip(app.renderProjectCyclePopup())
	lines := strings.Split(rendered, "\n")
	if len(lines) == 0 {
		t.Fatal("expected rendered popup lines")
	}
	if width, want := lipgloss.Width(lines[0]), projectCycleSplitWidth(app.width); width != want {
		t.Fatalf("popup width = %d, want split width %d\n\n%s", width, want, rendered)
	}
	if !strings.Contains(rendered, longName) {
		t.Fatalf("expected long project name to fit within wide split\n\n%s", rendered)
	}
}

func TestRenderProjectCyclePopupMatchesSplitAtNormalTerminal(t *testing.T) {
	app := newTestAppModel()
	app.width = 104
	app.height = 24
	app.showProjectCycle = true

	app.projectCycleAllItems = []ProjectInfo{
		{Project: &models.Project{ID: "p1", Name: "Data pipelines"}, TaskCount: 7},
		{Project: &models.Project{ID: "p2", Name: "Tradition Deployment Pipeline"}, TaskCount: 0},
	}
	app.projectCycleItems = app.projectCycleAllItems

	rendered := ansi.Strip(app.renderProjectCyclePopup())
	lines := strings.Split(rendered, "\n")
	if len(lines) == 0 {
		t.Fatal("expected rendered popup lines")
	}
	if width, want := lipgloss.Width(lines[0]), projectCycleSplitWidth(app.width); width != want {
		t.Fatalf("popup width = %d, want split width %d\n\n%s", width, want, rendered)
	}
	if !strings.Contains(rendered, "Tradition Deployment Pipeline") {
		t.Fatalf("expected project name to fit at normal split width\n\n%s", rendered)
	}
}

func TestProjectCycleMaxVisibleItemsLeavesBackgroundRows(t *testing.T) {
	app := newTestAppModel()
	app.height = 24

	got := app.cycleMaxVisibleItems()
	want := 9 // min(24-8 chrome, 24*50%) - 3 background rows.
	if got != want {
		t.Fatalf("cycleMaxVisibleItems() = %d, want %d", got, want)
	}
}

func TestProjectCycleJKNavigationStillMovesListCursor(t *testing.T) {
	app := newProjectCycleSearchTestApp()

	sendProjectCycleKeys(app, syntheticKey('j'), syntheticKey('j'), syntheticKey('k'))

	if got, want := app.projectCycleCursor, 1; got != want {
		t.Fatalf("projectCycleCursor = %d, want %d", got, want)
	}

	sendProjectCycleKeys(app, syntheticKey('k'), syntheticKey('k'))
	if got := app.projectCycleCursor; got != 0 {
		t.Fatalf("projectCycleCursor after moving above first item = %d, want 0", got)
	}
}

func newProjectCycleSearchTestApp() *AppModel {
	app := newTestAppModel()
	app.showProjectCycle = true
	projects := []*models.Project{
		{ID: "p1", Name: "Alpha Tools", Context: "work"},
		{ID: "p2", Name: "Beta Client", Context: "client"},
		{ID: "p3", Name: "Gamma Notes", Context: "personal"},
	}
	setTestAppProjects(app, projects...)
	syncTestRuntime(app)
	app.projectCycleAllItems = []ProjectInfo{
		{Project: projects[0], TaskCount: 2},
		{Project: projects[1], TaskCount: 4},
		{Project: projects[2], TaskCount: 1},
	}
	app.projectCycleItems = app.projectCycleAllItems
	return app
}

func sendProjectCycleKeys(app *AppModel, keys ...tea.KeyPressMsg) {
	for _, key := range keys {
		app.handleProjectCycleKey(key)
	}
}

func typeProjectCycleSearch(app *AppModel, query string) {
	for _, r := range query {
		app.handleProjectCycleKey(syntheticKey(r))
	}
}

func projectCycleBlankBase(width, height int) string {
	lines := make([]string, height)
	for i := range lines {
		lines[i] = strings.Repeat(" ", width)
	}
	return strings.Join(lines, "\n")
}

func lineIndexContaining(lines []string, needle string) int {
	for i, line := range lines {
		if strings.Contains(line, needle) {
			return i
		}
	}
	return -1
}

func TestProjectCycleSearchSlashFiltersProjects(t *testing.T) {
	app := newProjectCycleSearchTestApp()

	sendProjectCycleKeys(app, syntheticKey('/'))
	if !app.projectCycleSearch.IsSearchMode() {
		t.Fatal("expected / to enter Project Cycle search mode")
	}

	typeProjectCycleSearch(app, "beta")
	if got, want := len(app.projectCycleItems), 1; got != want {
		t.Fatalf("filtered item count = %d, want %d", got, want)
	}
	if got := app.projectCycleItems[0].Project.Name; got != "Beta Client" {
		t.Fatalf("filtered project = %q, want Beta Client", got)
	}
}

func TestProjectCycleSearchEnterValidatesAndKeepsFilter(t *testing.T) {
	app := newProjectCycleSearchTestApp()

	sendProjectCycleKeys(app, syntheticKey('/'))
	typeProjectCycleSearch(app, "beta")
	sendProjectCycleKeys(app, syntheticSpecialKey(tea.KeyEnter))

	if app.projectCycleSearch.IsSearchMode() {
		t.Fatal("expected Enter to exit Project Cycle search mode")
	}
	if got := app.projectCycleSearchQuery(); got != "beta" {
		t.Fatalf("search query = %q, want beta", got)
	}
	if got, want := len(app.projectCycleItems), 1; got != want {
		t.Fatalf("filtered item count = %d, want %d", got, want)
	}
}

func TestProjectCycleSearchEscClearsAndRestoresProjects(t *testing.T) {
	app := newProjectCycleSearchTestApp()

	sendProjectCycleKeys(app, syntheticKey('/'))
	typeProjectCycleSearch(app, "beta")
	sendProjectCycleKeys(app, syntheticSpecialKey(tea.KeyEsc))

	if app.projectCycleSearch.IsSearchMode() {
		t.Fatal("expected Esc to exit Project Cycle search mode")
	}
	if got := app.projectCycleSearchQuery(); got != "" {
		t.Fatalf("search query = %q, want empty", got)
	}
	if got, want := len(app.projectCycleItems), 3; got != want {
		t.Fatalf("visible item count = %d, want %d", got, want)
	}
	if !app.showProjectCycle {
		t.Fatal("expected Esc from search mode to keep Project Cycle popup open")
	}
}

func TestProjectCycleEscClearsConfirmedSearchBeforeClosing(t *testing.T) {
	app := newProjectCycleSearchTestApp()

	sendProjectCycleKeys(app, syntheticKey('/'))
	typeProjectCycleSearch(app, "beta")
	sendProjectCycleKeys(app, syntheticSpecialKey(tea.KeyEnter))
	sendProjectCycleKeys(app, syntheticSpecialKey(tea.KeyEsc))

	if !app.showProjectCycle {
		t.Fatal("expected first Esc with confirmed query to keep Project Cycle popup open")
	}
	if got := app.projectCycleSearchQuery(); got != "" {
		t.Fatalf("search query = %q, want empty after first Esc", got)
	}
	if got, want := len(app.projectCycleItems), 3; got != want {
		t.Fatalf("visible item count = %d, want %d after first Esc", got, want)
	}

	sendProjectCycleKeys(app, syntheticSpecialKey(tea.KeyEsc))
	if app.showProjectCycle {
		t.Fatal("expected second Esc to close Project Cycle popup")
	}
}

func TestRenderProjectCyclePopupShowsNoMatches(t *testing.T) {
	app := newProjectCycleSearchTestApp()
	app.ensureProjectCycleSearchInput()
	app.projectCycleSearch.SetSearchQuery("zzzz")
	app.applyProjectCycleFilter(true)

	rendered := ansi.Strip(app.renderProjectCyclePopup())
	if !strings.Contains(rendered, "No matching projects") {
		t.Fatalf("expected no-match message in popup:\n%s", rendered)
	}
	if !strings.Contains(rendered, `Search: "zzzz"`) {
		t.Fatalf("expected confirmed search query in popup:\n%s", rendered)
	}
}

func TestProjectCycleActionsUseFilteredItems(t *testing.T) {
	app := newProjectCycleSearchTestApp()

	sendProjectCycleKeys(app, syntheticKey('/'))
	typeProjectCycleSearch(app, "beta")
	sendProjectCycleKeys(app, syntheticSpecialKey(tea.KeyEnter))

	sendProjectCycleKeys(app, syntheticKey(' '))
	cycleIDs := app.runtime.Machine().GetCycleProjectIDsForContext("")
	if len(cycleIDs) != 1 || cycleIDs[0] != "p2" {
		t.Fatalf("cycle IDs after filtered space toggle = %v, want [p2]", cycleIDs)
	}

	sendProjectCycleKeys(app, syntheticKey('p'))
	beta := app.projectByID("p2")
	if beta == nil || !beta.Pinned {
		t.Fatal("expected filtered p key to pin Beta Client")
	}

	sendProjectCycleKeys(app, syntheticKey('1'))
	if app.currentProject == nil || app.currentProject.ID != "p2" {
		t.Fatalf("current project after filtered 1 jump = %v, want p2", app.currentProject)
	}
}
