package syncexchange

import (
	"os"
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/synclog"
)

func statusHistory(t *testing.T) *synclog.History {
	t.Helper()
	history, err := synclog.New(synclog.Config{
		DataDir:  t.TempDir(),
		DeviceID: "desk",
		Schemas:  map[string]int{synclog.SchemaCatalog: 1, synclog.SchemaDomainEnvelope: 1, synclog.SchemaTaskMarkdown: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	return history
}

func idleReport(at time.Time) *FolderReport {
	return &FolderReport{
		Format: statusFormat, At: at, Folder: "/exchange", DeviceID: "desk", Succeeded: true,
		Published: []string{}, Held: []Result{}, Unavailable: []string{},
		Peers: []DeviceAdvertisement{}, Results: []Result{}, Errors: []string{},
	}
}

// saveAndStamp saves report and reports whether status.json was rewritten,
// stamping it back to the epoch afterwards so a later rewrite is visible.
func saveAndStamp(t *testing.T, history *synclog.History, report *FolderReport) (rewritten bool) {
	t.Helper()
	if err := saveFolderStatus(history, report); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(StatusPath(history))
	if err != nil {
		t.Fatal(err)
	}
	epoch := time.Unix(0, 0)
	if err := os.Chtimes(StatusPath(history), epoch, epoch); err != nil {
		t.Fatal(err)
	}
	return !info.ModTime().Equal(epoch)
}

func TestIdlePassesDoNotRewriteTheStatusUntilItIsStale(t *testing.T) {
	history := statusHistory(t)
	start := time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)

	if !saveAndStamp(t, history, idleReport(start)) {
		t.Fatal("the first pass did not write the status")
	}
	if saveAndStamp(t, history, idleReport(start.Add(5*time.Second))) {
		t.Fatal("an idle pass five seconds later rewrote the status")
	}
	_, lastSuccess, err := LoadFolderStatus(history)
	if err != nil || !lastSuccess.Equal(start) {
		t.Fatalf("the kept status says last success %v (%v), want %v", lastSuccess, err, start)
	}
	if !saveAndStamp(t, history, idleReport(start.Add(statusRefreshInterval))) {
		t.Fatal("an idle pass a full refresh interval later did not rewrite the status")
	}
	_, lastSuccess, err = LoadFolderStatus(history)
	if want := start.Add(statusRefreshInterval); err != nil || !lastSuccess.Equal(want) {
		t.Fatalf("the refreshed status says last success %v (%v), want %v", lastSuccess, err, want)
	}
	if !saveAndStamp(t, history, idleReport(start)) {
		t.Fatal("a pass whose clock moved backwards kept a status time from the future")
	}
}

func TestAPassThatChangedAnythingRewritesTheStatusAtOnce(t *testing.T) {
	start := time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)
	changes := map[string]func(*FolderReport){
		"failed":      func(r *FolderReport) { r.Succeeded, r.Errors = false, []string{"unreachable"} },
		"imported":    func(r *FolderReport) { r.Imported = 1 },
		"published":   func(r *FolderReport) { r.Published = []string{"txn"} },
		"outstanding": func(r *FolderReport) { r.OutstandingUp = 2 },
		"unavailable": func(r *FolderReport) { r.Unavailable = []string{"workspace.json"} },
		"peer":        func(r *FolderReport) { r.Peers = []DeviceAdvertisement{{DeviceID: "phone"}} },
	}
	for name, change := range changes {
		t.Run(name, func(t *testing.T) {
			history := statusHistory(t)
			saveAndStamp(t, history, idleReport(start))
			report := idleReport(start.Add(time.Second))
			change(report)
			if !saveAndStamp(t, history, report) {
				t.Fatalf("a pass that %s did not rewrite the status", name)
			}
		})
	}
}

func TestAFirstSuccessAfterFailuresRewritesTheStatus(t *testing.T) {
	history := statusHistory(t)
	start := time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)
	failed := idleReport(start)
	failed.Succeeded = false
	saveAndStamp(t, history, failed)
	again := idleReport(start.Add(time.Second))
	again.Succeeded = false
	if saveAndStamp(t, history, again) {
		t.Fatal("a repeated identical failure rewrote the status")
	}
	if !saveAndStamp(t, history, idleReport(start.Add(2*time.Second))) {
		t.Fatal("the first success did not rewrite the status")
	}
	if _, lastSuccess, err := LoadFolderStatus(history); err != nil || lastSuccess.IsZero() {
		t.Fatalf("the status after a first success says last success %v (%v)", lastSuccess, err)
	}
}
