package app_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// maintRepo is an in-memory MaintenanceRepository: it embeds the fake mutation
// repository and adds the privileged raw-save escape hatch, recording each raw
// save so a test can assert repair went through the privileged path.
type maintRepo struct {
	*fakeRepo
	rawSaved []string
}

func (m *maintRepo) RawSaveProject(project *models.Project) error {
	m.rawSaved = append(m.rawSaved, project.ID)
	return nil
}

// TestMaintenanceRepairProject exercises the privileged repair path against both an
// in-memory and a filesystem MaintenanceRepository.
func TestMaintenanceRepairProject(t *testing.T) {
	t.Run("in-memory", func(t *testing.T) {
		project := newProject("p1", "alpha", newTask("t1", 1, "task"))
		repo := &maintRepo{fakeRepo: newFakeRepo(project)}
		m := app.NewMaintenance(repo)

		if err := m.RepairProject(project.ID); err != nil {
			t.Fatalf("RepairProject: %v", err)
		}
		if len(repo.rawSaved) != 1 || repo.rawSaved[0] != project.ID {
			t.Fatalf("rawSaved = %v, want [%s]", repo.rawSaved, project.ID)
		}
		// Repair goes through the privileged raw save, never the transactional Commit.
		if len(repo.commits) != 0 {
			t.Fatalf("repair committed %d transactional changesets, want 0", len(repo.commits))
		}

		if err := m.RepairProject("missing"); !errors.Is(err, app.ErrProjectNotFound) {
			t.Fatalf("RepairProject(missing) err = %v, want ErrProjectNotFound", err)
		}
	})

	t.Run("filesystem", func(t *testing.T) {
		dataDir := t.TempDir()
		commitSvc := storage.NewCommitService(filepath.Join(dataDir, ".tt", "transactions"), []string{dataDir})
		project := realTestProject(t, dataDir, "alpha", "seed task")
		repo := newStorageRepo(commitSvc, project)
		repo.seed(t)

		// Corrupt the on-disk tasks.md so repair has something to rewrite.
		tasksPath := filepath.Join(project.Path, "tasks.md")
		if err := os.WriteFile(tasksPath, []byte("garbage"), 0o600); err != nil {
			t.Fatalf("corrupt tasks.md: %v", err)
		}

		m := app.NewMaintenance(repo)
		if err := m.RepairProject(project.ID); err != nil {
			t.Fatalf("RepairProject: %v", err)
		}
		data, err := os.ReadFile(tasksPath)
		if err != nil {
			t.Fatalf("read repaired tasks.md: %v", err)
		}
		if string(data) == "garbage" {
			t.Fatalf("repair did not rewrite the corrupt tasks.md")
		}
	})
}
