package tui

import (
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/alerts"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

func TestDashboardRecentTasksContentRespectsVisualWidth(t *testing.T) {
	project, err := models.NewProject("proj-1", "Unicode Project", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}

	task, err := models.NewTask("task-1", 1, models.TaskTypeTask, "修复界面布局对齐问题")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	task.UpdatedAt = time.Now().Add(-2 * time.Hour)
	project.Tasks = []*models.Task{task}

	m := NewDashboardModel(testutil.NewTestConfig(), nil)
	contentWidth := 28
	rendered := m.renderRecentTasksContentWithWidth(project, contentWidth)

	for line := range strings.SplitSeq(rendered, "\n") {
		if lipgloss.Width(line) > contentWidth {
			t.Fatalf("dashboard recent task line exceeds width %d: %q (width=%d)\n\n%s", contentWidth, ansi.Strip(line), lipgloss.Width(line), ansi.Strip(rendered))
		}
	}
}

func TestNotificationCenterTruncateToWidthUsesVisualWidth(t *testing.T) {
	input := "修复界面布局  详情说明内容"
	truncated := truncateToWidth(input, 10)

	if lipgloss.Width(truncated) > 10 {
		t.Fatalf("truncateToWidth exceeded visual width: %q (width=%d)", truncated, lipgloss.Width(truncated))
	}
	if !strings.HasSuffix(truncated, "…") {
		t.Fatalf("expected ellipsis for truncated text, got %q", truncated)
	}
}

func TestNotificationCenterViewRespectsVisualWidth(t *testing.T) {
	snapshot := &alerts.AlertSnapshot{
		Groups: []alerts.AlertGroup{{
			Category: alerts.CategoryOverdue,
			Label:    "Overdue (1)",
			Alerts: []alerts.Alert{{
				Category: alerts.CategoryOverdue,
				Title:    "修复界面布局",
				Details:  "详情说明内容很长需要截断",
				Severity: alerts.SeverityWarning,
			}},
		}},
		TotalCount: 1,
	}

	m := NewNotificationCenterModel(snapshot, snapshot, map[string]bool{})
	m.SetSize(24, 10)
	rendered := m.View().Content

	for line := range strings.SplitSeq(rendered, "\n") {
		if lipgloss.Width(line) > 24 {
			t.Fatalf("notification center line exceeds width 24: %q (width=%d)\n\n%s", ansi.Strip(line), lipgloss.Width(line), ansi.Strip(rendered))
		}
	}
}

func TestRenderStatusBarFooterRespectsWidth(t *testing.T) {
	footer := renderStatusBarFooter(
		HelpStyle.Render("j/k/n/N: navigate · Tab: expand · Enter: open · d: dismiss · D: show all · Esc: close"),
		MutedStyle.Render("1 alerts"),
		24,
	)

	for line := range strings.SplitSeq(footer, "\n") {
		if lipgloss.Width(line) > 24 {
			t.Fatalf("footer line exceeds width 24: %q (width=%d)", ansi.Strip(line), lipgloss.Width(line))
		}
	}
}
