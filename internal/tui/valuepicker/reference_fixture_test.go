package valuepicker

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/listsurface"
)

// referenceSuiteWorkspace is the exhaustive population the assembly and diff
// cases read. Every shape a stored reference can wear is present at once —
// open, terminal, missing from a loaded project, pointing into a project this
// view never loaded, and pointing into an archived one — because the picker's
// first half is defined by what it refuses to filter, and a fixture holding one
// shape at a time could not show that.
//
// The returned population is what a caller passes as `projects`; `gone` is
// deliberately absent from it, which is what makes the fourth stored reference
// unreadable rather than merely closed.
func referenceSuiteWorkspace(t *testing.T) (subject *models.Task, projects []*models.Project) {
	t.Helper()
	sibling := &models.Task{ID: "sibling", ShortID: 2, Title: "Agree the schema", WorkflowState: models.StateBacklog}
	closed := &models.Task{ID: "closed", ShortID: 3, Title: "Closed groundwork", WorkflowState: models.StateDone}
	shelved := &models.Task{ID: "shelved", ShortID: 4, Title: "Shelved spike", WorkflowState: models.StateCanceled}
	subject = &models.Task{
		ID: "subject", ShortID: 1, Title: "Cut the release", WorkflowState: models.StateDoing,
		Dependencies: []models.TaskReference{
			{ProjectID: "remote", TaskID: "target"},
			{ProjectID: "home", TaskID: "closed"},
			{ProjectID: "home", TaskID: "ghost"},
			{ProjectID: "gone", TaskID: "gone-1"},
			{ProjectID: "attic", TaskID: "relic"},
		},
	}
	home := &models.Project{
		ID: "home", Name: "Home", Slug: "home",
		Tasks: []*models.Task{subject, sibling, closed, shelved},
	}
	remote := &models.Project{
		ID: "remote", Name: "Remote", Slug: "remote",
		Tasks: []*models.Task{
			{ID: "target", ShortID: 1, Title: "Ship the API", WorkflowState: models.StateWaiting},
			{ID: "other", ShortID: 2, Title: "Write the guide", WorkflowState: models.StateBacklog},
		},
	}
	attic := &models.Project{
		ID: "attic", Name: "Attic", Slug: "attic", Archived: true,
		Tasks: []*models.Task{
			{ID: "relic", ShortID: 1, Title: "Retire the shim", WorkflowState: models.StateBacklog},
			{ID: "spare", ShortID: 2, Title: "Sweep the attic", WorkflowState: models.StateBacklog},
		},
	}
	for _, project := range []*models.Project{home, remote, attic} {
		project.BuildTaskTree()
	}
	return subject, []*models.Project{home, remote, attic}
}

func referenceIDsFor(locators ...string) []listsurface.ListItemID {
	ids := make([]listsurface.ListItemID, 0, len(locators))
	for _, locator := range locators {
		ids = append(ids, listsurface.ListItemID("reference:"+locator))
	}
	return ids
}

func referenceRowIDs(source listsurface.ListSource) []listsurface.ListItemID {
	ids := make([]listsurface.ListItemID, 0, len(source.Items))
	for _, item := range source.Items {
		ids = append(ids, item.ID)
	}
	return ids
}

func referenceRow(t *testing.T, source listsurface.ListSource, locator string) listsurface.ListItem {
	t.Helper()
	want := listsurface.ListItemID("reference:" + locator)
	for _, item := range source.Items {
		if item.ID == want {
			return item
		}
	}
	t.Fatalf("the picker has no row for %s; rows = %v", locator, referenceRowIDs(source))
	return listsurface.ListItem{}
}

// keyPress spells one key the picker's own handler reads. The shell keeps its
// copy beside the tests that drive AppModel; a picker test needs only this much.
func keyPress(s string) tea.KeyPressMsg {
	switch s {
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	default:
		return tea.KeyPressMsg{Code: rune(s[0])}
	}
}
