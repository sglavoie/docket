package engine

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/syncexchange"
)

func TestFolderExchangeReportsFailedImportAndRetriesAfterRepair(t *testing.T) {
	env := newEnrollEnvironment(t)
	env.enroll(t)
	desk, peer := env.open(t, env.workingCopy, enrollingDevice), env.join(t)
	captureIn(t, peer, "First incoming task")
	captureIn(t, peer, "Second incoming task")
	exchangeNow(t, peer)

	path := filepath.Join(desk.DataDir(), "projects", "unfiled", "tasks.md")
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	invalid := []byte("an unfinished external edit\n")
	if err = os.WriteFile(path, invalid, 0o600); err != nil {
		t.Fatal(err)
	}
	opts := syncexchange.FolderOptions{Folder: env.exchange, WorkspaceID: desk.State().Catalog().WorkspaceID}
	report, err := syncexchange.ExchangeFolder(desk.SyncCommits(), opts)
	if err != nil || !report.Succeeded || report.Imported != 0 || report.OutstandingDown != 2 {
		t.Fatalf("expected a completed exchange with blocked imports: %+v, %v", report, err)
	}
	if len(report.Errors) != 1 || !strings.Contains(report.Errors[0], "projects/unfiled/tasks.md") {
		t.Fatalf("the failed import's cause was hidden: %+v", report)
	}
	persisted, _, err := syncexchange.LoadFolderStatus(desk.SyncCommits().SyncHistory())
	if err != nil || persisted == nil || len(persisted.Errors) != 1 || persisted.Errors[0] != report.Errors[0] {
		t.Fatalf("status and doctor cannot see the import failure: %+v, %v", persisted, err)
	}
	if data, readErr := os.ReadFile(path); readErr != nil || !bytes.Equal(data, invalid) {
		t.Fatalf("the blocked import overwrote the external edit: %q, %v", data, readErr)
	}
	if err = os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	report, err = syncexchange.ExchangeFolder(desk.SyncCommits(), opts)
	if err != nil || report.Imported != 2 || report.OutstandingDown != 0 || len(report.Errors) != 0 {
		t.Fatalf("repair did not drain the queue and clear the error: %+v, %v", report, err)
	}
	requireSameTree(t, "repaired imports", syncedDigests(t, desk.DataDir()), syncedDigests(t, peer.DataDir()))
}
