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

// TestLensAdoptionOrderNeverLosesOrInventsADefinition is the N-machine property:
// any number of machines, arbitrary lens sets, arbitrary adoption order, and the
// set of surviving lens definitions is the union of what the machines carried —
// no definition lost, none invented, and the collection valid in every order.
//
// Definitions are compared with identity and name erased on purpose. Neither is
// order-stable and neither can be asserted here:
//
//   - The ID multiset does not converge. On an ID collision carrying two
//     different definitions the base record keeps the plain ID and the incoming
//     one is derived, so arrival order decides which side keeps it. Starter and
//     built-in template lenses carry fixed IDs on every machine, so this is the
//     ordinary case rather than an exotic one.
//   - Record cardinality does not converge either, because renaming an incoming
//     record changes the record, so a later identical arrival no longer dedupes
//     against it and is kept as a third copy instead.
//
// Both are recorded as known facts about MergeNamedLenses rather than asserted
// loosely, since a weakened assertion would pass today and hide a regression.
func TestLensAdoptionOrderNeverLosesOrInventsADefinition(t *testing.T) {
	for _, seed := range []int64{1, 7, 42, 1337, 90210} {
		machines := randomMachineLensSets(rand.New(rand.NewSource(seed)))
		contributed := definitionKeySet(machines...)

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
		}
	}
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

// TestLensMergeRenamePathIsNotIdempotent pins the second half of the divergence
// the property test declines to assert. It is a characterization test, not an
// endorsement: the doc comment on CommitLensDefinitions calls MergeNamedLenses
// an idempotent join, and it is one on the id-collision path, but a record
// renamed on the way in no longer dedupes against its own original, so merging
// the same incoming collection twice grows the result.
func TestLensMergeRenamePathIsNotIdempotent(t *testing.T) {
	base := NamedLensList{adoptionLens(adoptionLensAlphaID, "Shared", "#query")}
	incoming := NamedLensList{adoptionLens(adoptionLensBetaID, "Shared", "#query")}

	once := MergeNamedLenses(base, incoming)
	if len(once) != 2 {
		t.Fatalf("first merge = %v, want both records kept under disambiguated names", lensNames(once))
	}
	twice := MergeNamedLenses(once, incoming)
	if len(twice) != 3 {
		t.Fatalf("second merge = %v, want the known duplicate; a fix here should update this test and the CommitLensDefinitions comment", lensNames(twice))
	}
	// It does settle: the third pass collides on the derived identity and dedupes.
	thrice := MergeNamedLenses(twice, incoming)
	if len(thrice) != len(twice) {
		t.Fatalf("third merge = %v, want no further growth", lensNames(thrice))
	}

	// The id-collision path really is idempotent, which is why the divergence is
	// worth naming narrowly rather than calling the whole join unstable.
	edited := NamedLensList{adoptionLens(adoptionLensAlphaID, "Shared", "#edited")}
	collided := MergeNamedLenses(base, edited)
	if remerged := MergeNamedLenses(collided, edited); len(remerged) != len(collided) {
		t.Fatalf("id-collision merge grew from %d to %d records", len(collided), len(remerged))
	}
}

// TestLensAdoptionIDMultisetDependsOnOrder is the witness behind the property
// test's comment, kept as a test so the fact is checked rather than remembered.
// If a content tie-break ever lands in MergeNamedLenses this test is the one
// that should start failing.
func TestLensAdoptionIDMultisetDependsOnOrder(t *testing.T) {
	left := NamedLensList{adoptionLens(adoptionLensAlphaID, "Shared", "#a")}
	right := NamedLensList{adoptionLens(adoptionLensAlphaID, "Shared", "#b")}

	forward := sortedLensIDs(MergeNamedLenses(left, right))
	reverse := sortedLensIDs(MergeNamedLenses(right, left))

	if len(forward) != 2 || len(reverse) != 2 {
		t.Fatalf("both orders should keep both definitions, got %d and %d records", len(forward), len(reverse))
	}
	if strings.Join(forward, ",") == strings.Join(reverse, ",") {
		t.Fatalf("id multisets agree (%v); the merge gained an order-independent identity rule and the property test can now assert it", forward)
	}
	// Whichever order ran, the definition that arrived as the base keeps the
	// plain identity — that is the rule, and it is what makes it order-bound.
	if forward[0] != adoptionLensAlphaID && forward[1] != adoptionLensAlphaID {
		t.Fatalf("forward ids = %v, want the base record to retain the original identity", forward)
	}
}

func sortedLensIDs(list NamedLensList) []string {
	ids := make([]string, 0, len(list))
	for _, lens := range list {
		ids = append(ids, lens.ID)
	}
	sort.Strings(ids)
	return ids
}
