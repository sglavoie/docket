package tui

import (
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/grammar"
)

// The fixtures below are the shell's copies of helpers that moved into
// internal/tui/grammar with the tests they were written for. A Go test helper
// cannot cross a package boundary, and the shell tests that still read them —
// the composer, inline row editor and reference picker surfaces — assert
// through the grammar boundary rather than inside it, so they cannot follow.
// Duplicating six small fixtures is cheaper than exporting a test harness the
// production build would then have to carry.

func strPtr(s string) *string {
	return &s
}

func quickCaptureTestProject() *models.Project {
	return &models.Project{
		ID:   "test-project",
		Name: "Test Project",
		Tasks: []*models.Task{
			{ID: "epic-1", ShortID: 1, Title: "User Authentication"},
			{ID: "story-2", ShortID: 2, Title: "Login Flow"},
			{ID: "task-3", ShortID: 3, Title: "Implement button"},
			{ID: "task-4", ShortID: 4, Title: "Implement search"},
		},
	}
}

// dependencyWorkspace is the two-project fixture every dependency grammar test
// reads. The home project holds the task under edit plus one closed and one
// ambiguous-by-prefix target; the routed project exists so a qualified alias has
// somewhere to point that a bare alias cannot reach.
func dependencyWorkspace() (home, routed *models.Project) {
	home = &models.Project{
		ID: "home-project", Name: "Home", Slug: "home",
		Tasks: []*models.Task{
			{ID: "home-1", ShortID: 1, Title: "Edited task", WorkflowState: models.StateBacklog},
			{ID: "home-2", ShortID: 2, Title: "Ship the API", WorkflowState: models.StateBacklog},
			{ID: "home-3", ShortID: 3, Title: "Closed groundwork", WorkflowState: models.StateDone},
			{ID: "home-4", ShortID: 4, Title: "Shipping receipts", WorkflowState: models.StateBacklog},
		},
	}
	routed = &models.Project{
		ID: "routed-project", Name: "Routed", Slug: "routed",
		Tasks: []*models.Task{
			{ID: "routed-1", ShortID: 1, Title: "Routed alpha", WorkflowState: models.StateBacklog},
			{ID: "routed-2", ShortID: 2, Title: "Routed beta", WorkflowState: models.StateDoing},
		},
	}
	home.BuildTaskTree()
	routed.BuildTaskTree()
	return home, routed
}

func dependencyLocatorStrings(locators []app.TaskLocator) []string {
	values := make([]string, len(locators))
	for i, locator := range locators {
		values[i] = locator.Reference().Locator()
	}
	return values
}

// draftedCaptureInput builds a QuickCaptureInput holding value and refreshes its
// autocomplete, so a test can assert what the composer would offer for a draft.
// The grammar-side copy takes a skipProject flag; no shell caller sets it, so
// this one pins the project route open rather than carrying a constant.
func draftedCaptureInput(t *testing.T, value string, project *models.Project, allProjects []*models.Project) *grammar.QuickCaptureInput {
	t.Helper()
	input := grammar.NewQuickCaptureInput(project, allProjects, "", false)
	q := &input
	q.Width = 80
	q.Input.SetValue(value)
	q.RefreshAutocomplete()
	return q
}

func suggestionDisplays(items []grammar.ACSuggestion) []string {
	displays := make([]string, len(items))
	for i, item := range items {
		displays[i] = ansi.Strip(item.Display)
	}
	return displays
}
