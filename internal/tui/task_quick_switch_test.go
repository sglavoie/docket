package tui

import (
	"fmt"
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

func newTestAppForQuickSwitch() *AppModel {
	cfg := &config.Config{}
	manager := &config.Manager{}
	return &AppModel{
		config:        cfg,
		configManager: manager,
	}
}

func TestRecordTaskVisit(t *testing.T) {
	t.Run("records a visit", func(t *testing.T) {
		app := newTestAppForQuickSwitch()
		task := &models.Task{ID: "t1", Title: "Task 1"}
		project := &models.Project{ID: "p1", Name: "Project 1"}

		app.recordTaskVisit(task, project)

		if len(app.recentTaskVisits) != 1 {
			t.Fatalf("expected 1 visit, got %d", len(app.recentTaskVisits))
		}
		if app.recentTaskVisits[0].Task.ID != "t1" {
			t.Errorf("expected task t1, got %s", app.recentTaskVisits[0].Task.ID)
		}
	})

	t.Run("dedup moves revisit to front", func(t *testing.T) {
		app := newTestAppForQuickSwitch()
		t1 := &models.Task{ID: "t1", Title: "Task 1"}
		t2 := &models.Task{ID: "t2", Title: "Task 2"}
		t3 := &models.Task{ID: "t3", Title: "Task 3"}
		p := &models.Project{ID: "p1", Name: "Project 1"}

		app.recordTaskVisit(t1, p)
		app.recordTaskVisit(t2, p)
		app.recordTaskVisit(t3, p)
		// Revisit t1 — should move to front
		app.recordTaskVisit(t1, p)

		if len(app.recentTaskVisits) != 3 {
			t.Fatalf("expected 3 visits, got %d", len(app.recentTaskVisits))
		}
		if app.recentTaskVisits[0].Task.ID != "t1" {
			t.Errorf("expected t1 at front, got %s", app.recentTaskVisits[0].Task.ID)
		}
		if app.recentTaskVisits[1].Task.ID != "t3" {
			t.Errorf("expected t3 at index 1, got %s", app.recentTaskVisits[1].Task.ID)
		}
		if app.recentTaskVisits[2].Task.ID != "t2" {
			t.Errorf("expected t2 at index 2, got %s", app.recentTaskVisits[2].Task.ID)
		}
	})

	t.Run("caps at maxRecentTaskVisits", func(t *testing.T) {
		app := newTestAppForQuickSwitch()
		p := &models.Project{ID: "p1", Name: "Project 1"}

		for i := 0; i < maxRecentTaskVisits+5; i++ {
			task := &models.Task{ID: fmt.Sprintf("t%d", i), Title: fmt.Sprintf("Task %d", i)}
			app.recordTaskVisit(task, p)
		}

		if len(app.recentTaskVisits) != maxRecentTaskVisits {
			t.Fatalf("expected %d visits, got %d", maxRecentTaskVisits, len(app.recentTaskVisits))
		}
	})

	t.Run("persists to config", func(t *testing.T) {
		app := newTestAppForQuickSwitch()
		t1 := &models.Task{ID: "t1", Title: "Task 1"}
		t2 := &models.Task{ID: "t2", Title: "Task 2"}
		p := &models.Project{ID: "p1", Name: "Project 1"}

		app.recordTaskVisit(t1, p)
		app.recordTaskVisit(t2, p)

		visits := app.config.GetRecentTaskVisits()
		if len(visits) != 2 {
			t.Fatalf("expected 2 persisted visits, got %d", len(visits))
		}
		if visits[0].TaskID != "t2" || visits[0].ProjectID != "p1" {
			t.Errorf("expected t2/p1 at front, got %s/%s", visits[0].TaskID, visits[0].ProjectID)
		}
	})

	t.Run("skips nil task or project", func(t *testing.T) {
		app := newTestAppForQuickSwitch()
		task := &models.Task{ID: "t1"}
		project := &models.Project{ID: "p1"}

		app.recordTaskVisit(nil, project)
		app.recordTaskVisit(task, nil)

		if len(app.recentTaskVisits) != 0 {
			t.Fatalf("expected 0 visits, got %d", len(app.recentTaskVisits))
		}
	})
}
