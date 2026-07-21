package tui

import (
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

func TestHomeResponsiveProjectsAndAttentionPanes(t *testing.T) {
	project := testutil.NewTestProject("Alpha")
	home := dashboardWithProjects(120, 24, []*models.Project{project})
	home.attentionRows = []homeAttentionRow{{
		Category: homeAttentionPipeline,
		Label:    "Plan one Next item",
		Action:   homeAttentionOpenPlannerNext,
	}}

	wide := ansi.Strip(home.View().Content)
	if !strings.Contains(wide, "Projects (1)") || !strings.Contains(wide, "Attention (1)") {
		t.Fatalf("wide Home should render both panes:\n%s", wide)
	}

	home.SetSize(80, 24)
	narrowProjects := ansi.Strip(home.View().Content)
	if !strings.Contains(narrowProjects, "Projects (1)") || strings.Contains(narrowProjects, "Attention (1)") {
		t.Fatalf("narrow Home should render only Projects while it is active:\n%s", narrowProjects)
	}
	_, _, handled := home.Dispatcher.Dispatch(syntheticKey('l'))
	if !handled {
		t.Fatal("l should focus Attention")
	}
	narrowAttention := ansi.Strip(home.View().Content)
	if !strings.Contains(narrowAttention, "Attention (1)") || strings.Contains(narrowAttention, "Projects (1)") {
		t.Fatalf("narrow Home should render only Attention after l:\n%s", narrowAttention)
	}
	_, _, handled = home.Dispatcher.Dispatch(syntheticKey('h'))
	if !handled || home.activeHomePane() != homePaneProjects {
		t.Fatal("h should restore Projects focus")
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
