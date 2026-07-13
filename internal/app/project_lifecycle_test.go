package app_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/transaction"
)

// opsRepo wraps fakeRepo and implements app.OpsCommitter by applying the planned
// operations directly on the real filesystem, so the project-lifecycle use cases —
// which plan real directory moves, tombstone writes, and recursive deletes — commit
// against temp directories. An injected commitErr simulates a transaction failure.
type opsRepo struct {
	*fakeRepo
	commitErr error
	committed [][]transaction.Op
}

func (r *opsRepo) CommitOps(ops []transaction.Op) error {
	if r.commitErr != nil {
		return r.commitErr
	}
	r.committed = append(r.committed, ops)
	for _, op := range ops {
		if err := applyLifecycleOp(op); err != nil {
			return err
		}
	}
	return nil
}

func applyLifecycleOp(op transaction.Op) error {
	switch op.Kind {
	case transaction.OpWrite:
		if err := os.MkdirAll(filepath.Dir(op.Target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(op.Target, op.Data, 0o600)
	case transaction.OpMove:
		if err := os.MkdirAll(filepath.Dir(op.Target), 0o755); err != nil {
			return err
		}
		return os.Rename(op.Source, op.Target)
	case transaction.OpDelete:
		if err := os.Remove(op.Target); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	return nil
}

// seedProjectDir creates a project physically on disk under <dataDir>/projects/<slug>.
func seedProjectDir(t *testing.T, dataDir, slug, name string) *models.Project {
	t.Helper()
	p, err := models.NewProject(uuid.New().String(), slug, "")
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}
	p.Slug = slug
	p.Name = name
	p.Path = filepath.Join(dataDir, "projects", slug)
	if err := os.MkdirAll(p.Path, 0o755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}
	if err := os.WriteFile(filepath.Join(p.Path, "tasks.md"), []byte("# "+name+"\n"), 0o600); err != nil {
		t.Fatalf("write tasks.md: %v", err)
	}
	return p
}

func configArtifact(dataDir string, data string) storage.ArtifactChange {
	return storage.ArtifactChange{Path: filepath.Join(dataDir, "config.json"), Data: []byte(data)}
}

// TestDeleteProjectRequiresConfirm proves an unconfirmed delete never touches
// storage: the destructive-confirmation guard fires before the port is even
// resolved.
func TestDeleteProjectRequiresConfirm(t *testing.T) {
	svc := newTestService(newFakeRepo())
	_, err := svc.DeleteProject(app.DeleteProjectRequest{
		Project: &models.Project{ID: "id", Slug: "x"},
		Confirm: false,
	})
	if !errors.Is(err, app.ErrDestructiveConfirmationRequired) {
		t.Fatalf("DeleteProject(unconfirmed) err = %v, want ErrDestructiveConfirmationRequired", err)
	}
}

// TestDeleteProjectFailsClosedWithoutOpsCommitter proves the use case refuses to run
// when the repository cannot commit raw ops, rather than silently degrading.
func TestDeleteProjectFailsClosedWithoutOpsCommitter(t *testing.T) {
	svc := newTestService(newFakeRepo())
	_, err := svc.DeleteProject(app.DeleteProjectRequest{
		Project: &models.Project{ID: uuid.New().String(), Slug: "x", Path: t.TempDir()},
		DataDir: t.TempDir(),
		Confirm: true,
	})
	if err == nil || !strings.Contains(err.Error(), "does not support") {
		t.Fatalf("expected fail-closed error, got %v", err)
	}
}

// TestProjectLifecycleRoundTripThroughApp exercises the complete delete → restore →
// purge lifecycle through the application service, proving the typed requests drive
// the journaled ops and the restore BuildConfig callback runs with the reconstructed
// identity.
func TestProjectLifecycleRoundTripThroughApp(t *testing.T) {
	dataDir := t.TempDir()
	project := seedProjectDir(t, dataDir, "alpha", "Alpha")
	repo := &opsRepo{fakeRepo: newFakeRepo()}
	svc := app.NewService(repo, nil, nil)

	del, err := svc.DeleteProject(app.DeleteProjectRequest{
		Project: project,
		DataDir: dataDir,
		Reason:  "deleted via test",
		Config:  []storage.ArtifactChange{configArtifact(dataDir, `{"v":"deleted"}`)},
		Confirm: true,
	})
	if err != nil {
		t.Fatalf("DeleteProject: %v", err)
	}
	if del.EntryID == "" {
		t.Fatal("expected a trash entry id")
	}
	if _, statErr := os.Stat(project.Path); !os.IsNotExist(statErr) {
		t.Fatalf("expected project dir moved to trash, stat err = %v", statErr)
	}

	var builtWith string
	res, err := svc.RestoreProject(app.RestoreProjectRequest{
		DataDir: dataDir,
		EntryID: del.EntryID,
		BuildConfig: func(p *models.Project, tomb *storage.TrashTombstone) ([]storage.ArtifactChange, error) {
			builtWith = p.Slug
			return []storage.ArtifactChange{configArtifact(dataDir, `{"v":"restored"}`)}, nil
		},
	})
	if err != nil {
		t.Fatalf("RestoreProject: %v", err)
	}
	if builtWith != "alpha" {
		t.Fatalf("BuildConfig received slug %q, want alpha", builtWith)
	}
	if res.Project == nil || res.Project.Slug != "alpha" {
		t.Fatalf("restore result project = %+v, want slug alpha", res.Project)
	}
	if _, statErr := os.Stat(filepath.Join(dataDir, "projects", "alpha", "tasks.md")); statErr != nil {
		t.Fatalf("expected project restored: %v", statErr)
	}
	if got, _ := os.ReadFile(filepath.Join(dataDir, "config.json")); string(got) != `{"v":"restored"}` {
		t.Fatalf("config after restore = %q", got)
	}

	// Delete again, then purge the entry permanently.
	del2, err := svc.DeleteProject(app.DeleteProjectRequest{
		Project: res.Project,
		DataDir: dataDir,
		Config:  []storage.ArtifactChange{configArtifact(dataDir, `{"v":"deleted2"}`)},
		Confirm: true,
	})
	if err != nil {
		t.Fatalf("DeleteProject(2): %v", err)
	}
	purge, err := svc.PurgeProject(app.PurgeProjectRequest{
		DataDir: dataDir,
		EntryID: del2.EntryID,
		Slug:    "alpha",
		Confirm: true,
	})
	if err != nil {
		t.Fatalf("PurgeProject: %v", err)
	}
	if !purge.Purged {
		t.Fatal("expected purge to remove the entry")
	}
	trashDir := filepath.Join(dataDir, ".tt", "trash")
	if entries, _ := os.ReadDir(trashDir); len(entries) != 0 {
		t.Fatalf("expected trash empty after purge, got %d entries", len(entries))
	}
}

// TestPurgeProjectRequiresConfirm and no-ops on a missing entry.
func TestPurgeProjectRequiresConfirmAndNoopsMissing(t *testing.T) {
	dataDir := t.TempDir()
	repo := &opsRepo{fakeRepo: newFakeRepo()}
	svc := app.NewService(repo, nil, nil)

	if _, err := svc.PurgeProject(app.PurgeProjectRequest{DataDir: dataDir, EntryID: "e", Slug: "x", Confirm: false}); !errors.Is(err, app.ErrDestructiveConfirmationRequired) {
		t.Fatalf("PurgeProject(unconfirmed) err = %v, want ErrDestructiveConfirmationRequired", err)
	}

	res, err := svc.PurgeProject(app.PurgeProjectRequest{DataDir: dataDir, EntryID: "missing", Slug: "x", Confirm: true})
	if err != nil {
		t.Fatalf("PurgeProject(missing): %v", err)
	}
	if res.Purged {
		t.Fatal("expected no-op purge for missing entry")
	}
	if len(repo.committed) != 0 {
		t.Fatalf("expected no commit for missing-entry purge, got %d", len(repo.committed))
	}
}
