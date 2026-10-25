package tui

import (
	"testing"

	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/projecthub"
)

func TestTUIProjectArchiveCleanupPreservesDuplicateIDReferencesInOtherProject(t *testing.T) {
	domains := config.NewDomains()
	domains.Planning.Next = []config.ProjectTaskLocator{
		{ProjectID: "alpha", TaskID: "shared-id"},
		{ProjectID: "beta", TaskID: "shared-id"},
	}
	domains.Planning.Today = []config.ProjectTaskLocator{
		{ProjectID: "alpha", TaskID: "shared-id"},
		{ProjectID: "beta", TaskID: "shared-id"},
	}
	alpha := &models.Project{ID: "alpha", Name: "Alpha", Slug: "alpha", Tasks: []*models.Task{{
		ID: "shared-id", WorkflowState: models.StateBacklog,
	}}}
	beta := &models.Project{ID: "beta", Name: "Beta", Slug: "beta", Tasks: []*models.Task{{
		ID: "shared-id", WorkflowState: models.StateBacklog,
	}}}

	m := newTestAppModel()
	m.domain.runtime = config.NewRuntimeState("", domains)
	setTestAppProjects(m, alpha, beta)
	m.handleProjectMessages(projecthub.ProjectArchivedMsg{Project: beta})
	if m.feedback.err != nil {
		t.Fatalf("project archive cleanup: %v", m.feedback.err)
	}

	planning := m.domain.runtime.Planning()
	if len(planning.Next) != 1 || planning.Next[0].ProjectID != alpha.ID ||
		len(planning.Today) != 1 || planning.Today[0].ProjectID != alpha.ID {
		t.Fatalf("project archive cleanup removed wrong planning refs: focus=%+v daily=%+v", planning.Next, planning.Today)
	}
}
