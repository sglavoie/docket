package tui

import (
	"sort"
	"testing"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/valuepicker"
)

// This file holds the fixtures and helpers the reference-picker suite shares.
// The assertions live beside them: row assembly in
// operation_task_reference_picker_source_test.go, the commit difference in
// operation_task_reference_picker_diff_test.go, and the `~` key path in
// operation_task_reference_picker_keys_test.go.

// referencePickerFixture is one task that already names a target in another
// project, with a second open task beside that target so the picker has both
// halves to offer: a stored row and an addition candidate.
func referencePickerFixture(t *testing.T) (*AppModel, *models.Task, *models.Project, *models.Project) {
	t.Helper()
	model := newUITestApp()
	target := &models.Task{ID: "target", ShortID: 1, Title: "Ship the API", WorkflowState: models.StateBacklog}
	other := &models.Task{ID: "other", ShortID: 2, Title: "Write the guide", WorkflowState: models.StateBacklog}
	remote := &models.Project{ID: "remote", Name: "Remote", Slug: "remote", Tasks: []*models.Task{target, other}}
	remote.BuildTaskTree()

	subject := &models.Task{
		ID:            "subject",
		ShortID:       1,
		Title:         "Cut the release",
		WorkflowState: models.StateBacklog,
		Dependencies:  []models.TaskReference{{ProjectID: remote.ID, TaskID: target.ID}},
	}
	home := &models.Project{ID: "home", Name: "Home", Slug: "home", Tasks: []*models.Task{subject}}
	home.BuildTaskTree()

	setTestAppProjects(model, home, remote)
	model.crossOutliner.Population.Projects = []*models.Project{home, remote}
	model.crossOutliner.Rows = []outliner.OutlinerRow{{Task: subject, ProjectID: home.ID}}
	return model, subject, home, remote
}

// referenceSuiteWorkspace is the exhaustive subject the diff cases read. Every
// shape a stored reference can wear is present at once — open, terminal,
// missing from a loaded project, pointing into a project this view never
// loaded, and pointing into an archived one — because the commit difference is
// defined by what it refuses to drop, and a fixture holding one shape at a time
// could not show that.
//
// The surrounding projects are built so the subject is linked exactly as a
// loaded workspace would carry it; `gone` is deliberately absent from them,
// which is what makes the fourth stored reference unreadable rather than merely
// closed.
func referenceSuiteWorkspace(t *testing.T) (subject *models.Task) {
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
	return subject
}

func referenceIDsFor(locators ...string) []listsurface.ListItemID {
	ids := make([]listsurface.ListItemID, 0, len(locators))
	for _, locator := range locators {
		ids = append(ids, listsurface.ListItemID("reference:"+locator))
	}
	return ids
}

// referenceMembershipAfter is the membership a picker holds once the user has
// left exactly `checked` ticked: every row the tasks opened with, moved to All
// where it was checked and None where it was not.
func referenceMembershipAfter(
	initial map[listsurface.ListItemID]valuepicker.OperationValueMembership, checked []string,
) map[listsurface.ListItemID]valuepicker.OperationValueMembership {
	current := make(map[listsurface.ListItemID]valuepicker.OperationValueMembership, len(initial)+len(checked))
	for id := range initial {
		current[id] = valuepicker.OperationValueNone
	}
	for _, locator := range checked {
		current[listsurface.ListItemID("reference:"+locator)] = valuepicker.OperationValueAll
	}
	return current
}

// referencePickerCommit replays the whole commit path in one call: the picker
// emits the difference between the membership it preselected from `opened` and
// its final checked set, and the command merges that difference into whatever
// `current` stores at execute time. Passing the same task twice is the ordinary
// case, where nothing moved underneath the overlay; passing two different tasks
// is the concurrent one.
//
// The checked payloads are sorted first because selectedValue() sorts them, so
// no assertion here can accidentally depend on the order rows were ticked in.
func referencePickerCommit(opened, current *models.Task, checked ...string) app.DependencyPatch {
	sorted := append([]string(nil), checked...)
	sort.Strings(sorted)
	initial := valuepicker.TaskReferenceMembership([]*models.Task{opened})
	return valuepicker.TaskReferencePatch(current, valuepicker.TaskReferenceSelection(
		initial, referenceMembershipAfter(initial, sorted),
	))
}

// concurrentReferencePickerFixture is one task storing two references, with a
// third target beside them to check and a same-project task the picker never
// preselects. Two stored rows is the minimum that can show a concurrent change
// landing on one of them while the user edits the other.
func concurrentReferencePickerFixture(t *testing.T) (*AppModel, *models.Task, *models.Project) {
	t.Helper()
	model := newUITestApp()
	remote := &models.Project{ID: "remote", Name: "Remote", Slug: "remote", Tasks: []*models.Task{
		{ID: "target", ShortID: 1, Title: "Ship the API", WorkflowState: models.StateBacklog},
		{ID: "legacy", ShortID: 2, Title: "Retire the shim", WorkflowState: models.StateBacklog},
		{ID: "other", ShortID: 3, Title: "Write the guide", WorkflowState: models.StateBacklog},
	}}
	home := republishedReferenceHome(t, "remote:target", "remote:legacy")
	for _, project := range []*models.Project{home, remote} {
		project.BuildTaskTree()
	}
	setTestAppProjects(model, home, remote)
	model.crossOutliner.Population.Projects = []*models.Project{home, remote}
	subject := home.GetTaskByID("subject")
	model.crossOutliner.Rows = []outliner.OutlinerRow{{Task: subject, ProjectID: home.ID}}
	return model, subject, home
}

// republishedReferenceHome builds the home project as a freshly loaded snapshot
// would carry it, with the subject's stored references set to exactly the given
// locators. Adopting one of these is what a cross-project load does to the shell
// cache, and it is deliberately a whole new task tree: adoptCrossProjectSnapshot
// overwrites the cached project in place, so nothing the picker captured earlier
// is updated by it.
func republishedReferenceHome(t *testing.T, locators ...string) *models.Project {
	t.Helper()
	references := make([]models.TaskReference, 0, len(locators))
	for _, raw := range locators {
		locator, ok := valuepicker.TaskReferenceLocator(raw)
		if !ok {
			t.Fatalf("%q is not a locator the picker could ever carry", raw)
		}
		references = append(references, locator.Reference())
	}
	home := &models.Project{ID: "home", Name: "Home", Slug: "home", Tasks: []*models.Task{
		{
			ID: "subject", ShortID: 1, Title: "Cut the release",
			WorkflowState: models.StateBacklog, Dependencies: references,
		},
		{ID: "chore", ShortID: 2, Title: "Update the changelog", WorkflowState: models.StateBacklog},
	}}
	home.BuildTaskTree()
	return home
}
