package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/alerts"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

func homeWithOneAttentionRow(width, height int) *DashboardModel {
	project := testutil.NewTestProject("Alpha")
	home := dashboardWithProjects(width, height, []*models.Project{project})
	home.attentionRows = []homeAttentionRow{{
		Category: homeAttentionPipeline,
		Label:    "Plan one Next item",
		Action:   homeAttentionOpenPlannerNext,
	}}
	return home
}

func countRenderedLines(out string) int {
	trimmed := strings.TrimRight(out, "\n")
	if trimmed == "" {
		return 0
	}
	return strings.Count(trimmed, "\n") + 1
}

// TestHomeAlwaysStacksProjectsAboveAttention confirms the restored layout: both
// cards are rendered at every supported width, Projects above Attention, with no
// vertical overflow past the terminal height.
func TestHomeAlwaysStacksProjectsAboveAttention(t *testing.T) {
	for _, width := range []int{60, 80, 120} {
		home := homeWithOneAttentionRow(width, 24)
		out := ansi.Strip(home.View().Content)

		pIdx := strings.Index(out, "Projects (1)")
		aIdx := strings.Index(out, "Attention (1)")
		if pIdx < 0 || aIdx < 0 {
			t.Fatalf("width %d: Home must render both panes:\n%s", width, out)
		}
		if pIdx >= aIdx {
			t.Fatalf("width %d: Projects must render above Attention:\n%s", width, out)
		}
		if lines := countRenderedLines(out); lines > 24 {
			t.Fatalf("width %d: Home overflowed height 24 with %d lines:\n%s", width, lines, out)
		}
	}
}

// TestHomeFocusSwitchesWithHLWhileBothVisible verifies h/l still switch the
// active pane even though both cards are always on screen.
func TestHomeFocusSwitchesWithHLWhileBothVisible(t *testing.T) {
	home := homeWithOneAttentionRow(80, 24)
	if home.activeHomePane() != homePaneProjects {
		t.Fatalf("Home should start focused on Projects, got %v", home.activeHomePane())
	}

	if _, _, handled := home.Dispatcher.Dispatch(syntheticKey('l')); !handled || home.activeHomePane() != homePaneAttention {
		t.Fatalf("l should focus Attention: handled=%v pane=%v", handled, home.activeHomePane())
	}
	if _, _, handled := home.Dispatcher.Dispatch(syntheticKey('h')); !handled || home.activeHomePane() != homePaneProjects {
		t.Fatalf("h should restore Projects: handled=%v pane=%v", handled, home.activeHomePane())
	}

	out := ansi.Strip(home.View().Content)
	if !strings.Contains(out, "Projects (1)") || !strings.Contains(out, "Attention (1)") {
		t.Fatalf("both panes must stay visible after focus switch:\n%s", out)
	}
}

// TestHomePaneHeightsAllocation exercises the deterministic split: default
// 60/40, a custom 70/30, exact-remainder summing, and a minimum usable row per
// pane at constrained heights.
func TestHomePaneHeightsAllocation(t *testing.T) {
	home := dashboardWithProjects(120, 24, nil)

	// Default 60/40. Body = 24 - 0 chrome - 1 status - 1 separator = 22 rows.
	projects, attention := home.homePaneHeights(0)
	if projects+attention != 22 {
		t.Fatalf("default split must sum to body 22, got projects=%d attention=%d", projects, attention)
	}
	if want := (22*config.DefaultHomeProjectsHeightPct + 50) / 100; abs(projects-want) > 1 {
		t.Fatalf("default projects height %d not within one row of %d", projects, want)
	}

	// Custom 70/30.
	pct := 70
	home.runtime.Machine().HomeProjectsHeightPct = &pct
	projects, attention = home.homePaneHeights(0)
	if projects+attention != 22 {
		t.Fatalf("custom split must sum to body 22, got projects=%d attention=%d", projects, attention)
	}
	if want := (22*70 + 50) / 100; abs(projects-want) > 1 {
		t.Fatalf("custom projects height %d not within one row of %d", projects, want)
	}

	// Constrained height: both panes keep at least a usable row.
	home.SetSize(120, 8)
	projects, attention = home.homePaneHeights(0)
	if projects < homeMinCardHeight || attention < homeMinCardHeight {
		t.Fatalf("constrained split must keep a usable row per pane, got projects=%d attention=%d min=%d", projects, attention, homeMinCardHeight)
	}
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// homeWithProjectAndAttention builds a Home with one named project and one
// Attention row at the given size and Projects split percentage. Both panes are
// nonempty so a render can assert each keeps a selectable item.
func homeWithProjectAndAttention(width, height, pct int) *DashboardModel {
	project := testutil.NewTestProject("Alpha")
	home := dashboardWithProjects(width, height, []*models.Project{project})
	home.runtime.Machine().HomeProjectsHeightPct = &pct
	home.attentionRows = []homeAttentionRow{{
		Category: homeAttentionPipeline,
		Label:    "Plan one Next item",
		Action:   homeAttentionOpenPlannerNext,
	}}
	return home
}

func assertHomeNoOverflow(t *testing.T, label string, out string, width, height int) {
	t.Helper()
	if lines := countRenderedLines(out); lines > height {
		t.Fatalf("%s: Home overflowed height %d with %d lines:\n%s", label, height, lines, out)
	}
	for line := range strings.SplitSeq(strings.TrimRight(out, "\n"), "\n") {
		if w := ansi.StringWidth(line); w > width {
			t.Fatalf("%s: line exceeds width %d (got %d): %q", label, width, w, line)
		}
	}
}

// TestHomeMinimumSplitShowsSelectableItems is the FR-008 regression: at the
// supported minimum terminal height, the pane clamped to homeMinCardHeight must
// still render at least one project entry (Projects at 10%) and at least one
// Attention entry (Attention at 90%), never truncating every row. Covers the
// three supported widths for both the minimum (10%) and maximum (90%) splits.
func TestHomeMinimumSplitShowsSelectableItems(t *testing.T) {
	for _, width := range []int{60, 80, 120} {
		for _, pct := range []int{config.MinHomeProjectsHeightPct, config.MaxHomeProjectsHeightPct} {
			home := homeWithProjectAndAttention(width, minTerminalHeight, pct)
			out := ansi.Strip(home.View().Content)

			if !strings.Contains(out, "Alpha") {
				t.Fatalf("width %d pct %d: Projects pane dropped every project row:\n%s", width, pct, out)
			}
			if !strings.Contains(out, "Plan one Next item") {
				t.Fatalf("width %d pct %d: Attention pane dropped every row:\n%s", width, pct, out)
			}
			assertHomeNoOverflow(t, fmt.Sprintf("width %d pct %d", width, pct), out, width, minTerminalHeight)
		}
	}
}

// TestHomeSplitRendersNoOverflowWithFilters exercises minimum, default, and
// maximum splits with an active search filter and confirms the matched project
// still renders without vertical or horizontal overflow.
func TestHomeSplitRendersNoOverflowWithFilters(t *testing.T) {
	splits := []int{config.MinHomeProjectsHeightPct, config.DefaultHomeProjectsHeightPct, config.MaxHomeProjectsHeightPct}
	for _, width := range []int{60, 80, 120} {
		for _, pct := range splits {
			home := homeWithProjectAndAttention(width, minTerminalHeight, pct)
			home.SetSearchQuery("Alpha")
			out := ansi.Strip(home.View().Content)

			if !strings.Contains(out, "Alpha") {
				t.Fatalf("width %d pct %d (filtered): matched project row missing:\n%s", width, pct, out)
			}
			assertHomeNoOverflow(t, fmt.Sprintf("filtered width %d pct %d", width, pct), out, width, minTerminalHeight)
		}
	}
}

func TestHomeEmptyAndArchivedProjectStates(t *testing.T) {
	empty := NewHomeModel(config.EmptyRuntimeState(""))
	empty.SetSize(120, 24)
	plain := ansi.Strip(empty.View().Content)
	if !strings.Contains(plain, "No projects yet. Press a to create one.") || !strings.Contains(plain, "Nothing needs attention.") {
		t.Fatalf("empty Home state is incomplete:\n%s", plain)
	}

	cfg := newTestConfig()
	archived := testutil.NewTestProject("Archived Alpha")
	archived.Archive()
	setTestProjects(cfg, []*models.Project{archived})
	home := NewHomeModel(testRuntimeAt(cfg.DataDir(), cfg))
	home.SetSize(120, 24)
	_, _, handled := home.Dispatcher.Dispatch(syntheticKey('v'))
	if !handled || !home.showArchived {
		t.Fatal("v should show archived projects")
	}
	plain = ansi.Strip(home.View().Content)
	if !strings.Contains(plain, "Archived (1)") || !strings.Contains(plain, archived.Name) {
		t.Fatalf("archived Home state missing project:\n%s", plain)
	}
}

func TestHomeAttentionCategoriesAreActionable(t *testing.T) {
	cfg := newTestConfig()
	project := testutil.NewTestProject("Delivery")

	overdue := testutil.NewTestTask("Ship overdue fix", models.TaskTypeTask)
	yesterday := time.Now().Add(-24 * time.Hour)
	overdue.DueDate = &yesterday

	parked := testutil.NewTestTask("Resume migration", models.TaskTypeTask)
	parkedAt := time.Now().Add(-time.Hour)
	parked.ParkedAt = &parkedAt
	parked.ParkedNote = "continue at phase two"

	inbox := testutil.NewTestTask("Clarify request", models.TaskTypeTask)
	inbox.SetSection(models.SectionInbox)

	next := testutil.NewTestTask("Prepare release", models.TaskTypeTask)
	next.WorkflowState = models.StateNext
	next.SetSection(models.SectionNext)

	project.Tasks = []*models.Task{overdue, parked, inbox, next}
	setTestProjects(cfg, []*models.Project{project})
	home := dashboardWithProjects(120, 30, []*models.Project{project})
	home.SetAttentionAlerts(&alerts.AlertSnapshot{
		TotalCount: 1,
		Groups: []alerts.AlertGroup{{
			Category: alerts.CategoryOverdue,
			Label:    "Overdue (1)",
			Alerts: []alerts.Alert{{
				Category: alerts.CategoryOverdue,
				Title:    "Overdue task", Details: overdue.Title,
				TaskID: overdue.ID, ProjectID: project.ID, ProjectName: project.Name,
			}},
		}},
	})

	wantCategories := map[homeAttentionCategory]bool{
		homeAttentionRecommendation: false,
		homeAttentionAlert:          false,
		homeAttentionParked:         false,
		homeAttentionPipeline:       false,
	}
	for i, row := range home.attentionRows {
		wantCategories[row.Category] = true
		home.homePane = homePaneAttention
		home.attentionCursor = i
		cmd := home.activateAttentionRow()
		if cmd == nil || cmd() == nil {
			t.Errorf("Attention row %d (%s %q) has no concrete activation", i, row.Category, row.Label)
		}
		if row.Task != nil {
			task, taskProject := home.getTaskRef()
			if task != row.Task || taskProject != row.Project {
				t.Errorf("Attention row %d lost its Peek entity", i)
			}
		}
	}
	for category, found := range wantCategories {
		if !found {
			t.Errorf("Home Attention missing %s category: %+v", category, home.attentionRows)
		}
	}

	plain := ansi.Strip(home.View().Content)
	for _, marker := range []string{"[Recommendation]", "[Alert]", "[Parked]", "[Pipeline]"} {
		if !strings.Contains(plain, marker) {
			t.Errorf("Home render missing %s:\n%s", marker, plain)
		}
	}
}

func TestHomeAttentionTaskPeekAndSearch(t *testing.T) {
	project := testutil.NewTestProject("Alpha")
	task := testutil.NewTestTask("Parked launch task", models.TaskTypeTask)
	project.Tasks = []*models.Task{task}
	home := dashboardWithProjects(80, 20, []*models.Project{project})
	home.homePane = homePaneAttention
	home.attentionRows = []homeAttentionRow{
		{Category: homeAttentionParked, Label: "Resume Parked launch task", Task: task, Project: project, Action: homeAttentionOpenTask},
		{Category: homeAttentionPipeline, Label: "Triage inbox", Action: homeAttentionOpenGlobalInbox},
	}

	home.SetSearchQuery("parked")
	rows := home.visibleAttentionRows()
	if len(rows) != 1 || rows[0].Task != task {
		t.Fatalf("Attention search rows = %+v, want parked task only", rows)
	}

	home.SetSearchQuery("")
	app := newTestAppModel()
	app.dashboard = home
	app.views.Register(ViewDashboard, home)
	app.viewStack = []PresentationID{ViewDashboard}
	handled, cmd := app.handleGlobalKeyPress(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModShift})
	if !handled || cmd != nil || !app.peekActive || app.peekTask != task || app.peekProject != project {
		t.Fatalf("Shift+Enter Peek state: handled=%v cmd=%v active=%v task=%p project=%p", handled, cmd, app.peekActive, app.peekTask, app.peekProject)
	}
}

func TestDashboardAndPipelineAliasesResolveHome(t *testing.T) {
	route, ok, err := routeForPresentation(ViewDashboard, "", "")
	if err != nil || !ok || route.ID() != Home {
		t.Fatalf("Dashboard alias route = %v ok=%v err=%v, want Home", route.ID(), ok, err)
	}
	app := newTestAppModel()
	app.dashboard = NewHomeModel(app.runtime)
	app.views.Register(ViewDashboard, app.dashboard)
	app.viewStack = []PresentationID{ViewDashboard}
	handled, cmd := app.handleViewMessages(NavigatePaletteAliasMsg{Target: PaletteAliasHomeAttention})
	if !handled || cmd != nil {
		t.Fatalf("Home Attention alias handled=%v cmd=%v", handled, cmd)
	}
	if app.currentView() != ViewDashboard || len(app.viewStack) != 1 || app.dashboard.activeHomePane() != homePaneAttention {
		t.Fatalf("alias did not return to Home Attention: view=%s stack=%v pane=%v", app.currentView(), app.viewStack, app.dashboard.activeHomePane())
	}
}
