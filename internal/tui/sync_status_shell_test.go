package tui

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/engine"
	"github.com/sglavoie/docket/internal/syncexchange"
)

// enrolledPair enrolls a workspace on one device and joins it from another,
// both real engines over real directories sharing one exchange folder.
type enrolledPair struct {
	desk, laptop *engine.Runtime
	original     *engine.Runtime
	deskMachine  engine.Machine
}

// reopenDesk opens a second runtime over the desk working copy as the same
// machine, the way a Docket command run beside a session would: it shares nothing
// with a session opened over pair.desk, so what it commits is a change made
// elsewhere.
func (p enrolledPair) reopenDesk(t *testing.T) *engine.Runtime {
	t.Helper()
	rt, err := engine.Open(engine.Config{DataDir: p.desk.DataDir(), Machine: p.deskMachine})
	if err != nil {
		t.Fatalf("reopen working copy: %v", err)
	}
	return rt
}

func newEnrolledPair(t *testing.T) enrolledPair {
	t.Helper()
	root := t.TempDir()
	deskMachine := engine.ExplicitMachine(filepath.Join(root, "home-desk"), "desk-a")
	original, err := engine.Open(engine.Config{DataDir: filepath.Join(root, "original"), Machine: deskMachine, CreateIfMissing: true})
	if err != nil {
		t.Fatalf("open original: %v", err)
	}
	if _, serviceErr := original.Service().Capture(app.CaptureRequest{ProjectID: original.State().SystemUnfiledProject().ID, Title: "Shared task"}); serviceErr != nil {
		t.Fatalf("capture: %v", serviceErr)
	}
	exchange := filepath.Join(root, "exchange")
	if mkdirErr := os.MkdirAll(exchange, 0o700); mkdirErr != nil {
		t.Fatal(mkdirErr)
	}
	enrolled, err := engine.Enroll(engine.EnrollConfig{
		Original:     original.DataDir(),
		WorkingCopy:  filepath.Join(root, "desk"),
		Exchange:     exchange,
		SnapshotRoot: filepath.Join(root, "snapshots"),
		Machine:      deskMachine,
	})
	if err != nil {
		t.Fatalf("enroll: %v", err)
	}
	laptop, _, err := engine.Join(engine.JoinConfig{
		DataDir:  filepath.Join(root, "laptop"),
		Exchange: exchange,
		Machine:  engine.ExplicitMachine(filepath.Join(root, "home-laptop"), "desk-b"),
	})
	if err != nil {
		t.Fatalf("join: %v", err)
	}
	desk, err := engine.Open(engine.Config{DataDir: enrolled.Enrollment.WorkingCopy, Machine: deskMachine})
	if err != nil {
		t.Fatalf("open working copy: %v", err)
	}
	return enrolledPair{desk: desk, laptop: laptop, original: original, deskMachine: deskMachine}
}

// retitleOnDisk edits the shared task's title in a device's tasks.md the way
// an editor would.
func retitleOnDisk(t *testing.T, rt *engine.Runtime, title string) {
	t.Helper()
	path := filepath.Join(rt.DataDir(), "projects", rt.State().SystemUnfiledProject().Slug, "tasks.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	edited := bytes.Replace(data, []byte("Shared task"), []byte(title), 1)
	if bytes.Equal(edited, data) {
		t.Fatalf("%s holds no shared task", path)
	}
	if err := os.WriteFile(path, edited, 0o600); err != nil {
		t.Fatal(err)
	}
}

func exchangeOnce(t *testing.T, rt *engine.Runtime) {
	t.Helper()
	if report, err := rt.SyncExchange(engine.SyncExchangeOptions{}); err != nil {
		t.Fatalf("exchange: %v (%+v)", err, report)
	}
}

// TestSyncStatusReachesTheStripAndTheConflictBrowser opens the Outliner over
// an enrolled working copy that has unpublished work and a real sync conflict
// with another device: the status strip names the unpublished work and counts
// the conflict, and gc lists the conflict beside the bundles with every
// competing version. A workspace that was never enrolled shows neither.
func TestSyncStatusReachesTheStripAndTheConflictBrowser(t *testing.T) {
	pair := newEnrolledPair(t)
	retitleOnDisk(t, pair.desk, "Desk title")
	retitleOnDisk(t, pair.laptop, "Laptop title")
	exchangeOnce(t, pair.desk)
	exchangeOnce(t, pair.laptop)
	exchangeOnce(t, pair.desk)
	if _, err := pair.desk.Service().Capture(app.CaptureRequest{ProjectID: pair.desk.State().SystemUnfiledProject().ID, Title: "Not yet published"}); err != nil {
		t.Fatalf("capture: %v", err)
	}

	model, err := NewAppModel(pair.desk)
	if err != nil {
		t.Fatalf("NewAppModel: %v", err)
	}
	line := model.feedback.syncAdvisoryLine()
	if !strings.Contains(line, "1 saved here, not yet published") || !strings.Contains(line, "exchanged") {
		t.Fatalf("sync line = %q", line)
	}
	if model.feedback.outstandingConflictCount() != 1 || len(model.feedback.syncConflicts) != 1 {
		t.Fatalf("conflict count = %d with %d sync conflicts, want the one sync conflict",
			model.feedback.outstandingConflictCount(), len(model.feedback.syncConflicts))
	}
	strip := model.crossOutliner.StatusStripBody(160)
	if !strings.Contains(strip, "sync: 1 saved here, not yet published") || !strings.Contains(strip, "1 unresolved merge conflict — gc to review") {
		t.Fatalf("status strip does not carry the sync line and the conflict count:\n%s", strip)
	}

	model.openConflictBrowser()
	if model.conflictBrowser == nil {
		t.Fatal("gc did not open the browser over a sync conflict")
	}
	if _, isBundle := model.conflictBrowser.CurrentRecord(); isBundle {
		t.Fatal("the sync conflict was listed as a bundle")
	}
	rendered := model.conflictBrowser.RenderWithin(160, 40)
	if !strings.Contains(rendered, "projects/unfiled/tasks.md") || !strings.Contains(rendered, "sync conflict · 2 versions") {
		t.Fatalf("browser does not list the sync conflict:\n%s", rendered)
	}
	// Inspecting it shows the base and both versions with who wrote each.
	model.conflictBrowser.HandleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	inspected := model.conflictBrowser.RenderWithin(200, 60)
	for range 80 {
		model.conflictBrowser.HandleKey(tea.KeyPressMsg{Code: 'j', Text: "j"})
		inspected += model.conflictBrowser.RenderWithin(200, 60)
	}
	for _, want := range []string{"Started from:", "Shared task", "Desk title", "Laptop title", ", this device", "edit outside Docket on", "v keep selected version"} {
		if !strings.Contains(inspected, want) {
			t.Fatalf("the inspected sync conflict lacks %q:\n%s", want, inspected)
		}
	}

	plain, err := NewAppModel(pair.original)
	if err != nil {
		t.Fatalf("NewAppModel over the original: %v", err)
	}
	if line := plain.feedback.syncAdvisoryLine(); line != "" || plain.feedback.outstandingConflictCount() != 0 {
		t.Fatalf("a workspace never enrolled reports sync state: %q, %d conflicts", line, plain.feedback.outstandingConflictCount())
	}
	if strings.Contains(plain.crossOutliner.StatusStripBody(160), "sync:") {
		t.Fatal("a workspace never enrolled drew a sync line")
	}
}

// TestSyncAdvisoryTextNamesEachStateApart keeps local saves, outgoing and
// incoming work, holds, and failures apart, and draws nothing when nothing
// waits.
func TestSyncAdvisoryTextNamesEachStateApart(t *testing.T) {
	now := time.Date(2026, time.October, 6, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name   string
		status engine.SyncStatus
		want   string
	}{
		{"download_request_failed", engine.SyncStatus{Enrolled: true, LastSuccess: now,
			LastExchange: &syncexchange.FolderReport{Succeeded: true, Unavailable: []string{"manifest"}, Errors: []string{"provider offline"}}},
			"sync: 1 file waiting to download · exchange errors (docket sync doctor) · exchanged <1m ago"},
		{"nothing_waiting", engine.SyncStatus{Enrolled: true, LastSuccess: now.Add(-time.Hour)}, ""},
		{"unpublished", engine.SyncStatus{Enrolled: true, OutstandingUp: 2, LastSuccess: now.Add(-5 * time.Minute)},
			"sync: 2 saved here, not yet published · exchanged 5m ago"},
		{"never_exchanged", engine.SyncStatus{Enrolled: true, OutstandingUp: 1},
			"sync: 1 saved here, not yet published · never exchanged"},
		{"held", engine.SyncStatus{Enrolled: true, OutstandingDown: 1, Upgrades: []string{"update Docket on this device"}, LastSuccess: now.Add(-3 * time.Hour)},
			"sync: update Docket to apply held changes (docket sync status) · 1 waiting to apply · exchanged 3h ago"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := syncAdvisoryText(&tc.status, now); got != tc.want {
				t.Fatalf("syncAdvisoryText = %q, want %q", got, tc.want)
			}
		})
	}
}
