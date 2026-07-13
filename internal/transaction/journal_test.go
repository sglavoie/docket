package transaction

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/util"
)

// newTestJournal builds a Journal whose transactions directory is under root,
// with root itself as the sole approved workspace root, and deterministic
// id/clock seams so tests can assert exact paths and timestamps.
func newTestJournal(t *testing.T, root string) *Journal {
	t.Helper()
	j := NewJournal(filepath.Join(root, ".tt", "transactions"), []string{root})
	var seq int
	j.newID = func() string {
		seq++
		return fmt.Sprintf("txn-%03d", seq)
	}
	j.now = func() time.Time { return time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC) }
	return j
}

func TestPrepare_StagesBytesAndWritesManifest(t *testing.T) {
	root := t.TempDir()
	j := newTestJournal(t, root)

	target := filepath.Join(root, "projects", "a", "tasks.md")
	prepared, err := j.Prepare([]Op{
		{Kind: OpWrite, Target: target, Data: []byte("staged content"), Mode: 0o600},
	})
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}

	if prepared.State != StatePrepared {
		t.Fatalf("state = %q, want %q", prepared.State, StatePrepared)
	}

	// The manifest is durably written.
	if _, statErr := os.Stat(j.ManifestPath(prepared.ID)); statErr != nil {
		t.Fatalf("manifest not written: %v", statErr)
	}

	// Exactly one staged file, beside the target, holding the replacement bytes.
	if len(prepared.Ops) != 1 {
		t.Fatalf("op count = %d, want 1", len(prepared.Ops))
	}
	staged := prepared.Ops[0].Staged
	if filepath.Dir(staged) != filepath.Dir(target) {
		t.Fatalf("staged file %q not beside target %q", staged, target)
	}
	if !strings.HasPrefix(filepath.Base(staged), stagePrefix) {
		t.Fatalf("staged file %q missing recognizable prefix %q", staged, stagePrefix)
	}
	got, err := os.ReadFile(staged)
	if err != nil {
		t.Fatalf("reading staged file: %v", err)
	}
	if string(got) != "staged content" {
		t.Fatalf("staged bytes = %q, want %q", got, "staged content")
	}

	// The target itself is untouched: preparation stages, it does not commit.
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("target should not exist after prepare, stat err = %v", err)
	}
}

func TestPrepare_DefaultsWriteMode(t *testing.T) {
	root := t.TempDir()
	j := newTestJournal(t, root)
	prepared, err := j.Prepare([]Op{
		{Kind: OpWrite, Target: filepath.Join(root, "f.md"), Data: []byte("x")},
	})
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if prepared.Ops[0].Mode != util.FilePermissions {
		t.Fatalf("mode = %v, want default %v", prepared.Ops[0].Mode, os.FileMode(util.FilePermissions))
	}
}

func TestPrepare_EmptyTransaction(t *testing.T) {
	j := newTestJournal(t, t.TempDir())
	_, err := j.Prepare(nil)
	if !errors.Is(err, ErrEmptyTransaction) {
		t.Fatalf("Prepare(nil) = %v, want ErrEmptyTransaction", err)
	}
}

func TestPrepare_RejectsTargetOutsideRoot(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	j := newTestJournal(t, root)

	_, err := j.Prepare([]Op{
		{Kind: OpWrite, Target: filepath.Join(outside, "evil.md"), Data: []byte("x")},
	})
	if !errors.Is(err, ErrTargetOutsideRoot) {
		t.Fatalf("Prepare outside root = %v, want ErrTargetOutsideRoot", err)
	}
	// Rejected before any manifest is written.
	assertNoManifests(t, j)
	// And no staging file leaked into the outside directory.
	assertNoStageFiles(t, outside)
}

func TestPrepare_RejectsTraversalTarget(t *testing.T) {
	root := t.TempDir()
	j := newTestJournal(t, root)
	// An absolute path that lexically climbs out of root via "..".
	escape := filepath.Join(root, "..", "escape.md")
	_, err := j.Prepare([]Op{
		{Kind: OpWrite, Target: escape, Data: []byte("x")},
	})
	if !errors.Is(err, ErrTargetOutsideRoot) {
		t.Fatalf("Prepare traversal = %v, want ErrTargetOutsideRoot", err)
	}
}

func TestPrepare_RejectsMoveSourceOutsideRoot(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	j := newTestJournal(t, root)
	_, err := j.Prepare([]Op{
		{Kind: OpMove, Target: filepath.Join(root, "dst.md"), Source: filepath.Join(outside, "src.md")},
	})
	if !errors.Is(err, ErrTargetOutsideRoot) {
		t.Fatalf("Prepare move-source-outside = %v, want ErrTargetOutsideRoot", err)
	}
	assertNoManifests(t, j)
}

func TestPrepare_RejectsUnknownKind(t *testing.T) {
	root := t.TempDir()
	j := newTestJournal(t, root)
	_, err := j.Prepare([]Op{
		{Kind: "frob", Target: filepath.Join(root, "f.md")},
	})
	if !errors.Is(err, ErrUnknownOpKind) {
		t.Fatalf("Prepare unknown kind = %v, want ErrUnknownOpKind", err)
	}
}

func TestPrepare_RejectsMoveWithoutSource(t *testing.T) {
	root := t.TempDir()
	j := newTestJournal(t, root)
	_, err := j.Prepare([]Op{
		{Kind: OpMove, Target: filepath.Join(root, "dst.md")},
	})
	if !errors.Is(err, ErrInvalidOp) {
		t.Fatalf("Prepare move-without-source = %v, want ErrInvalidOp", err)
	}
}

// TestPrepare_StagingFailureRollsBack drives a mid-staging failure and confirms
// earlier staged files are removed and no manifest is written.
func TestPrepare_StagingFailureRollsBack(t *testing.T) {
	root := t.TempDir()
	j := newTestJournal(t, root)

	first := filepath.Join(root, "projects", "a", "one.md")
	second := filepath.Join(root, "projects", "a", "two.md")

	realWrite := j.writeFile
	j.writeFile = func(path string, data []byte, perm os.FileMode) error {
		if strings.Contains(filepath.Base(path), "-001.stage") {
			return fmt.Errorf("boom staging second op")
		}
		return realWrite(path, data, perm)
	}

	_, err := j.Prepare([]Op{
		{Kind: OpWrite, Target: first, Data: []byte("one")},
		{Kind: OpWrite, Target: second, Data: []byte("two")},
	})
	if err == nil {
		t.Fatal("expected staging failure error")
	}

	// The first op's staged file must have been cleaned up.
	assertNoStageFiles(t, filepath.Dir(first))
	// No manifest was written.
	assertNoManifests(t, j)
}

// TestPrepare_ManifestWriteFailureRollsBack fails only the manifest write and
// confirms all staged files are swept.
func TestPrepare_ManifestWriteFailureRollsBack(t *testing.T) {
	root := t.TempDir()
	j := newTestJournal(t, root)

	target := filepath.Join(root, "projects", "a", "tasks.md")
	realWrite := j.writeFile
	j.writeFile = func(path string, data []byte, perm os.FileMode) error {
		if strings.HasSuffix(path, manifestSuffix) {
			return fmt.Errorf("boom manifest write")
		}
		return realWrite(path, data, perm)
	}

	_, err := j.Prepare([]Op{
		{Kind: OpWrite, Target: target, Data: []byte("content")},
	})
	if err == nil {
		t.Fatal("expected manifest write failure error")
	}
	assertNoStageFiles(t, filepath.Dir(target))
	assertNoManifests(t, j)
}

func TestList_ReturnsPreparedManifests(t *testing.T) {
	root := t.TempDir()
	j := newTestJournal(t, root)

	for i := 0; i < 3; i++ {
		if _, err := j.Prepare([]Op{
			{Kind: OpWrite, Target: filepath.Join(root, fmt.Sprintf("f%d.md", i)), Data: []byte("x")},
		}); err != nil {
			t.Fatalf("Prepare %d: %v", i, err)
		}
	}

	manifests, err := j.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(manifests) != 3 {
		t.Fatalf("List returned %d manifests, want 3", len(manifests))
	}
}

func TestList_MissingDirIsEmpty(t *testing.T) {
	j := newTestJournal(t, t.TempDir())
	manifests, err := j.List()
	if err != nil {
		t.Fatalf("List on missing dir: %v", err)
	}
	if len(manifests) != 0 {
		t.Fatalf("List = %d manifests, want 0", len(manifests))
	}
}

func assertNoManifests(t *testing.T, j *Journal) {
	t.Helper()
	manifests, err := j.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(manifests) != 0 {
		t.Fatalf("expected no manifests, found %d", len(manifests))
	}
}

func assertNoStageFiles(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return
	}
	if err != nil {
		t.Fatalf("ReadDir %q: %v", dir, err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), stagePrefix) {
			t.Fatalf("leftover staging file: %s", e.Name())
		}
	}
}
