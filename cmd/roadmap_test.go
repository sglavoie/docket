package cmd

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/metrics"
)

// TestRoadmapWeekStartPropagation pins the wiring between cfg.GetWeekEndDay()
// and metrics.WeekStartFromWeekEnd that runRoadmap performs at the
// ComputeAllAtWithWeekStart call site. The full Cobra path is awkward to
// drive in a unit test (it touches disk-backed config and project storage),
// so we exercise just the cfg→metrics handshake.
func TestRoadmapWeekStartPropagation(t *testing.T) {
	cases := []struct {
		weekEnd string
		want    time.Weekday
	}{
		{"sun", time.Monday},   // default
		{"fri", time.Saturday}, // week ends Fri → starts Sat
		{"sat", time.Sunday},
	}
	for _, c := range cases {
		cfg := config.NewConfig()
		cfg.WeekEndDay = c.weekEnd
		got := metrics.WeekStartFromWeekEnd(cfg.GetWeekEndDay())
		if got != c.want {
			t.Errorf("WeekEndDay=%q: got week-start %v, want %v", c.weekEnd, got, c.want)
		}
	}
}

// TestRoadmapWeekStartUnsetDefaults verifies that an unset WeekEndDay (the
// zero value) flows through GetWeekEndDay() (which returns "sun") and lands
// on Monday as the week-start — matching metrics.DefaultWeekStart.
func TestRoadmapWeekStartUnsetDefaults(t *testing.T) {
	cfg := config.NewConfig()
	// WeekEndDay deliberately left empty.
	got := metrics.WeekStartFromWeekEnd(cfg.GetWeekEndDay())
	if got != time.Monday {
		t.Errorf("unset WeekEndDay should default to Monday week-start, got %v", got)
	}
}
