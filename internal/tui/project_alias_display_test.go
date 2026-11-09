package tui

import (
	"strings"
	"testing"

	"github.com/sglavoie/docket/internal/models"
)

// A project with an alias is read by it wherever its slug would be bracketed;
// a project without one keeps its slug.
func TestProjectAliasReplacesTheSlugLabel(t *testing.T) {
	workspace, err := newUITestWorkspace()
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	t.Cleanup(func() { _ = workspace.Cleanup() })
	if _, err = workspace.addProject("Internal Archive Long Name", func(project *models.Project) {
		project.Alias = "IA"
		newFixtureTask(project, "Archive the quarter").ApplyWorkflowState(models.StateBacklog)
		newFixtureTask(project, "Index the boxes").ApplyWorkflowState(models.StateBacklog)
	}); err != nil {
		t.Fatalf("add aliased project: %v", err)
	}
	if _, err = workspace.addProject("Garden Plans", func(project *models.Project) {
		newFixtureTask(project, "Order the seeds").ApplyWorkflowState(models.StateBacklog)
		newFixtureTask(project, "Turn the beds").ApplyWorkflowState(models.StateBacklog)
	}); err != nil {
		t.Fatalf("add plain project: %v", err)
	}
	driver, err := workspace.launch(100, 35)
	if err != nil {
		t.Fatalf("launch: %v", err)
	}
	driver.App().crossOutliner.ClearAllFolds()
	obs := pressKeys(t, driver, "home")
	if !strings.Contains(obs.PlainScreen, "%IA") {
		t.Fatalf("the aliased project is not labeled by its alias:\n%s", obs.PlainScreen)
	}
	if strings.Contains(obs.PlainScreen, "%internal-archive") {
		t.Fatalf("the aliased project still shows its slug:\n%s", obs.PlainScreen)
	}
	if !strings.Contains(obs.PlainScreen, "%garden-plans") {
		t.Fatalf("a project without an alias lost its slug:\n%s", obs.PlainScreen)
	}
}
