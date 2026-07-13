package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/icons"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

// TestComputeGlobalAlertLinesSurfacesTriageCount verifies that any inbox
// backlog (even a single task) yields a prominent "to triage" line at the top
// of the alerts list, addressing session 5's "Where should this task go next?"
// mental model.
func TestComputeGlobalAlertLinesSurfacesTriageCount(t *testing.T) {
	icons.SetUseEmoji(false)
	t.Cleanup(func() { icons.SetUseEmoji(false) })

	cfg := testutil.NewTestConfig()
	cfg.DataDir = t.TempDir()
	project := testutil.NewTestProject("Triage Demo")
	inboxA := testutil.NewTestTask("triage me", models.TaskTypeTask)
	inboxA.SetSection(models.SectionInbox)
	inboxB := testutil.NewTestTask("triage me too", models.TaskTypeTask)
	inboxB.SetSection(models.SectionInbox)
	project.Tasks = []*models.Task{inboxA, inboxB}
	cfg.Projects = []*models.Project{project}

	m := &DashboardModel{config: cfg, projects: cfg.Projects}
	lines := m.computeGlobalAlertLines(nil)
	if len(lines) == 0 {
		t.Fatal("expected at least one alert line")
	}
	first := ansi.Strip(lines[0])
	if !strings.Contains(first, "2 to triage") {
		t.Fatalf("expected first alert line to surface triage count, got %q", first)
	}
}

// TestComputeGlobalAlertLinesSkipsTriageWhenEmpty verifies the triage line is
// absent when no project has inbox tasks.
func TestComputeGlobalAlertLinesSkipsTriageWhenEmpty(t *testing.T) {
	cfg := testutil.NewTestConfig()
	cfg.DataDir = t.TempDir()
	project := testutil.NewTestProject("Clean Inbox")
	project.Tasks = []*models.Task{}
	cfg.Projects = []*models.Project{project}

	m := &DashboardModel{config: cfg, projects: cfg.Projects}
	lines := m.computeGlobalAlertLines(nil)
	for _, line := range lines {
		if strings.Contains(ansi.Strip(line), "to triage") {
			t.Fatalf("expected no triage line for empty inbox, got %q", ansi.Strip(line))
		}
	}
}

// TestProjectRowStatusIndicatorIncludesInbox verifies the per-row status
// indicator surfaces inbox counts so users can see triage backlog at the row
// level, and that the precomputed width stays in sync with the rendered
// indicator (so column alignment does not drift).
func TestProjectRowStatusIndicatorIncludesInbox(t *testing.T) {
	icons.SetUseEmoji(true)
	t.Cleanup(func() { icons.SetUseEmoji(false) })

	project := testutil.NewTestProject("Inboxy")
	inbox := testutil.NewTestTask("pending triage", models.TaskTypeTask)
	inbox.SetSection(models.SectionInbox)
	project.Tasks = []*models.Task{inbox}

	m := &DashboardModel{}
	rendered := m.projectRowStatusIndicator(project, 3)
	plain := ansi.Strip(rendered)
	if !strings.Contains(plain, icons.Get("inbox")) {
		t.Fatalf("expected indicator to include inbox icon %q, got %q", icons.Get("inbox"), plain)
	}
	if !strings.Contains(plain, "1") {
		t.Fatalf("expected indicator to include inbox count, got %q", plain)
	}

	// Width predictor must match the rendered indicator width so that
	// projectListColumnWidths can budget the row layout correctly.
	width := m.projectStatusWidth(project, 3)
	got := lipgloss.Width(rendered)
	if width != got {
		t.Fatalf("projectStatusWidth = %d, lipgloss.Width = %d (%q)", width, got, plain)
	}
}

// TestPipelineCardLabelsUseTriage pins the renamed pipeline stage so future
// drift away from the "Triage" vocabulary fails a test.
func TestPipelineCardLabelsUseTriage(t *testing.T) {
	m := &DashboardModel{pipelineCounts: pipelineCounts{inboxCount: 3}}
	out := ansi.Strip(m.renderPipelineCardContent(60))
	if !strings.Contains(out, "Triage(3)") {
		t.Fatalf("expected pipeline card to show Triage(3), got %q", out)
	}
	if strings.Contains(out, "Organize") {
		t.Fatalf("pipeline card should no longer use legacy 'Organize' verb, got %q", out)
	}
}

// TestDashboardOpenInboxTriageKeyDispatchesMsg verifies the new `O` keybinding
// opens the inbox processor for the currently selected project.
func TestDashboardOpenInboxTriageKeyDispatchesMsg(t *testing.T) {
	cfg, project := testutil.NewTestConfigWithProject("Alpha")
	cfg.DataDir = t.TempDir()

	dashboard := NewDashboardModel(cfg, nil)
	_, cmd, consumed := dashboard.Dispatcher.Dispatch(syntheticKey('O'))
	if !consumed {
		t.Fatal("dashboard O binding should be active")
	}
	if cmd == nil {
		t.Fatal("dashboard O binding should return a ShowInboxProcessorMsg command")
	}

	msg, ok := cmd().(ShowInboxProcessorMsg)
	if !ok {
		t.Fatalf("dashboard O returned %T, want ShowInboxProcessorMsg", cmd())
	}
	if msg.Project != project {
		t.Fatal("dashboard O should target the currently selected project")
	}
}
