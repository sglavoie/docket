package app_test

import (
	"strings"
	"testing"

	"github.com/sglavoie/docket/internal/app"
)

// clipboardProjectionFixture builds two projects whose tasks share short IDs and
// task IDs, which is what makes a project-qualified locator the only thing that
// can tell them apart.
func clipboardProjectionFixture() (*app.Service, []app.TaskLocator) {
	first := newTask("shared-id", 1, "Alpha one")
	first.Description = "Alpha body"
	second := newTask("alpha-2", 2, "Alpha two")
	second.Description = "Second body"
	third := newTask("shared-id", 1, "Beta one")
	third.Description = "Beta body"

	alpha := newProject("project-alpha", "Alpha", first, second)
	beta := newProject("project-beta", "Beta", third)
	svc := newTestService(newFakeRepo(alpha, beta))
	return svc, []app.TaskLocator{
		operationTarget(alpha.ID, first.ID),
		operationTarget(beta.ID, third.ID),
		operationTarget(alpha.ID, second.ID),
	}
}

func projectClipboard(
	t *testing.T,
	svc *app.Service,
	operation app.TaskOperation,
	targets []app.TaskLocator,
) app.TaskClipboardProjection {
	t.Helper()
	result, err := svc.ExecuteTaskOperation(app.TaskOperationRequest{
		Operation: operation,
		Targets:   targets,
	})
	if err != nil {
		t.Fatalf("%s: %v", operation, err)
	}
	if result.Clipboard == nil {
		t.Fatalf("%s produced no clipboard projection", operation)
	}
	return *result.Clipboard
}

// TestClipboardProjectionKeepsCallerOrderAcrossProjects pins the half of the
// ordering contract the application layer owns: it emits the targets in the
// order it was handed, so the view is free to decide what that order means, and
// it qualifies every one by project so two tasks sharing a task ID stay distinct.
func TestClipboardProjectionKeepsCallerOrderAcrossProjects(t *testing.T) {
	svc, targets := clipboardProjectionFixture()

	for _, testCase := range []struct {
		operation app.TaskOperation
		want      string
	}{
		{app.TaskOperationCopyTitle, "Alpha one\nBeta one\nAlpha two"},
		{app.TaskOperationCopyDescription, "Alpha body\n\nBeta body\n\nSecond body"},
		{
			app.TaskOperationCopyTitleDescription,
			"Alpha one\n\nAlpha body\n\nBeta one\n\nBeta body\n\nAlpha two\n\nSecond body",
		},
	} {
		projection := projectClipboard(t, svc, testCase.operation, targets)
		if projection.Text != testCase.want {
			t.Errorf("%s text = %q, want %q", testCase.operation, projection.Text, testCase.want)
		}
		if projection.TaskCount != len(targets) {
			t.Errorf("%s count = %d, want %d", testCase.operation, projection.TaskCount, len(targets))
		}
	}

	reversed := []app.TaskLocator{targets[2], targets[1], targets[0]}
	if got := projectClipboard(t, svc, app.TaskOperationCopyTitle, reversed).Text; got != "Alpha two\nBeta one\nAlpha one" {
		t.Errorf("reversed order text = %q, want the caller's order", got)
	}
}

// TestClipboardProjectionDeduplicatesQualifiedIdentities keeps a selection that
// names one task twice from copying it twice, while leaving the same task ID in
// another project alone.
func TestClipboardProjectionDeduplicatesQualifiedIdentities(t *testing.T) {
	svc, targets := clipboardProjectionFixture()
	duplicated := []app.TaskLocator{targets[0], targets[1], targets[0]}

	projection := projectClipboard(t, svc, app.TaskOperationCopyTitle, duplicated)
	if projection.Text != "Alpha one\nBeta one" {
		t.Errorf("text = %q, want the first occurrence kept once", projection.Text)
	}
	if projection.TaskCount != 2 {
		t.Errorf("count = %d, want the deduplicated size", projection.TaskCount)
	}
}

// TestClipboardProjectionMatchesSingleTaskOutputExactly is the parity guard: one
// selected task must produce byte-for-byte what copying that task alone always
// produced, empty description included.
func TestClipboardProjectionMatchesSingleTaskOutputExactly(t *testing.T) {
	task := newTask("solo", 1, "Solo")
	project := newProject("project-solo", "Solo", task)
	svc := newTestService(newFakeRepo(project))
	target := []app.TaskLocator{operationTarget(project.ID, task.ID)}

	for _, testCase := range []struct {
		operation app.TaskOperation
		want      string
	}{
		{app.TaskOperationCopyTitle, "Solo"},
		{app.TaskOperationCopyDescription, ""},
		{app.TaskOperationCopyTitleDescription, "Solo\n\n"},
	} {
		projection := projectClipboard(t, svc, testCase.operation, target)
		if projection.Text != testCase.want {
			t.Errorf("%s text = %q, want %q", testCase.operation, projection.Text, testCase.want)
		}
		if projection.TaskCount != 1 {
			t.Errorf("%s count = %d, want 1", testCase.operation, projection.TaskCount)
		}
	}
}

// TestClipboardProjectionSeparatesEmptyDescriptions keeps the blank-line joins
// honest when a member of the selection has no description: the empty block
// still occupies its position rather than collapsing the ones around it.
func TestClipboardProjectionSeparatesEmptyDescriptions(t *testing.T) {
	first := newTask("first", 1, "First")
	first.Description = "Body"
	middle := newTask("middle", 2, "Middle")
	last := newTask("last", 3, "Last")
	last.Description = "Tail"
	project := newProject("project-1", "Project", first, middle, last)
	svc := newTestService(newFakeRepo(project))
	targets := []app.TaskLocator{
		operationTarget(project.ID, first.ID),
		operationTarget(project.ID, middle.ID),
		operationTarget(project.ID, last.ID),
	}

	if got := projectClipboard(t, svc, app.TaskOperationCopyDescription, targets).Text; got != "Body\n\n\n\nTail" {
		t.Errorf("description text = %q, want the empty block to hold its position", got)
	}
	want := "First\n\nBody\n\nMiddle\n\n\n\nLast\n\nTail"
	if got := projectClipboard(t, svc, app.TaskOperationCopyTitleDescription, targets).Text; got != want {
		t.Errorf("title and description text = %q, want %q", got, want)
	}
}

// TestClipboardProjectionRefusesMissingTargetWholly is the no-partial-success
// guard: a selection naming one task that is gone yields an error and no text,
// so nothing reaches an adapter that could write half of it.
func TestClipboardProjectionRefusesMissingTargetWholly(t *testing.T) {
	svc, targets := clipboardProjectionFixture()
	withMissing := append(append([]app.TaskLocator{}, targets...), operationTarget("project-alpha", "gone"))

	result, err := svc.ExecuteTaskOperation(app.TaskOperationRequest{
		Operation: app.TaskOperationCopyTitle,
		Targets:   withMissing,
	})
	if err == nil {
		t.Fatalf("a missing target still projected %+v", result.Clipboard)
	}
	if result.Clipboard != nil {
		t.Fatalf("a refused projection still carried text: %+v", result.Clipboard)
	}

	missingProject := []app.TaskLocator{targets[0], operationTarget("project-gone", "shared-id")}
	if _, err = svc.ExecuteTaskOperation(app.TaskOperationRequest{
		Operation: app.TaskOperationCopyTitle,
		Targets:   missingProject,
	}); err == nil {
		t.Fatal("a missing project still projected clipboard text")
	}
}

// TestClipboardProjectionCommitsNothing keeps the read-only claim honest for a
// bulk projection, not just the single-task one: no revision moves, so no
// timestamp is touched and no undo entry can be recorded.
func TestClipboardProjectionCommitsNothing(t *testing.T) {
	first := newTask("first", 1, "First")
	second := newTask("second", 2, "Second")
	project := newProject("project-1", "Project", first, second)
	repo := newFakeRepo(project)
	svc := newTestService(repo)
	before := len(repo.commits)
	updatedAt := first.UpdatedAt

	projection := projectClipboard(t, svc, app.TaskOperationCopyTitleDescription, []app.TaskLocator{
		operationTarget(project.ID, first.ID),
		operationTarget(project.ID, second.ID),
	})
	if !strings.Contains(projection.Text, "Second") {
		t.Fatalf("projection = %q, want both tasks", projection.Text)
	}
	if len(repo.commits) != before {
		t.Errorf("commits = %d, want %d: a copy writes nothing", len(repo.commits), before)
	}
	if !first.UpdatedAt.Equal(updatedAt) {
		t.Errorf("UpdatedAt = %v, want the untouched %v", first.UpdatedAt, updatedAt)
	}
}

// TestClipboardProjectionsNameOnlyTheFieldsTheyPromise holds the three copy keys
// to their names now that a task carries a third piece of prose. `y` is the
// title, `yd` the description, and `yD` both; none of them is "everything the
// task says", so a resume note the task happens to hold never rides along.
func TestClipboardProjectionsNameOnlyTheFieldsTheyPromise(t *testing.T) {
	const note = "stopped at the failing merge case"
	task := newTask("noted", 1, "Titled")
	task.Description = "Described"
	task.ResumeNote = note
	project := newProject("project", "Project", task)
	svc := newTestService(newFakeRepo(project))
	targets := []app.TaskLocator{operationTarget(project.ID, task.ID)}

	for _, testCase := range []struct {
		operation app.TaskOperation
		want      string
	}{
		{app.TaskOperationCopyTitle, "Titled"},
		{app.TaskOperationCopyDescription, "Described"},
		{app.TaskOperationCopyTitleDescription, "Titled\n\nDescribed"},
	} {
		got := projectClipboard(t, svc, testCase.operation, targets).Text
		if got != testCase.want {
			t.Fatalf("%s copied %q, want %q", testCase.operation, got, testCase.want)
		}
		if strings.Contains(got, note) {
			t.Fatalf("%s carried the resume note: %q", testCase.operation, got)
		}
	}
}
