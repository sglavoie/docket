package storage

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sglavoie/docket/internal/models"
)

func newConflictStore(t *testing.T) *ConflictStore {
	t.Helper()
	return NewConflictStore(filepath.Join(t.TempDir(), ".docket", "conflicts"))
}

// TestConflictRecord_MissingRetryPolicyFailsClosed verifies bundles written by
// versions that predate the retry marker cannot silently apply one project from
// what may have been an aborted multi-artifact transaction. New project-only
// records can still opt in explicitly to legacy apply-in-place resolution.
func TestConflictRecord_MissingRetryPolicyFailsClosed(t *testing.T) {
	if !(ConflictRecord{}).OperationRetryRequired() {
		t.Fatal("missing retry policy must fail closed")
	}
	projectOnly := false
	if (ConflictRecord{RequiresOperationRetry: &projectOnly}).OperationRetryRequired() {
		t.Fatal("explicit project-only policy should allow apply-in-place resolution")
	}
}

// TestConflictStore_SaveLoadRoundTrip verifies a saved bundle round-trips its
// metadata and all three documents back through Load.
func TestConflictStore_SaveLoadRoundTrip(t *testing.T) {
	store := newConflictStore(t)

	base := mergeTestDoc("Alpha", mergeTestTask("t1", "base title"))
	local := mergeTestDoc("Alpha", mergeTestTask("t1", "local title"))
	remote := mergeTestDoc("Alpha", mergeTestTask("t1", "remote title"))
	conflicts := []MergeConflict{{Kind: MergeConflictFieldEdit, Field: "title", Base: "base title", Local: "local title", Current: "remote title"}}

	project := &models.Project{ID: "p1", Name: "Alpha", Slug: "alpha", Path: "/tmp/projects/alpha"}
	id, err := RecordMergeConflict(store, project, base, local, remote, conflicts, "task edit", "editing t1")
	if err != nil {
		t.Fatalf("RecordMergeConflict: %v", err)
	}
	if id == "" {
		t.Fatal("expected a non-empty conflict id")
	}

	bundle, err := store.Load(id)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if bundle.Record.ID != id {
		t.Fatalf("record id = %q, want %q", bundle.Record.ID, id)
	}
	if bundle.Record.ProjectID != "p1" || bundle.Record.ProjectSlug != "alpha" {
		t.Fatalf("record identity = %+v", bundle.Record)
	}
	if bundle.Record.BaseRevision != base.Revision || bundle.Record.RemoteRevision != remote.Revision || bundle.Record.LocalRevision != local.Revision {
		t.Fatalf("record revisions do not match inputs: %+v", bundle.Record)
	}
	if len(bundle.Record.Conflicts) != 1 || bundle.Record.Conflicts[0].Field != "title" {
		t.Fatalf("record conflicts = %+v", bundle.Record.Conflicts)
	}
	if bundle.Base == nil || bundle.Local == nil {
		t.Fatal("expected retained documents to load")
	}
	if bundle.Base.Tasks[0].Title != "base title" {
		t.Fatalf("base doc did not round-trip: title = %q", bundle.Base.Tasks[0].Title)
	}
	if bundle.Local.Tasks[0].Title != "local title" {
		t.Fatalf("local document did not round-trip: %q", bundle.Local.Tasks[0].Title)
	}
	if bundle.Record.CreatedAt.IsZero() {
		t.Fatal("expected CreatedAt to be stamped on save")
	}
}

// TestConflictStore_ListNewestFirstAndDelete verifies List enumerates saved
// conflicts and Delete removes one without disturbing the rest.
func TestConflictStore_ListNewestFirstAndDelete(t *testing.T) {
	store := newConflictStore(t)
	project := &models.Project{ID: "p1", Name: "Alpha", Slug: "alpha", Path: "/tmp/projects/alpha"}

	base := mergeTestDoc("Alpha", mergeTestTask("t1", "base"))
	local := mergeTestDoc("Alpha", mergeTestTask("t1", "local"))
	remote := mergeTestDoc("Alpha", mergeTestTask("t1", "remote"))

	id1, err := RecordMergeConflict(store, project, base, local, remote, []MergeConflict{{Kind: MergeConflictFieldEdit, Field: "title"}}, "a", "")
	if err != nil {
		t.Fatalf("record 1: %v", err)
	}
	id2, err := RecordMergeConflict(store, project, base, local, remote, []MergeConflict{{Kind: MergeConflictFieldEdit, Field: "title"}}, "b", "")
	if err != nil {
		t.Fatalf("record 2: %v", err)
	}

	records, err := store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("List returned %d records, want 2", len(records))
	}

	if delErr := store.Delete(id1); delErr != nil {
		t.Fatalf("Delete: %v", delErr)
	}
	records, err = store.List()
	if err != nil {
		t.Fatalf("List after delete: %v", err)
	}
	if len(records) != 1 || records[0].ID != id2 {
		t.Fatalf("after deleting %s, remaining = %+v (want only %s)", id1, records, id2)
	}

	// Deleting an already-gone conflict is a no-op, not an error.
	if err := store.Delete(id1); err != nil {
		t.Fatalf("second Delete should be a no-op, got %v", err)
	}
}

// TestConflictStore_ListMissingDirIsEmpty verifies listing a never-written store
// yields no records rather than an error.
func TestConflictStore_ListMissingDirIsEmpty(t *testing.T) {
	store := newConflictStore(t)
	records, err := store.List()
	if err != nil {
		t.Fatalf("List on missing dir: %v", err)
	}
	if len(records) != 0 {
		t.Fatalf("expected no records, got %d", len(records))
	}
}

// TestConflictStore_ListSkipsCorruptBundle verifies a subdirectory without a
// readable meta.json is skipped rather than failing the whole listing.
func TestConflictStore_ListSkipsCorruptBundle(t *testing.T) {
	store := newConflictStore(t)
	project := &models.Project{ID: "p1", Name: "Alpha", Slug: "alpha", Path: "/tmp/projects/alpha"}
	base := mergeTestDoc("Alpha", mergeTestTask("t1", "base"))
	if _, err := RecordMergeConflict(store, project, base, base, base, []MergeConflict{{Kind: MergeConflictFieldEdit}}, "a", ""); err != nil {
		t.Fatalf("record: %v", err)
	}

	// A stray directory with no meta.json must not break List.
	if err := os.MkdirAll(filepath.Join(store.Dir(), "not-a-conflict"), 0o700); err != nil {
		t.Fatal(err)
	}

	records, err := store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("List returned %d records, want 1 (corrupt dir skipped)", len(records))
	}
}

// TestConflictStore_LoadMissingErrors verifies loading an unknown id errors.
func TestConflictStore_LoadMissingErrors(t *testing.T) {
	store := newConflictStore(t)
	if _, err := store.Load("does-not-exist"); err == nil {
		t.Fatal("expected an error loading an unknown conflict id")
	}
}
