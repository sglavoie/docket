package config

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestGetCycleProjects(t *testing.T) {
	cfg := NewConfig()

	active1 := &models.Project{ID: "p1", Name: "Active 1", Archived: false}
	active2 := &models.Project{ID: "p2", Name: "Active 2", Archived: false}
	archived := &models.Project{ID: "p3", Name: "Archived", Archived: true}

	cfg.AddProject(active1)
	cfg.AddProject(active2)
	cfg.AddProject(archived)

	cfg.SetCycleProjects([]string{"p1", "p2", "p3"})

	projects := cfg.GetCycleProjects()

	if len(projects) != 2 {
		t.Errorf("expected 2 cycle projects, got %d", len(projects))
	}

	if projects[0].ID != "p1" || projects[1].ID != "p2" {
		t.Error("cycle projects order not preserved")
	}
}

func TestGetCycleProjects_FiltersNonExistent(t *testing.T) {
	cfg := NewConfig()

	active := &models.Project{ID: "p1", Name: "Active", Archived: false}
	cfg.AddProject(active)

	cfg.SetCycleProjects([]string{"p1", "nonexistent", "also-missing"})

	projects := cfg.GetCycleProjects()

	if len(projects) != 1 {
		t.Errorf("expected 1 cycle project, got %d", len(projects))
	}
	if projects[0].ID != "p1" {
		t.Error("expected p1 in cycle projects")
	}
}

func TestGetCycleProjects_Empty(t *testing.T) {
	cfg := NewConfig()

	projects := cfg.GetCycleProjects()

	if len(projects) != 0 {
		t.Errorf("expected 0 cycle projects, got %d", len(projects))
	}
}

func TestSetCycleProjects(t *testing.T) {
	cfg := NewConfig()

	ids := []string{"p1", "p2", "p3"}
	cfg.SetCycleProjects(ids)

	if len(cfg.CycleProjects) != 3 {
		t.Errorf("expected 3 cycle project IDs, got %d", len(cfg.CycleProjects))
	}
}

func TestGetCycleProjectIndex(t *testing.T) {
	cfg := NewConfig()

	active1 := &models.Project{ID: "p1", Name: "Active 1", Archived: false}
	active2 := &models.Project{ID: "p2", Name: "Active 2", Archived: false}
	cfg.AddProject(active1)
	cfg.AddProject(active2)
	cfg.SetCycleProjects([]string{"p1", "p2"})

	if idx := cfg.GetCycleProjectIndex("p1"); idx != 0 {
		t.Errorf("expected index 0 for p1, got %d", idx)
	}
	if idx := cfg.GetCycleProjectIndex("p2"); idx != 1 {
		t.Errorf("expected index 1 for p2, got %d", idx)
	}
}

func TestGetCycleProjectIndex_NotFound(t *testing.T) {
	cfg := NewConfig()

	active := &models.Project{ID: "p1", Name: "Active", Archived: false}
	cfg.AddProject(active)
	cfg.SetCycleProjects([]string{"p1"})

	if idx := cfg.GetCycleProjectIndex("nonexistent"); idx != -1 {
		t.Errorf("expected -1 for non-existent project, got %d", idx)
	}
}

func TestGetCycleProjectIndex_Empty(t *testing.T) {
	cfg := NewConfig()

	if idx := cfg.GetCycleProjectIndex("p1"); idx != -1 {
		t.Errorf("expected -1 for empty cycle list, got %d", idx)
	}
}
