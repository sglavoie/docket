package tui

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/metrics"
)

// TestMetricsDashboardWeekStartPropagation verifies that the user's
// configured WeekEndDay flows through MetricsDashboardModel.computeMetrics
// into the weekly throughput bucket labels, instead of defaulting to Monday.
func TestMetricsDashboardWeekStartPropagation(t *testing.T) {
	cfg := config.NewConfig()
	cfg.WeekEndDay = "sat" // week ends Saturday → starts Sunday

	m := NewMetricsDashboardModel(cfg)
	m.timeRange = metrics.TimeRange90d // forces weekly granularity
	m.computeMetrics()

	if m.metrics == nil {
		t.Fatal("expected metrics to be computed")
	}

	want := metrics.WeekStartFromWeekEnd(cfg.GetWeekEndDay())
	if want != time.Sunday {
		t.Fatalf("test fixture broken: WeekStartFromWeekEnd(\"sat\") = %v, want Sunday", want)
	}

	expectAgg := metrics.ComputeAllAtWithWeekStart(nil, metrics.TimeRange90d, time.Now(), want)
	if len(m.metrics.Throughput.Labels) != len(expectAgg.Throughput.Labels) {
		t.Fatalf("Labels len = %d, want %d", len(m.metrics.Throughput.Labels), len(expectAgg.Throughput.Labels))
	}
	for i, lbl := range expectAgg.Throughput.Labels {
		if m.metrics.Throughput.Labels[i] != lbl {
			t.Errorf("Labels[%d] = %q, want %q (week-start %v not propagated)",
				i, m.metrics.Throughput.Labels[i], lbl, want)
		}
	}

	mondayAgg := metrics.ComputeAllAtWithWeekStart(nil, metrics.TimeRange90d, time.Now(), time.Monday)
	if len(mondayAgg.Throughput.Labels) > 0 && len(m.metrics.Throughput.Labels) > 0 &&
		m.metrics.Throughput.Labels[0] == mondayAgg.Throughput.Labels[0] {
		// First label only matches when "now" happens to fall such that
		// both week-starts produce the same first bucket boundary. We
		// only treat this as a hard failure when every label matches.
		allEqual := true
		for i := range mondayAgg.Throughput.Labels {
			if i >= len(m.metrics.Throughput.Labels) || m.metrics.Throughput.Labels[i] != mondayAgg.Throughput.Labels[i] {
				allEqual = false
				break
			}
		}
		if allEqual {
			t.Error("dashboard labels match the Monday default — week-start was not propagated")
		}
	}
}
