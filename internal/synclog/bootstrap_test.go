package synclog

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/sglavoie/docket/internal/transaction"
)

// Apply plans without durability costs here: storage tests exercise the real
// journal. This suite compares the two independent admission paths, including
// later imports against the state and blobs bootstrap left on disk.
func applyBootstrapTestPlan(t *testing.T, plan *ReconcilePlan) {
	t.Helper()
	for _, op := range plan.Ops {
		if err := os.MkdirAll(filepath.Dir(op.Target), 0700); err != nil {
			t.Fatal(err)
		}
		var err error
		switch op.Kind {
		case transaction.OpWrite:
			err = os.WriteFile(op.Target, op.Data, 0600)
		case transaction.OpDelete:
			err = os.Remove(op.Target)
			if os.IsNotExist(err) {
				err = nil
			}
		case transaction.OpMove:
			err = os.Rename(op.Source, op.Target)
		default:
			t.Fatalf("unexpected operation %s", op.Kind)
		}
		if err != nil {
			t.Fatal(err)
		}
	}
}

func bootstrapTestHistory(t *testing.T) *History {
	t.Helper()
	h, err := New(Config{DataDir: t.TempDir(), DeviceID: "observer", Schemas: map[string]int{
		SchemaCatalog: 1, SchemaDomainEnvelope: 1, SchemaTaskMarkdown: 1,
	}})
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func compareBootstrapHistories(t *testing.T, want, got *History) {
	t.Helper()
	a, err := want.LoadState()
	if err != nil {
		t.Fatal(err)
	}
	b, err := got.LoadState()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("tracking differs\nsequential: %+v\nbootstrap: %+v", a, b)
	}
	for rel := range a.Tracked {
		left, readErr := os.ReadFile(want.abs(rel))
		if readErr != nil {
			t.Fatal(readErr)
		}
		right, readErr := os.ReadFile(got.abs(rel))
		if readErr != nil || !bytes.Equal(left, right) {
			t.Fatalf("workspace differs at %s: %v", rel, readErr)
		}
	}
	x, err := want.OpenConflicts(synthMerger{})
	if err != nil {
		t.Fatal(err)
	}
	y, err := got.OpenConflicts(synthMerger{})
	if err != nil || !reflect.DeepEqual(x, y) {
		t.Fatalf("conflicts differ: %v\n%+v\n%+v", err, x, y)
	}
}

func TestBootstrapMatchesSequentialHistoryAndLaterImports(t *testing.T) {
	for seed := int64(0); seed < 12; seed++ {
		t.Run(fmt.Sprint(seed), func(t *testing.T) {
			s := seededHistory(t, seed, 30)
			sequential, batch := bootstrapTestHistory(t), bootstrapTestHistory(t)
			// Leave several real edits for the ordinary import path after join.
			cut := len(s.manifests) - 3
			for _, m := range s.manifests[:cut] {
				plan, err := sequential.PlanReconcile(m, s.blobs, synthMerger{}, "one")
				if err != nil {
					t.Fatal(err)
				}
				applyBootstrapTestPlan(t, plan)
			}
			// Incoming enumeration is not guaranteed to be causal.
			manifests := append([]*Manifest{}, s.manifests[:cut]...)
			for i, j := 0, len(manifests)-1; i < j; i, j = i+1, j-1 {
				manifests[i], manifests[j] = manifests[j], manifests[i]
			}
			plan, err := batch.PlanBootstrap(manifests, s.blobs, synthMerger{}, "batch")
			if err != nil || plan == nil {
				t.Fatalf("bootstrap: %v", err)
			}
			targets := map[string]bool{}
			for _, op := range plan.Ops {
				if targets[op.Target] {
					t.Fatalf("bootstrap writes the same target twice: %s", op.Target)
				}
				targets[op.Target] = true
			}
			applyBootstrapTestPlan(t, plan)
			compareBootstrapHistories(t, sequential, batch)
			for _, m := range manifests {
				if _, known, err := batch.Lookup(m.TxnID); err != nil || !known {
					t.Fatalf("missing receipt for %s: %v", m.TxnID, err)
				}
				for _, effect := range m.Effects {
					for _, digest := range []*string{effect.BaseBlob, effect.Blob} {
						if digest == nil {
							continue
						}
						data, err := batch.Blob(*digest)
						if err != nil || !bytes.Equal(data, s.blobs[*digest]) {
							t.Fatalf("historical blob lost: %s: %v", *digest, err)
						}
					}
				}
			}
			for _, m := range s.manifests[cut:] {
				for _, h := range []*History{sequential, batch} {
					plan, err := h.PlanReconcile(m, s.blobs, synthMerger{}, "later")
					if err != nil {
						t.Fatal(err)
					}
					applyBootstrapTestPlan(t, plan)
				}
				compareBootstrapHistories(t, sequential, batch)
			}
			if plan, err := batch.PlanBootstrap(manifests, s.blobs, synthMerger{}, "again"); err != nil || plan != nil {
				t.Fatalf("existing history must decline bootstrap: %v", err)
			}
		})
	}
}

func TestBootstrapRefusesIncompleteOrIncompatibleHistory(t *testing.T) {
	cases := map[string]func(*synthHistory){
		"missing genesis":    func(s *synthHistory) { s.manifests = s.manifests[1:] },
		"missing parent":     func(s *synthHistory) { s.manifests = append(s.manifests[:1], s.manifests[2:]...) },
		"missing old blob":   func(s *synthHistory) { delete(s.blobs, *s.manifests[0].Effects[1].Blob) },
		"corrupt old blob":   func(s *synthHistory) { s.blobs[*s.manifests[0].Effects[1].Blob] = []byte("corrupt") },
		"duplicate":          func(s *synthHistory) { s.manifests = append(s.manifests, s.manifests[0]) },
		"foreign":            func(s *synthHistory) { s.manifests[2].WorkspaceID = "elsewhere" },
		"unsupported schema": func(s *synthHistory) { s.manifests[2].Schemas[SchemaTaskMarkdown] = 999 },
		"own author":         func(s *synthHistory) { s.manifests[2].DeviceID = "observer" },
		"reused sequence":    func(s *synthHistory) { s.manifests[2].DeviceSeq = 2 },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			s, h := linearHistory(t, 3), bootstrapTestHistory(t)
			mutate(s)
			// Reseal mutations to exercise semantic checks, not only hashes.
			for _, m := range s.manifests {
				if err := m.seal(); err != nil {
					t.Fatal(err)
				}
			}
			if plan, err := h.PlanBootstrap(s.manifests, s.blobs, synthMerger{}, "bad"); err == nil || plan != nil {
				t.Fatalf("bad history accepted: %v", err)
			}
			entries, err := os.ReadDir(h.dataDir)
			if err != nil || len(entries) != 0 {
				t.Fatalf("refused plan changed workspace: %v", err)
			}
		})
	}
}

func TestBootstrapPreservesExistingWorkspace(t *testing.T) {
	s, h := linearHistory(t, 3), bootstrapTestHistory(t)
	path := h.abs(".tt/global-notes/local.md")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("unpublished"), 0600); err != nil {
		t.Fatal(err)
	}
	if plan, err := h.PlanBootstrap(s.manifests, s.blobs, synthMerger{}, "bad"); err != nil || plan != nil {
		t.Fatalf("occupied workspace did not decline: %v", err)
	}
}

func TestBootstrapDeclinesRetainedHistoryWithoutDeviceState(t *testing.T) {
	s, h := linearHistory(t, 3), bootstrapTestHistory(t)
	plan, err := h.PlanBootstrap(s.manifests, s.blobs, synthMerger{}, "first")
	if err != nil {
		t.Fatal(err)
	}
	applyBootstrapTestPlan(t, plan)
	state, err := h.LoadState()
	if err != nil {
		t.Fatal(err)
	}
	for path := range state.Tracked {
		if err := os.Remove(h.abs(path)); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Remove(h.StatePath()); err != nil {
		t.Fatal(err)
	}
	if plan, err := h.PlanBootstrap(s.manifests, s.blobs, synthMerger{}, "again"); err != nil || plan != nil {
		t.Fatalf("retained history must use ordinary reconciliation: %v", err)
	}
}

func TestBootstrapRejectsParentOrderContradictingAuthorSequence(t *testing.T) {
	s := newSynthHistory(t)
	root := s.commit("genesis", KindEnroll, nil)
	s.seq["peer"] = 1
	later := s.commit("peer", KindUser, []string{root})
	s.seq["peer"] = 0
	s.commit("peer", KindUser, []string{later})
	if plan, err := bootstrapTestHistory(t).PlanBootstrap(s.manifests, s.blobs, synthMerger{}, "bad"); err == nil || plan != nil {
		t.Fatalf("impossible author order accepted: %v", err)
	}
}
