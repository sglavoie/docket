package config

import (
	"testing"
)

// This file covers the stale-write path: two sessions publishing the same
// portable file in turn, each holding a baseline the other has superseded, and
// the machine-local pointer that has to follow a re-identified record. The
// merge's own laws are in sync_hazard_lens_adoption_test.go.

// TestLensCommitReconcilesRepeatedStaleWritesWithoutDuplicating drives two
// sessions writing the same portable file in turn, each holding a baseline the
// other has already superseded, so every commit after the first takes the
// stale-write merge path. The collection settles on the second round and stays
// there: repeated reconciliation adds no duplicate record.
func TestLensCommitReconcilesRepeatedStaleWritesWithoutDuplicating(t *testing.T) {
	s := newIsolatedStore(t)
	dataDir := s.DataDir()
	sessionA := NewDomainCoordinator(dataDir)
	sessionB := NewDomainCoordinator(dataDir)

	fromA := NamedLensList{
		adoptionLens(adoptionLensSharedID, "Shared", "#from-a"),
		adoptionLens(adoptionLensAlphaID, "Alpha", "#alpha"),
	}
	fromB := NamedLensList{
		adoptionLens(adoptionLensSharedID, "Shared", "#from-b"),
		adoptionLens(adoptionLensBetaID, "Beta", "#beta"),
	}
	commit := func(coordinator *DomainCoordinator, lenses NamedLensList) {
		t.Helper()
		if err := coordinator.CommitLensDefinitions(&WorkspaceLensDefinitions{Lenses: lenses.Clone()}); err != nil {
			t.Fatalf("commit lens definitions: %v", err)
		}
	}

	// Session A publishes and keeps its baseline; session B, which observed the
	// file only at write time, overwrites it. A is now stale.
	commit(sessionA, fromA)
	commit(sessionB, fromB)

	settled := ""
	for round := 0; round < 4; round++ {
		if round > 0 {
			rotatePublishedLensOrder(t, s, dataDir)
		}
		commit(sessionA, fromA)
		commit(sessionB, fromB)
		published := canonicalLensRecords(t, publishedLensDefinitions(t, s))
		if round == 0 {
			settled = published
			continue
		}
		if published != settled {
			t.Fatalf("round %d changed the reconciled collection\n settled %s\n now     %s", round, settled, published)
		}
	}

	final := publishedLensDefinitions(t, s)
	if len(final) != 4 {
		t.Fatalf("reconciliation should hold both sides of the id collision plus each one-sided lens, got %v", lensNames(final))
	}
	if err := final.Validate(); err != nil {
		t.Fatalf("a reconciled collection must satisfy the collection rules: %v", err)
	}
	queries := map[string]bool{}
	for _, lens := range final {
		queries[lens.Query] = true
	}
	for _, want := range []string{"#from-a", "#from-b", "#alpha", "#beta"} {
		if !queries[want] {
			t.Fatalf("reconciliation dropped %q: %v", want, lensNames(final))
		}
	}
}

// TestStaleLensReconciliationRemapsTheActiveLensPointer is the machine-local half
// of the same hazard. The pointer is not portable, so a reconciliation that
// re-identifies one of this machine's records would otherwise leave it naming
// whichever definition kept the plain id — a different lens under the same
// pointer. It is followed onto the derived identity instead.
func TestStaleLensReconciliationRemapsTheActiveLensPointer(t *testing.T) {
	s := newIsolatedStore(t)
	dataDir := s.DataDir()
	sessionA := NewDomainCoordinator(dataDir)
	sessionB := NewDomainCoordinator(dataDir)

	machineA := NewMachineState()
	machineA.LensHomeAdopted = true
	machineA.NamedLenses = NamedLensList{adoptionLens(adoptionLensSharedID, "Shared", "#from-a")}
	machineA.ActiveLensID = adoptionLensSharedID
	if err := sessionA.CommitMachine(machineA); err != nil {
		t.Fatalf("machine A commit: %v", err)
	}

	machineB := NewMachineState()
	machineB.LensHomeAdopted = true
	machineB.NamedLenses = NamedLensList{adoptionLens(adoptionLensSharedID, "Shared", "#from-b")}
	machineB.ActiveLensID = adoptionLensSharedID
	if err := sessionB.CommitMachine(machineB); err != nil {
		t.Fatalf("machine B commit: %v", err)
	}

	// A's baseline is now stale, so its next write reconciles. Its own record
	// keeps the contributed identity, so its pointer does not move.
	if err := sessionA.CommitMachine(machineA); err != nil {
		t.Fatalf("machine A reconciling commit: %v", err)
	}
	if machineA.ActiveLensID != adoptionLensSharedID {
		t.Fatalf("machine A's pointer moved off a record that kept its identity: %q", machineA.ActiveLensID)
	}

	// B's record is the one that was re-identified, so B's pointer follows it.
	if err := sessionB.CommitMachine(machineB); err != nil {
		t.Fatalf("machine B reconciling commit: %v", err)
	}
	if machineB.ActiveLensID == adoptionLensSharedID {
		t.Fatal("machine B's pointer still names the identity another definition now holds")
	}
	if machineB.ActiveLensID == "" {
		t.Fatal("machine B's lens survived the merge, so the pointer should have been remapped rather than dropped")
	}
	published := publishedLensDefinitions(t, s)
	var pointed *NamedLens
	for i, lens := range published {
		if lens.ID == machineB.ActiveLensID {
			pointed = &published[i]
		}
	}
	if pointed == nil {
		t.Fatalf("machine B's pointer %q names nothing in %v", machineB.ActiveLensID, lensNames(published))
	}
	if pointed.Query != "#from-b" {
		t.Fatalf("machine B's pointer followed the wrong definition: %q", pointed.Query)
	}

	// Reconciling again is a no-op on both machines, pointer included. Each round
	// re-arms staleness first, so these are real merges rather than writes that
	// found their own baseline current.
	for round := 0; round < 2; round++ {
		rotatePublishedLensOrder(t, s, dataDir)
		activeA, activeB := machineA.ActiveLensID, machineB.ActiveLensID
		if err := sessionA.CommitMachine(machineA); err != nil {
			t.Fatalf("machine A repeat commit: %v", err)
		}
		if err := sessionB.CommitMachine(machineB); err != nil {
			t.Fatalf("machine B repeat commit: %v", err)
		}
		if machineA.ActiveLensID != activeA || machineB.ActiveLensID != activeB {
			t.Fatalf("round %d moved a settled pointer: %q/%q", round, machineA.ActiveLensID, machineB.ActiveLensID)
		}
		if got := canonicalLensRecords(t, publishedLensDefinitions(t, s)); got != canonicalLensRecords(t, published) {
			t.Fatalf("round %d changed a settled collection: %s", round, got)
		}
	}
}

// rotatePublishedLensOrder republishes the settled collection with its first
// record moved to the back, through a coordinator holding no baseline of its
// own. Revisions are content hashes, so a session that rewrote byte-identical
// output would keep its baseline current and its next write would not take the
// merge path at all. Rotating changes the bytes without changing the records, so
// every later round is a genuine stale-write reconciliation — and, since the
// join reads the union rather than the slots, the canonical rendering has to
// come back unchanged anyway.
func rotatePublishedLensOrder(t *testing.T, store *DomainFileStore, dataDir string) {
	t.Helper()
	current := publishedLensDefinitions(t, store)
	if len(current) < 2 {
		t.Fatalf("rotating needs at least two records, got %d", len(current))
	}
	rotated := append(current[1:].Clone(), current[0].Clone())
	if err := NewDomainCoordinator(dataDir).CommitLensDefinitions(&WorkspaceLensDefinitions{Lenses: rotated}); err != nil {
		t.Fatalf("rotate the published lens order: %v", err)
	}
}

func publishedLensDefinitions(t *testing.T, store *DomainFileStore) NamedLensList {
	t.Helper()
	definitions, _, err := store.LoadLensDefinitionsChecked()
	if err != nil {
		t.Fatalf("load the published lens definitions: %v", err)
	}
	return definitions.Lenses
}
