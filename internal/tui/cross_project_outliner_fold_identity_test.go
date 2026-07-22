package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestCrossProjectOutlinerFoldToggleUsesQualifiedIdentity(t *testing.T) {
	alphaParent := foldIdentityTestTask("shared-parent", "Alpha parent")
	alphaChild := foldIdentityTestTask("alpha-child", "Alpha child")
	alphaChild.ParentID = &alphaParent.ID
	alpha := &models.Project{ID: "alpha", Name: "Alpha", Tasks: []*models.Task{alphaParent, alphaChild}}
	alpha.BuildTaskTree()

	betaParent := foldIdentityTestTask("shared-parent", "Beta parent")
	betaChild := foldIdentityTestTask("beta-child", "Beta child")
	betaChild.ParentID = &betaParent.ID
	beta := &models.Project{ID: "beta", Name: "Beta", Tasks: []*models.Task{betaParent, betaChild}}
	beta.BuildTaskTree()

	m := newCrossProjectFoldIdentityTestOutliner(alpha, beta)
	alphaIdentity := TaskIdentity{ProjectID: alpha.ID, TaskID: alphaParent.ID}
	betaIdentity := TaskIdentity{ProjectID: beta.ID, TaskID: betaParent.ID}

	m.cursor = requireCrossProjectFoldTestRow(t, m, alpha.ID, alphaParent.ID)
	m.handleFoldPrefix("a", alphaParent)

	if m.expandedNodes[alphaIdentity] {
		t.Fatal("alpha parent remained expanded after folding")
	}
	if !m.expandedNodes[betaIdentity] {
		t.Fatal("folding alpha parent changed beta parent with the same task ID")
	}
	if findCrossProjectFoldTestRow(m, alpha.ID, alphaChild.ID) >= 0 {
		t.Fatal("alpha child remained visible after folding alpha parent")
	}
	if findCrossProjectFoldTestRow(m, beta.ID, betaChild.ID) < 0 {
		t.Fatal("beta child disappeared when alpha parent with the same task ID was folded")
	}

	m.cursor = requireCrossProjectFoldTestRow(t, m, beta.ID, betaParent.ID)
	m.handleFoldPrefix("a", betaParent)
	m.cursor = requireCrossProjectFoldTestRow(t, m, alpha.ID, alphaParent.ID)
	m.handleFoldPrefix("a", alphaParent)

	if !m.expandedNodes[alphaIdentity] || m.expandedNodes[betaIdentity] {
		t.Fatalf("independent fold states = alpha:%t beta:%t, want alpha:true beta:false", m.expandedNodes[alphaIdentity], m.expandedNodes[betaIdentity])
	}
	if findCrossProjectFoldTestRow(m, alpha.ID, alphaChild.ID) < 0 {
		t.Fatal("alpha child was not restored after expanding alpha parent")
	}
	if findCrossProjectFoldTestRow(m, beta.ID, betaChild.ID) >= 0 {
		t.Fatal("beta child became visible when alpha parent with the same task ID was expanded")
	}
}

func TestCrossProjectOutlinerCollapseToDepthUsesQualifiedIdentity(t *testing.T) {
	alphaParent := foldIdentityTestTask("shared-parent", "Alpha root parent")
	alphaChild := foldIdentityTestTask("alpha-child", "Alpha child")
	alphaChild.ParentID = &alphaParent.ID
	alpha := &models.Project{ID: "alpha", Name: "Alpha", Tasks: []*models.Task{alphaParent, alphaChild}}
	alpha.BuildTaskTree()

	betaRoot := foldIdentityTestTask("beta-root", "Beta root")
	betaParent := foldIdentityTestTask("shared-parent", "Beta nested parent")
	betaParent.ParentID = &betaRoot.ID
	betaChild := foldIdentityTestTask("beta-child", "Beta child")
	betaChild.ParentID = &betaParent.ID
	beta := &models.Project{ID: "beta", Name: "Beta", Tasks: []*models.Task{betaRoot, betaParent, betaChild}}
	beta.BuildTaskTree()

	m := newCrossProjectFoldIdentityTestOutliner(alpha, beta)
	m.collapseToDepth(1)

	alphaIdentity := TaskIdentity{ProjectID: alpha.ID, TaskID: alphaParent.ID}
	betaIdentity := TaskIdentity{ProjectID: beta.ID, TaskID: betaParent.ID}
	if !m.expandedNodes[alphaIdentity] {
		t.Fatal("depth collapse folded alpha's root-level shared parent")
	}
	if m.expandedNodes[betaIdentity] {
		t.Fatal("depth collapse expanded beta's nested shared parent")
	}
	if findCrossProjectFoldTestRow(m, alpha.ID, alphaChild.ID) < 0 {
		t.Fatal("alpha child should remain visible below an expanded root parent")
	}
	if findCrossProjectFoldTestRow(m, beta.ID, betaChild.ID) >= 0 {
		t.Fatal("beta child should be hidden below a collapsed nested parent")
	}
}

func foldIdentityTestTask(id, title string) *models.Task {
	task := &models.Task{ID: id, Title: title}
	task.SetSection(models.SectionNext)
	return task
}

func newCrossProjectFoldIdentityTestOutliner(projects ...*models.Project) *OutlinerModel {
	m := NewOutlinerModel(nil, nil, newTestConfig())
	m.activePerspective = PerspectiveAll
	m.loadedProjects = projects
	m.crossForceRebuild = true
	m.crossProjectRebuildRows()
	return m
}

func requireCrossProjectFoldTestRow(t *testing.T, m *OutlinerModel, projectID, taskID string) int {
	t.Helper()
	index := findCrossProjectFoldTestRow(m, projectID, taskID)
	if index < 0 {
		t.Fatalf("missing row for task %q in project %q", taskID, projectID)
	}
	return index
}

func findCrossProjectFoldTestRow(m *OutlinerModel, projectID, taskID string) int {
	for index, row := range m.rows {
		if row.task != nil && row.ProjectID == projectID && row.task.ID == taskID {
			return index
		}
	}
	return -1
}
