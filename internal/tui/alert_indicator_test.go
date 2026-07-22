package tui

import (
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/alerts"
)

// ── renderAlertIndicator ──────────────────────────────────────────────────────

func TestRenderAlertIndicatorZero(t *testing.T) {
	if got := renderAlertIndicator(0); got != "" {
		t.Errorf("expected empty string for count=0, got %q", got)
	}
}

func TestRenderAlertIndicatorPositive(t *testing.T) {
	for _, count := range []int{1, 5, 99} {
		got := renderAlertIndicator(count)
		if got == "" {
			t.Errorf("count=%d: expected non-empty indicator", count)
		}
	}
}

// ── overlayAlertIndicator ────────────────────────────────────────────────────

func TestOverlayAlertIndicatorEmptyIndicator(t *testing.T) {
	content := "line one\nline two"
	got := overlayAlertIndicator(content, "", 80)
	if got != content {
		t.Errorf("empty indicator: content should be unchanged, got %q", got)
	}
}

func TestOverlayAlertIndicatorZeroWidth(t *testing.T) {
	content := "line one\nline two"
	got := overlayAlertIndicator(content, "📫 3", 0)
	if got != content {
		t.Errorf("zero width: content should be unchanged, got %q", got)
	}
}

func TestOverlayAlertIndicatorPlacesOnFirstLine(t *testing.T) {
	indicator := "📫 2"
	content := "Header line\nSecond line\nThird line"
	got := overlayAlertIndicator(content, indicator, 80)

	lines := strings.Split(got, "\n")
	if !strings.Contains(lines[0], indicator) {
		t.Errorf("first line should contain indicator %q, got %q", indicator, lines[0])
	}
	// Remaining lines should be unchanged.
	rest := strings.Join(lines[1:], "\n")
	if rest != "Second line\nThird line" {
		t.Errorf("lines after first should be unchanged, got %q", rest)
	}
}

func TestOverlayAlertIndicatorSingleLine(t *testing.T) {
	indicator := "📫 1"
	content := "Only line"
	got := overlayAlertIndicator(content, indicator, 80)
	if !strings.Contains(got, indicator) {
		t.Errorf("result should contain indicator %q, got %q", indicator, got)
	}
}

func TestOverlayAlertIndicatorInsufficientWidth(t *testing.T) {
	// Indicator + content exceeds width — indicator should be skipped.
	indicator := "📫 99"
	content := "A very long first line that takes up most of the space"
	got := overlayAlertIndicator(content, indicator, 10)
	if got != content {
		t.Errorf("when width is insufficient, content should be unchanged, got %q", got)
	}
}

// ── shouldShowAlertIndicator ─────────────────────────────────────────────────

func TestShouldShowAlertIndicatorNoAlerts(t *testing.T) {
	app := newTestAppModel()
	// proactiveAlerts is nil by default → alertCount() == 0
	if app.shouldShowAlertIndicator() {
		t.Error("expected false when no alerts are set")
	}
}

func TestShouldShowAlertIndicatorWithAlertsOnDashboard(t *testing.T) {
	app := newTestAppModel()
	app.viewStack = []PresentationID{ViewDashboard}
	app.proactiveAlerts = &alerts.AlertSnapshot{TotalCount: 3}
	if !app.shouldShowAlertIndicator() {
		t.Error("expected true for Dashboard with alerts")
	}
}

func TestShouldShowAlertIndicatorWithAlertsOnUnsupportedView(t *testing.T) {
	app := newTestAppModel()
	app.viewStack = []PresentationID{ViewHelp}
	app.proactiveAlerts = &alerts.AlertSnapshot{TotalCount: 3}
	if app.shouldShowAlertIndicator() {
		t.Error("expected false for unsupported view even with alerts")
	}
}

func TestShouldShowAlertIndicatorSupportedViews(t *testing.T) {
	supported := []PresentationID{
		ViewDashboard, ViewOutliner, ViewTemporaryOutliner, ViewFocusBoard,
		ViewCrossProjectOutliner,
		ViewDailyReview, ViewWeeklyReview,
		ViewMetricsDashboard,
		ViewRecentActivity,
		ViewRoadmap,
		ViewBlockers, ViewGlobalBlockers,
		ViewProjectRecentActivity,
		ViewGlobalInboxProcessor, ViewGlobalNotes,
	}
	for _, view := range supported {
		app := newTestAppModel()
		app.viewStack = []PresentationID{view}
		app.proactiveAlerts = &alerts.AlertSnapshot{TotalCount: 1}
		if !app.shouldShowAlertIndicator() {
			t.Errorf("view %v with alerts: expected true", view)
		}
	}
}
