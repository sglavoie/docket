package synclog

import (
	"fmt"
	"io/fs"
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
// orders them.
func (h *History) loadGraph(extra *Manifest) (*graph, error) {
	var manifests []*Manifest
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
		m, loadErr := h.Transaction(txnID)
		if loadErr != nil {
			return loadErr
		}
		manifests = append(manifests, m)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("reading sync history: %w", err)
	}
	if extra != nil {
		manifests = append(manifests, extra)
	}
	return buildGraph(manifests)
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
	g := &graph{index: map[string]int{}, writers: map[string][]int{}}
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

// maximal returns the writers of key inside set that no other writer inside
// set descends from, in topological order.
func (g *graph) maximal(key string, set bitset) []int {
	var inside []int
	for _, w := range g.writers[key] {
		if set.has(w) {
			inside = append(inside, w)
		}
	}
	var out []int
	for _, w := range inside {
		dominated := false
		for _, other := range inside {
			if other != w && g.anc[other].has(w) {
				dominated = true
				break
			}
		}
		if !dominated {
			out = append(out, w)
		}
	}
	return out
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

// maximalRoots are the writers of key that descend from no other writer.
func (g *graph) maximalRoots(key string) []int {
	var out []int
	for _, w := range g.writers[key] {
		root := true
		for _, other := range g.writers[key] {
			if other != w && g.anc[w].has(other) {
				root = false
				break
			}
		}
		if root {
			out = append(out, w)
		}
	}
	return out
}

func (g *graph) keys() []string {
	out := make([]string, 0, len(g.writers))
	for key := range g.writers {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}
