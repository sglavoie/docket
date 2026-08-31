package config

import (
	"encoding/json"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// These tests cover the cross-machine hazards a synced workspace exposes at the
// domain layer: a machine still running an older binary against a catalog a
// newer one already advanced, and a lens set arriving from an arbitrary number
// of machines in an arbitrary order. Neither can be expressed as a scenario —
// the TUI driver has one app, one binary, and one machine home — so they live
// here, where a second home and a hand-written schema marker are both reachable.

// TestOlderBinaryRefusesNewerCatalog is the first machine to sync a catalog a
// newer binary published. Refusing is the whole contract: there is no
// down-migration, so an older build that opened the workspace anyway would write
// a schema it cannot represent back over the newer one.
func TestOlderBinaryRefusesNewerCatalog(t *testing.T) {
	home := t.TempDir()
	dataDir, _ := stageSharedWorkspaceOn(t, home)

	bumpCatalogSchemaVersion(t, WorkspaceCatalogPath(dataDir), CurrentConfigVersion+1)
	before := snapshotWorkspaceFiles(t, dataDir)

	manager := NewManagerOnMachine(ConfigPathForDataDir(dataDir), home)
	_, err := manager.LoadOrCreateDomains()
	if err == nil {
		t.Fatal("LoadOrCreateDomains accepted a catalog newer than this build supports")
	}
	if !strings.Contains(err.Error(), "newer than supported") {
		t.Fatalf("refusal = %v, want it to name the unsupported schema version", err)
	}

	// A refusal that rewrote anything would be worse than no refusal: the older
	// binary would have touched a workspace it cannot represent.
	after := snapshotWorkspaceFiles(t, dataDir)
	if len(before) != len(after) {
		t.Fatalf("workspace file count changed from %d to %d during the refusal", len(before), len(after))
	}
	for path, content := range before {
		if after[path] != content {
			t.Fatalf("refusal rewrote %q", path)
		}
	}
}

// bumpCatalogSchemaVersion rewrites only the schema marker in workspace.json,
// leaving every other key exactly as written. Decoding into WorkspaceCatalog and
// re-encoding would drop unknown keys, which is precisely what a newer binary's
// catalog has.
func bumpCatalogSchemaVersion(t *testing.T, catalogPath string, version int) {
	t.Helper()
	raw, err := os.ReadFile(catalogPath)
	if err != nil {
		t.Fatalf("read catalog: %v", err)
	}
	var decoded map[string]any
	if decodeErr := json.Unmarshal(raw, &decoded); decodeErr != nil {
		t.Fatalf("decode catalog: %v", decodeErr)
	}
	decoded["schema_version"] = version
	encoded, err := json.MarshalIndent(decoded, "", "  ")
	if err != nil {
		t.Fatalf("encode catalog: %v", err)
	}
	if err := os.WriteFile(catalogPath, append(encoded, '\n'), 0o644); err != nil {
		t.Fatalf("write catalog: %v", err)
	}
}

// snapshotWorkspaceFiles records every regular file under the data directory by
// content, keyed on the path relative to it.
func snapshotWorkspaceFiles(t *testing.T, dataDir string) map[string]string {
	t.Helper()
	files := make(map[string]string)
	err := filepath.Walk(dataDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !info.Mode().IsRegular() {
			return nil
		}
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		rel, relErr := filepath.Rel(dataDir, path)
		if relErr != nil {
			return relErr
		}
		files[rel] = string(content)
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot workspace files: %v", err)
	}
	return files
}

// TestLensAdoptionConvergesAcrossOrders is the N-machine property: any number of
// machines, arbitrary lens sets, arbitrary adoption order, and every order
// reaches the same portable collection — the same records, under the same
// identities, wearing the same names, in the same number.
//
// Nothing is erased before the comparison. An earlier version of this property
// compared definitions with identity and name blanked out, because neither
// converged: the base side of an id collision kept the plain id, so arrival
// order decided who did, and a renamed record no longer deduped against its own
// original, so re-merging grew the collection. Both were characterizations of a
// defect. MergeNamedLenses now resolves identity and name from content alone, so
// the property can compare whole records and does.
//
// Only element order is excluded, and deliberately: a record occupies the slot
// of its earliest contribution, so order is presentation and follows the base
// collection. The records themselves are order-free.
func TestLensAdoptionConvergesAcrossOrders(t *testing.T) {
	for _, seed := range []int64{1, 7, 42, 1337, 90210} {
		machines := randomMachineLensSets(rand.New(rand.NewSource(seed)))
		contributed := definitionKeySet(machines...)

		settled := ""
		settledOrder := []int(nil)
		for _, order := range adoptionOrders(rand.New(rand.NewSource(seed)), len(machines)) {
			adopted := adoptLensSetsInOrder(t, machines, order)

			if err := adopted.Validate(); err != nil {
				t.Fatalf("seed %d order %v: adopted collection is invalid: %v", seed, order, err)
			}
			survived := definitionKeySet(adopted)
			for key := range contributed {
				if !survived[key] {
					t.Fatalf("seed %d order %v: definition %s was lost", seed, order, key)
				}
			}
			for key := range survived {
				if !contributed[key] {
					t.Fatalf("seed %d order %v: definition %s was invented", seed, order, key)
				}
			}

			canonical := canonicalLensRecords(t, adopted)
			if settled == "" {
				settled, settledOrder = canonical, order
				continue
			}
			if canonical != settled {
				t.Fatalf(
					"seed %d: adoption order %v and %v disagree\n order %v: %s\n order %v: %s",
					seed, settledOrder, order, settledOrder, settled, order, canonical,
				)
			}
		}

		// A count assertion that only ever saw one order would be vacuous, so it
		// rides the settled rendering: every order produced this many records.
		if got := len(adoptLensSetsInOrder(t, machines, identityAdoptionOrder(len(machines)))); got == 0 {
			t.Fatalf("seed %d: every machine contributed a lens, so the merge cannot be empty", seed)
		}
	}
}

// canonicalLensRecords renders a collection as its records alone, sorted so two
// collections holding the same lenses in different slots compare equal. Nothing
// is blanked: id, name, mode, scope, query and sort all take part, and the
// length of the rendering carries the cardinality.
func canonicalLensRecords(t *testing.T, list NamedLensList) string {
	t.Helper()
	rendered := make([]string, 0, len(list))
	for _, lens := range list {
		encoded, err := json.Marshal(lens)
		if err != nil {
			t.Fatalf("render lens %q: %v", lens.ID, err)
		}
		rendered = append(rendered, string(encoded))
	}
	sort.Strings(rendered)
	return "[" + strings.Join(rendered, ",") + "]"
}

func identityAdoptionOrder(count int) []int {
	order := make([]int, count)
	for i := range order {
		order[i] = i
	}
	return order
}

// adoptLensSetsInOrder runs the real startup adoption path once per machine, in
// the given order, over one freshly staged workspace, and returns what the
// portable home holds afterwards. Every machine is a distinct home, which is
// what makes it a distinct machine.
func adoptLensSetsInOrder(t *testing.T, machines []NamedLensList, order []int) NamedLensList {
	t.Helper()

	// The founder mints the workspace and is not one of the adopting machines,
	// so removing the file it published rewinds the workspace to the state a
	// real one is in before any machine has adopted.
	dataDir, _ := stageSharedWorkspaceOn(t, t.TempDir())
	if err := os.Remove(LensDefinitionsPath(dataDir)); err != nil {
		t.Fatalf("rewind the portable lens home: %v", err)
	}

	var manager *Manager
	for _, index := range order {
		manager = NewManagerOnMachine(ConfigPathForDataDir(dataDir), t.TempDir())
		stageUnadoptedMachine(t, manager, machines[index].Clone(), "dark", "")
		if _, err := manager.LoadOrCreateDomains(); err != nil {
			t.Fatalf("machine %d adoption: %v", index, err)
		}
	}
	return workspaceLenses(t, manager)
}

// randomMachineLensSets draws between two and five machines, each carrying a
// random subset of a fixed pool. The pool deliberately mixes the collisions a
// real fleet produces: the same template ID edited differently on two machines,
// the same name under two identities, and definitions unique to one machine.
func randomMachineLensSets(random *rand.Rand) []NamedLensList {
	pool := NamedLensList{
		adoptionLens(adoptionLensAlphaID, "Shared", "#alpha"),
		adoptionLens(adoptionLensAlphaID, "Shared", "#alpha-edited"),
		adoptionLens(adoptionLensBetaID, "Shared", "#beta"),
		adoptionLens(adoptionLensBetaID, "Beta", "#beta"),
		adoptionLens(adoptionLensGammaID, "Gamma", "#gamma"),
	}
	pool[4].Query = "*pin"

	count := 2 + random.Intn(4)
	machines := make([]NamedLensList, 0, count)
	for i := 0; i < count; i++ {
		set := NamedLensList{}
		for _, lens := range pool {
			if random.Intn(2) == 0 {
				set = append(set, lens.Clone())
			}
		}
		if len(set) == 0 {
			// A machine with no lenses adopts nothing, which teaches the
			// property nothing; give it one so every draw contributes.
			set = append(set, pool[random.Intn(len(pool))].Clone())
		}
		machines = append(machines, dedupeLensIdentities(set))
	}
	return machines
}

// dedupeLensIdentities drops the second of two records sharing one identity,
// because a single machine's own collection is already held to unique ids and
// names by NamedLensList.Validate. The pool intentionally repeats identities so
// that two machines can collide; one machine carrying both halves is not a state
// a real machine can reach.
func dedupeLensIdentities(set NamedLensList) NamedLensList {
	seenID := make(map[string]bool, len(set))
	seenName := make(map[string]bool, len(set))
	out := NamedLensList{}
	for _, lens := range set {
		name := strings.ToLower(lens.Name)
		if seenID[lens.ID] || seenName[name] {
			continue
		}
		seenID[lens.ID] = true
		seenName[name] = true
		out = append(out, lens)
	}
	return out
}

// adoptionOrders returns the identity order plus several random permutations of
// it, which is the axis the property quantifies over.
func adoptionOrders(random *rand.Rand, count int) [][]int {
	identity := make([]int, count)
	for i := range identity {
		identity[i] = i
	}
	orders := [][]int{identity}
	for attempt := 0; attempt < 3; attempt++ {
		shuffled := append([]int(nil), identity...)
		random.Shuffle(len(shuffled), func(i, j int) {
			shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
		})
		orders = append(orders, shuffled)
	}
	return orders
}

// definitionKeySet reduces collections to the set of definitions they hold, with
// identity and name erased so a derived id or a disambiguated name does not read
// as a different lens.
func definitionKeySet(lists ...NamedLensList) map[string]bool {
	keys := make(map[string]bool)
	for _, list := range lists {
		for _, lens := range list {
			keys[definitionKey(lens)] = true
		}
	}
	return keys
}

func definitionKey(lens NamedLens) string {
	anonymous := lens.Clone()
	anonymous.ID = ""
	anonymous.Name = ""
	encoded, err := json.Marshal(anonymous)
	if err != nil {
		// Marshaling a lens cannot fail, and a key that hid the failure would
		// silently collapse two definitions into one.
		return "unmarshalable:" + lens.ID
	}
	return string(encoded)
}

// TestLensMergeObeysTheJoinLaws checks the three laws directly on the function,
// over the same randomized fleet the adoption property draws from. The property
// above exercises them through the real startup path; this one names them, so a
// failure says which law broke rather than only that two orders disagreed.
func TestLensMergeObeysTheJoinLaws(t *testing.T) {
	for _, seed := range []int64{1, 7, 42, 1337, 90210} {
		machines := randomMachineLensSets(rand.New(rand.NewSource(seed)))
		for i, left := range machines {
			for j, right := range machines {
				forward := MergeNamedLenses(left, right)
				reverse := MergeNamedLenses(right, left)
				if a, b := canonicalLensRecords(t, forward), canonicalLensRecords(t, reverse); a != b {
					t.Fatalf("seed %d: merge(%d,%d) is not commutative\n forward %s\n reverse %s", seed, i, j, a, b)
				}
				if again := MergeNamedLenses(forward, right); canonicalLensRecords(t, again) != canonicalLensRecords(t, forward) {
					t.Fatalf(
						"seed %d: merging %d into merge(%d,%d) again changed it\n once %s\ntwice %s",
						seed, j, i, j, canonicalLensRecords(t, forward), canonicalLensRecords(t, again),
					)
				}
				if err := forward.Validate(); err != nil {
					t.Fatalf("seed %d: merge(%d,%d) is invalid: %v", seed, i, j, err)
				}
				for k, third := range machines {
					leftFirst := MergeNamedLenses(MergeNamedLenses(left, right), third)
					rightFirst := MergeNamedLenses(left, MergeNamedLenses(right, third))
					if a, b := canonicalLensRecords(t, leftFirst), canonicalLensRecords(t, rightFirst); a != b {
						t.Fatalf(
							"seed %d: merge over (%d,%d,%d) is not associative\n left-first %s\nright-first %s",
							seed, i, j, k, a, b,
						)
					}
				}
			}
		}
	}
}

// TestLensAdoptionIDsAreOrderIndependent is the narrow witness the convergence
// property rests on: one id carrying two definitions resolves the same way from
// either side, because the definition that keeps the plain id is chosen by
// content rather than by which side happened to be the base. This test replaces
// a characterization of the opposite behaviour.
func TestLensAdoptionIDsAreOrderIndependent(t *testing.T) {
	left := NamedLensList{adoptionLens(adoptionLensAlphaID, "Shared", "#a")}
	right := NamedLensList{adoptionLens(adoptionLensAlphaID, "Shared", "#b")}

	forward := MergeNamedLenses(left, right)
	reverse := MergeNamedLenses(right, left)

	if len(forward) != 2 || len(reverse) != 2 {
		t.Fatalf("both orders should keep both definitions, got %d and %d records", len(forward), len(reverse))
	}
	if a, b := canonicalLensRecords(t, forward), canonicalLensRecords(t, reverse); a != b {
		t.Fatalf("id resolution still depends on arrival order\n forward %s\n reverse %s", a, b)
	}
	// The content rule is stated, not merely order-free: the definition whose
	// core sorts first keeps the identity both sides contributed it under, and
	// the other one is derived. "#a" sorts before "#b", so the "#a" record keeps
	// it whichever side carried it.
	byQuery := map[string]NamedLens{}
	for _, lens := range forward {
		byQuery[lens.Query] = lens
	}
	if byQuery["#a"].ID != adoptionLensAlphaID {
		t.Fatalf("the first-sorting definition should keep the contributed id, got %q", byQuery["#a"].ID)
	}
	if byQuery["#b"].ID == adoptionLensAlphaID || byQuery["#b"].ID == "" {
		t.Fatalf("the second definition needs a derived identity of its own, got %q", byQuery["#b"].ID)
	}
	if err := ValidateNamedLensID(byQuery["#b"].ID); err != nil {
		t.Fatalf("a derived identity must stay canonical: %v", err)
	}
}

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
