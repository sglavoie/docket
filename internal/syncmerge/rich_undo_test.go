package syncmerge_test

import (
	"bytes"
	"errors"
	"path/filepath"
	"testing"

	"github.com/sglavoie/docket/internal/storage"
	"github.com/sglavoie/docket/internal/synclog"
)

// TestSyncUndo holds undo and redo to the sync contract: each is a new
// outgoing change every device applies, overlapping field changes refuse
// whole, independent scalar edits preserve peer work, and the history behind
// them belongs to the device that made it.
func TestSyncUndo(t *testing.T) {
	t.Run("undo_and_redo_publish_new_transactions", testUndoAndRedoPublishNewTransactions)
	t.Run("remote_edit_refuses_stale_undo", testUndoRemoteEditRefusesStaleUndo)
	t.Run("remote_edit_refuses_stale_redo", testUndoRemoteEditRefusesStaleRedo)
	t.Run("unrecorded_change_survives_undo_redo", testUndoUnrecordedChangeSurvivesUndoRedo)
	t.Run("undo_history_is_device_local", testUndoHistoryIsDeviceLocal)
}

func testUndoAndRedoPublishNewTransactions(t *testing.T) {
	for _, peer := range []struct {
		name, slug, id, title string
	}{
		{"different_project", homeSlug, homeTask1, "Retile the kitchen"},
		{"same_project", atlasSlug, localize, "Localize for 12 regions"},
	} {
		t.Run(peer.name, func(t *testing.T) {
			testUndoAndRedoPreservePeerTask(t, peer.slug, peer.id, peer.title)
		})
	}
}

func testUndoAndRedoPreservePeerTask(t *testing.T, peerSlug, peerID, peerTitle string) {
	t.Helper()
	c := newCluster(t, "a", "b")
	a, b := c.r("a"), c.r("b")
	original := a.task(atlasSlug, shipBeta)
	a.commitUndoable(a.retitleArtifact(atlasSlug, shipBeta, "Ship beta build v2"))
	edited := a.task(atlasSlug, shipBeta)
	b.pull(a)
	b.commit(b.retitleArtifact(peerSlug, peerID, peerTitle))
	a.pull(b)

	if _, err := a.ledger.Undo(storage.GlobalScope()); err != nil {
		t.Fatalf("undo: %v", err)
	}
	undo := lastOutgoing(t, a)
	if undo.Kind != synclog.KindUndo || len(undo.Effects) != 1 || undo.Effects[0].Path != atlasDoc {
		t.Fatalf("undo published %+v", undo)
	}
	heads := lastOutgoing(t, b).TxnID
	if !contains(undo.Parents, heads) {
		t.Fatalf("the undo does not descend from b's change it built on: parents %v", undo.Parents)
	}
	replicas := settleClean(t, c)
	requireTaskEverywhere(t, replicas, atlasSlug, shipBeta, original)
	for _, r := range replicas {
		if r.task(peerSlug, peerID).Title != peerTitle {
			t.Fatalf("%s lost b's change across a's undo", r.device)
		}
	}

	if _, err := a.ledger.Redo(storage.GlobalScope()); err != nil {
		t.Fatalf("redo: %v", err)
	}
	if redo := lastOutgoing(t, a); redo.Kind != synclog.KindRedo {
		t.Fatalf("redo published %+v", redo)
	}
	replicas = settleClean(t, c)
	requireTaskEverywhere(t, replicas, atlasSlug, shipBeta, edited)
	for _, r := range replicas {
		if r.task(peerSlug, peerID).Title != peerTitle {
			t.Fatalf("%s lost b's change across a's redo", r.device)
		}
	}
	all, _ := c.everything()
	requireOnlySyncedEffects(t, all)
}

func testUndoRemoteEditRefusesStaleUndo(t *testing.T) {
	c := newCluster(t, "a", "b")
	a, b := c.r("a"), c.r("b")
	a.commitUndoable(a.retitleArtifact(atlasSlug, shipBeta, "Ship beta build v2"))
	b.pull(a)
	b.commit(b.retitleArtifact(atlasSlug, shipBeta, "Ship beta build, phone title"))
	a.pull(b)

	current := a.read(atlasDoc)
	pending := len(a.outgoing())
	_, err := a.ledger.Undo(storage.GlobalScope())
	var conflict *storage.ArtifactConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("undo over a remote edit: %v, want an artifact conflict", err)
	}
	if !bytes.Equal(a.read(atlasDoc), current) {
		t.Fatal("a refused undo changed the document")
	}
	if len(a.outgoing()) != pending {
		t.Fatal("a refused undo published a transaction")
	}
	// The remote edit changed the same title and recorded no ledger entry, so
	// this entry cannot restore it: it is retired rather than left selected,
	// where it would wall off every older entry.
	var retired *storage.RetiredEntryError
	if !errors.As(err, &retired) {
		t.Fatalf("undo over a remote edit: %v, want the entry retired", err)
	}
	if _, ok, selectErr := a.ledger.SelectNewestUndo(storage.GlobalScope()); selectErr != nil || ok {
		t.Fatalf("a retired undo entry is still selected: %v %v", ok, selectErr)
	}
	replicas := settleClean(t, c)
	for _, r := range replicas {
		if r.task(atlasSlug, shipBeta).Title != "Ship beta build, phone title" {
			t.Fatalf("%s lost an edit", r.device)
		}
	}
}

func testUndoRemoteEditRefusesStaleRedo(t *testing.T) {
	c := newCluster(t, "a", "b")
	a, b := c.r("a"), c.r("b")
	a.commitUndoable(a.retitleArtifact(atlasSlug, shipBeta, "Ship beta build v2"))
	if _, err := a.ledger.Undo(storage.GlobalScope()); err != nil {
		t.Fatal(err)
	}
	b.pull(a)
	b.commit(b.retitleArtifact(atlasSlug, shipBeta, "Ship beta build, phone title"))
	a.pull(b)

	current := a.read(atlasDoc)
	_, err := a.ledger.Redo(storage.GlobalScope())
	var conflict *storage.ArtifactConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("redo over a remote edit: %v, want an artifact conflict", err)
	}
	if !bytes.Equal(a.read(atlasDoc), current) || a.task(atlasSlug, shipBeta).Title != "Ship beta build, phone title" {
		t.Fatal("a refused redo overwrote the remote edit")
	}
	settleClean(t, c)
}

func testUndoUnrecordedChangeSurvivesUndoRedo(t *testing.T) {
	c := newCluster(t, "a")
	a := c.r("a")
	original := a.task(atlasSlug, shipBeta)
	a.commitUndoable(a.retitleArtifact(atlasSlug, shipBeta, "Ship beta build v2"))
	edited := bytes.Replace(a.read(atlasDoc), []byte("Cut the release branch."), []byte("Cut it."), 1)
	a.write(atlasDoc, edited)
	if _, err := a.ledger.Undo(storage.GlobalScope()); err != nil {
		t.Fatalf("undo over an independent external edit: %v", err)
	}
	want := bytes.Replace(edited, []byte("Ship beta build v2"), []byte(original.Title), 1)
	if !bytes.Equal(a.read(atlasDoc), want) {
		t.Fatal("undo did not restore only the title while retaining the external description")
	}
	// A second direct edit while the entry waits on the redo stack must be
	// read again, rather than replaying the bytes observed during Undo.
	want = bytes.Replace(want, []byte("Cut it."), []byte("Cut it after review."), 1)
	a.write(atlasDoc, want)
	if _, err := a.ledger.Redo(storage.GlobalScope()); err != nil {
		t.Fatalf("redo over a newer independent external edit: %v", err)
	}
	want = bytes.Replace(want, []byte(original.Title), []byte("Ship beta build v2"), 1)
	if !bytes.Equal(a.read(atlasDoc), want) {
		t.Fatal("redo overwrote an external description edit")
	}
}

func testUndoHistoryIsDeviceLocal(t *testing.T) {
	c := newCluster(t, "a")
	a := c.r("a")
	a.commitUndoable(a.retitleArtifact(atlasSlug, shipBeta, "Ship beta build v2"))
	entry, ok, err := a.ledger.SelectNewestUndo(storage.GlobalScope())
	if err != nil || !ok || entry.Device != "a" {
		t.Fatalf("the entry is not stamped with its device: %+v %v %v", entry, ok, err)
	}

	// A copy of the workspace, ledger included, opened as another device
	// cannot replay a's history.
	copied := filepath.Join(t.TempDir(), "copy")
	copyTree(t, a.root, copied)
	other := c.open("c", copied)
	if count, countErr := other.ledger.UndoCount(storage.GlobalScope()); countErr != nil || count != 0 {
		t.Fatalf("another device sees %d undo entries (%v)", count, countErr)
	}
	if _, err := other.ledger.Undo(storage.GlobalScope()); !errors.Is(err, storage.ErrNoUndoHistory) {
		t.Fatalf("another device undid a's entry: %v", err)
	}
	if other.task(atlasSlug, shipBeta).Title != "Ship beta build v2" {
		t.Fatal("another device's refused undo changed the workspace")
	}
	if count, countErr := a.ledger.UndoCount(storage.GlobalScope()); countErr != nil || count != 1 {
		t.Fatalf("device a sees %d undo entries (%v)", count, countErr)
	}
	if _, err := a.ledger.Undo(storage.GlobalScope()); err != nil {
		t.Fatalf("device a cannot undo its own entry: %v", err)
	}
}
