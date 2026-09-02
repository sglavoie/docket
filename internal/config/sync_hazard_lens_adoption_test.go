package config

import (
	"math/rand"
	"os"
	"strings"
	"testing"
)

// This file covers the lens set arriving from an arbitrary number of machines in
// an arbitrary order: the convergence property, the join laws underneath it, and
// the narrow order-independence witness both rest on. The shared renderings live
// in sync_hazard_test.go, the adversarial identity bound in
// sync_hazard_lens_identity_test.go, and the stale-write path in
// sync_hazard_lens_reconcile_test.go.

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

// TestLensMergeObeysTheJoinLaws checks the three laws directly on the function,
// over the same randomized fleet the adoption property draws from. The property
// above exercises them through the real startup path; this one names them, so a
// failure says which law broke rather than only that two orders disagreed.
func TestLensMergeObeysTheJoinLaws(t *testing.T) {
	for _, seed := range []int64{1, 7, 42, 1337, 90210} {
		machines := randomMachineLensSets(rand.New(rand.NewSource(seed)))
		for i, left := range machines {
			for j, right := range machines {
				forward := mergedLenses(t, left, right)
				reverse := mergedLenses(t, right, left)
				if a, b := canonicalLensRecords(t, forward), canonicalLensRecords(t, reverse); a != b {
					t.Fatalf("seed %d: merge(%d,%d) is not commutative\n forward %s\n reverse %s", seed, i, j, a, b)
				}
				if again := mergedLenses(t, forward, right); canonicalLensRecords(t, again) != canonicalLensRecords(t, forward) {
					t.Fatalf(
						"seed %d: merging %d into merge(%d,%d) again changed it\n once %s\ntwice %s",
						seed, j, i, j, canonicalLensRecords(t, forward), canonicalLensRecords(t, again),
					)
				}
				if err := forward.Validate(); err != nil {
					t.Fatalf("seed %d: merge(%d,%d) is invalid: %v", seed, i, j, err)
				}
				for k, third := range machines {
					leftFirst := mergedLenses(t, mergedLenses(t, left, right), third)
					rightFirst := mergedLenses(t, left, mergedLenses(t, right, third))
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
// a characterization of the opposite behavior.
func TestLensAdoptionIDsAreOrderIndependent(t *testing.T) {
	left := NamedLensList{adoptionLens(adoptionLensAlphaID, "Shared", "#a")}
	right := NamedLensList{adoptionLens(adoptionLensAlphaID, "Shared", "#b")}

	forward := mergedLenses(t, left, right)
	reverse := mergedLenses(t, right, left)

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
