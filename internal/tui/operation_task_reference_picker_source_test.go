package tui

import (
	"slices"
	"testing"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
)

// This file covers how the reference picker's list is assembled: which rows it
// carries, in which order, and what each one says. The fixtures are in
// operation_task_reference_picker_test.go, the commit difference in
// operation_task_reference_picker_diff_test.go, and the key path in
// operation_task_reference_picker_keys_test.go.

// TestReferencePickerListsTheStoredHalfFirstAndNeverFiltersIt is the assembly
// contract. The stored half leads, in stored order, whatever its targets have
// since become; the addition half follows and is exactly the population an
// addition is allowed to name.
func TestReferencePickerListsTheStoredHalfFirstAndNeverFiltersIt(t *testing.T) {
	subject, projects := referenceSuiteWorkspace(t)
	source := operationTaskReferenceListSource([]*models.Task{subject}, projects)

	stored := referenceIDsFor(
		"remote:target",
		"home:closed",
		"home:ghost",
		"gone:gone-1",
		"attic:relic",
	)
	ids := referenceRowIDs(source)
	if len(ids) < len(stored) || !slices.Equal(ids[:len(stored)], stored) {
		t.Fatalf("the stored half is not the leading rows in stored order; rows = %v", ids)
	}

	// The addition half: every open task in every loaded non-archived project,
	// minus the source and minus the stored half. The terminal `home:shelved` and
	// the archived project's unreferenced `attic:spare` are the two exclusions
	// that would otherwise be invisible, and `home:subject` proves the source
	// cannot offer itself.
	additions := ids[len(stored):]
	if want := referenceIDsFor("home:sibling", "remote:other"); !slices.Equal(additions, want) {
		t.Errorf("addition half = %v, want %v", additions, want)
	}
}

// TestReferencePickerKeepsUnreachableStoredTargetsEnabled is the reason the
// stored half is never filtered: a reference whose target closed, vanished, or
// left the loaded population is precisely the one a user is most likely to be
// dropping, so it has to be listed, preselected, and enabled. Building the real
// picker model is the assertion that carries the most weight — SetSelectedIDs
// refuses a disabled row, so a preselection that survives construction is proof
// no stored row was quietly disabled.
func TestReferencePickerKeepsUnreachableStoredTargetsEnabled(t *testing.T) {
	subject, projects := referenceSuiteWorkspace(t)
	source := operationTaskReferenceListSource([]*models.Task{subject}, projects)

	want := referenceIDsFor(
		"remote:target",
		"home:closed",
		"home:ghost",
		"gone:gone-1",
		"attic:relic",
	)
	membership := taskReferenceMembership([]*models.Task{subject})
	preselected := make([]ListItemID, 0, len(membership))
	for _, id := range want {
		if membership[id] != operationValueAll {
			t.Fatalf("stored reference %s is not preselected; membership = %v", id, membership)
		}
		preselected = append(preselected, id)
	}
	if len(membership) != len(want) {
		t.Fatalf("preselection = %v, want every stored reference and nothing else", membership)
	}
	for _, id := range preselected {
		index := slices.IndexFunc(source.Items, func(item ListItem) bool { return item.ID == id })
		if index < 0 {
			t.Fatalf("preselected row %s is not in the list source", id)
		}
		if !source.Items[index].Enabled() {
			t.Errorf("stored row %s is disabled: %q", id, source.Items[index].DisabledReason)
		}
	}

	picker, err := newOperationValuePickerModel(
		OperationExecutionRequest{},
		ValueProviderTaskReference,
		"Set Dependencies",
		"",
		source,
		true,
		preselected,
	)
	if err != nil {
		t.Fatalf("the stored set could not be preselected: %v", err)
	}
	if selected := picker.surface.SelectedIDs(); !slices.Equal(selected, want) {
		t.Errorf("checked rows = %v, want the whole stored set", selected)
	}
}

// TestReferencePickerUnavailableRowKeepsItsLocator holds the one row that can
// say nothing about its target to saying exactly that, in the word Peek and the
// detail pane already use, over the durable locator storage holds.
func TestReferencePickerUnavailableRowKeepsItsLocator(t *testing.T) {
	subject, projects := referenceSuiteWorkspace(t)
	source := operationTaskReferenceListSource([]*models.Task{subject}, projects)

	// `home:ghost` is a loaded project with no such task and `gone:gone-1` is a
	// project this view never loaded. The picker cannot tell them apart and must
	// not pretend to: both keep their locator and say only `unavailable`.
	for _, locator := range []string{"home:ghost", "gone:gone-1"} {
		item := referenceRow(t, source, locator)
		if item.PrimaryText != locator {
			t.Errorf("%s label = %q, want the raw project-id:task-id", locator, item.PrimaryText)
		}
		if item.SecondaryText != dependencyDetailUnavailable {
			t.Errorf("%s status = %q, want %q", locator, item.SecondaryText, dependencyDetailUnavailable)
		}
		if item.Data != locator {
			t.Errorf("%s payload = %v, want the raw locator", locator, item.Data)
		}
		if item.CategoryID != "" {
			t.Errorf("%s was grouped under %q; an unreadable row names no project", locator, item.CategoryID)
		}
		if !item.Enabled() {
			t.Errorf("%s is disabled; an unreadable reference is still the user's to drop", locator)
		}
	}
}

// TestReferencePickerQualifiesCrossProjectRows checks the two things that keep a
// filtered list honest across a project boundary: the row is grouped under its
// project, and its search text is the one `~` autocomplete scores, so a query
// that finds a target when typed finds the same target when picked.
func TestReferencePickerQualifiesCrossProjectRows(t *testing.T) {
	subject, projects := referenceSuiteWorkspace(t)
	source := operationTaskReferenceListSource([]*models.Task{subject}, projects)

	home, remote, attic := projects[0], projects[1], projects[2]
	for _, testCase := range []struct {
		locator   string
		project   *models.Project
		taskID    string
		category  ListCategoryID
		secondary string
	}{
		{locator: "remote:target", project: remote, taskID: "target", category: "remote", secondary: "⏳ #1"},
		{locator: "home:closed", project: home, taskID: "closed", category: "home", secondary: "✓ #3"},
		{locator: "attic:relic", project: attic, taskID: "relic", category: "attic", secondary: "○ #1"},
		{locator: "home:sibling", project: home, taskID: "sibling", category: "home", secondary: "○ #2"},
	} {
		item := referenceRow(t, source, testCase.locator)
		task := testCase.project.GetTaskByID(testCase.taskID)
		if item.PrimaryText != task.Title {
			t.Errorf("%s label = %q, want %q", testCase.locator, item.PrimaryText, task.Title)
		}
		if item.CategoryID != testCase.category {
			t.Errorf("%s category = %q, want %q", testCase.locator, item.CategoryID, testCase.category)
		}
		candidate := dependencyCandidate{project: testCase.project, task: task, locator: app.TaskLocator{
			ProjectID: testCase.project.ID,
			TaskID:    testCase.taskID,
		}}
		if want := []string{dependencyCandidateSearchText(candidate)}; !slices.Equal(item.Aliases, want) {
			t.Errorf("%s aliases = %v, want the ~ completion's search text %v", testCase.locator, item.Aliases, want)
		}
	}

	// Categories are emitted once per project that actually owns a row, in
	// population order, and carry the project's display name — which is what lets
	// the label itself stay unqualified.
	labels := make(map[ListCategoryID]string, len(source.Categories))
	order := make([]ListCategoryID, 0, len(source.Categories))
	for _, category := range source.Categories {
		if _, seen := labels[category.ID]; seen {
			t.Fatalf("category %q is emitted twice", category.ID)
		}
		labels[category.ID] = category.Label
		order = append(order, category.ID)
	}
	if want := []ListCategoryID{"home", "remote", "attic"}; !slices.Equal(order, want) {
		t.Errorf("categories = %v, want one per row-owning project in population order %v", order, want)
	}
	for id, want := range map[ListCategoryID]string{"home": "Home", "remote": "Remote", "attic": "Attic"} {
		if labels[id] != want {
			t.Errorf("category %q label = %q, want %q", id, labels[id], want)
		}
	}
}
