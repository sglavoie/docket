package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

func TestTUIProjectArchiveCleanupPreservesDuplicateIDReferencesInOtherProject(t *testing.T) {
	domains := config.NewDomains()
	domains.Planning.NextOrder = []config.ProjectTaskLocator{
		{ProjectID: "alpha", TaskID: "shared-id"},
		{ProjectID: "beta", TaskID: "shared-id"},
	}
	domains.Planning.Today = []config.ProjectTaskLocator{
		{ProjectID: "alpha", TaskID: "shared-id"},
		{ProjectID: "beta", TaskID: "shared-id"},
	}
	alpha := &models.Project{ID: "alpha", Name: "Alpha", Slug: "alpha", Tasks: []*models.Task{{
		ID: "shared-id", WorkflowState: models.StateReady,
	}}}
	beta := &models.Project{ID: "beta", Name: "Beta", Slug: "beta", Tasks: []*models.Task{{
		ID: "shared-id", WorkflowState: models.StateReady,
	}}}

	m := newTestAppModel()
	m.runtime = config.NewRuntimeState("", domains)
	setTestAppProjects(m, alpha, beta)
	m.handleProjectMessages(ProjectArchivedMsg{Project: beta})
	if m.err != nil {
		t.Fatalf("project archive cleanup: %v", m.err)
	}

	planning := m.runtime.Planning()
	if len(planning.NextOrder) != 1 || planning.NextOrder[0].ProjectID != alpha.ID ||
		len(planning.Today) != 1 || planning.Today[0].ProjectID != alpha.ID {
		t.Fatalf("project archive cleanup removed wrong planning refs: focus=%+v daily=%+v", planning.NextOrder, planning.Today)
	}
}
