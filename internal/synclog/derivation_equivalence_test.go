package synclog

import (
	"fmt"
	"math/rand"
	"reflect"
	"sort"
	"testing"
)

// synthFile is one synced file a seeded device edits: a logical identity that
// survives its project's rename, which is what LogicalKey keys it by.
type synthFile struct {
	kind      EffectKind
	projectID string
	// rest is the path inside the project directory, or the whole path of a
	// file no project owns.
	rest string
}

var synthFiles = []synthFile{
	{kind: EffectProjectDocument, projectID: "p1", rest: "tasks.md"},
	{kind: EffectNote, projectID: "p1", rest: "notes/a.md"},
	// No transaction enrolls this one, so its first writers name a base no
	// writer left: the genesis path.
	{kind: EffectNote, projectID: "p1", rest: "notes/seed.md"},
	{kind: EffectProjectDocument, projectID: "p2", rest: "tasks.md"},
	{kind: EffectPlanning, rest: ".docket/planning.json"},
}

// synthDevice is one seeded device: the heads it has seen and the workspace it
// believes it holds. That belief is not reconciled — a pull adopts the other
// device's files at random — because the histories only have to be varied, and
// both derivations read the same one.
type synthDevice struct {
	id      string
	heads   map[string]bool
	values  map[synthFile]*string
	slugs   map[string]string
	catalog *string
}

func (d *synthDevice) parents() []string { return sortedKeys(d.heads) }

func (d *synthDevice) path(f synthFile) string {
	if f.projectID == "" {
		return f.rest
	}
	return "projects/" + d.slugs[f.projectID] + "/" + f.rest
}

// names reports whether d's catalog still names the project f belongs to; a
// file no project owns always stands.
func (d *synthDevice) names(f synthFile) bool {
	_, named := d.slugs[f.projectID]
	return named || f.projectID == ""
}

func (d *synthDevice) projects() []synthProject {
	out := make([]synthProject, 0, len(d.slugs))
	for id, slug := range d.slugs {
		out = append(out, synthProject{ID: id, Slug: slug})
	}
	return out
}

// synthSim is one seeded run: the history being minted and the dice.
type synthSim struct {
	s        *synthHistory
	rng      *rand.Rand
	revision int
}

func (sim *synthSim) record(d *synthDevice, kind Kind, effects ...Effect) {
	d.heads = map[string]bool{sim.s.commit(d.id, kind, d.parents(), effects...): true}
}

// put writes f from a small pool of contents, so concurrent writers sometimes
// agree.
func (sim *synthSim) put(d *synthDevice, f synthFile) Effect {
	blob := sim.s.blob(fmt.Sprintf("%s version %d", f.rest, sim.rng.Intn(5)))
	effect := Effect{Kind: f.kind, ProjectID: f.projectID, Path: d.path(f), Op: OpPut, BaseBlob: d.values[f], Blob: blob}
	d.values[f] = blob
	return effect
}

func (sim *synthSim) writeCatalog(d *synthDevice) Effect {
	sim.revision++
	blob := sim.s.blob(encodeSynthCatalog(sim.revision, d.projects()))
	effect := Effect{Kind: EffectCatalog, Path: catalogRel, Op: OpPut, BaseBlob: d.catalog, Blob: blob}
	d.catalog = blob
	return effect
}

// rename moves every file d holds of one project under a new slug and names
// that slug in the catalog.
func (sim *synthSim) rename(d *synthDevice, projectID string) {
	renamed := fmt.Sprintf("%s-%d", map[string]string{"p1": "alpha", "p2": "beta"}[projectID], sim.rng.Intn(3))
	if renamed == d.slugs[projectID] {
		return
	}
	var effects []Effect
	for _, f := range synthFiles {
		if f.projectID != projectID || d.values[f] == nil {
			continue
		}
		effects = append(effects, Effect{
			Kind: f.kind, ProjectID: f.projectID, Op: OpMove,
			Path: d.path(f), TargetPath: "projects/" + renamed + "/" + f.rest,
			BaseBlob: d.values[f], Blob: d.values[f],
		})
	}
	d.slugs[projectID] = renamed
	sim.record(d, KindUser, append(effects, sim.writeCatalog(d))...)
}

// deleteProject removes every file d holds of one project and drops it from
// the catalog, which another device's concurrent edit then has to restore.
func (sim *synthSim) deleteProject(d *synthDevice, projectID string) {
	var effects []Effect
	for _, f := range synthFiles {
		if f.projectID != projectID || d.values[f] == nil {
			continue
		}
		effects = append(effects, Effect{Kind: f.kind, ProjectID: f.projectID, Path: d.path(f), Op: OpDelete, BaseBlob: d.values[f]})
		d.values[f] = nil
	}
	delete(d.slugs, projectID)
	sim.record(d, KindUser, append(effects, sim.writeCatalog(d))...)
}

// pull has d see everything other has seen and adopt some of what it holds.
func (sim *synthSim) pull(d, other *synthDevice) {
	for head := range other.heads {
		d.heads[head] = true
	}
	if sim.rng.Intn(2) == 0 {
		d.catalog = other.catalog
		d.slugs = map[string]string{}
		for id, slug := range other.slugs {
			d.slugs[id] = slug
		}
	}
	for _, f := range synthFiles {
		if !d.names(f) {
			d.values[f] = nil
		} else if sim.rng.Intn(2) == 0 {
			d.values[f] = other.values[f]
		}
	}
}

// seededHistory builds one random multi-device history: concurrent edits that
// sometimes write the same bytes, deletions, project renames carried as moves,
// project deletions racing edits, merges, resolutions, and partial pulls in
// every direction, which is what makes the ancestry criss-cross.
func seededHistory(t *testing.T, seed int64, steps int) *synthHistory {
	t.Helper()
	sim := &synthSim{s: newSynthHistory(t), rng: rand.New(rand.NewSource(seed))}
	s, rng := sim.s, sim.rng

	initial := map[synthFile]*string{}
	catalog := s.blob(encodeSynthCatalog(0, []synthProject{{ID: "p1", Slug: "alpha"}, {ID: "p2", Slug: "beta"}}))
	enroll := []Effect{{Kind: EffectCatalog, Path: catalogRel, Op: OpPut, Blob: catalog}}
	origin := &synthDevice{slugs: map[string]string{"p1": "alpha", "p2": "beta"}}
	for _, f := range synthFiles {
		initial[f] = s.blob("genesis of " + f.projectID + "/" + f.rest)
		if f.rest == "notes/seed.md" {
			continue
		}
		enroll = append(enroll, Effect{Kind: f.kind, ProjectID: f.projectID, Path: origin.path(f), Op: OpPut, Blob: initial[f]})
	}
	root := s.commit("device-a", KindEnroll, nil, enroll...)

	devices := make([]*synthDevice, 0, 4)
	for _, id := range []string{"device-a", "device-b", "device-c", "device-d"}[:2+rng.Intn(3)] {
		d := &synthDevice{id: id, heads: map[string]bool{root: true}, values: map[synthFile]*string{}, slugs: map[string]string{"p1": "alpha", "p2": "beta"}, catalog: catalog}
		for f, v := range initial {
			d.values[f] = v
		}
		devices = append(devices, d)
	}

	for step := 0; step < steps; step++ {
		d := devices[rng.Intn(len(devices))]
		var files []synthFile
		for _, f := range synthFiles {
			if d.names(f) {
				files = append(files, f)
			}
		}
		f := files[rng.Intn(len(files))]
		switch op := rng.Intn(14); op {
		case 0, 1, 2, 3, 4:
			sim.record(d, KindUser, sim.put(d, f))
		case 5:
			if d.values[f] != nil {
				sim.record(d, KindUser, Effect{Kind: f.kind, ProjectID: f.projectID, Path: d.path(f), Op: OpDelete, BaseBlob: d.values[f]})
				d.values[f] = nil
			}
		case 6:
			if f.projectID != "" {
				sim.rename(d, f.projectID)
			}
		case 7:
			if f.projectID != "" {
				sim.deleteProject(d, f.projectID)
			}
		case 8, 9, 10:
			if other := devices[rng.Intn(len(devices))]; other != d {
				sim.pull(d, other)
			}
		case 11:
			sim.record(d, KindMerge, sim.put(d, f))
		case 12, 13:
			effects := []Effect{sim.put(d, f)}
			if op == 13 {
				effects = append(effects, sim.writeCatalog(d))
			}
			sim.record(d, KindResolution, effects...)
		}
	}
	return s
}

// TestSyncDerivationMatchesQuadraticOracle holds the predecessor scan to the
// derivation it replaced: over seeded random multi-device histories the two
// resolve every key to the same value, place it at the same path, and derive
// the same conflicts under the same ids, within the whole history and within
// the history short of any one transaction.
func TestSyncDerivationMatchesQuadraticOracle(t *testing.T) {
	const seeds = 240
	var transactions, merging, conflicts, carried, resolving, restored int
	for seed := int64(1); seed <= seeds; seed++ {
		s := seededHistory(t, seed, 20+int(seed%5)*12)
		g, err := buildGraph(s.manifests)
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		transactions += len(g.txns)
		for _, m := range g.txns {
			if len(m.Parents) > 1 {
				merging++
			}
			if m.Kind == KindResolution {
				resolving++
			}
		}
		requireSameMaximals(t, seed, g)

		rng := rand.New(rand.NewSource(seed))
		sets := map[string]bitset{"all": g.all()}
		for i := 0; i < 4; i++ {
			txn := g.txns[rng.Intn(len(g.txns))].TxnID
			sets["without "+txn] = g.without(txn)
		}
		names := make([]string, 0, len(sets))
		for name := range sets {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			want, wantErr := newDerivationOracle(s.materializerOver(g)).materialize(sets[name])
			got, gotErr := s.materializerOver(g).materialize(sets[name])
			if (wantErr == nil) != (gotErr == nil) || (wantErr != nil && wantErr.Error() != gotErr.Error()) {
				t.Fatalf("seed %d, %s: derivation failed with %v, the oracle with %v", seed, name, gotErr, wantErr)
			}
			if wantErr != nil {
				t.Fatalf("seed %d, %s: the seeded history does not materialize: %v", seed, name, wantErr)
			}
			if !reflect.DeepEqual(got.values, want.values) {
				t.Fatalf("seed %d, %s: values differ\n got %v\nwant %v", seed, name, got.values, want.values)
			}
			if !reflect.DeepEqual(got.placement, want.placement) {
				t.Fatalf("seed %d, %s: placement differs\n got %v\nwant %v", seed, name, got.placement, want.placement)
			}
			if !reflect.DeepEqual(got.conflicts, want.conflicts) {
				t.Fatalf("seed %d, %s: conflicts differ\n got %s\nwant %s", seed, name, conflictIDs(got.conflicts), conflictIDs(want.conflicts))
			}
			if name != "all" {
				continue
			}
			conflicts += len(want.conflicts)
			for _, conflict := range want.conflicts {
				if conflict.Kind == EffectCatalog && conflict.Key != catalogRel {
					restored++
				}
				if conflict.ResultBlob != nil {
					if value, held := want.values[conflict.Key]; held && len(conflict.Candidates) > 1 && !candidateHolds(conflict, value) {
						carried++
					}
				}
			}
		}
	}
	t.Logf("%d seeds: %d transactions, %d merging several heads, %d resolutions, %d open conflicts (%d restored projects, %d whose result no candidate holds)",
		seeds, transactions, merging, resolving, conflicts, restored, carried)
	// A corpus that never merged, conflicted, or resolved would agree trivially.
	if merging == 0 || conflicts == 0 || resolving == 0 || restored == 0 {
		t.Fatalf("the seeded histories are too plain to compare: %d merging, %d conflicts, %d resolutions, %d restored projects", merging, conflicts, resolving, restored)
	}
}

func candidateHolds(conflict Conflict, value string) bool {
	for _, candidate := range conflict.Candidates {
		if candidate.Blob != nil && *candidate.Blob == value {
			return true
		}
	}
	return false
}

func conflictIDs(conflicts []Conflict) []string {
	out := make([]string, 0, len(conflicts))
	for _, conflict := range conflicts {
		out = append(out, conflict.ID+" "+conflict.Key)
	}
	return out
}

// requireSameMaximals compares the scan with the oracle on every set shape
// reconciliation asks about — the whole history, the history short of one
// transaction, a writer's strict ancestry, the common ancestry of two writers —
// and on the roots genesis reads.
func requireSameMaximals(t *testing.T, seed int64, g *graph) {
	t.Helper()
	sets := []bitset{g.all(), newBitset(len(g.txns))}
	for i, m := range g.txns {
		sets = append(sets, g.without(m.TxnID))
		before := append(bitset{}, g.anc[i]...)
		before[i/64] &^= 1 << (uint(i) % 64)
		sets = append(sets, before)
		if i > 0 {
			sets = append(sets, g.anc[i].intersect(g.anc[(i*7)%i]))
		}
	}
	for _, key := range g.keys() {
		for _, set := range sets {
			if got, want := g.maximal(key, set), oracleMaximal(g, key, set); !sameInts(got, want) {
				t.Fatalf("seed %d: maximal writers of %s are %v, the oracle's %v", seed, key, got, want)
			}
		}
		if got, want := g.maximalRoots(key), oracleMaximalRoots(g, key); !sameInts(got, want) {
			t.Fatalf("seed %d: roots of %s are %v, the oracle's %v", seed, key, got, want)
		}
		for i, w := range g.writers[key] {
			before := append(bitset{}, g.anc[w]...)
			before[w/64] &^= 1 << (uint(w) % 64)
			if got, want := g.mergesPredecessors(key, i), len(oracleMaximal(g, key, before)) >= 2; got != want {
				t.Fatalf("seed %d: writer %d of %s merges predecessors: %v, the oracle says %v", seed, w, key, got, want)
			}
		}
	}
}

func sameInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
