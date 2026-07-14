package transaction

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCommit_AppliesPreparedWrite confirms an online Commit lands a prepared
// write on its target and reclaims the staged artifact and manifest, leaving the
// journal clean — the single-artifact roll-forward for the non-crash path.
func TestCommit_AppliesPreparedWrite(t *testing.T) {
	root := t.TempDir()
	j := newTestJournal(t, root)

	target := filepath.Join(root, "projects", "a", "tasks.md")
	m := prepareWrite(t, j, target, "committed content")

	if err := j.Commit(m); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if got := mustReadFile(t, target); got != "committed content" {
		t.Fatalf("target content = %q, want %q", got, "committed content")
	}
	if m.State != StateCommitted {
		t.Fatalf("manifest state = %q, want %q", m.State, StateCommitted)
	}
	assertMissing(t, m.Ops[0].Staged)
	assertMissing(t, j.ManifestPath(m.ID))
}

// TestCommit_MultipleArtifactsAtomic confirms a changeset's project write and its
// configuration artifacts all land in the same Commit.
func TestCommit_MultipleArtifactsAtomic(t *testing.T) {
	root := t.TempDir()
	j := newTestJournal(t, root)

	tasks := filepath.Join(root, "projects", "a", "tasks.md")
	artifact := filepath.Join(root, ".tt", "planning.json")
	m, err := j.Prepare([]Op{
		{Kind: OpWrite, Target: tasks, Data: []byte("# tasks")},
		{Kind: OpWrite, Target: artifact, Data: []byte("{}")},
	})
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if err := j.Commit(m); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if got := mustReadFile(t, tasks); got != "# tasks" {
		t.Fatalf("tasks content = %q", got)
	}
	if got := mustReadFile(t, artifact); got != "{}" {
		t.Fatalf("artifact content = %q", got)
	}
	assertMissing(t, j.ManifestPath(m.ID))
}

// TestCommit_RollsBackStagingOnPrepareFailure confirms that when staging one
// artifact fails, Prepare removes the staging already written for earlier
// artifacts and leaves no manifest — the single-artifact rollback path. A failed
// preparation commits nothing.
func TestCommit_RollsBackStagingOnPrepareFailure(t *testing.T) {
	root := t.TempDir()
	j := newTestJournal(t, root)

	first := filepath.Join(root, "projects", "a", "tasks.md")
	second := filepath.Join(root, "projects", "b", "tasks.md")

	// Fail staging of the second artifact only; the first has already been staged.
	j.writeFile = func(path string, data []byte, perm os.FileMode) error {
		if strings.Contains(path, filepath.Join("projects", "b")) {
			return fmt.Errorf("injected staging failure")
		}
		return os.WriteFile(path, data, perm)
	}

	_, err := j.Prepare([]Op{
		{Kind: OpWrite, Target: first, Data: []byte("one")},
		{Kind: OpWrite, Target: second, Data: []byte("two")},
	})
	if err == nil {
		t.Fatal("expected Prepare to fail")
	}

	// Rollback: the first artifact's staging must be gone, no target written, and
	// no manifest left behind.
	firstStage := j.stagePath(first, "txn-001", 0)
	assertMissing(t, firstStage)
	assertMissing(t, first)
	entries, _ := os.ReadDir(j.dir)
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), manifestSuffix) {
			t.Fatalf("unexpected manifest left after failed prepare: %s", e.Name())
		}
	}
}

// TestCommit_CrashMidApplyRollsForwardOnRecover confirms a Commit interrupted
// after some ops applied leaves a durable committing manifest that Recover
// re-drives to completion — a committed changeset is never lost once prepared.
func TestCommit_CrashMidApplyRollsForwardOnRecover(t *testing.T) {
	root := t.TempDir()
	j := newTestJournal(t, root)

	a := filepath.Join(root, "projects", "a", "tasks.md")
	b := filepath.Join(root, "projects", "b", "tasks.md")
	m, err := j.Prepare([]Op{
		{Kind: OpWrite, Target: a, Data: []byte("aaa")},
		{Kind: OpWrite, Target: b, Data: []byte("bbb")},
	})
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}

	// Simulate a crash while applying the second op: the first rename lands, the
	// second fails, so Commit returns an error with the manifest left committing.
	j.rename = func(oldpath, newpath string) error {
		if strings.Contains(newpath, filepath.Join("projects", "b")) {
			return fmt.Errorf("injected crash renaming %q", newpath)
		}
		return os.Rename(oldpath, newpath)
	}
	if commitErr := j.Commit(m); commitErr == nil {
		t.Fatal("expected Commit to fail mid-apply")
	}
	if got := mustReadFile(t, a); got != "aaa" {
		t.Fatalf("first target = %q, want applied", got)
	}
	assertMissing(t, b)

	// The durable manifest is left in committing state for recovery to finish.
	onDisk, err := LoadManifest(j.ManifestPath(m.ID))
	if err != nil {
		t.Fatalf("LoadManifest after crash: %v", err)
	}
	if onDisk.State != StateCommitting {
		t.Fatalf("manifest state after crash = %q, want %q", onDisk.State, StateCommitting)
	}

	// Recovery with the fault cleared rolls the transaction forward to completion.
	j.rename = os.Rename
	report, err := j.Recover()
	if err != nil {
		t.Fatalf("Recover: %v", err)
	}
	if got := mustReadFile(t, b); got != "bbb" {
		t.Fatalf("second target after recovery = %q, want %q", got, "bbb")
	}
	if got := mustReadFile(t, a); got != "aaa" {
		t.Fatalf("first target after recovery = %q, want unchanged", got)
	}
	if len(report.RolledForward) != 1 || report.RolledForward[0] != m.ID {
		t.Fatalf("RolledForward = %v, want [%s]", report.RolledForward, m.ID)
	}
	assertMissing(t, j.ManifestPath(m.ID))
}

// TestCommit_RefusesInvalidState confirms Commit rejects a nil manifest and a
// manifest that is not prepared or committing.
func TestCommit_RefusesInvalidState(t *testing.T) {
	root := t.TempDir()
	j := newTestJournal(t, root)

	if err := j.Commit(nil); !errors.Is(err, ErrInvalidManifest) {
		t.Fatalf("Commit(nil) err = %v, want ErrInvalidManifest", err)
	}

	target := filepath.Join(root, "projects", "a", "tasks.md")
	m := prepareWrite(t, j, target, "content")
	m.State = StateCommitted
	if err := j.Commit(m); !errors.Is(err, ErrInvalidManifest) {
		t.Fatalf("Commit(committed) err = %v, want ErrInvalidManifest", err)
	}
}

// TestRenameRecoveryConvergesAtEveryCommitBoundary exercises the durable
// directory-move + destination-write choreography at each boundary where a
// process can stop. A fresh journal must always roll forward without quarantine,
// preserving the moved directory and landing the rewritten tasks.md exactly once.
func TestRenameRecoveryConvergesAtEveryCommitBoundary(t *testing.T) {
	tests := []struct {
		name   string
		inject func(*Journal, *Manifest, string, string)
	}{
		{
			name: "mark-committing",
			inject: func(j *Journal, _ *Manifest, _, _ string) {
				realWrite := j.writeFile
				j.writeFile = func(path string, data []byte, perm os.FileMode) error {
					if strings.HasSuffix(path, manifestSuffix) && strings.Contains(string(data), `"state": "committing"`) {
						return fmt.Errorf("injected crash marking committing")
					}
					return realWrite(path, data, perm)
				}
			},
		},
		{
			name: "directory-move",
			inject: func(j *Journal, _ *Manifest, oldPath, newPath string) {
				realRename := j.rename
				j.rename = func(oldName, newName string) error {
					if oldName == oldPath && newName == newPath {
						return fmt.Errorf("injected crash moving project")
					}
					return realRename(oldName, newName)
				}
			},
		},
		{
			name: "tasks-rewrite",
			inject: func(j *Journal, _ *Manifest, _, newPath string) {
				realRename := j.rename
				j.rename = func(oldName, newName string) error {
					if newName == filepath.Join(newPath, "tasks.md") {
						return fmt.Errorf("injected crash rewriting tasks")
					}
					return realRename(oldName, newName)
				}
			},
		},
		{
			name: "mark-committed",
			inject: func(j *Journal, _ *Manifest, _, _ string) {
				realWrite := j.writeFile
				j.writeFile = func(path string, data []byte, perm os.FileMode) error {
					if strings.HasSuffix(path, manifestSuffix) && strings.Contains(string(data), `"state": "committed"`) {
						return fmt.Errorf("injected crash marking committed")
					}
					return realWrite(path, data, perm)
				}
			},
		},
		{
			name: "manifest-reclaim",
			inject: func(j *Journal, m *Manifest, _, _ string) {
				realRemove := j.remove
				j.remove = func(path string) error {
					if path == j.ManifestPath(m.ID) {
						return fmt.Errorf("injected crash reclaiming manifest")
					}
					return realRemove(path)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			oldPath := filepath.Join(root, "projects", "alpha")
			newPath := filepath.Join(root, "projects", "beta")
			if err := os.MkdirAll(filepath.Join(oldPath, ".attachments", "task"), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(oldPath, "tasks.md"), []byte("# Alpha\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(oldPath, ".attachments", "task", "note.txt"), []byte("keep"), 0o600); err != nil {
				t.Fatal(err)
			}

			j := newTestJournal(t, root)
			m, err := j.Prepare([]Op{
				{Kind: OpMove, Source: oldPath, Target: newPath},
				{Kind: OpWrite, Target: filepath.Join(newPath, "tasks.md"), Data: []byte("# Beta\n")},
			})
			if err != nil {
				t.Fatalf("Prepare: %v", err)
			}
			tc.inject(j, m, oldPath, newPath)
			if err := j.Commit(m); err == nil {
				t.Fatal("Commit unexpectedly crossed injected crash boundary")
			}

			restarted := NewJournal(filepath.Join(root, ".tt", "transactions"), []string{root})
			report, err := restarted.Recover()
			if err != nil {
				t.Fatalf("Recover: %v", err)
			}
			if report.Blocked() {
				t.Fatalf("rename recovery quarantined: %+v", report.Quarantined)
			}
			assertMissing(t, oldPath)
			if got := mustReadFile(t, filepath.Join(newPath, "tasks.md")); got != "# Beta\n" {
				t.Fatalf("renamed tasks = %q, want rewritten content", got)
			}
			if got := mustReadFile(t, filepath.Join(newPath, ".attachments", "task", "note.txt")); got != "keep" {
				t.Fatalf("moved attachment = %q, want preserved content", got)
			}
			assertMissing(t, restarted.ManifestPath(m.ID))
		})
	}
}
