package synclog

import (
	"fmt"
	"io/fs"
	"math/bits"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// graph is every transaction a device knows, in a deterministic topological
// order, with each transaction's ancestry and the final value it gave every
// logical key it wrote.
type graph struct {
	txns    []*Manifest
	index   map[string]int
	anc     []bitset
	writes  []map[string]keyWrite
	writers map[string][]int
	// keyIndexes caches each key's keyIndex, built on first use.
	keyIndexes map[string]*keyIndex
}

// keyWrite is what one transaction left at one logical key.
type keyWrite struct {
	kind      EffectKind
	projectID string
	// value is the digest the transaction left, nil when it removed the key.
	value *string
	// base is the digest the first effect on the key replaced: the key's value
	// before this transaction, as its author saw it.
	base *string
	// path is the physical path the transaction left the key at, or removed it
	// from.
	path string
}

type bitset []uint64

func newBitset(n int) bitset { return make(bitset, (n+63)/64) }

func (b bitset) set(i int)      { b[i/64] |= 1 << (uint(i) % 64) }
func (b bitset) has(i int) bool { return b[i/64]&(1<<(uint(i)%64)) != 0 }

func (b bitset) union(other bitset) {
	for i := range b {
		b[i] |= other[i]
	}
}

// countAnd is how many members b and other share.
func (b bitset) countAnd(other bitset) int {
	n := 0
	for i := range b {
		n += bits.OnesCount64(b[i] & other[i])
	}
	return n
}

// countAndNewest is how many members b and other share and the largest of
// them, -1 when they share none.
func (b bitset) countAndNewest(other bitset) (count, newest int) {
	newest = -1
	for i := range b {
		if shared := b[i] & other[i]; shared != 0 {
			count += bits.OnesCount64(shared)
			newest = i*64 + bits.Len64(shared) - 1
		}
	}
	return count, newest
}

// countAnd3 is how many members b, x and y all share.
func (b bitset) countAnd3(x, y bitset) int {
	n := 0
	for i := range b {
		n += bits.OnesCount64(b[i] & x[i] & y[i])
	}
	return n
}

func (b bitset) intersect(other bitset) bitset {
	out := make(bitset, len(b))
	for i := range b {
		out[i] = b[i] & other[i]
	}
	return out
}

// LogicalKey is the identity reconciliation merges a path's versions under.
// A project-scoped path is keyed by its project's id and its path inside the
// project directory, so a project renamed on one device and edited on another
// is one file in two places rather than two files. Any other path is its own
// key.
func LogicalKey(effect Effect, path string) string {
	if !effect.Kind.projectScoped() {
		return path
	}
	parts := strings.SplitN(path, "/", 3)
	rest := ""
	if len(parts) == 3 {
		rest = parts[2]
	}
	return "project/" + effect.ProjectID + "/" + rest
}

// projectKeyParts splits a project-scoped logical key.
func projectKeyParts(key string) (projectID, rest string, ok bool) {
	if !strings.HasPrefix(key, "project/") {
		return "", "", false
	}
	projectID, rest, ok = strings.Cut(strings.TrimPrefix(key, "project/"), "/")
	return projectID, rest, ok
}

// loadGraph reads every transaction the history indexes, plus extra, and
// orders them. The Outliner derives conflicts from the whole history after
// every pass that publishes, so a manifest an earlier call decoded is reused
// rather than read again, and the index is not walked at all while the state
// file is the one the last walk saw: every plan that indexes a transaction
// rewrites that file in the same journal transaction.
func (h *History) loadGraph(extra *Manifest) (*graph, error) {
	cache := h.manifests
	cache.mu.Lock()
	defer cache.mu.Unlock()
	state, statErr := os.Stat(h.StatePath())
	if statErr != nil || cache.state == nil || !sameFile(cache.state, state) {
		if err := h.walkIndex(cache); err != nil {
			return nil, err
		}
		cache.state = state
		if statErr != nil {
			cache.state = nil
		}
	}
	manifests := append(make([]*Manifest, 0, len(cache.indexed)+1), cache.indexed...)
	if extra != nil {
		manifests = append(manifests, extra)
	}
	return buildGraph(manifests)
}

// walkIndex replaces cache's indexed manifests with every transaction the
// index names, decoding only the ones the cache does not already hold.
func (h *History) walkIndex(cache *manifestCache) error {
	var manifests []*Manifest
	seen := make(map[string]*Manifest, len(cache.byID))
	root := filepath.Join(h.Dir(), "index")
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) && p == root {
				return filepath.SkipDir
			}
			return err
		}
		if d.IsDir() || !strings.HasSuffix(p, ".json") {
			return nil
		}
		txnID := "sha256:" + strings.TrimSuffix(filepath.Base(p), ".json")
		m, ok := cache.byID[txnID]
		if !ok {
			var loadErr error
			if m, loadErr = h.Transaction(txnID); loadErr != nil {
				return loadErr
			}
		}
		seen[txnID] = m
		manifests = append(manifests, m)
		return nil
	})
	if err != nil {
		cache.state = nil
		return fmt.Errorf("reading sync history: %w", err)
	}
	cache.byID, cache.indexed = seen, manifests
	return nil
}

func sameFile(a, b os.FileInfo) bool {
	return os.SameFile(a, b) && a.Size() == b.Size() && a.ModTime().Equal(b.ModTime())
}

// buildGraph orders manifests parents first, breaking ties by txn id so every
// device holding the same transactions builds the same graph.
func buildGraph(manifests []*Manifest) (*graph, error) {
	byID := make(map[string]*Manifest, len(manifests))
	for _, m := range manifests {
		byID[m.TxnID] = m
	}
	pending := make(map[string]int, len(byID))
	children := map[string][]string{}
	for id, m := range byID {
		count := 0
		for _, parent := range m.Parents {
			if _, ok := byID[parent]; !ok {
				return nil, fmt.Errorf("%w: parent %s of %s is unknown", ErrIncomplete, parent, id)
			}
			children[parent] = append(children[parent], id)
			count++
		}
		pending[id] = count
	}
	var ready []string
	for id, count := range pending {
		if count == 0 {
			ready = append(ready, id)
		}
	}
	g := &graph{index: map[string]int{}, writers: map[string][]int{}, keyIndexes: map[string]*keyIndex{}}
	for len(ready) > 0 {
		sort.Strings(ready)
		id := ready[0]
		ready = ready[1:]
		g.index[id] = len(g.txns)
		g.txns = append(g.txns, byID[id])
		for _, child := range children[id] {
			pending[child]--
			if pending[child] == 0 {
				ready = append(ready, child)
			}
		}
	}
	if len(g.txns) != len(byID) {
		return nil, fmt.Errorf("synclog: sync history has a cycle")
	}
	n := len(g.txns)
	g.anc = make([]bitset, n)
	g.writes = make([]map[string]keyWrite, n)
	for i, m := range g.txns {
		g.anc[i] = newBitset(n)
		g.anc[i].set(i)
		for _, parent := range m.Parents {
			g.anc[i].union(g.anc[g.index[parent]])
		}
		g.writes[i] = transactionWrites(m)
		keys := make([]string, 0, len(g.writes[i]))
		for key := range g.writes[i] {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			g.writers[key] = append(g.writers[key], i)
		}
	}
	return g, nil
}

// transactionWrites folds a manifest's effects, in order, into the final value
// it left at each logical key.
func transactionWrites(m *Manifest) map[string]keyWrite {
	out := map[string]keyWrite{}
	touch := func(effect Effect, path string, value, base *string) {
		key := LogicalKey(effect, path)
		write, seen := out[key]
		if !seen {
			write = keyWrite{kind: effect.Kind, projectID: effect.ProjectID, base: base}
		}
		write.value = value
		write.path = path
		out[key] = write
	}
	for _, effect := range m.Effects {
		switch effect.Op {
		case OpPut:
			touch(effect, effect.Path, effect.Blob, effect.BaseBlob)
		case OpDelete:
			touch(effect, effect.Path, nil, effect.BaseBlob)
		case OpMove:
			touch(effect, effect.Path, nil, effect.BaseBlob)
			touch(effect, effect.TargetPath, effect.Blob, nil)
		}
	}
	return out
}

// heads are the tips of all indexed branches, independent of device-local
// tracking state. Parenting these tips acknowledges every known transaction.
func (g *graph) heads() []string {
	heads := make(map[string]bool, len(g.txns))
	for _, m := range g.txns {
		heads[m.TxnID] = true
	}
	for _, m := range g.txns {
		for _, parent := range m.Parents {
			delete(heads, parent)
		}
	}
	return sortedKeys(heads)
}

// all is the set of every transaction in the graph.
func (g *graph) all() bitset {
	set := newBitset(len(g.txns))
	for i := range g.txns {
		set.set(i)
	}
	return set
}

// without is the set of every transaction except the one with txnID and its
// descendants.
func (g *graph) without(txnID string) bitset {
	set := g.all()
	skip, ok := g.index[txnID]
	if !ok {
		return set
	}
	for i := range g.txns {
		if g.anc[i].has(skip) {
			set[i/64] &^= 1 << (uint(i) % 64)
		}
	}
	return set
}

// keyIndex is what the predecessor scan needs to know about one key's writers
// without walking them: which transactions they are, and how many of them each
// one descends from.
type keyIndex struct {
	// mask holds every writer of the key.
	mask bitset
	// reach[i] is how many writers of the key are in writers[key][i]'s
	// ancestry, itself included.
	reach []int
	// roots are the writers that descend from no other writer.
	roots []int
}

// keyIndex builds key's index on first use. A graph is never changed after
// buildGraph, so the index stands for the graph's lifetime.
func (g *graph) keyIndex(key string) *keyIndex {
	if ki, ok := g.keyIndexes[key]; ok {
		return ki
	}
	writers := g.writers[key]
	ki := &keyIndex{mask: newBitset(len(g.txns)), reach: make([]int, len(writers))}
	for _, w := range writers {
		ki.mask.set(w)
	}
	position := make(map[int]int, len(writers))
	for i, w := range writers {
		ki.reach[i] = g.reach(ki, position, w)
		position[w] = i
		if ki.reach[i] == 1 {
			ki.roots = append(ki.roots, w)
		}
	}
	g.keyIndexes[key] = ki
	return ki
}

// reachHops bounds how far reach follows single parents before it counts.
const reachHops = 8

// reach is how many of a key's writers are in writer w's ancestry, itself
// included. A transaction with one parent adds only itself to that parent's
// ancestry, so w is one more than the nearest writer up a run of single
// parents, whose position already holds its count; a transaction merging
// several parents, or a run longer than reachHops, is counted outright.
func (g *graph) reach(ki *keyIndex, position map[int]int, w int) int {
	at := w
	for hop := 0; hop < reachHops; hop++ {
		parents := g.txns[at].Parents
		if len(parents) == 0 {
			return 1
		}
		if len(parents) > 1 {
			break
		}
		at = g.index[parents[0]]
		if i, writes := position[at]; writes {
			return ki.reach[i] + 1
		}
	}
	return g.anc[w].countAnd(ki.mask)
}

// maximal returns the writers of key inside set that no other writer inside
// set descends from, in topological order. It walks the writers newest first:
// one inside set is maximal unless a maximal already found descends from it,
// and the walk ends once the found maximals' ancestry accounts for every
// writer inside set, so a linear history stops at its newest writer.
func (g *graph) maximal(key string, set bitset) []int {
	writers := g.writers[key]
	if len(writers) == 1 {
		if set.has(writers[0]) {
			return []int{writers[0]}
		}
		return nil
	}
	ki := g.keyIndex(key)
	total, newest := ki.mask.countAndNewest(set)
	if total == 0 {
		return nil
	}
	var out []int
	// covered is the ancestry of every maximal found so far, and accounted how
	// many writers inside set it holds. It borrows the first maximal's own
	// ancestry and is copied only once a second joins it.
	var covered bitset
	accounted := 0
	// writers is in topological order, so the newest writer inside set is found
	// by its index and nothing newer than it is walked.
	for i := sort.SearchInts(writers, newest); i >= 0 && accounted < total; i-- {
		w := writers[i]
		if !set.has(w) || (covered != nil && covered.has(w)) {
			continue
		}
		out = append(out, w)
		switch len(out) {
		case 1:
			covered = g.anc[w]
		case 2:
			covered = append(bitset{}, covered...)
			covered.union(g.anc[w])
		default:
			covered.union(g.anc[w])
		}
		accounted = covered.countAnd3(ki.mask, set)
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}

// mergesPredecessors reports whether writers[key][i] was written over two or
// more maximal writers of key: whether its strict ancestry holds a merge. It
// asks the newest writer in that ancestry alone, which is the one maximal the
// walk in maximal would find first: every other writer there is either beneath
// it or a second maximal.
func (g *graph) mergesPredecessors(key string, i int) bool {
	ki := g.keyIndex(key)
	before := ki.reach[i] - 1
	if before < 2 {
		return false
	}
	writers := g.writers[key]
	anc := g.anc[writers[i]]
	for j := i - 1; j >= 0; j-- {
		if anc.has(writers[j]) {
			return ki.reach[j] != before
		}
	}
	return false
}

// genesis is the value a key had before any known transaction wrote it: the
// base the earliest writers replaced. Devices start from one shared copy, so
// every earliest writer names the same base; the smallest txn id breaks a tie
// deterministically if they ever disagree.
func (g *graph) genesis(key string) (keyWrite, bool) {
	roots := g.maximalRoots(key)
	if len(roots) == 0 {
		return keyWrite{}, false
	}
	sort.Slice(roots, func(i, j int) bool { return g.txns[roots[i]].TxnID < g.txns[roots[j]].TxnID })
	return g.writes[roots[0]][key], true
}

// maximalRoots are the writers of key that descend from no other writer, in
// topological order.
func (g *graph) maximalRoots(key string) []int {
	return append([]int(nil), g.keyIndex(key).roots...)
}

func (g *graph) keys() []string {
	out := make([]string, 0, len(g.writers))
	for key := range g.writers {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}
