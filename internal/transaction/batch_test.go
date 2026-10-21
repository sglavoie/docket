package transaction

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/util"
)

type observedBatch struct {
	*util.DurableBatch
	flush func() error
}

func (b *observedBatch) Flush() error {
	if err := b.flush(); err != nil {
		return err
	}
	return b.DurableBatch.Flush()
}

func batchTestOps(root string) []Op {
	return []Op{
		{Kind: OpWrite, Target: filepath.Join(root, "a", "one"), Data: []byte("one")},
		{Kind: OpWrite, Target: filepath.Join(root, "b", "two"), Data: []byte("two")},
	}
}

func TestBatchedWritesFlushBeforePublishingJournalMarkers(t *testing.T) {
	root := t.TempDir()
	j := newTestJournal(t, root).WithBatchedWrites()
	created, flushed := 0, 0
	j.newBatch = func() fileBatch {
		created++
		return &observedBatch{util.NewDurableBatch(), func() error { flushed++; return nil }}
	}
	write := j.writeFile
	j.writeFile = func(path string, data []byte, mode os.FileMode) error {
		want := 1
		if strings.Contains(string(data), `"state": "committed"`) {
			want = 2
		}
		if flushed != want {
			t.Fatalf("journal marker preceded its durability barrier: %s, flushes %d", data, flushed)
		}
		return write(path, data, mode)
	}
	m, err := j.Prepare(batchTestOps(root))
	if err != nil {
		t.Fatal(err)
	}
	if err := j.Commit(m); err != nil {
		t.Fatal(err)
	}
	if created != 2 || flushed != 2 {
		t.Fatalf("wanted one prepare and one apply barrier: %d, %d", created, flushed)
	}
}

func TestBatchedPrepareFlushFailurePublishesNothing(t *testing.T) {
	root := t.TempDir()
	j := newTestJournal(t, root).WithBatchedWrites()
	injected := errors.New("staging barrier failed")
	j.newBatch = func() fileBatch {
		return &observedBatch{util.NewDurableBatch(), func() error { return injected }}
	}
	ops := batchTestOps(root)
	if _, err := j.Prepare(ops); !errors.Is(err, injected) {
		t.Fatalf("flush failure hidden: %v", err)
	}
	assertMissing(t, j.ManifestPath("txn-001"))
	for i, op := range ops {
		assertMissing(t, op.Target)
		assertMissing(t, j.stagePath(op.Target, "txn-001", i))
	}
}

func TestBatchedApplyFlushFailureRecoversWithOrdinaryJournal(t *testing.T) {
	root := t.TempDir()
	j := newTestJournal(t, root).WithBatchedWrites()
	ops := batchTestOps(root)
	m, err := j.Prepare(ops)
	if err != nil {
		t.Fatal(err)
	}
	injected := errors.New("applied directory barrier failed")
	j.newBatch = func() fileBatch {
		return &observedBatch{util.NewDurableBatch(), func() error { return injected }}
	}
	if err := j.Commit(m); !errors.Is(err, injected) {
		t.Fatalf("flush failure hidden: %v", err)
	}
	onDisk, err := LoadManifest(j.ManifestPath(m.ID))
	if err != nil || onDisk.State != StateCommitting {
		t.Fatalf("failed barrier was acknowledged: %+v, %v", onDisk, err)
	}
	restarted := newTestJournal(t, root)
	synced := map[string]bool{}
	restarted.syncDir = func(path string) error { synced[path] = true; return util.SyncDir(path) }
	report, err := restarted.Recover()
	if err != nil || len(report.Quarantined) != 0 || len(report.RolledForward) != 1 {
		t.Fatalf("recovery: %+v, %v", report, err)
	}
	for _, op := range ops {
		if got := mustReadFile(t, op.Target); got != string(op.Data) {
			t.Fatalf("recovered %s: %q", op.Target, got)
		}
		if !synced[filepath.Dir(op.Target)] {
			t.Fatalf("already-applied rename was not made durable: %s", op.Target)
		}
	}
	assertMissing(t, j.ManifestPath(m.ID))
}

func TestBatchedWritesKeepRepeatedTargetsOrdered(t *testing.T) {
	root := t.TempDir()
	j := newTestJournal(t, root).WithBatchedWrites()
	target := filepath.Join(root, "same")
	m, err := j.Prepare([]Op{{Kind: OpWrite, Target: target, Data: []byte("first")},
		{Kind: OpWrite, Target: target, Data: []byte("second")}})
	if err != nil {
		t.Fatal(err)
	}
	j.newBatch = func() fileBatch { t.Fatal("overlapping writes cannot share the apply barrier"); return nil }
	if err := j.Commit(m); err != nil {
		t.Fatal(err)
	}
	if got := mustReadFile(t, target); got != "second" {
		t.Fatalf("write ordering lost: %q", got)
	}
}

func TestIndependentWritesRefusesAliasesAndNestedTargets(t *testing.T) {
	root := t.TempDir()
	real := filepath.Join(root, "real")
	if err := os.Mkdir(real, 0700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "alias")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	for name, targets := range map[string][]string{
		"symlink":  {filepath.Join(real, "file"), filepath.Join(link, "file")},
		"case":     {filepath.Join(real, "file"), filepath.Join(real, "FILE")},
		"ancestor": {real, filepath.Join(real, "file")},
		"unicode":  {filepath.Join(real, "café"), filepath.Join(real, "café")},
	} {
		t.Run(name, func(t *testing.T) {
			ops := []FileOp{
				{Kind: OpWrite, Target: targets[0], Staged: filepath.Join(root, "stage-one")},
				{Kind: OpWrite, Target: targets[1], Staged: filepath.Join(root, "stage-two")},
			}
			if independentWrites(ops) {
				t.Fatal("dependent paths accepted")
			}
		})
	}
}

func TestBatchedWritesKeepDirectoryMovePreparationAndRecovery(t *testing.T) {
	root := t.TempDir()
	j := newTestJournal(t, root).WithBatchedWrites()
	j.newBatch = func() fileBatch { t.Fatal("mixed transaction must use ordinary barriers"); return nil }
	old, next := filepath.Join(root, "old"), filepath.Join(root, "next")
	if err := os.Mkdir(old, 0700); err != nil {
		t.Fatal(err)
	}
	m, err := j.Prepare([]Op{{Kind: OpMove, Source: old, Target: next},
		{Kind: OpWrite, Target: filepath.Join(next, "file"), Data: []byte("moved")}})
	if err != nil {
		t.Fatal(err)
	}
	if err := j.Commit(m); err != nil {
		t.Fatal(err)
	}
	if got := mustReadFile(t, filepath.Join(next, "file")); got != "moved" {
		t.Fatal(got)
	}
}

func TestIndependentWritesFindsAncestorsAcrossLexicalNeighbors(t *testing.T) {
	root := t.TempDir()
	ancestor := filepath.Join(root, "a")
	if err := os.Mkdir(ancestor, 0700); err != nil {
		t.Fatal(err)
	}
	var ops []FileOp
	for i, target := range []string{ancestor, filepath.Join(root, "a-between"), filepath.Join(ancestor, "child")} {
		ops = append(ops, FileOp{Kind: OpWrite, Target: target, Staged: filepath.Join(root, string(rune('x'+i)))})
	}
	if independentWrites(ops) {
		t.Fatal("a non-adjacent ancestor was overlooked")
	}
}
