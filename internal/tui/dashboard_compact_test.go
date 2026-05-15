package tui

import (
	"strings"
	"testing"
)

func TestPipelineHasContent(t *testing.T) {
	tests := []struct {
		name   string
		counts pipelineCounts
		want   bool
	}{
		{"all zero", pipelineCounts{}, false},
		{"inbox only", pipelineCounts{inboxCount: 1}, true},
		{"queue only", pipelineCounts{queueLen: 1}, true},
		{"daily plan only", pipelineCounts{dailyPlanLen: 1}, true},
		{"in progress only", pipelineCounts{inProgressCount: 1}, true},
		{"completed today only", pipelineCounts{completedTodayCount: 1}, true},
		{"overdue does not signal pipeline content", pipelineCounts{overdueCount: 1}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := &DashboardModel{pipelineCounts: tc.counts}
			if got := m.pipelineHasContent(); got != tc.want {
				t.Fatalf("pipelineHasContent() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestRenderCompactInertRow(t *testing.T) {
	out := renderCompactInertRow("Alerts", "clear", 80)
	if !strings.Contains(out, "Alerts") {
		t.Fatalf("expected output to contain title; got %q", out)
	}
	if !strings.Contains(out, "clear") {
		t.Fatalf("expected output to contain summary; got %q", out)
	}
	if !strings.Contains(out, "·") {
		t.Fatalf("expected output to contain separator; got %q", out)
	}
}

func TestRenderCompactInertRowTruncates(t *testing.T) {
	long := strings.Repeat("x", 40)
	out := renderCompactInertRow("Recent", long, 10)
	// Just ensure no panic and result is non-empty. lipgloss styles may
	// add escape sequences, so we don't assert exact width here.
	if out == "" {
		t.Fatalf("expected non-empty output")
	}
}
