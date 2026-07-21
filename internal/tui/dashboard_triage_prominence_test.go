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

// TestHomePipelineTriageRowDispatchesMsg verifies pipeline triage moved from a
// dedicated key and hub into an actionable Attention row.
func TestHomePipelineTriageRowDispatchesMsg(t *testing.T) {
	_, project := newTestConfigWithProject("Alpha")
	inbox := testutil.NewTestTask("Clarify request", models.TaskTypeTask)
	inbox.SetSection(models.SectionInbox)
	project.Tasks = []*models.Task{inbox}

	dashboard := dashboardWithProjects(120, 24, []*models.Project{project})
	dashboard.FocusAttention()
	var triageIndex = -1
	for i, row := range dashboard.visibleAttentionRows() {
		if row.Category == homeAttentionPipeline && row.Action == homeAttentionOpenGlobalInbox {
			triageIndex = i
			break
		}
	}
	if triageIndex < 0 {
		t.Fatal("Home Attention should include an actionable pipeline triage row")
	}
	dashboard.attentionCursor = triageIndex
	cmd := dashboard.activateAttentionRow()
	if cmd == nil {
		t.Fatal("pipeline triage row should return a command")
	}

	_, ok := cmd().(ShowGlobalInboxProcessorMsg)
	if !ok {
		t.Fatalf("pipeline triage row returned %T, want ShowGlobalInboxProcessorMsg", cmd())
	}
}
