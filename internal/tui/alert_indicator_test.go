package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/alerts"
	"github.com/sglavoie/tt/internal/models"
)

// ── renderAlertIndicator / alertIndicatorReserve ──────────────────────────────

func TestAlertIndicatorZeroCountRendersNothing(t *testing.T) {
	if got := renderAlertIndicator(0); got != "" {
		t.Errorf("expected empty string for count=0, got %q", got)
	}
	indicator, reserve := alertIndicatorReserve(0)
	if indicator != "" || reserve != 0 {
		t.Errorf("zero alerts must reserve nothing, got (%q, %d)", indicator, reserve)
	}
}

func TestAlertIndicatorReserveMatchesRenderedWidth(t *testing.T) {
	for _, count := range []int{1, 9, 42, 100} {
		indicator, reserve := alertIndicatorReserve(count)
		if indicator == "" {
			t.Fatalf("count=%d: expected non-empty indicator", count)
		}
		if want := lipgloss.Width(indicator) + alertIndicatorGap; reserve != want {
			t.Errorf("count=%d: reserve = %d, want %d", count, reserve, want)
		}
	}
}

// ── overlayAlertIndicator (the generic post-render overlay) ───────────────────

func TestAlertIndicatorOverlayLeavesContentAloneWithoutIndicator(t *testing.T) {
	content := "line one\nline two"
	if got := overlayAlertIndicator(content, "", 80); got != content {
		t.Errorf("empty indicator: content should be unchanged, got %q", got)
	}
	if got := overlayAlertIndicator(content, "📫 3", 0); got != content {
		t.Errorf("zero width: content should be unchanged, got %q", got)
	}
}

func TestAlertIndicatorOverlayPlacesOnFirstLine(t *testing.T) {
	indicator := "📫 2"
	lines := strings.Split(overlayAlertIndicator("Header line\nSecond line\nThird line", indicator, 80), "\n")
	if !strings.Contains(lines[0], indicator) {
		t.Errorf("first line should contain indicator %q, got %q", indicator, lines[0])
	}
	if rest := strings.Join(lines[1:], "\n"); rest != "Second line\nThird line" {
		t.Errorf("lines after first should be unchanged, got %q", rest)
	}
}

func TestAlertIndicatorOverlaySkippedWhenTooNarrow(t *testing.T) {
	content := "A very long first line that takes up most of the space"
	if got := overlayAlertIndicator(content, "📫 99", 10); got != content {
		t.Errorf("when width is insufficient, content should be unchanged, got %q", got)
	}
}

// ── shouldShowAlertIndicator ─────────────────────────────────────────────────

func TestAlertIndicatorOverlaySkippedWithoutAlerts(t *testing.T) {
	app := newTestAppModel()
	setTestPresentationPath(app, ViewGlobalNotes)
	if app.shouldShowAlertIndicator() {
		t.Error("expected false when no alerts are set")
	}
}

func TestAlertIndicatorOverlayCoversSupportedViews(t *testing.T) {
	supported := []PresentationID{
		ViewDailyReview, ViewWeeklyReview,
		ViewMetricsDashboard,
		ViewRecentActivity,
		ViewRoadmap,
		ViewBlockers, ViewGlobalBlockers,
		ViewProjectRecentActivity,
		ViewGlobalNotes,
	}
	for _, view := range supported {
		app := newTestAppModel()
		setTestPresentationPath(app, view)
		app.proactiveAlerts = &alerts.AlertSnapshot{TotalCount: 1}
		if !app.shouldShowAlertIndicator() {
			t.Errorf("view %v with alerts: expected true", view)
		}
	}
}

func TestAlertIndicatorOverlaySkippedOnUnsupportedView(t *testing.T) {
	app := newTestAppModel()
	setTestPresentationPath(app, ViewHelp)
	app.proactiveAlerts = &alerts.AlertSnapshot{TotalCount: 3}
	if app.shouldShowAlertIndicator() {
		t.Error("expected false for unsupported view even with alerts")
	}
}

// TestAlertIndicatorOverlaySkippedOnOutliner guards against a doubled
// indicator: the Outliner composes the count into its own title row, so the
// post-render overlay must not add a second one.
func TestAlertIndicatorOverlaySkippedOnOutliner(t *testing.T) {
	app := newTestAppModel()
	setTestPresentationPath(app, ViewCrossProjectOutliner)
	app.proactiveAlerts = &alerts.AlertSnapshot{TotalCount: 3}
	if app.shouldShowAlertIndicator() {
		t.Error("the Outliner renders its own indicator; the overlay must be skipped")
	}
}

// TestAlertIndicatorRendersExactlyOnceInOutlinerFrame drives a full app frame so
// injection, composition, and the shell's overlay decision are exercised
// together rather than only in isolation.
func TestAlertIndicatorRendersExactlyOnceInOutlinerFrame(t *testing.T) {
	app := typedTestApp(t)
	app.proactiveAlerts = &alerts.AlertSnapshot{TotalCount: 4}

	frame := ansi.Strip(app.View().Content)

	if app.crossOutliner.alertCount != 4 {
		t.Fatalf("alert count injected into outliner = %d, want 4", app.crossOutliner.alertCount)
	}
	if got := strings.Count(frame, "📫 4"); got != 1 {
		t.Fatalf("indicator rendered %d time(s), want exactly 1:\n%s", got, frame)
	}
}

// ── Outliner title-row composition ───────────────────────────────────────────

func alertTitleOutliner(t *testing.T, alertCount int) *OutlinerModel {
	t.Helper()
	tasks := []*models.Task{
		renderFixtureTask(t, "task-1", 1, models.TaskTypeTask, "Ship the release", models.StateReady, 2, models.StateNext),
	}
	project := renderFixtureProject(t, "proj-alerts", "alerts", "Alerts", tasks)
	m := NewOutlinerModel(nil, nil, testConfigWithProjects(project))
	m.loadedProjects = []*models.Project{project}
	m.SetSize(100, 30)
	m.setAlertCount(alertCount)
	return m
}

func TestOutlinerTitleRowRightAlignsAlertIndicator(t *testing.T) {
	m := alertTitleOutliner(t, 3)
	m.setOpenLoops(OpenLoops{Inbox: 5, Overdue: 1})

	const width = 80
	line := m.buildOutlinerTitleLine(width, m.buildModeLabel(), "")
	plain := ansi.Strip(line)

	if !strings.HasSuffix(plain, "📫 3") {
		t.Errorf("title row must end with the indicator, got %q", plain)
	}
	if got := lipgloss.Width(line); got != width {
		t.Errorf("title row width = %d, want exactly %d (indicator flush right): %q", got, width, plain)
	}
	if !strings.Contains(plain, "inbox 5") {
		t.Errorf("open-loops counter should still fit at width %d: %q", width, plain)
	}
}

func TestOutlinerTitleRowZeroAlertsReserveNoSpace(t *testing.T) {
	m := alertTitleOutliner(t, 0)
	m.setOpenLoops(OpenLoops{Inbox: 5, Overdue: 1})

	const width = 80
	plain := ansi.Strip(m.buildOutlinerTitleLine(width, m.buildModeLabel(), ""))

	if strings.Contains(plain, "📫") {
		t.Errorf("zero alerts must not render an indicator: %q", plain)
	}
	if !strings.HasSuffix(plain, "overdue 1") {
		t.Errorf("zero alerts must leave the counter flush right, got %q", plain)
	}
	if got := lipgloss.Width(plain); got != width {
		t.Errorf("title row width = %d, want %d: %q", got, width, plain)
	}
}

// TestOutlinerTitleRowDegradesCounterBeforeIndicator pins the priority order:
// when the title consumes the row, the open-loops counter is dropped and the
// alert indicator still lands at the right edge.
func TestOutlinerTitleRowDegradesCounterBeforeIndicator(t *testing.T) {
	tasks := []*models.Task{
		renderFixtureTask(t, "task-1", 1, models.TaskTypeTask, "Ship it", models.StateReady, 2, models.StateNext),
	}
	longName := "Extremely Long Project Name That Consumes The Entire Title Row Budget"
	project := renderFixtureProject(t, "proj-long", "long", longName, tasks)
	m := NewOutlinerModel(project, nil, testConfigWithProjects(project))
	m.SetSize(60, 20)
	m.setAlertCount(7)
	m.setOpenLoops(OpenLoops{Inbox: 5, Overdue: 1})

	const width = 60
	plain := ansi.Strip(m.buildOutlinerTitleLine(width, m.buildModeLabel(), ""))

	if !strings.HasSuffix(plain, "📫 7") {
		t.Errorf("indicator must survive a full title row, got %q", plain)
	}
	if strings.Contains(plain, "inbox 5") {
		t.Errorf("open-loops counter must degrade before the indicator, got %q", plain)
	}
	if got := lipgloss.Width(plain); got > width {
		t.Errorf("title row width = %d, exceeds %d: %q", got, width, plain)
	}
}

func TestOutlinerTitleRowNeverOverflowsAtSupportedWidths(t *testing.T) {
	for _, width := range []int{60, 72, 80, 100, 120} {
		m := alertTitleOutliner(t, 12)
		m.setOpenLoops(OpenLoops{Inbox: 9, Overdue: 4})
		m.multiSelectMode = true
		m.filter.Tags = map[string]bool{"deep-work": true, "review-queue": true}
		m.sort = SortState{Field: SortByPriority, Ascending: true}

		line := m.buildOutlinerTitleLine(width, m.buildModeLabel(), m.buildOutlinerSearchIndicator())
		plain := ansi.Strip(line)

		if got := lipgloss.Width(line); got > width {
			t.Errorf("width %d: title row rendered %d cells: %q", width, got, plain)
		}
		if !strings.HasSuffix(plain, "📫 12") {
			t.Errorf("width %d: indicator missing from the right edge: %q", width, plain)
		}
	}
}

func TestOutlinerTitleRowEmptyStateShowsAlertIndicator(t *testing.T) {
	m := NewOutlinerModel(nil, nil, newTestConfig())
	m.SetSize(60, 20)
	m.setAlertCount(2)

	content := m.renderOutlinerTreeContent(60)
	first := ansi.Strip(strings.SplitN(content, "\n", 2)[0])

	if len(m.rows) != 0 {
		t.Fatalf("precondition: expected the empty state, got %d rows", len(m.rows))
	}
	if !strings.HasSuffix(first, "📫 2") {
		t.Errorf("empty-state title row must end with the indicator, got %q", first)
	}
	if got := lipgloss.Width(first); got > 60 {
		t.Errorf("empty-state title row width = %d, exceeds 60: %q", got, first)
	}
}
