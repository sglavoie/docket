package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

func TestDashboardVerticalProjectSummaryFitsAlerts(t *testing.T) {
	cfg := testutil.NewTestConfig()
	cfg.DataDir = t.TempDir()
	cfg.SetDashboardBottomLayout(config.DashboardLayoutVertical)
	cfg.SetInfoPanelMode(config.InfoPanelProjectSummary)

	project := testutil.NewTestProject("Data pipelines")
	completed := testutil.NewTestTaskWithStatus("ship dashboard fix", models.StatusDone)
	now := time.Now()
	completed.CompletedAt = &now
	project.Tasks = []*models.Task{completed}
	cfg.Projects = []*models.Project{project}

	m := &DashboardModel{
		config:   cfg,
		projects: []*models.Project{project},
	}
	m.SetSize(140, 32)

	rendered := m.renderDashboard([]*models.Project{project}, 0)
	if got := lipgloss.Height(rendered); got > m.height {
		t.Fatalf("dashboard height = %d, want <= %d\n\n%s", got, m.height, ansi.Strip(rendered))
	}

	plain := ansi.Strip(rendered)
	if !strings.Contains(plain, "Alerts") {
		t.Fatalf("expected Alerts card in dashboard\n\n%s", plain)
	}
	if strings.Contains(plain, "Resume") {
		t.Fatalf("expected Project Summary to hide empty Resume card\n\n%s", plain)
	}
	if !strings.Contains(plain, "completed this week") {
		t.Fatalf("expected completed-this-week alert in dashboard\n\n%s", plain)
	}

	alertsStart := strings.Index(plain, "Alerts")
	if alertsStart == -1 || !strings.Contains(plain[alertsStart:], "╰") {
		t.Fatalf("expected Alerts card bottom border to be visible\n\n%s", plain)
	}
}

func TestDashboardProjectSummaryResumeCardForParkedTasks(t *testing.T) {
	now := time.Now()
	cases := []struct {
		name   string
		layout config.DashboardBottomLayout
		width  int
		height int
	}{
		{name: "horizontal", layout: config.DashboardLayoutHorizontal, width: 180, height: 36},
		{name: "vertical", layout: config.DashboardLayoutVertical, width: 180, height: 36},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := testutil.NewTestConfig()
			cfg.DataDir = t.TempDir()
			cfg.SetDashboardBottomLayout(tc.layout)
			cfg.SetInfoPanelMode(config.InfoPanelProjectSummary)

			project := testutil.NewTestProject("Ops")
			project.Tasks = []*models.Task{
				parkedDashboardTask("restart deploy", "review rollback notes", now.Add(-time.Hour)),
			}
			cfg.Projects = []*models.Project{project}

			m := &DashboardModel{
				config:   cfg,
				projects: []*models.Project{project},
			}
			m.SetSize(tc.width, tc.height)

			rendered := m.renderDashboard([]*models.Project{project}, 0)
			if got := lipgloss.Height(rendered); got > m.height {
				t.Fatalf("dashboard height = %d, want <= %d\n\n%s", got, m.height, ansi.Strip(rendered))
			}

			plain := ansi.Strip(rendered)
			for _, want := range []string{"Ops Summary", "Resume", "Alerts", "restart deploy", "review rollback"} {
				if !strings.Contains(plain, want) {
					t.Fatalf("expected %q in Project Summary dashboard\n\n%s", want, plain)
				}
			}
		})
	}
}

func TestDashboardProjectSummaryResumeHiddenWhenInfoAreaHidden(t *testing.T) {
	now := time.Now()
	cases := []struct {
		name  string
		setup func(*config.Config, *DashboardModel)
	}{
		{
			name: "list only layout",
			setup: func(cfg *config.Config, _ *DashboardModel) {
				cfg.SetDashboardBottomLayout(config.DashboardLayoutListOnly)
			},
		},
		{
			name: "info panel off",
			setup: func(cfg *config.Config, _ *DashboardModel) {
				cfg.SetInfoPanelMode(config.InfoPanelOff)
			},
		},
		{
			name: "active search",
			setup: func(_ *config.Config, m *DashboardModel) {
				m.SetSearchQuery("ops")
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := testutil.NewTestConfig()
			cfg.DataDir = t.TempDir()
			cfg.SetDashboardBottomLayout(config.DashboardLayoutHorizontal)
			cfg.SetInfoPanelMode(config.InfoPanelProjectSummary)

			project := testutil.NewTestProject("Ops")
			project.Tasks = []*models.Task{
				parkedDashboardTask("restart deploy", "review rollback notes", now.Add(-time.Hour)),
			}
			cfg.Projects = []*models.Project{project}

			m := &DashboardModel{
				config:   cfg,
				projects: []*models.Project{project},
			}
			m.SetSize(140, 32)
			tc.setup(cfg, m)

			plain := ansi.Strip(m.renderDashboard([]*models.Project{project}, 0))
			if strings.Contains(plain, "Resume") || strings.Contains(plain, "review rollback notes") {
				t.Fatalf("expected hidden info area to omit Resume content\n\n%s", plain)
			}
		})
	}
}

func TestDashboardResumeCardSortsAndLimitsParkedTasks(t *testing.T) {
	cfg := testutil.NewTestConfig()
	limit := 2
	cfg.ResumeCardLimit = &limit
	now := time.Now()

	project := testutil.NewTestProject("Ops")
	newest := parkedDashboardTask("newest parked", "pick this back up", now.Add(-time.Hour))
	middle := parkedDashboardTask("middle parked", "second choice", now.Add(-2*time.Hour))
	oldest := parkedDashboardTask("oldest parked", "too old for limit", now.Add(-3*time.Hour))
	completed := parkedDashboardTask("completed parked", "done already", now)
	completed.Status = models.StatusDone
	project.Tasks = []*models.Task{oldest, completed, middle, newest}

	m := &DashboardModel{
		config:   cfg,
		projects: []*models.Project{project},
	}
	m.SetSize(240, 32)

	entries := m.collectParkedResumeTasks(m.resumeCardLimit())
	if len(entries) != 2 {
		t.Fatalf("resume entries = %d, want 2", len(entries))
	}
	if entries[0].task.Title != "newest parked" || entries[1].task.Title != "middle parked" {
		t.Fatalf("resume entries out of order: %q then %q", entries[0].task.Title, entries[1].task.Title)
	}

	rendered := ansi.Strip(m.renderResumeCardContent(entries, 120))
	for _, want := range []string{"newest parked", "middle parked", "pick this back up"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected %q in resume card\n\n%s", want, rendered)
		}
	}
	for _, line := range strings.Split(rendered, "\n") {
		if got := lipgloss.Width(line); got > 120 {
			t.Fatalf("resume card line width = %d, want <= 120\n\n%s", got, rendered)
		}
	}
	if strings.Contains(rendered, "oldest parked") {
		t.Fatalf("resume card ignored configured limit\n\n%s", rendered)
	}
	if strings.Contains(rendered, "completed parked") {
		t.Fatalf("resume card should not include completed parked tasks\n\n%s", rendered)
	}
	full := ansi.Strip(m.renderInfoSectionFull(220, 18, project))
	if !strings.Contains(full, "Resume") {
		t.Fatalf("expected expanded dashboard to include Resume card when parked tasks exist\n\n%s", full)
	}
}

func TestDashboardResumeCardSnippetUsesRowWidthPolicy(t *testing.T) {
	project := testutil.NewTestProject("Ops")
	task := parkedDashboardTask(
		"resume deploy",
		"coordinate operator handoff after staging deploy clears",
		time.Now().Add(-time.Hour),
	)
	entry := parkedResumeEntry{task: task, project: project}

	narrow := ansi.Strip(renderResumeCardLine(entry, 36))
	wide := ansi.Strip(renderResumeCardLine(entry, 180))

	if strings.Contains(narrow, "operator handoff") {
		t.Fatalf("narrow resume card exposed too much parked note\n\n%s", narrow)
	}
	if !strings.Contains(wide, "operator handoff") {
		t.Fatalf("wide resume card did not expose expanded parked note\n\n%s", wide)
	}
	for _, tc := range []struct {
		name  string
		line  string
		width int
	}{
		{name: "narrow", line: narrow, width: 36},
		{name: "wide", line: wide, width: 180},
	} {
		if got := lipgloss.Width(tc.line); got > tc.width {
			t.Fatalf("%s resume card line width = %d, want <= %d\n\n%s", tc.name, got, tc.width, tc.line)
		}
	}
}

func TestDashboardResumeCardCompactEmptyState(t *testing.T) {
	cfg := testutil.NewTestConfig()
	project := testutil.NewTestProject("Ops")
	m := &DashboardModel{
		config:   cfg,
		projects: []*models.Project{project},
	}
	m.SetSize(140, 32)

	rendered := ansi.Strip(m.renderInfoSectionCompact(120, 18, project))
	if !strings.Contains(rendered, "Resume · No parked tasks") {
		t.Fatalf("expected inert resume empty state\n\n%s", rendered)
	}
}

func TestDashboardResumeCollectorClampsLimit(t *testing.T) {
	cfg := testutil.NewTestConfig()
	tooHigh := 999
	cfg.ResumeCardLimit = &tooHigh
	project := testutil.NewTestProject("Ops")
	now := time.Now()
	for i := range config.ResumeCardLimitUpperBound + 5 {
		project.Tasks = append(project.Tasks, parkedDashboardTask(fmt.Sprintf("parked %02d", i), "resume", now.Add(-time.Duration(i)*time.Minute)))
	}
	m := &DashboardModel{
		config:   cfg,
		projects: []*models.Project{project},
	}

	entries := m.collectParkedResumeTasks(m.resumeCardLimit())
	if len(entries) != config.ResumeCardLimitUpperBound {
		t.Fatalf("collectParkedResumeTasks returned %d entries, want %d", len(entries), config.ResumeCardLimitUpperBound)
	}
}

func parkedDashboardTask(title, note string, parkedAt time.Time) *models.Task {
	task := testutil.NewTestTask(title, models.TaskTypeTask)
	task.Status = models.StatusTodo
	task.ParkedNote = note
	task.ParkedAt = &parkedAt
	return task
}
