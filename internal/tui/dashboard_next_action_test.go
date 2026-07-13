package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
	"github.com/sglavoie/tt/internal/util"
)

// dashboardWithProjects builds a sized DashboardModel with the supplied
// projects. It bypasses loadProjects() so tests control the project slice
// directly without touching the filesystem.
func dashboardWithProjects(width, height int, projects []*models.Project) *DashboardModel {
	cfg := testutil.NewTestConfig()
	cfg.Projects = projects
	m := &DashboardModel{
		config:                cfg,
		projects:              projects,
		dashboardCompactCards: true,
	}
	m.InitSearchInput("Search projects...", 100, 40)
	m.SetSize(width, height)
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
	if !strings.Contains(plain, "No projects yet. Press n to create one.") {
		t.Fatalf("expected dashboard project-admin empty state, got:\n%s", plain)
	}
}

// TestDashboardNoLongerRendersPlannedTodayRecommendation verifies Dashboard
// does not surface planned-today task recommendations above the project list.
// Today and Pipeline Hub own the Next Action strip; Dashboard remains a
// project administration and overview surface.
func TestDashboardNoLongerRendersPlannedTodayRecommendation(t *testing.T) {
	cfg := testutil.NewTestConfig()
	project := testutil.NewTestProject("Cockpit Plan")
	planned, err := models.NewTask("task-planned", 1, models.TaskTypeTask, "Write design doc draft")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	planned.WorkflowState = models.StateInProgress
	planned.SetSection(models.SectionNext)
	project.Tasks = []*models.Task{planned}
	cfg.Projects = []*models.Project{project}
	todayKey := util.StartOfDay(time.Now()).Format(util.DateFormat)
	cfg.DailyPlan = []config.DailyPlanItem{{
		TaskID:      planned.ID,
		ProjectID:   project.ID,
		PlannedDate: todayKey,
	}}

	m := &DashboardModel{
		config:                cfg,
		projects:              cfg.Projects,
		dashboardCompactCards: true,
	}
	m.InitSearchInput("Search projects...", 100, 40)
	m.SetSize(120, 30)

	plain := ansi.Strip(m.View().Content)
	if strings.Contains(plain, "Next: Write design doc draft") {
		t.Fatalf("dashboard should not render a planned-today Next Action strip, got:\n%s", plain)
	}
	if strings.Contains(plain, "planned today") {
		t.Fatalf("dashboard should not render daily-plan reason badges, got:\n%s", plain)
	}
	if !strings.Contains(plain, "Projects (1)") || !strings.Contains(plain, "Cockpit Plan") {
		t.Fatalf("expected dashboard project list to remain visible, got:\n%s", plain)
	}
}

// TestDashboardNoLongerRendersInboxOnlyRecommendation verifies app-level
// inbox counts cannot create a Dashboard "Triage inbox" recommendation strip.
// Per-project triage still appears through alerts, row indicators, the
// pipeline card, and the explicit O key.
func TestDashboardNoLongerRendersInboxOnlyRecommendation(t *testing.T) {
	project := testutil.NewTestProject("Inboxy")
	inbox := testutil.NewTestTask("triage me", models.TaskTypeTask)
	inbox.SetSection(models.SectionInbox)
	project.Tasks = []*models.Task{inbox}

	m := dashboardWithProjects(120, 30, []*models.Project{project})
	plain := ansi.Strip(m.View().Content)

	if strings.Contains(plain, "Triage inbox") {
		t.Fatalf("dashboard should not render an inbox-only Next Action strip, got:\n%s", plain)
	}
	if !strings.Contains(plain, "Projects (1)") || !strings.Contains(plain, "Inboxy") {
		t.Fatalf("expected dashboard project list to remain visible, got:\n%s", plain)
	}
}

// TestDashboardArchivedModeRemainsFocusedOnRecovery verifies archive recovery
// stays stripped of daily-execution CTAs while still rendering archived
// project administration.
func TestDashboardArchivedModeRemainsFocusedOnRecovery(t *testing.T) {
	cfg := testutil.NewTestConfig()
	archived := testutil.NewTestProject("Archived Project")
	archived.Archive()
	cfg.Projects = []*models.Project{archived}

	m := &DashboardModel{
		config:                cfg,
		projects:              []*models.Project{archived},
		dashboardCompactCards: true,
		showArchived:          true,
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
	app.config.SetTutorDismissed(true)
	app.openLoops = OpenLoops{TodayDone: 1, TodayTotal: 3, HasTasks: true}

	plain := ansi.Strip(app.renderStatusBar())
	if strings.Contains(plain, "today:") {
		t.Fatalf("dashboard status bar should not show Today progress, got:\n%s", plain)
	}
	if !strings.Contains(plain, "Dashboard") {
		t.Fatalf("expected dashboard breadcrumb to remain, got:\n%s", plain)
	}
}
