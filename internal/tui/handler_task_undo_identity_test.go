package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/coordinator"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/taskref"
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
		nextOrder: []outliner.PlanningTask{
			{Task: oldAlphaTask, Project: oldAlpha},
			{Task: oldBetaTask, Project: oldBeta},
		},
		today: []outliner.PlanningTask{{Task: oldBetaTask, Project: oldBeta}},
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

// undoRefocusShell wires the one projection and the one workspace an undo-focus
// correlation is spent against: the outliner draws `visible` alone, while the
// shell can still resolve tasks in every project handed to it.
func undoRefocusShell(visible *models.Project, projects ...*models.Project) *AppModel {
	byID := make(map[string]*models.Project, len(projects))
	for _, project := range projects {
		byID[project.ID] = project
	}
	return &AppModel{
		domain:        domainController{projects: byID},
		crossOutliner: scopeTestOutliner(visible),
	}
}

func TestUndoRefocusSeatsTheCursorOnACorrelatedRestoredTask(t *testing.T) {
	alpha := scopeTestProject("alpha", "Alpha", "one", "two", "three")
	shell := undoRefocusShell(alpha, alpha)
	restored := taskref.TaskIdentity{ProjectID: "alpha", TaskID: "three"}
	destination, visible := shell.crossOutliner.RowIndexForTask(restored)
	if !visible {
		t.Fatal("fixture projection does not draw the restored task")
	}
	shell.crossOutliner.Cursor = 0
	if destination == 0 {
		t.Fatal("fixture seats the restored task under the cursor already")
	}

	shell.undoFocus.ObserveCommit("tx", coordinator.CommittedFocusTarget{
		Before: app.TaskLocator{ProjectID: "alpha", TaskID: "three"},
		After:  app.TaskLocator{ProjectID: "alpha", TaskID: "three"},
	})
	cmd, detail := shell.refocusRestoredTask("tx", false)
	if cmd == nil {
		t.Fatal("refocus emitted no cursor flash for a moved cursor")
	}
	if detail != "Ctrl+O origin" {
		t.Fatalf("detail = %q, want the origin hint", detail)
	}
	if shell.crossOutliner.Cursor != destination {
		t.Fatalf("cursor = %d, want %d", shell.crossOutliner.Cursor, destination)
	}
}

func TestUndoRefocusNamesWhereARestoredTaskTheViewDoesNotDrawWentInstead(t *testing.T) {
	alpha := scopeTestProject("alpha", "Alpha", "one")
	beta := scopeTestProject("beta", "Beta", "hidden")
	shell := undoRefocusShell(alpha, alpha, beta)
	before := shell.crossOutliner.Cursor

	shell.undoFocus.ObserveCommit("tx", coordinator.CommittedFocusTarget{
		Before: app.TaskLocator{ProjectID: "beta", TaskID: "hidden"},
	})
	cmd, detail := shell.refocusRestoredTask("tx", false)
	if cmd != nil {
		t.Fatal("an undrawn restored task moved the cursor")
	}
	if detail != "now in Beta (Backlog)" {
		t.Fatalf("detail = %q, want the restored location", detail)
	}
	if shell.crossOutliner.Cursor != before {
		t.Fatalf("cursor = %d, want the untouched %d", shell.crossOutliner.Cursor, before)
	}
}

func TestUndoRefocusLeavesUncorrelatedRestoresAlone(t *testing.T) {
	alpha := scopeTestProject("alpha", "Alpha", "one", "two")
	shell := undoRefocusShell(alpha, alpha)
	shell.crossOutliner.Cursor = 0

	// A bulk gesture recorded nothing, so the ledger entry above it correlates to
	// no target and the view keeps the position and selection the user still holds.
	shell.undoFocus.ObserveCommit("tx-bulk", coordinator.CommittedFocusTarget{})
	cmd, detail := shell.refocusRestoredTask("tx-bulk", false)
	if cmd != nil || detail != "" {
		t.Fatalf("uncorrelated restore = cmd %v detail %q, want neither", cmd != nil, detail)
	}
	if shell.crossOutliner.Cursor != 0 {
		t.Fatalf("cursor = %d, want the untouched 0", shell.crossOutliner.Cursor)
	}
}

func TestAppendRestoreDetailKeepsTheMessageWhenThereIsNothingToAdd(t *testing.T) {
	if got := appendRestoreDetail("Undone: moved task", ""); got != "Undone: moved task" {
		t.Fatalf("bare message = %q", got)
	}
	if got := appendRestoreDetail("Undone: moved task", "Ctrl+O origin"); got != "Undone: moved task · Ctrl+O origin" {
		t.Fatalf("decorated message = %q", got)
	}
}
