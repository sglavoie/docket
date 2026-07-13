package transaction

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// prepareWrite stages a single write and returns the prepared manifest.
func prepareWrite(t *testing.T, j *Journal, target string, data string) *Manifest {
	t.Helper()
	m, err := j.Prepare([]Op{{Kind: OpWrite, Target: target, Data: []byte(data)}})
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	return m
}

func mustReadFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile %q: %v", path, err)
	}
	return string(b)
}

func assertMissing(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected %q to be absent, stat err = %v", path, err)
	}
}

// TestRecover_RollsForwardPreparedWrite confirms a prepared write is applied to
// its target and its staged artifact and manifest are reclaimed.
func TestRecover_RollsForwardPreparedWrite(t *testing.T) {
	root := t.TempDir()
	j := newTestJournal(t, root)

	target := filepath.Join(root, "projects", "a", "tasks.md")
	m := prepareWrite(t, j, target, "hello world")

	report, err := j.Recover()
	if err != nil {
		t.Fatalf("Recover: %v", err)
	}
	if got := mustReadFile(t, target); got != "hello world" {
		t.Fatalf("target content = %q, want %q", got, "hello world")
	}
	assertMissing(t, m.Ops[0].Staged)
	assertMissing(t, j.ManifestPath(m.ID))
	if len(report.RolledForward) != 1 || report.RolledForward[0] != m.ID {
		t.Fatalf("RolledForward = %v, want [%s]", report.RolledForward, m.ID)
	}
	if report.Blocked() {
		t.Fatalf("unexpected quarantine: %+v", report.Quarantined)
	}
}

// TestRecover_RollsForwardMoveAndDelete confirms move and delete ops apply.
func TestRecover_RollsForwardMoveAndDelete(t *testing.T) {
	root := t.TempDir()
	j := newTestJournal(t, root)

	moveSrc := filepath.Join(root, "projects", "a", "old.md")
	moveDst := filepath.Join(root, "projects", "a", "new.md")
	deleteTarget := filepath.Join(root, "projects", "a", "gone.md")
	if err := os.MkdirAll(filepath.Dir(moveSrc), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(moveSrc, []byte("moved"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(deleteTarget, []byte("delete me"), 0o600); err != nil {
		t.Fatal(err)
	}

	m, err := j.Prepare([]Op{
		{Kind: OpMove, Target: moveDst, Source: moveSrc},
		{Kind: OpDelete, Target: deleteTarget},
	})
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}

	if _, err := j.Recover(); err != nil {
		t.Fatalf("Recover: %v", err)
	}
	if got := mustReadFile(t, moveDst); got != "moved" {
		t.Fatalf("move dst = %q, want %q", got, "moved")
	}
	assertMissing(t, moveSrc)
	assertMissing(t, deleteTarget)
	assertMissing(t, j.ManifestPath(m.ID))
}

// TestRecover_ReclaimsCommittedManifest confirms a manifest that crashed after
// being marked committed (but before cleanup) has its artifacts reclaimed and its
// target left untouched.
func TestRecover_ReclaimsCommittedManifest(t *testing.T) {
	root := t.TempDir()
	j := newTestJournal(t, root)

	target := filepath.Join(root, "f.md")
	if err := os.WriteFile(target, []byte("final"), 0o600); err != nil {
		t.Fatal(err)
	}
	m := prepareWrite(t, j, target, "staged-but-applied")
	// Simulate a crash after commit: the write already landed (target has final
	// content) and the manifest advanced to committed, but cleanup never ran.
	m.State = StateCommitted
	if err := j.writeManifest(m); err != nil {
		t.Fatal(err)
	}

	report, err := j.Recover()
	if err != nil {
		t.Fatalf("Recover: %v", err)
	}
	if got := mustReadFile(t, target); got != "final" {
		t.Fatalf("committed target must be untouched, got %q", got)
	}
	assertMissing(t, m.Ops[0].Staged)
	assertMissing(t, j.ManifestPath(m.ID))
	if len(report.CleanedUp) != 1 || report.CleanedUp[0] != m.ID {
		t.Fatalf("CleanedUp = %v, want [%s]", report.CleanedUp, m.ID)
	}
}

// TestRecover_Idempotent confirms a second pass makes no further changes.
func TestRecover_Idempotent(t *testing.T) {
	root := t.TempDir()
	j := newTestJournal(t, root)
	target := filepath.Join(root, "projects", "a", "tasks.md")
	prepareWrite(t, j, target, "content")

	if _, err := j.Recover(); err != nil {
		t.Fatalf("first Recover: %v", err)
	}
	report, err := j.Recover()
	if err != nil {
		t.Fatalf("second Recover: %v", err)
	}
	if len(report.RolledForward) != 0 || len(report.CleanedUp) != 0 ||
		len(report.SweptOrphans) != 0 || report.Blocked() {
		t.Fatalf("second pass not a no-op: %+v", report)
	}
	if got := mustReadFile(t, target); got != "content" {
		t.Fatalf("target content changed on second pass: %q", got)
	}
}

// TestRecover_SweepsOrphanStaging removes a staging file with no referencing
// manifest while leaving one referenced by a prepared manifest intact.
func TestRecover_SweepsOrphanStaging(t *testing.T) {
	root := t.TempDir()
	j := newTestJournal(t, root)

	// An orphan staging file: crash between staging and the manifest write leaves
	// bytes named for a transaction id that has no manifest.
	orphanDir := filepath.Join(root, "projects", "a")
	if err := os.MkdirAll(orphanDir, 0o700); err != nil {
		t.Fatal(err)
	}
	orphan := filepath.Join(orphanDir, stagePrefix+"txn-999-000"+stageSuffix)
	if err := os.WriteFile(orphan, []byte("orphaned"), 0o600); err != nil {
		t.Fatal(err)
	}

	report, err := j.Recover()
	if err != nil {
		t.Fatalf("Recover: %v", err)
	}
	assertMissing(t, orphan)
	if len(report.SweptOrphans) != 1 || report.SweptOrphans[0] != orphan {
		t.Fatalf("SweptOrphans = %v, want [%s]", report.SweptOrphans, orphan)
	}
}

// TestRecover_KeepsReferencedStagingDuringSweep confirms the orphan sweep never
// removes a staging file that a valid manifest still references. This is checked
// by intercepting apply so the roll-forward leaves the staged file in place, then
// asserting the sweep did not delete it.
func TestRecover_KeepsReferencedStagingDuringSweep(t *testing.T) {
	root := t.TempDir()
	j := newTestJournal(t, root)
	target := filepath.Join(root, "projects", "a", "tasks.md")
	m := prepareWrite(t, j, target, "keep me")

	// Fail the rename so roll-forward aborts before the staged file is consumed;
	// the manifest is then quarantined but its id stays protected.
	j.rename = func(oldpath, newpath string) error {
		if strings.HasPrefix(filepath.Base(oldpath), stagePrefix) {
			return fmt.Errorf("injected rename failure")
		}
		return os.Rename(oldpath, newpath)
	}

	report, err := j.Recover()
	if err != nil {
		t.Fatalf("Recover: %v", err)
	}
	if len(report.SweptOrphans) != 0 {
		t.Fatalf("referenced staging swept as orphan: %v", report.SweptOrphans)
	}
	// The staged file survives under the quarantined transaction's protection.
	if _, err := os.Stat(m.Ops[0].Staged); err != nil {
		t.Fatalf("referenced staged file removed: %v", err)
	}
	if !report.Blocked() {
		t.Fatal("apply failure should have quarantined the manifest")
	}
}

// TestRecover_QuarantinesCorruptManifest confirms a corrupt manifest is moved to
// quarantine and blocks further mutation.
func TestRecover_QuarantinesCorruptManifest(t *testing.T) {
	root := t.TempDir()
	j := newTestJournal(t, root)

	if err := os.MkdirAll(j.dir, 0o700); err != nil {
		t.Fatal(err)
	}
	corrupt := filepath.Join(j.dir, "txn-corrupt.json")
	if err := os.WriteFile(corrupt, []byte("{ not valid json"), 0o600); err != nil {
		t.Fatal(err)
	}

	report, err := j.Recover()
	if err != nil {
		t.Fatalf("Recover: %v", err)
	}
	if !report.Blocked() {
		t.Fatal("corrupt manifest should block")
	}
	assertMissing(t, corrupt)
	quarantined := filepath.Join(j.dir, quarantineDirName, "txn-corrupt.json")
	if _, err := os.Stat(quarantined); err != nil {
		t.Fatalf("manifest not moved to quarantine: %v", err)
	}
	if e := report.Err(); !errors.Is(e, ErrRecoveryQuarantine) {
		t.Fatalf("Err() = %v, want ErrRecoveryQuarantine", e)
	}
}

// TestRecover_QuarantinesTamperedStaging confirms a staged file whose bytes no
// longer match the manifest's recorded hash is not applied — the manifest is
// quarantined instead, and the target is never written.
func TestRecover_QuarantinesTamperedStaging(t *testing.T) {
	root := t.TempDir()
	j := newTestJournal(t, root)
	target := filepath.Join(root, "projects", "a", "tasks.md")
	m := prepareWrite(t, j, target, "trusted content")

	// Corrupt the staged bytes after preparation.
	if err := os.WriteFile(m.Ops[0].Staged, []byte("tampered"), 0o600); err != nil {
		t.Fatal(err)
	}

	report, err := j.Recover()
	if err != nil {
		t.Fatalf("Recover: %v", err)
	}
	if !report.Blocked() {
		t.Fatal("tampered staging should quarantine the manifest")
	}
	// The target must never receive the tampered bytes.
	assertMissing(t, target)
	if !strings.Contains(report.Quarantined[0].Reason, "hash") {
		t.Fatalf("quarantine reason should mention hash mismatch: %q", report.Quarantined[0].Reason)
	}
}

// TestRecover_PersistsQuarantineAcrossPasses confirms a quarantined manifest is
// re-reported and its staged bytes are protected on subsequent passes.
func TestRecover_PersistsQuarantineAcrossPasses(t *testing.T) {
	root := t.TempDir()
	j := newTestJournal(t, root)
	target := filepath.Join(root, "projects", "a", "tasks.md")
	m := prepareWrite(t, j, target, "trusted")
	if err := os.WriteFile(m.Ops[0].Staged, []byte("tampered"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := j.Recover(); err != nil {
		t.Fatalf("first Recover: %v", err)
	}
	report, err := j.Recover()
	if err != nil {
		t.Fatalf("second Recover: %v", err)
	}
	if !report.Blocked() || len(report.Quarantined) != 1 {
		t.Fatalf("quarantine not persisted: %+v", report)
	}
	// Its staged bytes remain for inspection and are not swept.
	if len(report.SweptOrphans) != 0 {
		t.Fatalf("quarantined staging swept: %v", report.SweptOrphans)
	}
	if _, err := os.Stat(m.Ops[0].Staged); err != nil {
		t.Fatalf("quarantined staged bytes removed: %v", err)
	}
}

// TestRecover_ResumesAfterMidApplyCrash drives a crash after the committing
// marker is written but before all ops apply, then confirms a fresh pass finishes
// the transaction. This exercises the committing-state roll-forward path.
func TestRecover_ResumesAfterMidApplyCrash(t *testing.T) {
	root := t.TempDir()
	j := newTestJournal(t, root)
	targetA := filepath.Join(root, "projects", "a", "one.md")
	targetB := filepath.Join(root, "projects", "a", "two.md")
	m, err := j.Prepare([]Op{
		{Kind: OpWrite, Target: targetA, Data: []byte("one")},
		{Kind: OpWrite, Target: targetB, Data: []byte("two")},
	})
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}

	// First pass: allow the committing marker and the first op, then crash before
	// the second op by failing its rename.
	j.rename = func(oldpath, newpath string) error {
		if strings.HasSuffix(newpath, "two.md") {
			return fmt.Errorf("injected crash before second op")
		}
		return os.Rename(oldpath, newpath)
	}
	report, err := j.Recover()
	if err != nil {
		t.Fatalf("first Recover: %v", err)
	}
	if !report.Blocked() {
		t.Fatal("mid-apply failure should quarantine on the first pass")
	}

	// The manifest was quarantined; simulate an operator clearing the fault and
	// restoring the manifest to the transactions dir to resume recovery.
	quarantined := filepath.Join(j.dir, quarantineDirName, m.ID+manifestSuffix)
	restored := j.ManifestPath(m.ID)
	if err := os.Rename(quarantined, restored); err != nil {
		t.Fatalf("restoring manifest: %v", err)
	}

	// Second pass with the fault cleared: the first op is already applied (staged
	// gone, target present) and the second now succeeds.
	j.rename = os.Rename
	if _, err := j.Recover(); err != nil {
		t.Fatalf("second Recover: %v", err)
	}
	if got := mustReadFile(t, targetA); got != "one" {
		t.Fatalf("targetA = %q, want %q", got, "one")
	}
	if got := mustReadFile(t, targetB); got != "two" {
		t.Fatalf("targetB = %q, want %q", got, "two")
	}
	assertMissing(t, restored)
}

// TestRecover_ManifestUpdateFailureQuarantines confirms a failure at the
// manifest-update boundary is surfaced as a quarantine rather than a silent loss.
func TestRecover_ManifestUpdateFailureQuarantines(t *testing.T) {
	root := t.TempDir()
	j := newTestJournal(t, root)
	target := filepath.Join(root, "projects", "a", "tasks.md")
	prepareWrite(t, j, target, "content")

	realWrite := j.writeFile
	j.writeFile = func(path string, data []byte, perm os.FileMode) error {
		if strings.HasSuffix(path, manifestSuffix) {
			return fmt.Errorf("injected manifest-update failure")
		}
		return realWrite(path, data, perm)
	}

	report, err := j.Recover()
	if err != nil {
		t.Fatalf("Recover: %v", err)
	}
	if !report.Blocked() {
		t.Fatal("manifest-update failure should quarantine")
	}
}

// TestRecover_RejectsTargetOutsideRoots confirms a manifest naming a target
// outside the approved roots is quarantined rather than applied.
func TestRecover_RejectsTargetOutsideRoots(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	j := newTestJournal(t, root)

	// Build a manifest by hand pointing outside the approved root, then persist it
	// in the transactions dir (bypassing Prepare's validation).
	if err := os.MkdirAll(j.dir, 0o700); err != nil {
		t.Fatal(err)
	}
	m := &Manifest{
		SchemaVersion: SchemaVersion,
		ID:            "txn-escape",
		State:         StatePrepared,
		CreatedAt:     "2026-07-10T00:00:00Z",
		Ops: []FileOp{
			{Kind: OpDelete, Target: filepath.Join(outside, "victim.md")},
		},
	}
	if err := j.writeManifest(m); err != nil {
		t.Fatal(err)
	}

	report, err := j.Recover()
	if err != nil {
		t.Fatalf("Recover: %v", err)
	}
	if !report.Blocked() {
		t.Fatal("out-of-root target should quarantine")
	}
	if !strings.Contains(report.Quarantined[0].Reason, "escapes") {
		t.Fatalf("reason should mention escape: %q", report.Quarantined[0].Reason)
	}
}

func TestParseStageTxnID(t *testing.T) {
	tests := []struct {
		name   string
		file   string
		wantID string
		wantOK bool
	}{
		{"simple id", ".tt-stage-txn-001-000.stage", "txn-001", true},
		{"uuid id", ".tt-stage-550e8400-e29b-41d4-a716-446655440000-012.stage", "550e8400-e29b-41d4-a716-446655440000", true},
		{"wrong prefix", "stage-txn-001-000.stage", "", false},
		{"wrong suffix", ".tt-stage-txn-001-000.tmp", "", false},
		{"non-numeric index", ".tt-stage-txn-001-abc.stage", "", false},
		{"no index", ".tt-stage-txn.stage", "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			id, ok := parseStageTxnID(tc.file)
			if ok != tc.wantOK || id != tc.wantID {
				t.Fatalf("parseStageTxnID(%q) = (%q, %v), want (%q, %v)", tc.file, id, ok, tc.wantID, tc.wantOK)
			}
		})
	}
}
