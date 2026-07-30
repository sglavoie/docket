package app_test

import (
	"errors"
	"testing"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
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

// TestMaintenanceRepairProject exercises the privileged repair path through an
// in-memory MaintenanceRepository.
func TestMaintenanceRepairProject(t *testing.T) {
	project := newProject("p1", "alpha", newTask("t1", 1, "task"))
	repo := &maintRepo{fakeRepo: newFakeRepo(project)}
	m := app.NewMaintenance(repo)

	if err := m.RepairProject(project.ID); err != nil {
		t.Fatalf("RepairProject: %v", err)
	}
	if len(repo.rawSaved) != 1 || repo.rawSaved[0] != project.ID {
		t.Fatalf("rawSaved = %v, want [%s]", repo.rawSaved, project.ID)
	}
	if len(repo.commits) != 0 {
		t.Fatalf("repair committed %d transactional changesets, want 0", len(repo.commits))
	}

	if err := m.RepairProject("missing"); !errors.Is(err, app.ErrProjectNotFound) {
		t.Fatalf("RepairProject(missing) err = %v, want ErrProjectNotFound", err)
	}
}
