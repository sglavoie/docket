package engine

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/storage"
	"github.com/sglavoie/docket/internal/synclog"
)

func copyWorkspace(t *testing.T, from, to string) {
	t.Helper()
	err := filepath.WalkDir(from, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(from, path)
		target := filepath.Join(to, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o700)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o600)
	})
	if err != nil {
		t.Fatalf("copy workspace: %v", err)
	}
}

// TestRuntimeRecordsSyncHistoryAndAppliesItElsewhere drives the runtime's
// mutations — a task capture, an artifact-only note, and an undo — and checks
// each became one outgoing record from the configured device, chained by
// ancestry, that a second device applies in order to reach the same bytes.
func TestRuntimeRecordsSyncHistoryAndAppliesItElsewhere(t *testing.T) {
	env := newHeadlessEnvironment(t)
	device := openHeadless(t, env.config(true))
	peerDir := filepath.Join(filepath.Dir(env.dataDir), "peer-workspace")
	copyWorkspace(t, env.dataDir, peerDir)

	unfiled := device.State().SystemUnfiledProject()
	captured, captureErr := device.Service().Capture(app.CaptureRequest{ProjectID: unfiled.ID, Title: "Synced capture"})
	if captureErr != nil {
		t.Fatalf("Capture: %v", captureErr)
	}
	if _, noteErr := device.Service().CreateNote(app.NoteCreateRequest{
		Location: app.ProjectNoteLocation(unfiled.ID),
		Note:     &models.Note{Title: "Synced note", Body: "body"},
	}); noteErr != nil {
		t.Fatalf("CreateNote: %v", noteErr)
	}
	if _, undoErr := device.Ledger().Undo(storage.GlobalScope()); undoErr != nil {
		t.Fatalf("Undo: %v", undoErr)
	}

	history := device.repo.svc.SyncHistory()
	if history == nil || history.DeviceID() != "headless-device-1" {
		t.Fatalf("runtime history %v does not record as the configured device", history)
	}
	records, err := history.Pending()
	if err != nil {
		t.Fatalf("Pending: %v", err)
	}
	if len(records) != 3 {
		t.Fatalf("outbox holds %d records, want capture, note, and undo", len(records))
	}
	capture, note, undo := records[0], records[1], records[2]
	requireOutgoingChain(t, env, unfiled.ID, records)

	peerEnv := env
	peerEnv.dataDir = peerDir
	peerEnv.machineHome = filepath.Join(filepath.Dir(env.machineHome), "peer-device")
	peerCfg := peerEnv.config(false)
	peerCfg.Machine = ExplicitMachine(peerEnv.machineHome, "headless-device-2")
	peer := openHeadless(t, peerCfg)
	for _, record := range records {
		if _, err := peer.repo.svc.CommitIncoming(record, history); err != nil {
			t.Fatalf("peer applies seq %d: %v", record.DeviceSeq, err)
		}
	}
	for _, rel := range []string{"projects/unfiled/tasks.md", note.Effects[0].Path} {
		mine, mineErr := os.ReadFile(filepath.Join(env.dataDir, rel))
		theirs, theirsErr := os.ReadFile(filepath.Join(peerDir, rel))
		if !bytes.Equal(mine, theirs) || (mineErr == nil) != (theirsErr == nil) {
			t.Fatalf("%s differs between devices after apply (%v / %v)", rel, mineErr, theirsErr)
		}
	}
	if err := peer.Refresh(); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	inbox := runQuery(t, peer, queryOf(t, models.QueryModeInbox))
	requireTitles(t, "peer inbox after apply", inbox, "Synced capture")
	if inbox[0].Locator() != (app.TaskLocator{ProjectID: captured.ProjectID, TaskID: captured.TaskID}) {
		t.Fatalf("peer reads the capture under another identity: %+v", inbox[0])
	}
	if pending, _ := peer.repo.svc.SyncHistory().Pending(); len(pending) != 0 {
		t.Fatalf("applying incoming transactions queued %d outgoing records", len(pending))
	}
	if err := device.repo.svc.CommitSyncPublished([]string{capture.TxnID, note.TxnID, undo.TxnID}); err != nil {
		t.Fatalf("ConfirmPublished: %v", err)
	}
	if pending, _ := history.Pending(); len(pending) != 0 {
		t.Fatalf("outbox after confirmation = %d", len(pending))
	}
}

// requireOutgoingChain checks the capture, note, and undo records: their
// kinds, device sequence, injected clock, ancestry, schemas, and effects.
func requireOutgoingChain(t *testing.T, env headlessEnvironment, unfiledID string, records []*synclog.Manifest) {
	t.Helper()
	capture, note, undo := records[0], records[1], records[2]
	wantKinds := []synclog.Kind{synclog.KindUser, synclog.KindUser, synclog.KindUndo}
	for i, record := range records {
		if record.Kind != wantKinds[i] || record.DeviceSeq != uint64(i+1) || record.DeviceID != "headless-device-1" {
			t.Fatalf("record %d = kind %s seq %d device %s", i, record.Kind, record.DeviceSeq, record.DeviceID)
		}
		if record.CreatedAt != env.clock.Now().UTC().Format("2006-01-02T15:04:05Z07:00") {
			t.Fatalf("record %d created_at %s does not read the injected clock", i, record.CreatedAt)
		}
		if i > 0 && (len(record.Parents) != 1 || record.Parents[0] != records[i-1].TxnID) {
			t.Fatalf("record %d parents %v, want [%s]", i, record.Parents, records[i-1].TxnID)
		}
		if record.Schemas[synclog.SchemaTaskMarkdown] != storage.CurrentTaskSchemaVersion {
			t.Fatalf("record %d schemas %v", i, record.Schemas)
		}
	}
	if len(capture.Effects) != 1 || capture.Effects[0].Kind != synclog.EffectProjectDocument || capture.Effects[0].ProjectID != unfiledID {
		t.Fatalf("capture effects = %+v, want unfiled's tasks.md", capture.Effects)
	}
	if len(note.Effects) != 1 || note.Effects[0].Kind != synclog.EffectNote || note.Effects[0].Op != synclog.OpPut || note.Effects[0].BaseBlob != nil {
		t.Fatalf("artifact-only note effects = %+v, want one created note", note.Effects)
	}
	if len(undo.Effects) != 1 || undo.Effects[0].Op != synclog.OpDelete || undo.Effects[0].Path != note.Effects[0].Path {
		t.Fatalf("undo effects = %+v, want the note's explicit deletion", undo.Effects)
	}

}
