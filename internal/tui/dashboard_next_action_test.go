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
// projects and openLoops counters wired in. It bypasses loadProjects() so
// tests control the project slice directly without touching the filesystem.
func dashboardWithProjects(width, height int, projects []*models.Project, openLoops OpenLoops) *DashboardModel {
	cfg := testutil.NewTestConfig()
	cfg.Projects = projects
	m := &DashboardModel{
		config:                cfg,
		projects:              projects,
		dashboardCompactCards: true,
	}
	m.InitSearchInput("Search projects...", 100, 40)
	m.setOpenLoops(openLoops)
	m.SetSize(width, height)
	return m
}

// TestDashboardRecommendationStripFirstRun pins the strip's first-run copy on
// an empty cockpit. No projects + no tasks should surface "Create your first
// project" above the project list so a brand-new user sees the same
// onboarding cue from both Today and Dashboard.
func TestDashboardRecommendationStripFirstRun(t *testing.T) {
	m := dashboardWithProjects(120, 30, nil, OpenLoops{})
	plain := ansi.Strip(m.View().Content)
	if !strings.Contains(plain, "Create your first project") {
		t.Fatalf("expected first-run strip CTA in dashboard view, got:\n%s", plain)
	}
	if !strings.Contains(plain, "p new project") {
		t.Fatalf("expected first-run hint cluster in dashboard view, got:\n%s", plain)
	}
}

// TestDashboardRecommendationStripWorkOnTaskWithPlannedToday verifies that the
// dashboard summary calls the shared recommendation engine and surfaces a
// planned-today task above the project list with the matching reason badge.
// Confirms the Dashboard and Today cockpit agree on the next action when the
// same Projects + DailyPlan inputs are presented.
func TestDashboardRecommendationStripWorkOnTaskWithPlannedToday(t *testing.T) {
	cfg := testutil.NewTestConfig()
	project := testutil.NewTestProject("Cockpit Plan")
	planned, err := models.NewTask("task-planned", 1, models.TaskTypeTask, "Write design doc draft")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	planned.Status = models.StatusInProgress
	planned.Section = models.SectionNext
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
	m.setOpenLoops(OpenLoops{TodayTotal: 1, HasTasks: true})
	m.SetSize(120, 30)

	plain := ansi.Strip(m.View().Content)
	if !strings.Contains(plain, "Next: Write design doc draft") {
		t.Fatalf("expected WorkOnTask strip title above project list, got:\n%s", plain)
	}
	if !strings.Contains(plain, "planned today") {
		t.Fatalf("expected planned-today reason badge in dashboard strip, got:\n%s", plain)
	}
	if !strings.Contains(plain, "[Cockpit Plan]") {
		t.Fatalf("expected project badge in dashboard strip, got:\n%s", plain)
	}
}

// TestDashboardRecommendationStripTriageInboxFromOpenLoops verifies the
// inbox-only variant: when the engine returns AllDone (no actionable tasks
// loaded into Projects) but openLoops carries an inbox count, the strip pulls
// the count into a direct "Triage inbox (N items)" CTA — matching the Today
// cockpit's behavior so users get the same hint from both surfaces.
func TestDashboardRecommendationStripTriageInboxFromOpenLoops(t *testing.T) {
	project := testutil.NewTestProject("Inboxy")
	cfg := testutil.NewTestConfig()
	cfg.Projects = []*models.Project{project}
	m := &DashboardModel{
		config:                cfg,
		projects:              cfg.Projects,
		dashboardCompactCards: true,
	}
	m.InitSearchInput("Search projects...", 100, 40)
	m.setOpenLoops(OpenLoops{Inbox: 3, HasTasks: true})
	m.SetSize(120, 30)

	plain := ansi.Strip(m.View().Content)
	if !strings.Contains(plain, "Triage inbox") {
		t.Fatalf("expected triage CTA in dashboard strip, got:\n%s", plain)
	}
	if !strings.Contains(plain, "3 items") {
		t.Fatalf("expected inbox count in dashboard strip, got:\n%s", plain)
	}
}

// TestDashboardRecommendationStripSuppressedInArchivedMode verifies that the
// dashboard suppresses the strip when browsing archived projects. The
// archive-recovery surface is a focused recovery flow, not a primary command
// center, so a "next action" CTA would distract from the unarchive choices.
func TestDashboardRecommendationStripSuppressedInArchivedMode(t *testing.T) {
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
	m.setOpenLoops(OpenLoops{Inbox: 2, HasTasks: true})
	m.SetSize(120, 30)

	plain := ansi.Strip(m.View().Content)
	if strings.Contains(plain, "Triage inbox") || strings.Contains(plain, "Next: ") || strings.Contains(plain, "Create your first project") {
		t.Fatalf("expected no recommendation strip in archived mode, got:\n%s", plain)
	}
}

// TestDashboardRecommendationStripSuppressedWhenNarrow verifies the strip
// renderer's NextActionStripMinWidth gate flows through to the dashboard:
// when m.width is below the threshold the strip is suppressed and the
// project list still renders without the extra chrome rows.
func TestDashboardRecommendationStripSuppressedWhenNarrow(t *testing.T) {
	project := testutil.NewTestProject("Narrow Project")
	m := dashboardWithProjects(NextActionStripMinWidth-1, 24,
		[]*models.Project{project}, OpenLoops{Inbox: 1, HasTasks: true})

	strip, height := m.renderDashboardRecommendationStrip()
	if strip != "" {
		t.Fatalf("expected suppressed strip below NextActionStripMinWidth, got %q", strip)
	}
	if height != 0 {
		t.Fatalf("expected zero chrome lines when strip suppressed, got %d", height)
	}
}

// TestDashboardRecommendationStripAppearsAboveProjectList verifies the
// positional invariant: the strip's CTA must appear before the "Projects (N)"
// card header in the rendered output. This lets a future regression (e.g.
// strip accidentally rendered inside a card or below the project list) fail
// fast.
func TestDashboardRecommendationStripAppearsAboveProjectList(t *testing.T) {
	project := testutil.NewTestProject("Cockpit Plan")
	planned, err := models.NewTask("planned-1", 1, models.TaskTypeTask, "Ship release notes")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	planned.Status = models.StatusInProgress
	planned.Section = models.SectionNext
	project.Tasks = []*models.Task{planned}

	cfg := testutil.NewTestConfig()
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
	m.setOpenLoops(OpenLoops{TodayTotal: 1, HasTasks: true})
	m.SetSize(120, 30)

	plain := ansi.Strip(m.View().Content)
	stripIdx := strings.Index(plain, "Next: Ship release notes")
	if stripIdx < 0 {
		t.Fatalf("expected WorkOnTask CTA in dashboard view, got:\n%s", plain)
	}
	headerIdx := strings.Index(plain, "Projects (")
	if headerIdx < 0 {
		t.Fatalf("expected project list header, got:\n%s", plain)
	}
	if stripIdx >= headerIdx {
		t.Fatalf("expected strip (idx=%d) above project list header (idx=%d) in:\n%s",
			stripIdx, headerIdx, plain)
	}
}

// TestDashboardComputeRecommendationMatchesEngine ensures the dashboard's
// recommendation helper feeds the same Projects + DailyPlan + time.Now()
// triple into the shared engine that Today consumes. Same inputs → same Kind
// → guaranteed cockpit-wide alignment.
func TestDashboardComputeRecommendationMatchesEngine(t *testing.T) {
	project := testutil.NewTestProject("Cockpit Compare")
	overdue, err := models.NewTask("overdue-1", 1, models.TaskTypeTask, "Refund customer")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	due := time.Now().Add(-48 * time.Hour)
	overdue.DueDate = &due
	overdue.Section = models.SectionNext
	overdue.Status = models.StatusTodo
	project.Tasks = []*models.Task{overdue}

	cfg := testutil.NewTestConfig()
	cfg.Projects = []*models.Project{project}

	m := &DashboardModel{config: cfg, projects: cfg.Projects}
	got := m.computeDashboardRecommendation()
	if got.Kind != RecommendationKindWorkOnTask {
		t.Fatalf("expected WorkOnTask for overdue task, got Kind=%q", got.Kind)
	}
	if got.Task != overdue {
		t.Fatalf("expected engine to return the overdue task, got %v", got.Task)
	}
	if got.Project != project {
		t.Fatalf("expected engine to return the parent project, got %v", got.Project)
	}
}

// TestDashboardOpenLoopsAwareInterface verifies that DashboardModel
// implements openLoopsAware so AppModel.View injects the latest counts
// before each render. Without this the strip would fall back to stale
// zero-valued OpenLoops and miss the inbox-only / first-run signals.
func TestDashboardOpenLoopsAwareInterface(t *testing.T) {
	var m any = &DashboardModel{}
	if _, ok := m.(openLoopsAware); !ok {
		t.Fatal("expected *DashboardModel to implement openLoopsAware")
	}
}
