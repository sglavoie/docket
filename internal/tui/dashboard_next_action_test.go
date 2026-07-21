package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
	"github.com/sglavoie/tt/internal/tui/keymap"
	"github.com/sglavoie/tt/internal/util"
)

// dashboardWithProjects builds a sized DashboardModel with the supplied
// projects. It bypasses loadProjects() so tests control the project slice
// directly without touching the filesystem.
func dashboardWithProjects(width, height int, projects []*models.Project) *DashboardModel {
	cfg := newTestConfig()
	setTestProjects(cfg, projects)
	m := &DashboardModel{
		runtime:  testRuntimeAt(cfg.DataDir(), cfg),
		projects: projects,
	}
	m.InitSearchInput("Search projects...", 100, 40)
	m.SetSize(width, height)
	m.rebuildHomeAttention()
	m.Dispatcher = keymap.NewDispatcher(m.buildKeyMap())
	return m
}

// TestDashboardNoLongerRendersFirstRunNextActionStrip verifies Dashboard no
// longer competes with Today for first-run daily execution guidance. The
// project list still gives the project-admin create cue.
func TestDashboardNoLongerRendersFirstRunNextActionStrip(t *testing.T) {
	m := dashboardWithProjects(120, 30, nil)
	plain := ansi.Strip(m.View().Content)

	if strings.Contains(plain, "Create your first project") {
		t.Fatalf("dashboard should not render the Today first-run CTA, got:\n%s", plain)
	}
	if !strings.Contains(plain, "No projects yet. Press a to create one.") {
		t.Fatalf("expected dashboard project-admin empty state, got:\n%s", plain)
	}
}

// TestHomeRendersPlannedTodayRecommendation verifies actionable guidance moved
// into Home's Attention pane without restoring the old Next Action strip.
func TestHomeRendersPlannedTodayRecommendation(t *testing.T) {
	cfg := newTestConfig()
	project := testutil.NewTestProject("Cockpit Plan")
	planned, err := models.NewTask("task-planned", 1, models.TaskTypeTask, "Write design doc draft")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	planned.WorkflowState = models.StateInProgress
	planned.SetSection(models.SectionNext)
	project.Tasks = []*models.Task{planned}
	setTestProjects(cfg, []*models.Project{project})
	todayKey := util.StartOfDay(time.Now()).Format(util.DateFormat)
	cfg.Planning().DailyPlan = []config.DailyPlanItem{{
		TaskID:      planned.ID,
		ProjectID:   project.ID,
		PlannedDate: todayKey,
	}}

	m := &DashboardModel{
		runtime:  testRuntimeAt(cfg.DataDir(), cfg),
		projects: []*models.Project{project},
	}
	m.InitSearchInput("Search projects...", 100, 40)
	m.SetSize(120, 30)
	m.rebuildHomeAttention()

	plain := ansi.Strip(m.View().Content)
	if !strings.Contains(plain, "[Recommendation]") || !strings.Contains(plain, "Work on Write design doc draft") {
		t.Fatalf("Home Attention should render the planned recommendation, got:\n%s", plain)
	}
	if len(m.attentionRows) == 0 || !strings.Contains(m.attentionRows[0].Detail, "planned today") {
		t.Fatalf("Home recommendation detail = %+v, want planned today rationale", m.attentionRows)
	}
	if !strings.Contains(plain, "Projects (1)") || !strings.Contains(plain, "Cockpit Plan") {
		t.Fatalf("expected dashboard project list to remain visible, got:\n%s", plain)
	}
}

// TestHomeRendersInboxRecommendation verifies inbox guidance is actionable in
// Attention instead of being repeated in an inert Dashboard summary.
func TestHomeRendersInboxRecommendation(t *testing.T) {
	project := testutil.NewTestProject("Inboxy")
	inbox := testutil.NewTestTask("triage me", models.TaskTypeTask)
	inbox.SetSection(models.SectionInbox)
	project.Tasks = []*models.Task{inbox}

	m := dashboardWithProjects(120, 30, []*models.Project{project})
	plain := ansi.Strip(m.View().Content)

	if !strings.Contains(plain, "Triage the inbox") || !strings.Contains(plain, "[Pipeline]") {
		t.Fatalf("Home should render actionable inbox guidance, got:\n%s", plain)
	}
	if !strings.Contains(plain, "Projects (1)") || !strings.Contains(plain, "Inboxy") {
		t.Fatalf("expected dashboard project list to remain visible, got:\n%s", plain)
	}
}

// TestDashboardArchivedModeRemainsFocusedOnRecovery verifies archive recovery
// stays stripped of daily-execution CTAs while still rendering archived
// project administration.
func TestDashboardArchivedModeRemainsFocusedOnRecovery(t *testing.T) {
	cfg := newTestConfig()
	archived := testutil.NewTestProject("Archived Project")
	archived.Archive()
	setTestProjects(cfg, []*models.Project{archived})

	m := &DashboardModel{
		runtime:      testRuntimeAt(cfg.DataDir(), cfg),
		projects:     []*models.Project{archived},
		showArchived: true,
	}
	m.InitSearchInput("Search projects...", 100, 40)
	m.SetSize(120, 30)

	plain := ansi.Strip(m.View().Content)
	if strings.Contains(plain, "Triage inbox") || strings.Contains(plain, "Next: ") || strings.Contains(plain, "Create your first project") {
		t.Fatalf("expected no recommendation strip in archived mode, got:\n%s", plain)
	}
	if !strings.Contains(plain, "Archived (1)") || !strings.Contains(plain, "Archived Project") {
		t.Fatalf("expected archived project recovery list, got:\n%s", plain)
	}
}

// TestDashboardNoLongerOpenLoopsAwareInterface pins the repositioning:
// Dashboard should not receive daily-driver open-loop counters for rendering.
func TestDashboardNoLongerOpenLoopsAwareInterface(t *testing.T) {
	var m any = &DashboardModel{}
	if _, ok := m.(openLoopsAware); ok {
		t.Fatal("dashboard should no longer implement openLoopsAware")
	}
}

// TestDashboardStatusBarNoLongerShowsTodayProgress verifies Dashboard does
// not append daily progress in the app shell status bar after the tutor prompt
// is dismissed.
func TestDashboardStatusBarNoLongerShowsTodayProgress(t *testing.T) {
	app := newTestAppModel()
	app.runtime.Settings().SetTutorDismissed(true)
	app.openLoops = OpenLoops{TodayDone: 1, TodayTotal: 3, HasTasks: true}

	plain := ansi.Strip(app.renderStatusBar())
	if strings.Contains(plain, "today:") {
		t.Fatalf("dashboard status bar should not show Today progress, got:\n%s", plain)
	}
	if !strings.Contains(plain, "Home") {
		t.Fatalf("expected Home breadcrumb, got:\n%s", plain)
	}
}
