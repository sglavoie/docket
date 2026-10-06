package storage_test

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/synclog"
)

const (
	faultShipBeta  = "0a7e5d10-2c3b-4f6a-8e9d-000000000101"
	faultProofread = "0a7e5d10-2c3b-4f6a-8e9d-000000000103"
	faultHomeTask1 = "0a7e5d10-2c3b-4f6a-8e9d-000000000201"
)

// requireStat fails unless the run exercised a fault or outcome at least once,
// so a scenario whose fault quietly stopped firing is missing evidence.
func requireStat(s *faultSim, stats ...string) {
	s.t.Helper()
	for _, stat := range stats {
		if s.stats[stat] == 0 {
			s.fatalf("scenario never exercised %s (%s)", stat, s.summary())
		}
	}
}

// edits makes n seeded local edits on d without exchanging anything.
func edits(s *faultSim, d *simDevice, n int) {
	for i := 0; i < n; i++ {
		s.tick++
		switch s.rng.Intn(6) {
		case 0, 1:
			s.retitleRandom(d)
		case 2:
			s.capture(d)
		case 3:
			s.appendNote(d)
		case 4:
			s.plan(d)
		default:
			s.reprioritize(d)
		}
	}
}

// rounds exchanges for n ticks: every device publishes and drains once a tick.
func rounds(s *faultSim, n int) {
	for i := 0; i < n; i++ {
		s.tick++
		for _, d := range s.devices {
			s.publish(d)
		}
		for _, d := range s.devices {
			s.drain(d)
		}
	}
}

// TestSyncFaultScenarios drives one named fault at a time, each from a fixed
// seed, through three devices and genesis observers, and holds every run to
// convergence, preservation of unresolved user content, idempotent
// redelivery, and atomic commits.
func TestSyncFaultScenarios(t *testing.T) {
	devices := []string{"device-a", "device-b", "device-c"}

	t.Run("delayed_delivery", func(t *testing.T) {
		s := newFaultSim(t, "delayed_delivery", 101, faultConfig{MaxDelay: 6}, devices...)
		for i := 0; i < 4; i++ {
			for _, d := range s.authors() {
				edits(s, d, 2)
				s.publish(d)
			}
			rounds(s, 2)
		}
		s.finish(2)
		requireStat(s, "delayed", "applied")
	})

	t.Run("duplicated_delivery", func(t *testing.T) {
		s := newFaultSim(t, "duplicated_delivery", 102, faultConfig{MaxDelay: 2, Duplicate: 1}, devices...)
		for i := 0; i < 3; i++ {
			for _, d := range s.authors() {
				edits(s, d, 2)
				s.publish(d)
			}
			rounds(s, 3)
		}
		s.finish(2)
		requireStat(s, "sent_duplicate", "repeat_ignored")
	})

	t.Run("reordered_delivery", func(t *testing.T) {
		s := newFaultSim(t, "reordered_delivery", 103, faultConfig{MaxDelay: 8}, devices...)
		for i := 0; i < 3; i++ {
			for _, d := range s.authors() {
				edits(s, d, 3)
				s.publish(d)
			}
		}
		rounds(s, 10)
		s.finish(2)
		requireStat(s, "applied_out_of_order", "waiting_for_ancestor")
	})

	t.Run("incomplete_delivery", func(t *testing.T) {
		s := newFaultSim(t, "incomplete_delivery", 104, faultConfig{MaxDelay: 3, Truncate: 0.6, BlobLag: 0.7}, devices...)
		for i := 0; i < 3; i++ {
			for _, d := range s.authors() {
				edits(s, d, 2)
				s.publish(d)
			}
			rounds(s, 2)
		}
		s.finish(2)
		requireStat(s, "refused_truncated", "blob_lagged", "waiting_for_blob")
	})

	t.Run("corrupted_delivery", func(t *testing.T) {
		s := newFaultSim(t, "corrupted_delivery", 105, faultConfig{MaxDelay: 2, Corrupt: 0.7, CorruptBlob: 0.5}, devices...)
		for i := 0; i < 3; i++ {
			for _, d := range s.authors() {
				edits(s, d, 2)
				s.publish(d)
			}
			rounds(s, 2)
		}
		s.finish(2)
		requireStat(s, "refused_corrupted", "blob_corrupt_reads", "waiting_for_blob")
	})

	t.Run("missing_ancestors", func(t *testing.T) {
		s := newFaultSim(t, "missing_ancestors", 106, faultConfig{}, devices...)
		a, b, c := s.device("device-a"), s.device("device-b"), s.device("device-c")
		// a's chain reaches b newest first, so every transaction arrives before
		// the ancestors it names; c receives b's reply to a chain it has none of.
		edits(s, a, 6)
		s.publish(a)
		reverse(b.inbox)
		for i := range b.inbox {
			b.inbox[i].due = s.tick
		}
		c.offline = true
		rounds(s, 8)
		edits(s, b, 2)
		s.publish(b)
		c.offline = false
		for i := range c.inbox {
			if s.decode(c.inbox[i].txnID).DeviceID == "device-a" {
				c.inbox[i].due = s.tick + 6
			}
		}
		rounds(s, 8)
		s.finish(2)
		requireStat(s, "waiting_for_ancestor")
	})

	t.Run("prolonged_offline", func(t *testing.T) {
		s := newFaultSim(t, "prolonged_offline", 107, faultConfig{MaxDelay: 2}, devices...)
		a, b, c := s.device("device-a"), s.device("device-b"), s.device("device-c")
		c.offline = true
		s.logf("device-c goes offline")
		for i := 0; i < 12; i++ {
			edits(s, c, 3)
			edits(s, a, 1)
			edits(s, b, 1)
			rounds(s, 1)
		}
		pending, err := c.history.Pending()
		if err != nil || len(pending) < 30 {
			s.fatalf("device-c holds %d unpublished transactions (%v), want a long offline history", len(pending), err)
		}
		s.logf("device-c comes back with %d unpublished transactions", len(pending))
		c.offline = false
		s.finish(2)
		requireStat(s, "applied")
	})

	t.Run("three_device_concurrency", func(t *testing.T) {
		s := newFaultSim(t, "three_device_concurrency", 108, faultConfig{MaxDelay: 3, Duplicate: 0.3}, devices...)
		// Every device edits the same task, the same note, and its own task,
		// with nothing exchanged in between.
		for _, d := range s.authors() {
			s.retitle(d, "atlas-launch", faultShipBeta)
			s.appendNote(d)
			s.capture(d)
			s.plan(d)
		}
		s.retitle(s.device("device-c"), "home-renovation", faultHomeTask1)
		conflicts := s.finish(2)
		if len(conflictsOn(conflicts, "tasks.md")) == 0 || len(conflictsOn(conflicts, "notes/")) == 0 {
			s.fatalf("three concurrent edits of one task and one note left conflicts %v", conflictKeys(conflicts))
		}
		requireStat(s, "preserved_in_conflict")
	})

	t.Run("conflicting_resolutions", func(t *testing.T) {
		s := newFaultSim(t, "conflicting_resolutions", 109, faultConfig{MaxDelay: 3, Duplicate: 0.4, Corrupt: 0.3}, devices...)
		a, b, c := s.device("device-a"), s.device("device-b"), s.device("device-c")
		for _, d := range s.authors() {
			s.retitle(d, "atlas-launch", faultShipBeta)
		}
		s.settle()
		open := s.conflicts(a)
		if len(open) != 1 {
			s.fatalf("setup left conflicts %v, want one", conflictKeys(open))
		}
		original := open[0]
		// a and b resolve the same conflict differently while c, which has not
		// resolved it, edits another task in the same document.
		s.resolveRetitling(a, original, "atlas-launch", faultShipBeta)
		s.resolveRetitling(b, original, "atlas-launch", faultShipBeta)
		s.retitle(c, "atlas-launch", faultProofread)
		s.settle()
		competing := s.conflicts(c)
		if len(competing) != 1 || competing[0].ID == original.ID {
			s.fatalf("disagreeing resolutions left %v, want one new conflict", conflictKeys(competing))
		}
		// The new conflict competes both resolutions; c's concurrent edit of
		// another task is a third maximal writer and merges in.
		resolutions := map[string]bool{}
		for _, candidate := range competing[0].Candidates {
			for _, w := range candidate.Writers {
				if m := s.decode(w); m.Kind == synclog.KindResolution {
					resolutions[m.DeviceID] = true
				}
			}
		}
		if !resolutions["device-a"] || !resolutions["device-b"] {
			s.fatalf("the new conflict competes resolutions from %v, want device-a's and device-b's", sortedKeys(resolutions))
		}
		if doc := s.doc(c, "atlas-launch"); !strings.Contains(findDocTask(doc, faultProofread).Title, "mk109x0006c") {
			s.fatalf("c's concurrent edit did not merge: %q", findDocTask(doc, faultProofread).Title)
		}
		// c settles the dispute; everyone converges with nothing left open.
		s.resolve(c, competing[0], -1)
		if conflicts := s.finish(2); len(conflicts) != 0 {
			s.fatalf("a final resolution left %v open", conflictKeys(conflicts))
		}
		requireStat(s, "resolutions")
	})

	t.Run("ordinary_edit_keeps_conflict_open", func(t *testing.T) {
		s := newFaultSim(t, "ordinary_edit_keeps_conflict_open", 110, faultConfig{MaxDelay: 1}, "device-a", "device-b")
		a, b := s.device("device-a"), s.device("device-b")
		s.retitle(a, "atlas-launch", faultShipBeta)
		s.retitle(b, "atlas-launch", faultShipBeta)
		s.settle()
		// Editing another task of the merged document is not a resolution: the
		// losing title must stay recoverable in an open conflict.
		s.retitle(a, "atlas-launch", faultProofread)
		s.appendNote(b)
		conflicts := s.finish(2)
		if len(conflictsOn(conflicts, "tasks.md")) != 1 {
			s.fatalf("an ordinary edit after a merge left conflicts %v, want the title conflict still open", conflictKeys(conflicts))
		}
		requireStat(s, "preserved_in_conflict")
	})

	t.Run("crash_restart", func(t *testing.T) {
		s := newFaultSim(t, "crash_restart", 111, faultConfig{MaxDelay: 2, Crash: 0.35, Restart: 0.15}, devices...)
		for step := 0; step < 30; step++ {
			s.randomStep()
		}
		s.finish(2)
		requireStat(s, "crashes", "restarts")
	})
}

// TestSyncFaultPropertiesConverge runs seeded histories with every fault at
// once: delays, duplicates, reordering, truncated and corrupted manifests,
// lagging and damaged blobs, crashes at every journal point, restarts, and
// devices going offline for stretches. Each seed must converge on every
// device and genesis observer, preserve every unresolved user write, ignore
// redelivery, and keep each commit atomic. Set TT_SYNC_FAULT_SEED to replay one
// seed.
func TestSyncFaultPropertiesConverge(t *testing.T) {
	seeds := []int64{1, 2, 3, 4, 5, 6}
	if raw := os.Getenv(faultSeedEnv); raw != "" {
		seed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			t.Fatalf("%s=%q: %v", faultSeedEnv, raw, err)
		}
		seeds = []int64{seed}
	}
	cfg := faultConfig{
		MaxDelay:    4,
		Duplicate:   0.2,
		Truncate:    0.15,
		Corrupt:     0.15,
		BlobLag:     0.2,
		CorruptBlob: 0.1,
		Crash:       0.12,
		Restart:     0.05,
		Offline:     0.06,
	}
	for _, seed := range seeds {
		t.Run(fmt.Sprintf("seed_%d", seed), func(t *testing.T) {
			s := newFaultSim(t, "properties", seed, cfg, "device-a", "device-b", "device-c")
			for step := 0; step < 60; step++ {
				s.randomStep()
			}
			conflicts := s.finish(2)
			t.Logf("seed %d: %d transactions, %d open conflicts; %s", seed, len(s.ex.order), len(conflicts), s.summary())
		})
	}
}

// TestSyncFaultSeedsReplayDeterministically runs one seed twice and requires
// the same trace, the same transaction ids, and the same bytes: a failing
// seed replays exactly.
func TestSyncFaultSeedsReplayDeterministically(t *testing.T) {
	cfg := faultConfig{MaxDelay: 3, Duplicate: 0.3, Truncate: 0.2, Corrupt: 0.2, BlobLag: 0.3, CorruptBlob: 0.2, Crash: 0.2, Restart: 0.1, Offline: 0.1}
	run := func(t *testing.T) (string, []string, map[string]string) {
		s := newFaultSim(t, "determinism", 42, cfg, "device-a", "device-b", "device-c")
		for step := 0; step < 20; step++ {
			s.randomStep()
		}
		s.finish(1)
		// Device roots differ between runs; the trace names only devices.
		return strings.Join(s.trace, "\n"), append([]string{}, s.ex.order...), s.syncedFiles(s.devices[0])
	}
	var traces [2]string
	var orders [2][]string
	var files [2]map[string]string
	for i := range traces {
		t.Run(fmt.Sprintf("run_%d", i+1), func(t *testing.T) {
			traces[i], orders[i], files[i] = run(t)
		})
	}
	if traces[0] != traces[1] {
		t.Fatalf("seed 42 produced two different traces:\n%s\n---\n%s", traces[0], traces[1])
	}
	if strings.Join(orders[0], ",") != strings.Join(orders[1], ",") {
		t.Fatal("seed 42 minted different transaction ids")
	}
	for _, rel := range sortedKeys(files[0]) {
		if files[0][rel] != files[1][rel] {
			t.Fatalf("seed 42 left different bytes at %s", rel)
		}
	}
	if len(orders[0]) == 0 || !strings.Contains(traces[0], "crash") {
		t.Fatalf("the determinism run exercised too little: %d transactions", len(orders[0]))
	}
}

// TestSyncFaultFailuresReportSeedAndTrace breaks an invariant on purpose and
// requires the failure to name its seed, a replay command, and the trace,
// written in full to the trace directory.
func TestSyncFaultFailuresReportSeedAndTrace(t *testing.T) {
	traceDir := t.TempDir()
	t.Setenv(faultTraceEnv, traceDir)
	s := newFaultSim(t, "report", 7, faultConfig{MaxDelay: 1}, "device-a", "device-b")
	type aborted struct{ report string }
	s.fail = func(report string) { panic(aborted{report}) }
	s.retitle(s.device("device-a"), "atlas-launch", faultShipBeta)
	s.settle()
	// A file changed behind the history's back is what convergence must catch.
	tampered := s.device("device-b").path("projects/atlas-launch/tasks.md")
	if err := os.WriteFile(tampered, []byte("tampered\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var report string
	func() {
		defer func() {
			if r := recover(); r != nil {
				a, ok := r.(aborted)
				if !ok {
					panic(r)
				}
				report = a.report
			}
		}()
		s.requireConverged()
	}()
	if report == "" {
		t.Fatal("a diverged replica passed the convergence check")
	}
	for _, want := range []string{
		"sync fault failure: device-a and device-b disagree on projects/atlas-launch/tasks.md",
		"scenario=report seed=7",
		"replay: TT_SYNC_FAULT_SEED=7 go test -count=1 -run '^TestSyncFaultFailuresReportSeedAndTrace$' ./internal/storage/",
		"device-a publish",
		"device-b apply",
		"FAIL device-a and device-b disagree",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("failure report lacks %q:\n%s", want, report)
		}
	}
	written, err := os.ReadFile(traceDir + "/TestSyncFaultFailuresReportSeedAndTrace-seed-7.trace")
	if err != nil {
		t.Fatalf("the full trace was not written: %v\n%s", err, report)
	}
	if !strings.Contains(string(written), "start seed=7") || !strings.Contains(string(written), "FAIL ") {
		t.Fatalf("the written trace is incomplete:\n%s", written)
	}
}

func reverse[T any](items []T) {
	for i, j := 0, len(items)-1; i < j; i, j = i+1, j-1 {
		items[i], items[j] = items[j], items[i]
	}
}

func conflictsOn(conflicts []synclog.Conflict, fragment string) []synclog.Conflict {
	var out []synclog.Conflict
	for _, conflict := range conflicts {
		if strings.Contains(conflict.Path, fragment) {
			out = append(out, conflict)
		}
	}
	return out
}

func conflictKeys(conflicts []synclog.Conflict) []string {
	out := make([]string, 0, len(conflicts))
	for _, conflict := range conflicts {
		out = append(out, conflict.Path)
	}
	return out
}
