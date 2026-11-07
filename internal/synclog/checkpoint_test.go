package synclog

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// checkpointFrom replays manifests into a fresh observer history and plans
// its checkpoint, which the test then lands on disk.
func checkpointFrom(t *testing.T, s *synthHistory, manifests []*Manifest) (*History, *Checkpoint) {
	t.Helper()
	author := bootstrapTestHistory(t)
	for _, m := range manifests {
		plan, err := author.PlanReconcile(m, s.blobs, synthMerger{}, "replay")
		if err != nil {
			t.Fatal(err)
		}
		applyBootstrapTestPlan(t, plan)
	}
	plan, c, err := author.PlanCheckpoint()
	if err != nil {
		t.Fatal(err)
	}
	applyBootstrapTestPlan(t, &ReconcilePlan{Plan: *plan})
	return author, c
}

// authorBlobs is what a published checkpoint delivers: the author publishes
// every blob of its image from its own store, merged bytes no transaction
// carries included.
type authorBlobs struct {
	author *History
	synth  synthBlobs
}

func (b authorBlobs) Blob(digest string) ([]byte, error) {
	if data, err := b.author.Blob(digest); err == nil {
		return data, nil
	}
	return b.synth.Blob(digest)
}

func joiningTestHistory(t *testing.T) *History {
	t.Helper()
	h, err := New(Config{DataDir: t.TempDir(), DeviceID: "joiner", Schemas: map[string]int{
		SchemaCatalog: 1, SchemaDomainEnvelope: 1, SchemaTaskMarkdown: 1,
	}})
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func installTestCheckpoint(t *testing.T, h *History, c *Checkpoint, blobs BlobSource) {
	t.Helper()
	plan, err := h.PlanCheckpointInstall(c, blobs, nil, "install")
	if err != nil || plan == nil {
		t.Fatalf("install: %v", err)
	}
	applyBootstrapTestPlan(t, plan)
}

// descendsFromAll reports whether m's ancestry, over every manifest, holds
// every manifest in prefix: what a joiner may treat as descending from the
// checkpoint of that prefix.
func descendsFromAll(t *testing.T, all []*Manifest, prefix []*Manifest, m *Manifest) bool {
	t.Helper()
	g, err := buildGraph(all)
	if err != nil {
		t.Fatal(err)
	}
	anc := g.anc[g.index[m.TxnID]]
	for _, p := range prefix {
		if !anc.has(g.index[p.TxnID]) {
			return false
		}
	}
	return true
}

func TestCheckpointRecordRoundTripsAndRefusesTampering(t *testing.T) {
	s := linearHistory(t, 4)
	_, c := checkpointFrom(t, s, s.manifests)
	if len(c.Covered) != 4 || !reflect.DeepEqual(c.Heads, []string{s.manifests[3].TxnID}) {
		t.Fatalf("checkpoint summarizes %d with heads %v", len(c.Covered), c.Heads)
	}
	data, err := c.Encode()
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeCheckpoint(data)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.CheckpointID != c.CheckpointID || !reflect.DeepEqual(decoded.Covered, c.Covered) || !reflect.DeepEqual(decoded.Files, c.Files) {
		t.Fatal("decoding changed the record")
	}
	tampered := bytes.Replace(data, []byte(`"device_seq": 2`), []byte(`"device_seq": 9`), 1)
	if _, err := DecodeCheckpoint(tampered); !errors.Is(err, ErrInvalidCheckpoint) {
		t.Fatalf("tampered record accepted: %v", err)
	}
	newer := bytes.Replace(data, []byte(`"format_version": 1`), []byte(`"format_version": 2`), 1)
	if _, err := DecodeCheckpoint(newer); !errors.Is(err, ErrHeld) {
		t.Fatalf("newer record not held: %v", err)
	}
}

// A joiner that installs a checkpoint of a prefix and then imports the rest
// must hold exactly what a device that replayed everything holds, and must
// hold back — never misapply — a transaction concurrent with the prefix.
func TestCheckpointJoinMatchesFullReplay(t *testing.T) {
	checked := 0
	for seed := int64(0); seed < 16; seed++ {
		s := seededHistory(t, seed, 30)
		for _, cut := range []int{1, len(s.manifests) / 3, len(s.manifests) / 2, len(s.manifests) - 2} {
			t.Run(fmt.Sprintf("%d/%d", seed, cut), func(t *testing.T) {
				prefix, suffix := s.manifests[:cut], s.manifests[cut:]
				author, c := checkpointFrom(t, s, prefix)
				if open, err := author.OpenConflicts(synthMerger{}); err != nil || len(open) > 0 {
					// A device refuses to checkpoint with open conflicts.
					t.Skip("the prefix leaves conflicts open")
				}
				joiner := joiningTestHistory(t)
				installTestCheckpoint(t, joiner, c, authorBlobs{author, s.blobs})
				for _, m := range prefix {
					if _, err := joiner.PlanReconcile(m, s.blobs, synthMerger{}, "again"); !errors.Is(err, ErrAlreadyApplied) {
						t.Fatalf("summarized %s not known: %v", m.TxnID, err)
					}
				}
				whole := true
				for _, m := range suffix {
					plan, err := joiner.PlanReconcile(m, s.blobs, synthMerger{}, "later")
					if !descendsFromAll(t, s.manifests, prefix, m) {
						var gap *CheckpointGapError
						if !errors.As(err, &gap) && !errors.Is(err, ErrIncomplete) {
							t.Fatalf("concurrent %s not held: %v", m.TxnID, err)
						}
						whole = false
						continue
					}
					if !whole {
						// It may descend from something held; nothing to compare.
						continue
					}
					if err != nil {
						t.Fatalf("whole descendant %s refused: %v", m.TxnID, err)
					}
					applyBootstrapTestPlan(t, plan)
				}
				if !whole {
					return
				}
				requireSameAsReplay(t, s, joiner)
				checked++
			})
		}
	}
	t.Logf("%d joins compared against full replay", checked)
	if checked < 5 {
		t.Fatalf("only %d joins were comparable", checked)
	}
}

func TestCheckpointInstallRefusals(t *testing.T) {
	s := linearHistory(t, 3)
	_, c := checkpointFrom(t, s, s.manifests)

	t.Run("other schemas are held", func(t *testing.T) {
		h, err := New(Config{DataDir: t.TempDir(), DeviceID: "joiner", Schemas: map[string]int{
			SchemaCatalog: 2, SchemaDomainEnvelope: 1, SchemaTaskMarkdown: 1,
		}})
		if err != nil {
			t.Fatal(err)
		}
		var held *HeldError
		if _, err := h.PlanCheckpointInstall(c, s.blobs, nil, "x"); !errors.As(err, &held) || !held.UpgradeAuthor() {
			t.Fatalf("older checkpoint not held for its author: %v", err)
		}
	})
	t.Run("an occupied working copy declines", func(t *testing.T) {
		h := joiningTestHistory(t)
		if err := os.MkdirAll(filepath.Join(h.dataDir, "projects", "alpha"), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(h.dataDir, "projects", "alpha", "tasks.md"), []byte("mine"), 0600); err != nil {
			t.Fatal(err)
		}
		if plan, err := h.PlanCheckpointInstall(c, s.blobs, nil, "x"); err != nil || plan != nil {
			t.Fatalf("occupied copy accepted: %v", err)
		}
	})
	t.Run("missing content is incomplete", func(t *testing.T) {
		h := joiningTestHistory(t)
		if _, err := h.PlanCheckpointInstall(c, synthBlobs{}, nil, "x"); !errors.Is(err, ErrIncomplete) {
			t.Fatalf("missing content not incomplete: %v", err)
		}
	})
	t.Run("own transactions are refused", func(t *testing.T) {
		h, err := New(Config{DataDir: t.TempDir(), DeviceID: "device-a", Schemas: map[string]int{
			SchemaCatalog: 1, SchemaDomainEnvelope: 1, SchemaTaskMarkdown: 1,
		}})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := h.PlanCheckpointInstall(c, s.blobs, nil, "x"); !errors.Is(err, ErrOwnTransaction) {
			t.Fatalf("own history accepted: %v", err)
		}
	})
}

// A device that joined from a checkpoint authors work, and checkpoints of its
// own, that name only real transactions: a peer never learns of the node.
func TestJoinedDeviceNamesRealParents(t *testing.T) {
	s := linearHistory(t, 3)
	_, c := checkpointFrom(t, s, s.manifests)
	joiner := joiningTestHistory(t)
	installTestCheckpoint(t, joiner, c, s.blobs)
	g, err := joiner.loadGraph(nil)
	if err != nil {
		t.Fatal(err)
	}
	if heads := g.heads(); !reflect.DeepEqual(heads, c.Heads) {
		t.Fatalf("heads %v, want the summarized tips %v", heads, c.Heads)
	}
	if node, nodeErr := joiner.Transaction(c.CheckpointID); nodeErr != nil || node.Kind != KindCheckpoint {
		t.Fatalf("checkpoint node unreadable: %v", nodeErr)
	}
	if _, summarizedErr := joiner.Transaction(s.manifests[0].TxnID); !errors.Is(summarizedErr, ErrSummarized) {
		t.Fatalf("summarized manifest pretended to be held: %v", summarizedErr)
	}
	due, err := joiner.CheckpointDue(500)
	if err != nil || due {
		t.Fatalf("a fresh joiner wants to republish its checkpoint: %v", err)
	}
	plan, again, err := joiner.PlanCheckpoint()
	if err != nil {
		t.Fatal(err)
	}
	applyBootstrapTestPlan(t, &ReconcilePlan{Plan: *plan})
	if !reflect.DeepEqual(again.Covered, c.Covered) || !reflect.DeepEqual(again.Heads, c.Heads) {
		t.Fatal("a joiner's own checkpoint lost or invented history")
	}
}

func TestCheckpointDueAfterMigration(t *testing.T) {
	s := linearHistory(t, 3)
	h, err := New(Config{DataDir: t.TempDir(), DeviceID: "observer", Schemas: map[string]int{
		SchemaCatalog: 1, SchemaDomainEnvelope: 1, SchemaTaskMarkdown: 1,
	}})
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range s.manifests {
		plan, replayErr := h.PlanReconcile(m, s.blobs, synthMerger{}, "replay")
		if replayErr != nil {
			t.Fatal(replayErr)
		}
		applyBootstrapTestPlan(t, plan)
	}
	if due, dueErr := h.CheckpointDue(500); dueErr != nil || due {
		t.Fatalf("a replayable history wants a checkpoint: %v", dueErr)
	}
	migrated, err := New(Config{DataDir: h.dataDir, DeviceID: "observer", Schemas: map[string]int{
		SchemaCatalog: 2, SchemaDomainEnvelope: 1, SchemaTaskMarkdown: 1,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if due, dueErr := migrated.CheckpointDue(500); dueErr != nil || !due {
		t.Fatalf("a history no longer replayable at its schemas wants no checkpoint: %v", dueErr)
	}
	if due, dueErr := h.CheckpointDue(2); dueErr != nil || !due {
		t.Fatalf("a long history wants no checkpoint: %v", dueErr)
	}
}

// requireSameAsReplay compares joiner with a device that replayed every
// manifest from the genesis: tracking, workspace bytes, and open conflicts.
func requireSameAsReplay(t *testing.T, s *synthHistory, joiner *History) {
	t.Helper()
	reference := bootstrapTestHistory(t)
	for _, m := range s.manifests {
		plan, err := reference.PlanReconcile(m, s.blobs, synthMerger{}, "replay")
		if err != nil {
			t.Fatal(err)
		}
		applyBootstrapTestPlan(t, plan)
	}
	want, err := reference.LoadState()
	if err != nil {
		t.Fatal(err)
	}
	got, err := joiner.LoadState()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(want.Tracked, got.Tracked) || !reflect.DeepEqual(want.Received, got.Received) || !reflect.DeepEqual(want.Heads, got.Heads) {
		t.Fatalf("joiner differs\nreplay: %+v\njoiner: %+v", want, got)
	}
	for rel := range want.Tracked {
		left, _ := os.ReadFile(reference.abs(rel))
		right, readErr := os.ReadFile(joiner.abs(rel))
		if readErr != nil || !bytes.Equal(left, right) {
			t.Fatalf("workspace differs at %s: %v", rel, readErr)
		}
	}
	x, err := reference.OpenConflicts(synthMerger{})
	if err != nil {
		t.Fatal(err)
	}
	y, err := joiner.OpenConflicts(synthMerger{})
	if err != nil {
		t.Fatal(err)
	}
	if len(x) != len(y) {
		t.Fatalf("conflicts differ: replay %d, joiner %d", len(x), len(y))
	}
	for i := range x {
		if x[i].Path != y[i].Path || x[i].ResultBlob == nil != (y[i].ResultBlob == nil) {
			t.Fatalf("conflict %d differs: %+v vs %+v", i, x[i], y[i])
		}
	}
}
