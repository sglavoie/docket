package outliner

import (
	"reflect"
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

// clipboardOrderTask builds a leaf task whose parent link is optional, so one
// helper covers both the flat rows and the parent/child shapes the ordering has
// to survive.
func clipboardOrderTask(id string, shortID int, parentID string) *models.Task {
	task := &models.Task{
		ID: id, ShortID: shortID, Title: id,
		WorkflowState: models.StateBacklog,
	}
	if parentID != "" {
		owner := parentID
		task.ParentID = &owner
	}
	return task
}

// clipboardOrderProject wires a project and its tree so GetRootTasks walks the
// same shape the Outliner would draw.
func clipboardOrderProject(id, slug string, tasks ...*models.Task) *models.Project {
	project := &models.Project{ID: id, Name: slug, Slug: slug, Tasks: tasks}
	project.BuildTaskTree()
	return project
}

// clipboardOrderModel places rows in the order given, which is the displayed
// order the visible half of the contract reads from. Tasks in the population
// with no row stand for the members a fold, a filter, or another mode hides.
func clipboardOrderModel(projects []*models.Project, rows ...*models.Task) *OutlinerModel {
	model := &OutlinerModel{Population: populationState{Projects: projects}}
	for _, task := range rows {
		for _, project := range projects {
			if project.GetTaskByID(task.ID) == task {
				model.Rows = append(model.Rows, OutlinerRow{Task: task, ProjectID: project.ID})
				break
			}
		}
	}
	return model
}

func clipboardOrderIdentities(
	t *testing.T,
	project *models.Project,
	tasks ...*models.Task,
) []taskref.TaskIdentity {
	t.Helper()
	identities := make([]taskref.TaskIdentity, 0, len(tasks))
	for _, task := range tasks {
		identity := taskref.For(task, project)
		if !identity.Valid() {
			t.Fatalf("task %s in %s produced an invalid identity", task.ID, project.ID)
		}
		identities = append(identities, identity)
	}
	return identities
}

func clipboardOrderTaskIDs(identities []taskref.TaskIdentity) []string {
	ids := make([]string, len(identities))
	for i, identity := range identities {
		ids[i] = identity.TaskID
	}
	return ids
}

// TestClipboardSelectionOrderFollowsDisplayedRowsForNonadjacentMembers is the
// visible half of the contract: a selection scattered across the list reads in
// the order the rows are drawn, not in the order it was built and not sorted by
// identity.
func TestClipboardSelectionOrderFollowsDisplayedRowsForNonadjacentMembers(t *testing.T) {
	zulu := clipboardOrderTask("zulu", 1, "")
	alpha := clipboardOrderTask("alpha", 2, "")
	mike := clipboardOrderTask("mike", 3, "")
	kilo := clipboardOrderTask("kilo", 4, "")
	project := clipboardOrderProject("project-1", "one", zulu, alpha, mike, kilo)
	model := clipboardOrderModel([]*models.Project{project}, zulu, alpha, mike, kilo)

	// Selected out of order and skipping the row between them.
	selected := clipboardOrderIdentities(t, project, kilo, zulu, mike)
	got := clipboardOrderTaskIDs(model.ClipboardSelectionOrder(selected))
	if want := []string{"zulu", "mike", "kilo"}; !reflect.DeepEqual(got, want) {
		t.Errorf("order = %v, want the displayed row order %v", got, want)
	}
}

// TestClipboardSelectionOrderPlacesHiddenMembersAfterVisibleOnes covers what a
// fold or a filter leaves behind: the members with no row still copy, after
// every visible one, by project slug and then the project's own tree order.
func TestClipboardSelectionOrderPlacesHiddenMembersAfterVisibleOnes(t *testing.T) {
	parent := clipboardOrderTask("parent", 1, "")
	child := clipboardOrderTask("child", 2, "parent")
	grandchild := clipboardOrderTask("grandchild", 3, "child")
	sibling := clipboardOrderTask("sibling", 4, "")
	beta := clipboardOrderProject("project-beta", "beta", parent, child, grandchild, sibling)

	early := clipboardOrderTask("early", 1, "")
	alphaProject := clipboardOrderProject("project-alpha", "alpha", early)

	// Only the fold's root and the other project's task have rows.
	model := clipboardOrderModel([]*models.Project{beta, alphaProject}, parent, early)

	selected := append(
		clipboardOrderIdentities(t, beta, sibling, grandchild, child, parent),
		clipboardOrderIdentities(t, alphaProject, early)...,
	)
	got := clipboardOrderTaskIDs(model.ClipboardSelectionOrder(selected))
	want := []string{"parent", "early", "child", "grandchild", "sibling"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("order = %v, want visible rows then slug and tree order %v", got, want)
	}
}

// TestClipboardSelectionOrderCopiesOnlyExplicitMembers is the descendant rule: a
// selected parent brings no child along, and a selected child with no selected
// parent still copies.
func TestClipboardSelectionOrderCopiesOnlyExplicitMembers(t *testing.T) {
	parent := clipboardOrderTask("parent", 1, "")
	child := clipboardOrderTask("child", 2, "parent")
	other := clipboardOrderTask("other", 3, "")
	project := clipboardOrderProject("project-1", "one", parent, child, other)
	model := clipboardOrderModel([]*models.Project{project}, parent, child, other)

	got := clipboardOrderTaskIDs(model.ClipboardSelectionOrder(
		clipboardOrderIdentities(t, project, parent),
	))
	if want := []string{"parent"}; !reflect.DeepEqual(got, want) {
		t.Errorf("selected parent alone = %v, want %v", got, want)
	}
	got = clipboardOrderTaskIDs(model.ClipboardSelectionOrder(
		clipboardOrderIdentities(t, project, child),
	))
	if want := []string{"child"}; !reflect.DeepEqual(got, want) {
		t.Errorf("selected child alone = %v, want %v", got, want)
	}
	got = clipboardOrderTaskIDs(model.ClipboardSelectionOrder(
		clipboardOrderIdentities(t, project, child, parent),
	))
	if want := []string{"parent", "child"}; !reflect.DeepEqual(got, want) {
		t.Errorf("both selected = %v, want %v", got, want)
	}
}

// TestClipboardSelectionOrderIsStableAndDeduplicated keeps repeated calls and
// repeated members from changing the text: a duplicate identity is one member,
// and the hidden half's map iteration never leaks into the result.
func TestClipboardSelectionOrderIsStableAndDeduplicated(t *testing.T) {
	visible := clipboardOrderTask("visible", 1, "")
	hiddenTasks := []*models.Task{
		clipboardOrderTask("hidden-a", 2, ""),
		clipboardOrderTask("hidden-b", 3, ""),
		clipboardOrderTask("hidden-c", 4, ""),
		clipboardOrderTask("hidden-d", 5, ""),
	}
	project := clipboardOrderProject(
		"project-1", "one",
		append([]*models.Task{visible}, hiddenTasks...)...,
	)
	model := clipboardOrderModel([]*models.Project{project}, visible)

	selected := clipboardOrderIdentities(t, project, append(hiddenTasks, visible, visible)...)
	first := clipboardOrderTaskIDs(model.ClipboardSelectionOrder(selected))
	want := []string{"visible", "hidden-a", "hidden-b", "hidden-c", "hidden-d"}
	if !reflect.DeepEqual(first, want) {
		t.Fatalf("order = %v, want %v", first, want)
	}
	for attempt := 0; attempt < 20; attempt++ {
		if got := clipboardOrderTaskIDs(model.ClipboardSelectionOrder(selected)); !reflect.DeepEqual(got, first) {
			t.Fatalf("attempt %d order = %v, want the stable %v", attempt, got, first)
		}
	}
}

// TestClipboardSelectionOrderQualifiesDuplicateTaskIDsByProject keeps two
// projects that reuse a task ID apart, in both halves of the order.
func TestClipboardSelectionOrderQualifiesDuplicateTaskIDsByProject(t *testing.T) {
	alphaTask := clipboardOrderTask("shared", 1, "")
	alphaHidden := clipboardOrderTask("hidden", 2, "")
	alphaProject := clipboardOrderProject("project-alpha", "alpha", alphaTask, alphaHidden)
	betaTask := clipboardOrderTask("shared", 1, "")
	betaHidden := clipboardOrderTask("hidden", 2, "")
	betaProject := clipboardOrderProject("project-beta", "beta", betaTask, betaHidden)

	model := clipboardOrderModel([]*models.Project{betaProject, alphaProject}, betaTask, alphaTask)
	selected := append(
		clipboardOrderIdentities(t, alphaProject, alphaTask, alphaHidden),
		clipboardOrderIdentities(t, betaProject, betaTask, betaHidden)...,
	)

	ordered := model.ClipboardSelectionOrder(selected)
	if len(ordered) != 4 {
		t.Fatalf("order = %v, want all four qualified identities", ordered)
	}
	want := []taskref.TaskIdentity{
		taskref.For(betaTask, betaProject),
		taskref.For(alphaTask, alphaProject),
		taskref.For(alphaHidden, alphaProject),
		taskref.For(betaHidden, betaProject),
	}
	if !reflect.DeepEqual(ordered, want) {
		t.Errorf("order = %v, want %v", ordered, want)
	}
}

// TestClipboardSelectionOrderKeepsUnresolvableMembers is the degradation rule: a
// project the population no longer holds sorts by its ID rather than dropping
// its tasks out of the copied text.
func TestClipboardSelectionOrderKeepsUnresolvableMembers(t *testing.T) {
	present := clipboardOrderTask("present", 1, "")
	project := clipboardOrderProject("project-1", "one", present)
	model := clipboardOrderModel([]*models.Project{project}, present)

	orphan := taskref.TaskIdentity{ProjectID: "project-gone", TaskID: "orphan"}
	selected := append(clipboardOrderIdentities(t, project, present), orphan)
	got := clipboardOrderTaskIDs(model.ClipboardSelectionOrder(selected))
	if want := []string{"present", "orphan"}; !reflect.DeepEqual(got, want) {
		t.Errorf("order = %v, want the unresolvable member kept last %v", got, want)
	}
}
