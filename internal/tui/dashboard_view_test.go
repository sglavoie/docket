package tui

import (
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
	if !strings.Contains(plain, "completed this week") {
		t.Fatalf("expected completed-this-week alert in dashboard\n\n%s", plain)
	}

	alertsStart := strings.Index(plain, "Alerts")
	if alertsStart == -1 || !strings.Contains(plain[alertsStart:], "╰") {
		t.Fatalf("expected Alerts card bottom border to be visible\n\n%s", plain)
	}
}
