package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
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

// lensIdentitySaltAttempts is how many salted derivations back the plain
// contributed id. Exhausting the bound requires as many distinct UUIDv5 values
// over distinct seeds to land on identities already taken by other content,
// which is not reachable by accident; the bound exists so a hand-written
// lenses.json cannot spin the merge forever.
//
// Every candidate is checked before it is claimed, the final derivation
// included, and the bound is spent rather than shortened: salt
// lensIdentitySaltAttempts is offered like every salt before it. Claiming an
// unchecked candidate would hand a contribution an identity another core already
// holds, and collapsing then folds two different definitions into one record — a
// silent deletion the union contract forbids; withholding a checkable one would
// refuse a merge an available identity could have settled. A set that occupies
// every candidate is refused whole with ErrLensIdentityExhausted instead.
const lensIdentitySaltAttempts = 8

// lensIdentityCandidateCount is how many identities one contribution may settle
// on in total: the contributed id plus one derivation per salt.
const lensIdentityCandidateCount = lensIdentitySaltAttempts + 1

// ErrLensIdentityExhausted reports that one contributed id carried more distinct
// definitions than the bounded derivation can give identities to. It is the
// merge's one refusal, and it refuses the whole collection: publishing a merge
// that dropped a definition would be worse than not merging at all, since the
// lens domain records no undo entry and the dropped definition is unrecoverable.
var ErrLensIdentityExhausted = errors.New("lens identities are exhausted")

// MergeNamedLenses joins two lens collections into the one every machine
// converges on, whatever order the collections arrive in.
//
// The join is a set operation, not an arrival-ordered fold: every record on
// both sides is a contribution, and the result is computed from the union of
// those contributions alone. That makes it commutative, associative and
// idempotent over the records it yields — their identities, their names, their
// definitions and their count — which is what a synced workspace needs, since
// two machines adopting in opposite orders must reach the same file and a stale
// write reconciled twice must not grow.
//
// Three rules decide the union:
//
//   - Identity is the id a record was contributed under together with its core
//     (mode, scope, query and sort). Two contributions agreeing on both are the
//     same lens wearing two labels; the label is reconciled, not duplicated.
//   - When one id carries several cores, the core that sorts first keeps that
//     id and every other core is re-identified with a UUIDv5 derived from the
//     contributed id and that core. The derivation reads neither the name nor
//     the arrival side, so a record already split out re-derives to the identity
//     it is already stored under and dedupes instead of splitting again.
//   - Names are then assigned over the resolved identities, records that kept a
//     contributed identity first and canonical id order within that, each taking
//     its preferred label when free and the first free " (N)" variant of that
//     label's base otherwise. NamedLensList.Validate enforces case-folded name
//     uniqueness, so the collection has to resolve it.
//
// Element order is the one axis that still follows arrival: a record occupies
// the slot of its earliest contribution, so the base collection's order leads
// and a machine's own lens order survives a merge. Order carries no meaning
// beyond presentation, and because the workspace file is always the base, the
// records it already holds keep their places on every machine.
//
// The merge has exactly one refusal, and it yields no collection at all: when a
// contributed id carries more distinct cores than the bounded derivation has
// identities for, it returns ErrLensIdentityExhausted rather than a collection
// one definition short. The refusal is a function of the contributions like
// everything else here, so both arrival orders refuse alike.
func MergeNamedLenses(base, incoming NamedLensList) (NamedLensList, error) {
	contributed := contributedLensRecords(base, incoming)
	if err := resolveLensIdentities(contributed); err != nil {
		return nil, err
	}
	resolved := collapseLensRecords(contributed)
	assignLensNames(resolved)
	sort.SliceStable(resolved, func(i, j int) bool { return resolved[i].order < resolved[j].order })
	merged := make(NamedLensList, 0, len(resolved))
	for _, record := range resolved {
		lens := record.lens
		lens.ID = record.id
		lens.Name = record.name
		merged = append(merged, lens)
	}
	return merged, nil
}

// contributedLensRecord is one definition as it was contributed: the identity it
// arrived under, its core, and every label the contributing collections gave
// that pair.
type contributedLensRecord struct {
	originID string
	core     string
	lens     NamedLens
	names    []string
	order    int
	id       string
}

// resolvedLensRecord is one lens of the merged collection: a settled identity,
// the labels contributed for it, and the labels contributed under that very
// identity, which are the ones that win when the two disagree.
type resolvedLensRecord struct {
	id    string
	core  string
	lens  NamedLens
	names []string
	own   []string
	order int
	name  string
}

// contributedLensRecords folds both sides into one contribution per (id, core)
// pair, collecting the labels rather than choosing between them. Position is
// recorded so the merged collection can keep the earliest contributor's slot.
func contributedLensRecords(sides ...NamedLensList) []*contributedLensRecord {
	records := make([]*contributedLensRecord, 0)
	byPair := make(map[string]*contributedLensRecord)
	position := 0
	for _, side := range sides {
		for _, lens := range side {
			core := lensCoreKey(lens)
			pair := lens.ID + "\x00" + core
			record, seen := byPair[pair]
			if !seen {
				record = &contributedLensRecord{
					originID: lens.ID,
					core:     core,
					lens:     lens.Clone(),
					order:    position,
				}
				byPair[pair] = record
				records = append(records, record)
			}
			record.names = append(record.names, lens.Name)
			position++
		}
	}
	return records
}

// resolveLensIdentities settles the identity of every contribution. It walks the
// contributions in canonical order — by contributed id, then by core — so the
// walk is a function of the content rather than of the arrival side: within one
// contributed id the first core keeps it and each later core derives its own.
// A derived identity is allowed to land on a contribution that already carries
// it, which is exactly how a record split out by an earlier merge is recognized
// rather than split a second time.
//
// An identity is claimed only after it is known free, so two different cores can
// never settle on one identity and collapsing can never fold them together. A
// contribution whose every candidate is held by another core exhausts the bound
// and refuses the merge.
func resolveLensIdentities(records []*contributedLensRecord) error {
	ordered := append([]*contributedLensRecord(nil), records...)
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].originID != ordered[j].originID {
			return ordered[i].originID < ordered[j].originID
		}
		return ordered[i].core < ordered[j].core
	})
	claimed := make(map[string]string, len(ordered))
	for _, record := range ordered {
		candidate, free := freeLensIdentity(record, claimed)
		if !free {
			return fmt.Errorf(
				"%w: lens id %q carries more distinct definitions than %d candidate identities can hold",
				ErrLensIdentityExhausted, record.originID, lensIdentityCandidateCount,
			)
		}
		claimed[candidate] = record.core
		record.id = candidate
	}
	return nil
}

// freeLensIdentity returns the first candidate identity nothing else holds, or
// the first one this very core already holds, which is the same answer: a
// contribution recognizing the identity an earlier merge settled it under.
func freeLensIdentity(record *contributedLensRecord, claimed map[string]string) (string, bool) {
	for _, candidate := range lensIdentityCandidates(record.originID, record.core) {
		if held, taken := claimed[candidate]; !taken || held == record.core {
			return candidate, true
		}
	}
	return "", false
}

// lensIdentityCandidates lists, in order, every identity a contribution may
// settle on: the id it was contributed under and one salted derivation for each
// salt through lensIdentitySaltAttempts. The sequence is complete on both ends —
// the final derivation is a candidate like every earlier one, never a value the
// merge claims without asking and never one it withholds. The merge and
// RemapActiveLensID walk this one sequence, so a machine's pointer is followed
// over exactly the identities the merge can mint — never one it cannot, and
// never short of one it can.
func lensIdentityCandidates(originID, core string) []string {
	candidates := make([]string, 0, lensIdentityCandidateCount)
	candidates = append(candidates, originID)
	for salt := 1; salt <= lensIdentitySaltAttempts; salt++ {
		candidates = append(candidates, disambiguatedLensID(originID, core, salt))
	}
	return candidates
}

// collapseLensRecords turns settled contributions into one record per identity,
// keeping the earliest slot and remembering which labels arrived under that
// identity. Contributions can only share an identity when they share a core, so
// the definition itself is never in question here.
func collapseLensRecords(records []*contributedLensRecord) []*resolvedLensRecord {
	byID := make(map[string]*resolvedLensRecord, len(records))
	resolved := make([]*resolvedLensRecord, 0, len(records))
	for _, record := range records {
		existing, seen := byID[record.id]
		if !seen {
			existing = &resolvedLensRecord{
				id:    record.id,
				core:  record.core,
				lens:  record.lens,
				order: record.order,
			}
			byID[record.id] = existing
			resolved = append(resolved, existing)
		} else if record.order < existing.order {
			existing.order = record.order
			existing.lens = record.lens
		}
		existing.names = append(existing.names, record.names...)
		if record.originID == record.id {
			existing.own = append(existing.own, record.names...)
		}
	}
	return resolved
}

// assignLensNames gives every resolved record a label, in an order derived from
// the records themselves: a record that kept an identity it was contributed
// under is labelled before one that had to derive a new identity, and canonical
// id order decides within each half. A record takes its preferred label when
// nothing holds it and otherwise the first free " (N)" variant of that label's
// base, so a record already carrying a disambiguated label settles back onto it
// instead of accumulating suffixes.
func assignLensNames(records []*resolvedLensRecord) {
	ordered := append([]*resolvedLensRecord(nil), records...)
	sort.Slice(ordered, func(i, j int) bool {
		leftDerived, rightDerived := len(ordered[i].own) == 0, len(ordered[j].own) == 0
		if leftDerived != rightDerived {
			return rightDerived
		}
		return ordered[i].id < ordered[j].id
	})
	taken := make(map[string]bool, len(ordered))
	for _, record := range ordered {
		preferred := preferredLensName(record)
		if folded := strings.ToLower(preferred); !taken[folded] {
			record.name = preferred
			taken[folded] = true
			continue
		}
		record.name = unusedLensName(baseLensName(preferred), taken)
		taken[strings.ToLower(record.name)] = true
	}
}

// preferredLensName is the label a resolved record asks for. Labels contributed
// under the record's own identity win over labels a re-identified contribution
// carried, which is what keeps a settled collection settled: a machine still
// holding the pre-merge spelling of a record it has since re-identified no
// longer drags that spelling back. Case-folded order decides between equals, so
// the choice is a function of the label set and not of arrival.
func preferredLensName(record *resolvedLensRecord) string {
	candidates := record.own
	if len(candidates) == 0 {
		candidates = record.names
	}
	best := candidates[0]
	for _, name := range candidates[1:] {
		if lensNameLess(name, best) {
			best = name
		}
	}
	return best
}

func lensNameLess(a, b string) bool {
	foldedA, foldedB := strings.ToLower(a), strings.ToLower(b)
	if foldedA != foldedB {
		return foldedA < foldedB
	}
	return a < b
}

// lensCoreKey renders what a lens shows and nothing about how it is labelled or
// identified: the mode, scope, query and sort axes. It compares the persisted
// rendering rather than field by field so a lens axis added later joins the
// comparison automatically instead of silently dropping out of it and turning a
// genuine difference into a dedupe.
func lensCoreKey(lens NamedLens) string {
	core := lens
	core.ID = ""
	core.Name = ""
	encoded, err := json.Marshal(core)
	if err != nil {
		// Marshaling a lens cannot fail; a key that hid the failure would
		// collapse two different definitions into one record.
		return "unmarshalable\x00" + core.Mode + "\x00" + core.Scope + "\x00" + core.Query
	}
	return string(encoded)
}

// disambiguatedLensID derives the identity a definition is kept under when the
// id it was contributed under already belongs to a different definition. It is a
// UUIDv5 over the contributed id and the core alone, so it is canonical
// (ValidateNamedLensID accepts it), stable across runs and machines, and — since
// it reads neither the label nor which side the record arrived on — identical
// whichever machine computes it and however many times it is recomputed.
func disambiguatedLensID(collidingID, core string, salt int) string {
	seed := collidingID + "\x00" + core + "\x00" + strconv.Itoa(salt)
	return uuid.NewSHA1(lensAdoptionNamespace, []byte(seed)).String()
}

// baseLensName strips a trailing " (N)" disambiguation suffix so a record that
// already carries one asks for the same series again rather than nesting a
// second suffix inside the first. A name that does not end in one is its own
// base, and the suffix is only stripped when something is left in front of it.
func baseLensName(name string) string {
	trimmed := strings.TrimSuffix(name, ")")
	if trimmed == name {
		return name
	}
	open := strings.LastIndex(trimmed, " (")
	if open <= 0 {
		return name
	}
	digits := trimmed[open+2:]
	if digits == "" {
		return name
	}
	for _, digit := range digits {
		if digit < '0' || digit > '9' {
			return name
		}
	}
	return trimmed[:open]
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

// RemapActiveLensID follows a machine's active-lens pointer across a merge.
//
// The pointer is machine-local while the definitions are portable, so a merge
// that re-identifies a record would otherwise leave the pointer naming whichever
// definition kept the plain id — silently swapping one lens for another rather
// than failing closed. Following it is the same derivation the merge used: the
// pointer's own definition is looked up on the side that carried it, and the
// merged collection is asked for that definition under the contributed id first
// and each derived id after.
//
// A pointer naming no lens on the contributing side is returned untouched, since
// there is nothing to follow and the caller's existing fail-closed reading still
// applies. A pointer whose definition survived nowhere returns empty, which is
// that same fail-closed reading stated at the merge instead of at the next load.
func RemapActiveLensID(active string, before, merged NamedLensList) string {
	if active == "" {
		return ""
	}
	core := ""
	carried := false
	for _, lens := range before {
		if lens.ID == active {
			core, carried = lensCoreKey(lens), true
			break
		}
	}
	if !carried {
		return active
	}
	cores := make(map[string]string, len(merged))
	for _, lens := range merged {
		cores[lens.ID] = lensCoreKey(lens)
	}
	for _, candidate := range lensIdentityCandidates(active, core) {
		if cores[candidate] == core {
			return candidate
		}
	}
	return ""
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
	merged, err := MergeNamedLenses(workspaceLenses.Lenses, adoptable)
	if err != nil {
		return fmt.Errorf("lens adoption could not merge the two homes: %w", err)
	}
	if err := merged.Validate(); err != nil {
		return fmt.Errorf("lens adoption produced an invalid collection: %w", err)
	}
	// The pointer is followed before the machine's own list is replaced, because
	// the list it names is the one being merged away.
	machine.ActiveLensID = RemapActiveLensID(machine.ActiveLensID, machine.NamedLenses, merged)
	machine.NamedLenses = merged
	machine.LensHomeAdopted = true
	return m.DomainCoordinator().CommitMachine(machine)
}
