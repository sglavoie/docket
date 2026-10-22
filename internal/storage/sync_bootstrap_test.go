package storage

import (
	"fmt"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/sglavoie/tt/internal/synclog"
)

// This history is strictly linear; invoking merge would itself be a bug.
type bootstrapLinearMerger struct{}

// Source bytes come from the checked storage fixtures; this test exercises
// recovery, not the format readers owned by syncmerge.
func (bootstrapLinearMerger) ValidateArtifact(synclog.EffectKind, []byte) error { return nil }

func (bootstrapLinearMerger) Merge(synclog.MergeRequest) (synclog.MergeResult, error) {
	return synclog.MergeResult{}, fmt.Errorf("unexpected concurrent merge")
}

func (bootstrapLinearMerger) RestoreProject([]byte, string, []byte) ([]byte, bool, error) {
	return nil, false, fmt.Errorf("unexpected project restoration")
}

func TestBootstrapRecoveryKeepsWorkspaceAndAllReceiptsTogether(t *testing.T) {
	source := newSyncWorkspace(t)
	source.service.WithSyncMerger(bootstrapLinearMerger{})
	if _, err := source.service.CommitSyncGenesis(); err != nil {
		t.Fatal(err)
	}
	if _, err := source.service.Commit(source.retitleAlpha("latest title")); err != nil {
		t.Fatal(err)
	}
	manifests, err := source.history.Pending()
	if err != nil || len(manifests) != 2 {
		t.Fatalf("source history: %v, %v", manifests, err)
	}
	want := syncedFiles(t, source.root)
	for _, boundary := range []string{"prepared", "workspace applied", "all applied"} {
		t.Run(boundary, func(t *testing.T) {
			root := t.TempDir()
			history, err := synclog.New(synclog.Config{DataDir: root, DeviceID: "joining", Schemas: syncTestSchemas})
			if err != nil {
				t.Fatal(err)
			}
			dir := filepath.Join(root, ".tt", "transactions")
			service := NewCommitService(dir, []string{root}).WithSyncHistory(history).WithSyncMerger(bootstrapLinearMerger{})
			prepared := crashDuring(t, func() error {
				id, accepted, bootErr := service.BootstrapIncoming(manifests, source.history)
				if !accepted || id != "" {
					t.Fatalf("bootstrap did not reach journal boundary: %s, %v, %v", id, accepted, bootErr)
				}
				return bootErr
			})
			prefix := 0
			switch boundary {
			case "workspace applied":
				prefix = len(want)
			case "all applied":
				prefix = len(prepared.Ops)
			}
			simulateCrashAfter(t, dir, prepared, prefix)
			// A fresh service represents process restart; only the journal is
			// needed to finish, not the original inbox or in-memory plan.
			restarted := NewCommitService(dir, []string{root}).WithSyncHistory(history).WithSyncMerger(bootstrapLinearMerger{})
			report, err := restarted.RecoverWorkspace()
			if err != nil || report.Blocked() {
				t.Fatalf("recovery: %+v, %v", report, err)
			}
			if got := syncedFiles(t, root); !reflect.DeepEqual(want, got) {
				t.Fatalf("recovered workspace differs: %v, want %v", got, want)
			}
			state, err := history.LoadState()
			if err != nil || state == nil || state.Received[source.history.DeviceID()] != 2 {
				t.Fatalf("recovered tracking: %+v, %v", state, err)
			}
			for _, m := range manifests {
				if _, known, err := history.Lookup(m.TxnID); err != nil || !known {
					t.Fatalf("missing recovered receipt: %v", err)
				}
			}
			if _, accepted, err := restarted.BootstrapIncoming(manifests, source.history); err != nil || accepted {
				t.Fatalf("recovered history bootstrapped twice: %v, %v", accepted, err)
			}
			assertJournalClean(t, root)
		})
	}
}
