package tui

import (
	"fmt"
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
	truncated := truncateToVisualWidth(input, 10)

	if lipgloss.Width(truncated) > 10 {
		t.Fatalf("truncateToVisualWidth exceeded visual width: %q (width=%d)", truncated, lipgloss.Width(truncated))
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
				Category:    alerts.CategoryOverdue,
				Title:       "修复界面布局",
				Details:     "详情说明内容很长需要截断",
				ProjectID:   "proj-1",
				ProjectName: "超长项目名称用于截断",
				Severity:    alerts.SeverityWarning,
			}},
		}},
		TotalCount: 1,
	}

	m := NewNotificationCenterModel(snapshot, snapshot, map[string]bool{})
	m.SetSize(18, 10)
	rendered := m.View().Content

	for line := range strings.SplitSeq(rendered, "\n") {
		if lipgloss.Width(line) > 18 {
			t.Fatalf("notification center line exceeds width 18: %q (width=%d)\n\n%s", ansi.Strip(line), lipgloss.Width(line), ansi.Strip(rendered))
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

// TestRenderStatusBarFooterWrapsHelpInsteadOfClipping verifies that when a
// multi-item help string is too wide for the terminal, renderStatusBarFooter
// wraps it on " · " boundaries onto extra lines rather than clipping required
// help text. The wrapped footer records no audit issue and each rendered line
// stays within the terminal width (no geometry overflow).
func TestRenderStatusBarFooterWrapsHelpInsteadOfClipping(t *testing.T) {
	const width = 48

	var footer string
	_, issues := withRenderAudit(func() string {
		footer = renderStatusBarFooter(
			HelpStyle.Render("j/k/n/N: navigate · Tab: expand · Enter: open · d: dismiss · D: show all · Esc: close"),
			MutedStyle.Render("1 alerts"),
			width,
		)
		return footer
	})

	if hasIssueKind(issues, requiredTextTruncatedKind) {
		t.Fatalf("expected no %q audit issue for a wrappable footer, got %+v", requiredTextTruncatedKind, issues)
	}

	// The wrapped footer is taller than one help line but never wider than the
	// terminal: reserve enough height so normalizeScreen only checks width.
	lines := strings.Split(footer, "\n")
	if len(lines) < 3 {
		t.Fatalf("expected the footer to wrap onto extra lines, got %d line(s):\n%s", len(lines), ansi.Strip(footer))
	}
	plain, geomIssues := normalizeScreen(footer, width, len(lines))
	if len(geomIssues) > 0 {
		t.Fatalf("expected no geometry issues for the wrapped footer, got %+v\n\n%s", geomIssues, plain)
	}
}

// TestRenderStatusBarFooterRecordsAuditIssue verifies that last-resort audited
// truncation still fires when the help genuinely cannot be wrapped to fit: a
// single indivisible help item that is wider than the whole terminal must be
// clipped, and that clip is recorded as a required_text_truncated issue even
// though the resulting footer still fits geometrically.
func TestRenderStatusBarFooterRecordsAuditIssue(t *testing.T) {
	const width, height = 12, 2

	// A single item (no " · ") longer than the terminal cannot be wrapped, so it
	// falls back to audited truncation.
	oneWideItem := HelpStyle.Render("supercalifragilisticexpialidocious: run")

	var footer string
	_, issues := withRenderAudit(func() string {
		footer = renderStatusBarFooter(oneWideItem, MutedStyle.Render("x"), width)
		return footer
	})

	if !hasIssueKind(issues, requiredTextTruncatedKind) {
		t.Fatalf("expected a %q audit issue for the unwrappable footer, got %+v", requiredTextTruncatedKind, issues)
	}

	// The clip leaves no geometric trace: the footer still fits its width.
	plain, geomIssues := normalizeScreen(footer, width, height)
	if len(geomIssues) > 0 {
		t.Fatalf("expected no geometry issues for the fitted footer, got %+v\n\n%s", geomIssues, plain)
	}
}

// TestFullscreenContentHeightForBlocksBudgetsWrappedFooter verifies the shared
// JoinVertical-based content-height helper (used by Roadmap and the metrics
// dashboard) reserves the actual rendered footer height. At a narrow width the
// footer help wraps beyond the historical two rows, and the joined
// title/tabBar/content/footer view must still fit its height budget rather than
// overflowing by the extra wrapped rows.
func TestFullscreenContentHeightForBlocksBudgetsWrappedFooter(t *testing.T) {
	const width, height = 40, 20

	title := renderFullscreenTitle("Roadmap", "Week · All Projects", nil)
	tabBar := RenderTabBar([]string{"Timeline", "Critical Path", "Forecasts"}, 0, width, 1, nil)
	helpText := HelpStyle.Render("Tab tabs · j/k cursor · H/L scroll · z zoom · space expand · ↵ task · c crit path · r refresh · ? help")
	footer := renderStatusBarFooter("", helpText, width, nil)

	// Guard: the scenario only exercises the multi-row-footer budgeting path when
	// the footer actually wraps beyond the historical two rows.
	if got := lipgloss.Height(footer); got <= 2 {
		t.Fatalf("expected footer to wrap beyond two rows at width %d, got height %d", width, got)
	}

	contentHeight := fullscreenContentHeightForBlocks(height, title, tabBar, footer)
	view := lipgloss.JoinVertical(lipgloss.Left, title, tabBar, fitBlockHeight("", contentHeight), footer)
	if got := lipgloss.Height(view); got > height {
		t.Fatalf("joined view height %d exceeds budget %d\n\n%s", got, height, ansi.Strip(view))
	}
}

// TestRenderStatusBarFooterConstrainsOverWideLabel verifies that when the fixed
// (non-help) side is wider than the terminal, renderStatusBarFooter constrains it
// to the available width instead of overflowing while it wraps the opposite help
// side. The over-wide label is clipped with audited truncation (so the render
// check flags it) and no rendered line exceeds the terminal width.
func TestRenderStatusBarFooterConstrainsOverWideLabel(t *testing.T) {
	const width = 40

	// A fixed label wider than the whole terminal paired with a wrappable help
	// string: the help wraps and the label must be constrained, not left full-width.
	overWideLabel := MutedStyle.Render("Absurdly Long Fixed Status Label That Exceeds The Terminal Width")
	helpText := HelpStyle.Render("j/k nav · Tab expand · Enter open · d dismiss · Esc close")

	var footer string
	_, issues := withRenderAudit(func() string {
		footer = renderStatusBarFooter(helpText, overWideLabel, width)
		return footer
	})

	if !hasIssueKind(issues, requiredTextTruncatedKind) {
		t.Fatalf("expected a %q audit issue for the over-wide fixed label, got %+v", requiredTextTruncatedKind, issues)
	}

	lines := strings.Split(footer, "\n")
	plain, geomIssues := normalizeScreen(footer, width, len(lines))
	if len(geomIssues) > 0 {
		t.Fatalf("expected no geometry issues for the constrained footer, got %+v\n\n%s", geomIssues, plain)
	}
}

// TestAllTasksViewBudgetsWrappedFooterHeight renders the real All Tasks view at a
// narrow width where its footer help wraps beyond two rows, with enough rows to
// fill the content budget, and asserts the whole builder-composed view
// (title + chrome + content + footer) still fits its height budget. This
// exercises fullscreenContentHeightWithFooter end to end: reserving a fixed two
// footer rows would let the wrapped footer push the filled content past height.
func TestAllTasksViewBudgetsWrappedFooterHeight(t *testing.T) {
	ws, err := newUITestWorkspace()
	if err != nil {
		t.Fatalf("newUITestWorkspace: %v", err)
	}
	t.Cleanup(func() { _ = ws.Cleanup() })

	if _, err := ws.addProject("Budget Project", func(p *models.Project) {
		// The built-in "Active" tab keeps StatusInProgress tasks; add plenty so
		// the task list overflows the content area and must scroll.
		for i := 0; i < 40; i++ {
			task := newFixtureTask(p, models.TaskTypeTask, fmt.Sprintf("In-progress budgeting task %d", i+1))
			task.Status = models.StatusInProgress
			task.SetSection(models.SectionNext)
		}
	}); err != nil {
		t.Fatalf("addProject: %v", err)
	}

	m := NewAllTasksViewModel(ws.Config, ws.Manager)

	const width, height = 40, 18
	m.SetSize(width, height)

	// Guard: more rows than the height budget (so content fills and scrolls) and
	// a footer that wraps beyond two rows, so the budgeting path is exercised.
	if m.totalNavigableItems() <= height {
		t.Fatalf("expected the fixture to overflow the content budget, got %d navigable items", m.totalNavigableItems())
	}
	if got := lipgloss.Height(m.renderAllTasksFooter()); got <= 4 {
		t.Fatalf("expected the All Tasks footer to wrap beyond two rows at width %d, got block height %d", width, got)
	}

	view := m.View().Content
	if got := lipgloss.Height(view); got > height {
		t.Fatalf("All Tasks view height %d exceeds budget %d\n\n%s", got, height, ansi.Strip(view))
	}
}

// TestRoadmapViewBudgetsWrappedFooterHeight renders a real Roadmap timeline view
// at a narrow, short terminal size where the footer help wraps beyond two rows
// and there are far more rows than fit, so the timeline appends its
// "row N/M" scroll-position line. It asserts the final RoadmapModel.View().Content
// height stays within the configured terminal height. Without the renderer's
// post-render clamp (fitBlockHeight in roadmap_render.go), the appended
// scroll-position row would push the joined view one line past the budget.
func TestRoadmapViewBudgetsWrappedFooterHeight(t *testing.T) {
	const width, height = 40, 18

	m := &RoadmapModel{
		config:     testutil.NewTestConfig(),
		isGlobal:   true,
		zoom:       ZoomWeek,
		collapsed:  make(map[string]bool),
		labelWidth: 30,
		activeTab:  RoadmapTabTimeline,
	}
	m.SetSize(width, height)

	// A narrow time axis with a label for every column so renderAxisHeader can
	// index Labels safely.
	const totalCols = 8
	labels := make([]string, totalCols)
	for i := range labels {
		labels[i] = fmt.Sprintf("W%d", i+1)
	}
	m.axis = TimelineAxis{
		UnitDays:  1,
		TotalCols: totalCols,
		Labels:    labels,
		TodayCol:  1,
	}

	// Far more rows than the content budget so the timeline overflows and appends
	// its "row N/M" scroll-position line.
	const rowCount = 40
	for i := 0; i < rowCount; i++ {
		task, err := models.NewTask(fmt.Sprintf("task-%d", i+1), i+1, models.TaskTypeTask, fmt.Sprintf("Roadmap timeline task %d", i+1))
		if err != nil {
			t.Fatalf("NewTask: %v", err)
		}
		m.rows = append(m.rows, TimelineRow{
			Task:     task,
			BarStart: 1,
			BarEnd:   4,
		})
	}

	// Guard: the footer help must actually wrap beyond two rows at this width, or
	// the wrapped-footer budgeting path is not exercised. Mirror View's timeline
	// help composition.
	helpText := HelpStyle.Render(strings.Join(
		[]string{"Tab tabs", "j/k cursor", "H/L scroll", "z zoom", "space expand", "↵ task", "c crit path", "r refresh", "? help"},
		" · "))
	footer := renderStatusBarFooter("", helpText, width, m.AccentColor())
	if got := lipgloss.Height(footer); got <= 2 {
		t.Fatalf("expected the Roadmap footer to wrap beyond two rows at width %d, got height %d", width, got)
	}

	// Guard: the fixture must actually trigger the scroll-position path. Rendered
	// unclamped at a small budget, the timeline appends its "row N/M" line beyond
	// the requested height — precisely the overflow the View-level clamp defends
	// against. (The final clamped View trims this extra row, so it is asserted
	// here on the raw tab, not on the final view.)
	const probeHeight = 9
	rawTab := m.renderTimelineTab(probeHeight, m.timelineLayout())
	if !strings.Contains(ansi.Strip(rawTab), fmt.Sprintf("row 1/%d", rowCount)) {
		t.Fatalf("expected the unclamped timeline tab to append the scroll-position row, got:\n\n%s", ansi.Strip(rawTab))
	}
	if got := lipgloss.Height(rawTab); got <= probeHeight {
		t.Fatalf("expected the unclamped timeline tab to overflow probe height %d, got %d", probeHeight, got)
	}

	view := m.View().Content
	if got := lipgloss.Height(view); got > height {
		t.Fatalf("Roadmap view height %d exceeds budget %d\n\n%s", got, height, ansi.Strip(view))
	}
}
