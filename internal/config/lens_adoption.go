package config

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
)

// Lens adoption is the one-time-per-machine step that moves a machine's lens set
// into the workspace's portable home.
//
// It cannot live in the schema migration. The migration chain is guarded by the
// catalog schema, which is the synced artifact: the first machine to reach v16
// advances that marker for every machine sharing the data directory, so a second
// machine's migration is a no-op and its machine-local lenses would be lost
// without ever being read. Adoption is therefore keyed off machine-local state
// and runs after the schema is already current.
//
// Nothing here deletes. A lens present on one side only survives, and one id
// carrying two different definitions yields both records rather than a winner,
// because the two homes carry no ordering between them: machine.json has no
// edit clock, so "newer" is not a question the data can answer.

// lensAdoptionNamespace is the frozen UUID namespace a disambiguated lens
// identity is derived under. Deriving rather than minting is what makes the
// merge idempotent: re-merging the same colliding definition computes the same
// identity, finds it already present, and adopts nothing a second time.
var lensAdoptionNamespace = uuid.MustParse("9c8f2b17-5d3e-4a6b-8c21-7f0e4d9b3a55")

// MergeNamedLenses merges an incoming collection into a base one under the
// adoption rules: dedupe by lens id, keep both sides when one id carries two
// differing definitions, and never drop a lens that exists on only one side.
// Base order is preserved and incoming records append in their own order, so the
// result is a pure function of its inputs.
//
// Name uniqueness is a collection rule (NamedLensList.Validate), so an incoming
// record whose name is already taken is renamed on the way in, whether or not
// its identity collided. The result satisfies Validate whenever both inputs do.
func MergeNamedLenses(base, incoming NamedLensList) NamedLensList {
	merged := base.Clone()
	if merged == nil {
		merged = NamedLensList{}
	}
	positions := make(map[string]int, len(merged)+len(incoming))
	names := make(map[string]bool, len(merged)+len(incoming))
	for i, lens := range merged {
		positions[lens.ID] = i
		names[strings.ToLower(lens.Name)] = true
	}
	for _, lens := range incoming {
		candidate := lens.Clone()
		if at, collides := positions[candidate.ID]; collides {
			if namedLensesIdentical(merged[at], candidate) {
				continue
			}
			candidate.ID = disambiguatedLensID(candidate)
			if _, adopted := positions[candidate.ID]; adopted {
				// A previous adoption already split this exact definition out.
				continue
			}
		}
		candidate.Name = unusedLensName(candidate.Name, names)
		names[strings.ToLower(candidate.Name)] = true
		positions[candidate.ID] = len(merged)
		merged = append(merged, candidate)
	}
	return merged
}

// namedLensesIdentical reports whether two definitions are the same record. It
// compares the persisted rendering rather than field by field so a lens axis
// added later cannot silently drop out of the comparison and turn a genuine
// difference into a dedupe.
func namedLensesIdentical(a, b NamedLens) bool {
	left, leftErr := json.Marshal(a)
	right, rightErr := json.Marshal(b)
	if leftErr != nil || rightErr != nil {
		return false
	}
	return string(left) == string(right)
}

// disambiguatedLensID derives the identity the incoming half of an id collision
// is kept under. It is a UUIDv5 over the definition's own persisted bytes, so it
// is canonical (ValidateNamedLensID accepts it), stable across runs and
// machines, and distinct for two definitions that differ in any axis.
func disambiguatedLensID(lens NamedLens) string {
	seed, err := json.Marshal(lens)
	if err != nil {
		seed = []byte(lens.ID + "\x00" + lens.Name)
	}
	return uuid.NewSHA1(lensAdoptionNamespace, seed).String()
}

// unusedLensName returns name, or the first " (N)" variant of it that is not
// already taken. The base is truncated when the suffix would push it past the
// durable rune boundary, so a disambiguated name still validates.
func unusedLensName(name string, taken map[string]bool) string {
	if !taken[strings.ToLower(name)] {
		return name
	}
	for n := 2; ; n++ {
		suffix := fmt.Sprintf(" (%d)", n)
		candidate := truncateLensNameRunes(name, MaxNamedLensNameRunes-utf8.RuneCountInString(suffix)) + suffix
		if !taken[strings.ToLower(candidate)] {
			return candidate
		}
	}
}

// truncateLensNameRunes cuts a name to at most limit runes. A trimmed name can
// only lose a trailing rune, never gain leading whitespace, so the result is
// still trimmed and still non-empty for any positive limit.
func truncateLensNameRunes(name string, limit int) string {
	if limit <= 0 || utf8.RuneCountInString(name) <= limit {
		return name
	}
	runes := []rune(name)
	return strings.TrimRight(string(runes[:limit]), " ")
}

// AdoptPortableLenses runs the one-time-per-machine merge of this machine's lens
// set into the workspace's portable home, and is a no-op once this machine's
// marker is set. It is the step that makes the schema-16 marker true: until a
// machine adopts, nothing in production creates lenses.json.
//
// Adoption is refused before schema 17 and on an uninitialized workspace, so a
// workspace still on an older catalog cannot acquire a lens home ahead of the
// boundary that announces one. The floor is 17 rather than 16 because a lens
// definition is canonical only from 17 on: adopting at 16 would merge this
// machine's legacy-shaped records into a portable file the v17 migration has
// already planned, and the merge would land either behind that plan's revision
// check or ahead of the rewrite that makes the records canonical.
//
// The merge is committed through CommitMachine alone: setting the marker before
// the commit is what makes publishOwnedLensDefinitions publish the portable
// file, so one call writes both homes, and a failure to publish leaves the
// marker unwritten and the whole step to be retried on the next launch.
func (m *Manager) AdoptPortableLenses() error {
	store := m.domainStore()
	catalog, catalogRev, err := store.LoadCatalogChecked()
	if err != nil {
		return err
	}
	if catalogRev == "" || catalog == nil || catalog.SchemaVersion < V17ConfigSchemaVersion {
		return nil
	}
	machine, _, err := store.LoadMachineChecked()
	if err != nil {
		return err
	}
	if machine.LensHomeAdopted {
		return nil
	}
	workspaceLenses, _, err := store.LoadLensDefinitionsChecked()
	if err != nil {
		return err
	}
	adoptable, err := store.LoadPersistedMachineLenses()
	if err != nil {
		return err
	}
	if adoptable == nil && !workspaceLenses.Defined() {
		// Neither home holds a record, so this machine founds the portable one
		// and the workspace inherits the set the machine already shows —
		// including the starters a machine that never wrote a lens list still
		// displays. Once the workspace does hold a record it is authoritative
		// about the starters, and an omitted key contributes nothing, so a
		// machine joining later cannot reseed lenses their owner deleted.
		adoptable = machine.NamedLenses
	}
	merged := MergeNamedLenses(workspaceLenses.Lenses, adoptable)
	if err := merged.Validate(); err != nil {
		return fmt.Errorf("lens adoption produced an invalid collection: %w", err)
	}
	machine.NamedLenses = merged
	machine.LensHomeAdopted = true
	return m.DomainCoordinator().CommitMachine(machine)
}
