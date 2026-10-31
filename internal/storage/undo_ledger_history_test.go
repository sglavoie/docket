package storage

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestLedgerHistoryReadsRetainedImagesAndPreservesRestoreOrder(t *testing.T) {
	ledger, root := newTestLedger(t)
	path := filepath.Join(root, "note.md")
	for _, step := range []struct{ id, before, after string }{{"one", "a", "b"}, {"two", "b", "c"}, {"three", "c", "d"}} {
		writeFile(t, path, step.after)
		mustRecord(t, ledger, step.id, "p", path, step.before, step.after)
	}
	for range 2 {
		if _, err := ledger.Undo(GlobalScope()); err != nil {
			t.Fatal(err)
		}
	}
	before, err := os.ReadFile(ledger.ledgerPath())
	if err != nil {
		t.Fatal(err)
	}
	history, err := ledger.History(GlobalScope())
	if err != nil {
		t.Fatal(err)
	}
	if len(history.Undo) != 1 || history.Undo[0].TransactionID != "one" || len(history.Redo) != 2 || history.Redo[0].TransactionID != "two" || history.Redo[1].TransactionID != "three" {
		t.Fatalf("wrong replay order: %+v", history)
	}
	images, err := ledger.HistoryArtifacts("three")
	if err != nil {
		t.Fatal(err)
	}
	if len(images) != 1 || string(images[0].Before) != "c" || string(images[0].After) != "d" {
		t.Fatalf("details read live bytes instead of retained images: %+v", images)
	}
	after, err := os.ReadFile(ledger.ledgerPath())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("browsing changed the ledger")
	}
	if _, _, err := ledger.RedoTransactionWithEntry(GlobalScope(), "three"); !errors.Is(err, ErrUndoSuperseded) {
		t.Fatalf("stale redo was not refused: %v", err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "b" {
		t.Fatalf("stale redo wrote data: %q, %v", data, err)
	}
	if _, _, err := ledger.RedoTransactionWithEntry(GlobalScope(), "two"); err != nil {
		t.Fatal(err)
	}
}

func TestLedgerHistoryCorruptDetailsDoNotRemoveEntry(t *testing.T) {
	ledger, root := newTestLedger(t)
	path := filepath.Join(root, "note.md")
	mustRecord(t, ledger, "one", "p", path, "before", "after")
	writeFile(t, blobPathFor(ledger, "before"), "corrupt")
	before, err := os.ReadFile(ledger.ledgerPath())
	if err != nil {
		t.Fatal(err)
	}
	if _, detailErr := ledger.HistoryArtifacts("one"); detailErr == nil {
		t.Fatal("corrupt image accepted")
	}
	after, err := os.ReadFile(ledger.ledgerPath())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("reading corrupt details retired the transaction")
	}
	if _, err := ledger.HistoryArtifacts("missing"); err == nil {
		t.Fatal("missing transaction accepted")
	}
	if history, err := ledger.History(LedgerScope{ProjectID: "other"}); err != nil || len(history.Undo) != 0 {
		t.Fatalf("scope ignored: %+v, %v", history, err)
	}
}

func TestLedgerHistoryUsesDeviceOwnershipOfRecoveryKeys(t *testing.T) {
	workspace := newSyncWorkspace(t)
	path := filepath.Join(workspace.root, "projects", "alpha", "notes", "draft.md")
	mustRecord(t, workspace.ledger, "owned", "project-alpha", path, "one", "two")
	peer := openSyncWorkspace(t, workspace.root, "device-b")
	mustRecord(t, peer.ledger, "foreign", "project-alpha", path, "two", "three")
	history, err := workspace.ledger.History(GlobalScope())
	if err != nil {
		t.Fatal(err)
	}
	if len(history.Undo) != 1 || history.Undo[0].TransactionID != "owned" {
		t.Fatalf("foreign history offered as reversible: %+v", history)
	}
	if _, detailErr := workspace.ledger.HistoryArtifacts("foreign"); detailErr == nil {
		t.Fatal("foreign images entered this device's recovery browser")
	}
	newest, ok, err := workspace.ledger.SelectNewestUndo(GlobalScope())
	if err != nil || !ok || newest.TransactionID != history.Undo[0].TransactionID {
		t.Fatalf("browser and key disagree: %+v, %v", newest, err)
	}
}
