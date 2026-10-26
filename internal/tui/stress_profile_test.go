package tui

import (
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/engine"
)

// TestStressNavigationLatency times keyboard navigation over a workspace
// scripts/stressgen built. It is skipped unless DOCKET_STRESS_DIR names one, and
// is meant to run under -cpuprofile; see `just stress-profile`.
func TestStressNavigationLatency(t *testing.T) {
	root := os.Getenv("DOCKET_STRESS_DIR")
	if root == "" {
		t.Skip("DOCKET_STRESS_DIR names no stress workspace")
	}
	key := envOr("DOCKET_STRESS_KEY", "ctrl+d")
	presses, err := strconv.Atoi(envOr("DOCKET_STRESS_PRESSES", "200"))
	if err != nil {
		t.Fatal(err)
	}

	start := time.Now()
	rt, err := engine.Open(engine.Config{
		DataDir:         filepath.Join(root, "workspace"),
		Machine:         engine.ExplicitMachine(filepath.Join(root, "home"), "docket-stress"),
		RefuseMigration: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	app, err := NewAppModel(rt)
	if err != nil {
		t.Fatal(err)
	}
	d := NewUITestDriver(app, 160, 48)
	d.SetDrainTimeout(time.Minute)
	if d.app.lifecycle.startup != nil {
		d.dispatch(d.app.lifecycle.startup)
		d.drain()
	}
	d.bootLaunchCommands()
	d.app.View()
	t.Logf("launch to first frame: %s, %d rows", time.Since(start).Round(time.Millisecond), len(d.app.Outliner().Rows))

	updates := make([]time.Duration, 0, presses)
	frames := make([]time.Duration, 0, presses)
	for range presses {
		start := time.Now()
		if err := d.SendKey(key); err != nil {
			t.Fatal(err)
		}
		d.drain()
		updated := time.Now()
		d.app.View()
		updates = append(updates, updated.Sub(start))
		frames = append(frames, time.Since(updated))
	}
	t.Logf("%d × %s, cursor ended at row %d of %d", presses, key, d.app.Outliner().Cursor, len(d.app.Outliner().Rows))
	t.Logf("update+drain: %s", latencySummary(updates))
	t.Logf("view:         %s", latencySummary(frames))
}

func envOr(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}

func latencySummary(samples []time.Duration) string {
	sorted := slices.Clone(samples)
	slices.Sort(sorted)
	var total time.Duration
	for _, s := range sorted {
		total += s
	}
	at := func(p float64) time.Duration { return sorted[int(p*float64(len(sorted)-1))].Round(time.Microsecond) }
	return "mean " + (total / time.Duration(len(sorted))).Round(time.Microsecond).String() +
		" p50 " + at(0.5).String() + " p95 " + at(0.95).String() + " max " + at(1).String()
}
