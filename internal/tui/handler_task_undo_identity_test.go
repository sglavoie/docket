package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestUndoReferenceRepairUsesProjectQualifiedTaskIdentity(t *testing.T) {
	oldAlphaTask := &models.Task{ID: "shared", Title: "old alpha"}
	oldBetaTask := &models.Task{ID: "shared", Title: "old beta"}
	oldAlpha := &models.Project{ID: "alpha", Tasks: []*models.Task{oldAlphaTask}}
	oldBeta := &models.Project{ID: "beta", Tasks: []*models.Task{oldBetaTask}}
	newAlphaTask := &models.Task{ID: "shared", Title: "new alpha"}
	newBetaTask := &models.Task{ID: "shared", Title: "new beta"}
	newAlpha := &models.Project{ID: "alpha", Tasks: []*models.Task{newAlphaTask}}
	newBeta := &models.Project{ID: "beta", Tasks: []*models.Task{newBetaTask}}

	m := &AppModel{
		nextOrder: []PlanningTask{
			{Task: oldAlphaTask, Project: oldAlpha},
			{Task: oldBetaTask, Project: oldBeta},
		},
		today: []PlanningTask{{Task: oldBetaTask, Project: oldBeta}},
	}
	m.rewireTaskProjectRefsAfterRestore([]*models.Project{newAlpha, newBeta})

	if m.nextOrder[0].Task != newAlphaTask || m.nextOrder[1].Task != newBetaTask {
		t.Fatalf("focus queue rewired to wrong duplicate: %+v", m.nextOrder)
	}
	if m.today[0].Task != newBetaTask || m.today[0].Project != newBeta {
		t.Fatalf("daily plan rewired to wrong duplicate: %+v", m.today)
	}
}

func TestRestoredTaskProjectFailsClosedForDuplicateIDs(t *testing.T) {
	projects := map[string][]string{"shared": {"alpha", "beta"}, "unique": {"beta"}}
	if got, ok := restoredTaskProject(projects, "shared", "alpha"); !ok || got != "alpha" {
		t.Fatalf("existing qualified owner = %q/%v, want alpha", got, ok)
	}
	if got, ok := restoredTaskProject(projects, "shared", "missing"); ok || got != "" {
		t.Fatalf("ambiguous owner = %q/%v, want unresolved", got, ok)
	}
	if got, ok := restoredTaskProject(projects, "unique", "old"); !ok || got != "beta" {
		t.Fatalf("unique moved owner = %q/%v, want beta", got, ok)
	}
}
