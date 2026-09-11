package app_test

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/app"
)

// This file covers the typed evidence a committed mutation reports about
// deadlines and task addresses: which tasks were given an explicit deadline
// instruction, and where a relocated task and its descendants ended up. A caller
// holding project-qualified session state reads these instead of parsing status
// text or re-deriving intent from a reloaded workspace.

func dueMoment(day, hour, minute int) time.Time {
	return time.Date(2026, 9, day, hour, minute, 0, 0, time.Local)
}

func locatorSet(locators []app.TaskLocator) map[app.TaskLocator]bool {
	set := make(map[app.TaskLocator]bool, len(locators))
	for _, locator := range locators {
		set[locator] = true
	}
	return set
}

// The outline buffer is declarative: every line restates the deadline it was
// dumped with, so only a changed deadline is the user naming one.
func TestOutlinePlanReportsOnlyChangedDeadlinesAsDueEdits(t *testing.T) {
	moved := newTask("t1", 1, "Moved deadline")
	movedDue := dueMoment(20, 9, 0)
	moved.DueDate = &movedDue
	renamed := newTask("t2", 2, "Renamed")
	renamedDue := dueMoment(21, 14, 30)
	renamed.DueDate = &renamedDue
	cleared := newTask("t3", 3, "Cleared")
	clearedDue := dueMoment(22, 16, 45)
	cleared.DueDate = &clearedDue
	untouched := newTask("t4", 4, "Untouched")

	project := newProject("p1", "Alpha Project", moved, renamed, cleared, untouched)
	repo := newFakeRepo(project)

	newDeadline := dueMoment(25, 11, 15)
	result, err := newTestService(repo).ApplyOutlinePlan(app.OutlinePlanRequest{
		ProjectID:        project.ID,
		ExpectedRevision: repo.revisions[project.ID],
		ScopeTaskIDs:     []string{moved.ID, renamed.ID, cleared.ID, untouched.ID},
		Lines: []app.OutlinePlanLine{
			{SourceLine: 1, TaskID: moved.ID, Title: "Moved deadline", Due: &newDeadline},
			// A title-only edit restates the deadline without naming one.
			{SourceLine: 2, TaskID: renamed.ID, Title: "Renamed again", Due: &renamedDue},
			{SourceLine: 3, TaskID: cleared.ID, Title: "Cleared"},
			{SourceLine: 4, TaskID: untouched.ID, Title: "Untouched"},
		},
	})
	if err != nil {
		t.Fatalf("ApplyOutlinePlan: %v", err)
	}

	edits := locatorSet(result.DueEdits)
	if len(edits) != 2 {
		t.Fatalf("DueEdits = %v, want exactly the moved and cleared deadlines", result.DueEdits)
	}
	if !edits[app.TaskLocator{ProjectID: project.ID, TaskID: moved.ID}] {
		t.Errorf("a changed deadline was not reported as a due edit")
	}
	if !edits[app.TaskLocator{ProjectID: project.ID, TaskID: cleared.ID}] {
		t.Errorf("a cleared deadline was not reported as a due edit")
	}
	if edits[app.TaskLocator{ProjectID: project.ID, TaskID: renamed.ID}] {
		t.Errorf("a title-only edit was reported as a due edit")
	}
}

// DueEdited is the line's intent rather than the deadline it produced, so an
// explicit clear of an already-absent deadline still reports true even though the
// edit committed nothing at all.
func TestTaskLineEditReportsExplicitDueIntentEvenWhenNothingChanged(t *testing.T) {
	task := newTask("t1", 1, "Alpha")
	project := newProject("p1", "Alpha Project", task)
	svc := newTestService(newFakeRepo(project))
	source := app.TaskLocator{ProjectID: project.ID, TaskID: task.ID}

	result, err := svc.EditTaskLine(app.TaskLineEditRequest{
		Source: source,
		Title:  "Alpha",
		DueSet: true,
	})
	if err != nil {
		t.Fatalf("EditTaskLine: %v", err)
	}
	if result.Changed {
		t.Fatalf("clearing an absent deadline committed a transaction")
	}
	if !result.DueEdited {
		t.Fatalf("an explicit due token was not reported as a due edit")
	}

	silent, err := svc.EditTaskLine(app.TaskLineEditRequest{Source: source, Title: "Alpha renamed"})
	if err != nil {
		t.Fatalf("EditTaskLine: %v", err)
	}
	if silent.DueEdited {
		t.Fatalf("a title-only edit reported a due edit")
	}
}

// A cross-project edit reports one address change per relocated task, the
// descendants that traveled inside the subtree included, because nothing in the
// request ever named them.
func TestTaskLineEditReportsEveryRelocatedAddress(t *testing.T) {
	root := newTask("root", 1, "Root")
	child := childOf("child", 2, "Child", root)
	grandchild := childOf("grandchild", 3, "Grandchild", child)
	source := newProject("source", "Source", root, child, grandchild)
	destination := newProject("destination", "Destination")

	result, err := newTestService(newFakeRepo(source, destination)).EditTaskLine(app.TaskLineEditRequest{
		Source:               app.TaskLocator{ProjectID: source.ID, TaskID: root.ID},
		Title:                "Root",
		DestinationProjectID: destination.ID,
	})
	if err != nil {
		t.Fatalf("EditTaskLine: %v", err)
	}
	if len(result.Moves) != 3 {
		t.Fatalf("Moves = %v, want one entry per relocated task", result.Moves)
	}
	moved := map[string]app.TaskLocator{}
	for _, move := range result.Moves {
		if move.From.ProjectID != source.ID || move.To.ProjectID != destination.ID {
			t.Fatalf("move %v does not run from the source project to the destination", move)
		}
		if move.From.TaskID != move.To.TaskID {
			t.Fatalf("move %v changed the task UUID, which a relocation never does", move)
		}
		moved[move.From.TaskID] = move.To
	}
	for _, id := range []string{root.ID, child.ID, grandchild.ID} {
		if _, ok := moved[id]; !ok {
			t.Fatalf("Moves named %v, want the whole subtree including %s", result.Moves, id)
		}
	}
}

// The batch form reports the same map, which is what a caller re-addresses
// project-qualified session state from after a move it did not itself enumerate.
func TestBulkMoveToProjectReportsEveryRelocatedAddress(t *testing.T) {
	root := newTask("root", 1, "Root")
	child := childOf("child", 2, "Child", root)
	source := newProject("source", "Source", root, child)
	destination := newProject("destination", "Destination")
	repo := newFakeRepo(source, destination)

	result, err := newTestService(repo).BulkMoveToProject(
		repoLocators(repo, []string{"root"}),
		destination.ID,
	)
	if err != nil {
		t.Fatalf("BulkMoveToProject: %v", err)
	}
	if len(result.Moves) != 2 {
		t.Fatalf("Moves = %v, want the root and its descendant", result.Moves)
	}
	seen := map[string]bool{}
	for _, move := range result.Moves {
		if move.From.ProjectID != source.ID || move.To.ProjectID != destination.ID {
			t.Fatalf("move %v does not run from the source project to the destination", move)
		}
		seen[move.From.TaskID] = true
	}
	if !seen[root.ID] || !seen[child.ID] {
		t.Fatalf("Moves named %v, want both %s and %s", result.Moves, root.ID, child.ID)
	}
}

// A bulk deadline mutation names every task it wrote, whatever deadline resulted.
func TestBulkDueDateMutationsReportTheirTargets(t *testing.T) {
	first := newTask("t1", 1, "First")
	second := newTask("t2", 2, "Second")
	project := newProject("p1", "Alpha Project", first, second)
	repo := newFakeRepo(project)
	svc := newTestService(repo)
	targets := repoLocators(repo, []string{first.ID, second.ID})

	set, err := svc.BulkSetDueDate(targets, dueMoment(24, 10, 0))
	if err != nil {
		t.Fatalf("BulkSetDueDate: %v", err)
	}
	if len(set.DueEdits) != 2 {
		t.Fatalf("BulkSetDueDate DueEdits = %v, want both targets", set.DueEdits)
	}

	// Clearing reports its targets too, whatever deadline it found: the
	// instruction is what a caller remembering a wall clock has to hear.
	cleared, err := svc.BulkClearDueDate(targets)
	if err != nil {
		t.Fatalf("BulkClearDueDate: %v", err)
	}
	if len(cleared.DueEdits) != 2 {
		t.Fatalf("BulkClearDueDate DueEdits = %v, want both targets", cleared.DueEdits)
	}

	// A bulk mutation that is not about deadlines names nothing.
	priorities, err := svc.BulkSetPriority(targets, 2)
	if err != nil {
		t.Fatalf("BulkSetPriority: %v", err)
	}
	if len(priorities.DueEdits) != 0 {
		t.Fatalf("BulkSetPriority reported due edits %v", priorities.DueEdits)
	}
}

// The one operation pipeline reports the same evidence, so a surface that
// dispatches everything through it does not need a second channel.
func TestTaskOperationSetDueDateReportsItsTargets(t *testing.T) {
	task := newTask("t1", 1, "Alpha")
	project := newProject("p1", "Alpha Project", task)
	repo := newFakeRepo(project)
	due := dueMoment(26, 13, 45)

	result, err := newTestService(repo).ExecuteTaskOperation(app.TaskOperationRequest{
		Operation: app.TaskOperationSetDueDate,
		Targets:   repoLocators(repo, []string{task.ID}),
		Date:      &due,
	})
	if err != nil {
		t.Fatalf("ExecuteTaskOperation: %v", err)
	}
	if len(result.DueEdits) != 1 {
		t.Fatalf("DueEdits = %v, want the one target", result.DueEdits)
	}
	if result.DueEdits[0] != (app.TaskLocator{ProjectID: project.ID, TaskID: task.ID}) {
		t.Fatalf("DueEdits named %v, want the requested target", result.DueEdits[0])
	}
}

// A delete names the addresses nothing may attach session state to again, the
// whole removed subtree included.
func TestDeleteReportsEveryRemovedAddress(t *testing.T) {
	root := newTask("root", 1, "Root")
	child := childOf("child", 2, "Child", root)
	project := newProject("p1", "Alpha Project", root, child)
	repo := newFakeRepo(project)

	result, err := newTestService(repo).Delete(app.DeleteRequest{
		Locator: app.TaskLocator{ProjectID: project.ID, TaskID: root.ID},
		Confirm: true,
	})
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	removed := locatorSet(result.RemovedTaskLocators)
	if len(removed) != 2 {
		t.Fatalf("RemovedTaskLocators = %v, want the root and its descendant", result.RemovedTaskLocators)
	}
	for _, id := range []string{root.ID, child.ID} {
		if !removed[app.TaskLocator{ProjectID: project.ID, TaskID: id}] {
			t.Fatalf("RemovedTaskLocators %v did not name %s", result.RemovedTaskLocators, id)
		}
	}
}
