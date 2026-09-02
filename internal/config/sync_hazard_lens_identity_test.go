package config

import (
	"errors"
	"fmt"
	"testing"
)

// This file covers the adversarial end of the merge: a collection that occupies
// every identity a contribution could settle on, and the bounded sequence the
// merge walks before it reports exhaustion. The convergence property those laws
// serve is in sync_hazard_lens_adoption_test.go.

// lensIdentityStarvationFixture builds the adversarial shape both identity
// exhaustion tests are written against: one contribution — the victim — and one
// occupier per identity that contribution may settle on, each occupier carrying
// a different core so none of its claims can be shared.
//
// The victim's id is near-maximal, so every derived candidate sorts before it
// and each occupier is therefore resolved — and its claim recorded — ahead of
// the victim; the occupier cores sort ahead of the victim's for the same reason.
// A merge the occupiers do not reach first is not adversarial at all, so the
// ordering is asserted rather than assumed.
//
// The returned occupiers are index-aligned with the returned candidates, which
// is what lets a caller free exactly one identity and name which one it freed.
func lensIdentityStarvationFixture(t *testing.T) (NamedLens, []string, NamedLensList) {
	t.Helper()
	const collidingID = "ffffffff-ffff-4fff-8fff-ffffffffffff"
	victim := adoptionLens(collidingID, "Victim", "#zzz-victim")
	candidates := lensIdentityCandidates(collidingID, lensCoreKey(victim))
	if len(candidates) != lensIdentityCandidateCount {
		t.Fatalf("candidate count = %d, want %d", len(candidates), lensIdentityCandidateCount)
	}
	occupiers := make(NamedLensList, 0, len(candidates))
	for i, id := range candidates {
		if i > 0 && id >= collidingID {
			t.Fatalf("candidate %d (%q) does not sort before %q, so it is claimed too late to starve the victim", i, id, collidingID)
		}
		occupiers = append(occupiers, adoptionLens(id, fmt.Sprintf("Occupier %d", i), fmt.Sprintf("#aaa-%02d", i)))
	}
	// Neither side is malformed on its own: the hazard is the join, so a refusal
	// cannot be blamed on an invalid contribution.
	if err := occupiers.Validate(); err != nil {
		t.Fatalf("the occupying collection must be individually valid: %v", err)
	}
	if err := (NamedLensList{victim}).Validate(); err != nil {
		t.Fatalf("the contributed collection must be individually valid: %v", err)
	}
	return victim, candidates, occupiers
}

// lensListWithout returns the collection minus the record at index, which is how
// these tests free exactly one of the victim's candidate identities.
func lensListWithout(list NamedLensList, index int) NamedLensList {
	freed := make(NamedLensList, 0, len(list)-1)
	freed = append(freed, list[:index]...)
	return append(freed, list[index+1:]...)
}

// assertLensMergeRefusesBothOrders checks the merge's one refusal from both
// arrival sides: ErrLensIdentityExhausted and no collection at all, since a
// partial publication is the loss the refusal exists to prevent.
func assertLensMergeRefusesBothOrders(t *testing.T, occupiers, contributed NamedLensList) {
	t.Helper()
	for _, order := range []struct {
		name           string
		base, incoming NamedLensList
	}{
		{"occupiers first", occupiers, contributed},
		{"contribution first", contributed, occupiers},
	} {
		merged, err := MergeNamedLenses(order.base, order.incoming)
		if !errors.Is(err, ErrLensIdentityExhausted) {
			t.Fatalf("%s: merge error = %v, want ErrLensIdentityExhausted", order.name, err)
		}
		if merged != nil {
			t.Fatalf("%s: a refused merge must publish nothing, got %d records", order.name, len(merged))
		}
	}
}

// assertLensMergeSettlesVictimOn checks the other half: with one identity free
// the merge identifies every contribution, invents nothing, loses nothing, and
// settles the victim on that identity from either arrival side.
func assertLensMergeSettlesVictimOn(t *testing.T, occupiers, contributed NamedLensList, victim NamedLens, want string) {
	t.Helper()
	for _, order := range []struct {
		name           string
		base, incoming NamedLensList
	}{
		{"occupiers first", occupiers, contributed},
		{"contribution first", contributed, occupiers},
	} {
		merged, err := MergeNamedLenses(order.base, order.incoming)
		if err != nil {
			t.Fatalf("%s: one free candidate is enough to identify every contribution: %v", order.name, err)
		}
		if err := merged.Validate(); err != nil {
			t.Fatalf("%s: the merged collection is invalid: %v", order.name, err)
		}
		if wantCount := len(occupiers) + len(contributed); len(merged) != wantCount {
			t.Fatalf("%s: merged record count = %d, want %d", order.name, len(merged), wantCount)
		}
		wanted := definitionKeySet(occupiers, contributed)
		got := definitionKeySet(merged)
		for key := range wanted {
			if !got[key] {
				t.Fatalf("%s: definition %s was lost by the merge", order.name, key)
			}
		}
		for key := range got {
			if !wanted[key] {
				t.Fatalf("%s: definition %s was invented by the merge", order.name, key)
			}
		}
		settled := ""
		for _, lens := range merged {
			if lens.Query == victim.Query {
				settled = lens.ID
			}
		}
		if settled != want {
			t.Fatalf("%s: the contribution settled on %q, want %q", order.name, settled, want)
		}
	}
}

// TestLensMergeIdentityExhaustionNeverDropsADefinition is the adversarial half
// of the union contract: a collection nobody would write by accident, but which
// each side is individually allowed to hold, must not be able to make the merge
// lose a definition.
//
// The set preoccupies every identity one contribution can settle on — the id it
// carries plus every salted derivation. Handing the victim one of those anyway
// would put two different cores under one resolved id, and collapseLensRecords
// would then fold them into a single record: a silent deletion, unrecoverable
// because the lens domain records no undo entry. The merge must refuse whole
// instead, from either arrival side.
//
// Freeing one identity in the middle of the sequence is the complementary
// witness: the scan takes the first free candidate rather than a fixed slot, so
// the bound is a sequence the merge walks and not a number it counts.
func TestLensMergeIdentityExhaustionNeverDropsADefinition(t *testing.T) {
	victim, candidates, occupiers := lensIdentityStarvationFixture(t)
	contributed := NamedLensList{victim}

	assertLensMergeRefusesBothOrders(t, occupiers, contributed)

	middle := len(candidates) / 2
	if middle == 0 || middle == len(candidates)-1 {
		t.Fatalf("the freed candidate %d must be neither the contributed id nor the final derivation", middle)
	}
	assertLensMergeSettlesVictimOn(t, lensListWithout(occupiers, middle), contributed, victim, candidates[middle])
}

// TestLensMergeChecksFinalBoundedDerivationBeforeExhaustion pins the far end of
// that sequence, which is the end an off-by-one silently trims.
//
// The final bounded derivation — salt lensIdentitySaltAttempts — is a candidate
// like every earlier one: it is checked before it is claimed, so it can never
// collapse two cores into one record, and it is offered before exhaustion is
// reported, so a contribution whose only free identity is that derivation is
// identified rather than refused. Occupying it as well is what genuinely
// exhausts the bound, and only then does the merge refuse without publishing.
func TestLensMergeChecksFinalBoundedDerivationBeforeExhaustion(t *testing.T) {
	victim, candidates, occupiers := lensIdentityStarvationFixture(t)
	contributed := NamedLensList{victim}

	last := len(candidates) - 1
	finalDerivation := disambiguatedLensID(victim.ID, lensCoreKey(victim), lensIdentitySaltAttempts)
	if candidates[last] != finalDerivation {
		t.Fatalf("the last candidate is %q, want the salt-%d derivation %q", candidates[last], lensIdentitySaltAttempts, finalDerivation)
	}

	// The contributed id and every salt below the last are occupied, so the
	// final derivation is the victim's one remaining identity. Taking it proves
	// that derivation is checked and claimed rather than treated as outside the
	// sequence.
	assertLensMergeSettlesVictimOn(t, lensListWithout(occupiers, last), contributed, victim, finalDerivation)

	// Occupying it too leaves nothing free, which is the only state that may
	// report exhaustion — and it still publishes no collection.
	assertLensMergeRefusesBothOrders(t, occupiers, contributed)

	// RemapActiveLensID walks the same sequence, so a pointer at the victim
	// follows it onto the final derivation rather than stopping short of it.
	merged, err := MergeNamedLenses(lensListWithout(occupiers, last), contributed)
	if err != nil {
		t.Fatalf("the merge with a free final derivation must succeed: %v", err)
	}
	if moved := RemapActiveLensID(victim.ID, contributed, merged); moved != finalDerivation {
		t.Fatalf("the active pointer followed to %q, want the final derivation %q", moved, finalDerivation)
	}
}
