package storage

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/transaction"
	"github.com/sglavoie/tt/internal/tterrors"
)

// newLifecycleTestProject creates a project physically on disk under the canonical
// <data>/projects/<slug> layout with a tasks.md and a nested attachments file, so
// the whole directory is moved by the trash/restore transactions.
func newLifecycleTestProject(t *testing.T, dataDir, slug, name string) *models.Project {
	t.Helper()
	p, err := models.NewProject(uuid.New().String(), slug, "")
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}
	p.Slug = slug
	p.Name = name
	p.Path = filepath.Join(dataDir, "projects", slug)
	if err := os.MkdirAll(filepath.Join(p.Path, ".attachments", "abc"), DirPermissions); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}
	if err := os.WriteFile(tasksFilePath(p.Path), []byte("# "+name+"\n"), 0o600); err != nil {
		t.Fatalf("write tasks.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(p.Path, ".attachments", "abc", "note.txt"), []byte("hi"), 0o600); err != nil {
		t.Fatalf("write attachment: %v", err)
	}
	return p
}

func lifecycleService(t *testing.T) (*CommitService, string) {
	t.Helper()
	dataDir := t.TempDir()
	return NewCommitService(filepath.Join(dataDir, ".tt", "transactions"), []string{dataDir}), dataDir
}

// TestProjectTrashRestoreRoundTripCommitsAtomically proves that a trash plan plus a
// config-artifact op commits atomically through CommitOps (project directory moved
// into a versioned trash entry, tombstone and config written), and that a restore
// plan reverses it (data back under projects/, entry drained), preserving the whole
// nested directory across both moves.
func TestProjectTrashRestoreRoundTripCommitsAtomically(t *testing.T) {
	svc, dataDir := lifecycleService(t)
	project := newLifecycleTestProject(t, dataDir, "alpha", "Alpha")
	configPath := filepath.Join(dataDir, "config.json")
	if err := os.WriteFile(configPath, []byte(`{"v":"before"}`), 0o600); err != nil {
		t.Fatalf("seed config: %v", err)
	}

	plan, err := PlanTrash(project, dataDir, "deleted via test", nil)
	if err != nil {
		t.Fatalf("PlanTrash: %v", err)
	}
	ops := append(plan.Ops, transaction.Op{Kind: transaction.OpWrite, Target: configPath, Data: []byte(`{"v":"after-delete"}`), Mode: FilePermissions})
	if commitErr := svc.CommitOps(ops); commitErr != nil {
		t.Fatalf("CommitOps(trash): %v", commitErr)
	}

	if _, statErr := os.Stat(project.Path); !os.IsNotExist(statErr) {
		t.Fatalf("expected project directory to be moved out of projects/, stat err = %v", statErr)
	}
	movedTasks := filepath.Join(plan.EntryPath, trashDataDirName, tasksFileName)
	if _, statErr := os.Stat(movedTasks); statErr != nil {
		t.Fatalf("expected moved tasks.md in trash entry: %v", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(plan.EntryPath, trashTombstoneFile)); statErr != nil {
		t.Fatalf("expected tombstone in trash entry: %v", statErr)
	}
	if got, _ := os.ReadFile(configPath); string(got) != `{"v":"after-delete"}` {
		t.Fatalf("config not rewritten atomically: %q", got)
	}

	// The nested attachment survived the move.
	if _, statErr := os.Stat(filepath.Join(plan.EntryPath, trashDataDirName, ".attachments", "abc", "note.txt")); statErr != nil {
		t.Fatalf("expected nested attachment preserved in trash: %v", statErr)
	}

	rplan, err := PlanRestore(dataDir, plan.EntryID)
	if err != nil {
		t.Fatalf("PlanRestore: %v", err)
	}
	if err := svc.CommitOps(rplan.Ops); err != nil {
		t.Fatalf("CommitOps(restore): %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(dataDir, "projects", "alpha", tasksFileName)); statErr != nil {
		t.Fatalf("expected project restored under projects/: %v", statErr)
	}
	if _, statErr := os.Stat(plan.EntryPath); !os.IsNotExist(statErr) {
		t.Fatalf("expected trash entry removed after restore, stat err = %v", statErr)
	}
	if rplan.Project.Slug != "alpha" {
		t.Fatalf("restored project slug = %q, want alpha", rplan.Project.Slug)
	}
}

// TestPlanTrashRejectsInvalidProject covers the identity guards.
func TestPlanTrashRejectsInvalidProject(t *testing.T) {
	_, dataDir := lifecycleService(t)
	if _, err := PlanTrash(nil, dataDir, "", nil); err == nil {
		t.Fatal("expected error for nil project")
	}
	bad := &models.Project{ID: "id", Slug: "../escape", Path: filepath.Join(dataDir, "projects", "x")}
	if _, err := PlanTrash(bad, dataDir, "", nil); err == nil {
		t.Fatal("expected error for unsafe slug")
	}
}

// TestPlanRestoreMissingEntry surfaces a not-found trash entry.
func TestPlanRestoreMissingEntry(t *testing.T) {
	_, dataDir := lifecycleService(t)
	if _, err := PlanRestore(dataDir, "does-not-exist"); !errors.Is(err, tterrors.ErrProjectNotFound) {
		t.Fatalf("PlanRestore(missing) err = %v, want ErrProjectNotFound", err)
	}
}

// TestPlanRestoreDestinationOccupied refuses to restore over an active project that
// already occupies the destination slug.
func TestPlanRestoreDestinationOccupied(t *testing.T) {
	svc, dataDir := lifecycleService(t)
	project := newLifecycleTestProject(t, dataDir, "beta", "Beta")
	plan, err := PlanTrash(project, dataDir, "deleted", nil)
	if err != nil {
		t.Fatalf("PlanTrash: %v", err)
	}
	if commitErr := svc.CommitOps(plan.Ops); commitErr != nil {
		t.Fatalf("CommitOps(trash): %v", commitErr)
	}
	// Recreate an active project at the destination slug.
	newLifecycleTestProject(t, dataDir, "beta", "Beta Reborn")

	if _, err := PlanRestore(dataDir, plan.EntryID); !errors.Is(err, tterrors.ErrProjectExists) {
		t.Fatalf("PlanRestore(occupied) err = %v, want ErrProjectExists", err)
	}
}

// TestPlanPurgeRemovesEntryAndNoopsWhenMissing proves purge enumerates and removes
// the whole trash entry subtree, and that a missing entry yields no operations.
func TestPlanPurgeRemovesEntryAndNoopsWhenMissing(t *testing.T) {
	svc, dataDir := lifecycleService(t)
	project := newLifecycleTestProject(t, dataDir, "gamma", "Gamma")
	plan, err := PlanTrash(project, dataDir, "deleted", nil)
	if err != nil {
		t.Fatalf("PlanTrash: %v", err)
	}
	if commitErr := svc.CommitOps(plan.Ops); commitErr != nil {
		t.Fatalf("CommitOps(trash): %v", commitErr)
	}

	purgeOps, err := PlanPurge(dataDir, plan.EntryID)
	if err != nil {
		t.Fatalf("PlanPurge: %v", err)
	}
	if len(purgeOps) == 0 {
		t.Fatal("expected purge operations for a populated entry")
	}
	if commitErr := svc.CommitOps(purgeOps); commitErr != nil {
		t.Fatalf("CommitOps(purge): %v", commitErr)
	}
	if _, statErr := os.Stat(plan.EntryPath); !os.IsNotExist(statErr) {
		t.Fatalf("expected trash entry purged, stat err = %v", statErr)
	}

	missingOps, err := PlanPurge(dataDir, "not-here")
	if err != nil {
		t.Fatalf("PlanPurge(missing): %v", err)
	}
	if len(missingOps) != 0 {
		t.Fatalf("expected no-op purge for missing entry, got %d ops", len(missingOps))
	}
}

// TestTrashCommitAbortsAtomicallyOnInvalidOp proves the delete is atomic: when a
// co-committed op targets a path outside the approved roots, the whole transaction
// is rejected during Prepare and the project directory is never moved.
func TestTrashCommitAbortsAtomicallyOnInvalidOp(t *testing.T) {
	svc, dataDir := lifecycleService(t)
	project := newLifecycleTestProject(t, dataDir, "delta", "Delta")

	plan, err := PlanTrash(project, dataDir, "deleted", nil)
	if err != nil {
		t.Fatalf("PlanTrash: %v", err)
	}
	// Inject a config-write op whose target escapes the workspace root.
	outside := filepath.Join(t.TempDir(), "escape.json")
	ops := append(plan.Ops, transaction.Op{Kind: transaction.OpWrite, Target: outside, Data: []byte("nope"), Mode: FilePermissions})

	if err := svc.CommitOps(ops); !errors.Is(err, transaction.ErrTargetOutsideRoot) {
		t.Fatalf("CommitOps err = %v, want ErrTargetOutsideRoot", err)
	}
	// The project directory is untouched — the move never ran.
	if _, statErr := os.Stat(project.Path); statErr != nil {
		t.Fatalf("expected project directory intact after aborted delete: %v", statErr)
	}
	if _, statErr := os.Stat(outside); !os.IsNotExist(statErr) {
		t.Fatalf("expected no file written outside root, stat err = %v", statErr)
	}
}

// TestLifecycleCommitStaleArtifactAbortsBeforeMove proves project lifecycle
// operations preserve a checked domain artifact's optimistic-concurrency
// contract. A concurrent catalog writer wins, and the accompanying trash move
// never starts.
func TestLifecycleCommitStaleArtifactAbortsBeforeMove(t *testing.T) {
	svc, dataDir := lifecycleService(t)
	project := newLifecycleTestProject(t, dataDir, "epsilon", "Epsilon")
	catalogPath := filepath.Join(dataDir, "workspace.json")
	loaded := []byte(`{"projects":["epsilon"]}`)
	if err := os.WriteFile(catalogPath, loaded, FilePermissions); err != nil {
		t.Fatalf("seed catalog: %v", err)
	}
	expected := RawArtifactRevision(loaded)

	plan, err := PlanTrash(project, dataDir, "deleted", nil)
	if err != nil {
		t.Fatalf("PlanTrash: %v", err)
	}
	concurrent := []byte(`{"projects":["epsilon","zeta"]}`)
	if writeErr := os.WriteFile(catalogPath, concurrent, FilePermissions); writeErr != nil {
		t.Fatalf("concurrent catalog write: %v", writeErr)
	}

	err = svc.CommitOpsWithArtifacts(plan.Ops, []ArtifactChange{{
		Path:             catalogPath,
		Data:             []byte(`{"projects":[]}`),
		ExpectedRevision: &expected,
	}})
	var conflict *ArtifactConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("CommitOpsWithArtifacts err = %v, want ArtifactConflictError", err)
	}
	if _, statErr := os.Stat(project.Path); statErr != nil {
		t.Fatalf("project moved despite stale catalog: %v", statErr)
	}
	if got, readErr := os.ReadFile(catalogPath); readErr != nil || string(got) != string(concurrent) {
		t.Fatalf("catalog after conflict = %q, %v; want concurrent bytes", got, readErr)
	}
}

// TestLifecycleArtifactRevisionSourceMustStayWithinRoot covers the rename-only
// ExpectedRevisionPath seam. A lifecycle artifact may validate an old source and
// write a new target, but both paths (and the source's advisory lock) must remain
// inside the approved workspace before the commit creates anything.
func TestLifecycleArtifactRevisionSourceMustStayWithinRoot(t *testing.T) {
	svc, dataDir := lifecycleService(t)
	target := filepath.Join(dataDir, "projects", "beta", "tasks.md")
	outsideDir := t.TempDir()
	outside := filepath.Join(outsideDir, "tasks.md")
	base := []byte("# Outside\n")
	if err := os.WriteFile(outside, base, FilePermissions); err != nil {
		t.Fatalf("seed outside revision source: %v", err)
	}
	expected := RawArtifactRevision(base)

	err := svc.CommitOpsWithArtifacts(nil, []ArtifactChange{{
		Path:                 target,
		Data:                 []byte("# Beta\n"),
		ExpectedRevision:     &expected,
		ExpectedRevisionPath: outside,
	}})
	if !errors.Is(err, transaction.ErrTargetOutsideRoot) {
		t.Fatalf("CommitOpsWithArtifacts err = %v, want ErrTargetOutsideRoot", err)
	}
	if _, statErr := os.Stat(target); !os.IsNotExist(statErr) {
		t.Fatalf("target created despite out-of-root revision source: %v", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(outsideDir, LockFileName)); !os.IsNotExist(statErr) {
		t.Fatalf("outside lock created during rejected preflight: %v", statErr)
	}
}
