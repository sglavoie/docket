package synclog

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"testing"
	"time"
)

// scalingSizes are the history lengths the guard compares. Each doubles the
// one before, so linear derivation doubles its time, quadratic derivation
// quadruples it, and the cubic derivation this replaced multiplied it by eight.
var scalingSizes = []int{2000, 4000, 8000}

// scalingRatioLimit bounds how much longer the largest history may take than
// the smallest, which is four times its length. Linear work sits at 4,
// quadratic work at 16, and the retired derivation sat near 64; the room above
// 4 is for the per-transaction ancestry bitsets, which grow with the history
// and outgrow the processor's cache between these sizes, and for a loaded
// machine.
const scalingRatioLimit = 12.0

// linearHistory is one device writing one file transactions times over: every
// writer has exactly one maximal predecessor, the shape a single user's
// workspace takes.
func linearHistory(t testing.TB, transactions int) *synthHistory {
	t.Helper()
	s := newSynthHistory(t)
	catalog := s.blob(encodeSynthCatalog(0, []synthProject{{ID: "p1", Slug: "alpha"}}))
	value := s.blob("tasks 0")
	head := s.commit("device-a", KindEnroll, nil,
		Effect{Kind: EffectCatalog, Path: catalogRel, Op: OpPut, Blob: catalog},
		Effect{Kind: EffectProjectDocument, ProjectID: "p1", Path: "projects/alpha/tasks.md", Op: OpPut, Blob: value})
	for i := 1; i < transactions; i++ {
		next := s.blob(fmt.Sprintf("tasks %d", i))
		head = s.commit("device-a", KindUser, []string{head},
			Effect{Kind: EffectProjectDocument, ProjectID: "p1", Path: "projects/alpha/tasks.md", Op: OpPut, BaseBlob: value, Blob: next})
		value = next
	}
	return s
}

// interleavedHistory is two devices taking turns on one file in rounds of four
// transactions: both edit it concurrently, one merges the pair, and the other
// then edits on top of that merge. A quarter of the writers are therefore
// written over a merge, some of those merges leave a conflict every later
// writer carries, and every fortieth round's merge is a resolution, so the
// carried-conflict and resolution paths are part of what is timed.
func interleavedHistory(t testing.TB, transactions int) *synthHistory {
	t.Helper()
	s := newSynthHistory(t)
	const path = "projects/alpha/tasks.md"
	doc := func(base, blob *string) Effect {
		return Effect{Kind: EffectProjectDocument, ProjectID: "p1", Path: path, Op: OpPut, BaseBlob: base, Blob: blob}
	}
	catalog := s.blob(encodeSynthCatalog(0, []synthProject{{ID: "p1", Slug: "alpha"}}))
	value := s.blob("tasks 0")
	head := s.commit("device-a", KindEnroll, nil,
		Effect{Kind: EffectCatalog, Path: catalogRel, Op: OpPut, Blob: catalog}, doc(nil, value))
	for round := 0; len(s.manifests)+4 <= transactions; round++ {
		left, right := s.blob(fmt.Sprintf("round %d on a", round)), s.blob(fmt.Sprintf("round %d on b", round))
		a := s.commit("device-a", KindUser, []string{head}, doc(value, left))
		b := s.commit("device-b", KindUser, []string{head}, doc(value, right))
		kind := KindMerge
		if round%40 == 39 {
			kind = KindResolution
		}
		merged := s.blob(fmt.Sprintf("round %d merged", round))
		head = s.commit("device-a", kind, []string{a, b}, doc(left, merged))
		value = s.blob(fmt.Sprintf("round %d continued", round))
		head = s.commit("device-b", KindUser, []string{head}, doc(merged, value))
	}
	for len(s.manifests) < transactions {
		next := s.blob(fmt.Sprintf("tail %d", len(s.manifests)))
		head = s.commit("device-b", KindUser, []string{head}, doc(value, next))
		value = next
	}
	return s
}

// derivationTime is the fastest of several whole derivations over one history.
// The graph is built once, outside the clock, since its ancestry bitsets are
// history-sized work of their own; each run starts from an empty memo and no
// key index, which is what a real pass starts from, and from every blob already
// read, so the clock holds derivation and no file lookups.
func derivationTime(t *testing.T, s *synthHistory) (time.Duration, *materialized) {
	t.Helper()
	g, err := buildGraph(s.manifests)
	if err != nil {
		t.Fatal(err)
	}
	mz := s.materializerOver(g)
	var best time.Duration
	var state *materialized
	for run := 0; run < 5; run++ {
		g.keyIndexes = map[string]*keyIndex{}
		mz.cache, mz.memo = make(map[string][]byte, len(s.blobs)), map[string]resolution{}
		for digest, data := range s.blobs {
			mz.cache[digest] = data
		}
		set := g.all()
		// The collector's work follows the heap the history occupies rather
		// than what derivation allocates, so it is settled first and held off.
		runtime.GC()
		percent := debug.SetGCPercent(-1)
		start := time.Now()
		state, err = mz.materialize(set)
		elapsed := time.Since(start)
		debug.SetGCPercent(percent)
		if err != nil {
			t.Fatal(err)
		}
		if run == 0 || elapsed < best {
			best = elapsed
		}
	}
	return best, state
}

// TestSyncDerivationScalesNearLinearly is the guard on conflict derivation's
// growth, which runs after every exchange pass inside the Outliner's update
// loop. It compares times between history sizes rather than against a clock, so
// the race detector and a slow machine scale both sides alike.
func TestSyncDerivationScalesNearLinearly(t *testing.T) {
	shapes := []struct {
		name  string
		build func(testing.TB, int) *synthHistory
		// conflicts reports whether the shape must leave conflicts open, so the
		// guard cannot pass by timing a history that derives none.
		conflicts bool
	}{
		{name: "linear chain", build: linearHistory},
		{name: "interleaved two devices", build: interleavedHistory, conflicts: true},
	}
	for _, shape := range shapes {
		t.Run(shape.name, func(t *testing.T) {
			histories := make([]*synthHistory, len(scalingSizes))
			for i, size := range scalingSizes {
				histories[i] = shape.build(t, size)
				if len(histories[i].manifests) != size {
					t.Fatalf("built %d transactions, want %d", len(histories[i].manifests), size)
				}
			}
			// A size keeps the fastest time any attempt gave it, so one attempt
			// run beside a busy neighbor is measured again rather than believed.
			times := make([]time.Duration, len(scalingSizes))
			for attempt := 0; attempt < 3; attempt++ {
				for i, size := range scalingSizes {
					elapsed, state := derivationTime(t, histories[i])
					if shape.conflicts != (len(state.conflicts) > 0) {
						t.Fatalf("%d transactions left %d open conflicts", size, len(state.conflicts))
					}
					if attempt == 0 || elapsed < times[i] {
						times[i] = elapsed
					}
					t.Logf("%d transactions: %v, %d open conflicts", size, elapsed, len(state.conflicts))
				}
				if float64(times[len(times)-1])/float64(times[0]) <= scalingRatioLimit {
					break
				}
			}
			ratio := float64(times[len(times)-1]) / float64(times[0])
			t.Logf("fastest: %v, a ratio of %.1f", times, ratio)
			if ratio > scalingRatioLimit {
				t.Fatalf("deriving %d transactions took %.1f times as long as %d (%v against %v); linear growth is 4 and the limit is %.0f",
					scalingSizes[len(scalingSizes)-1], ratio, scalingSizes[0], times[len(times)-1], times[0], scalingRatioLimit)
			}
		})
	}
}
