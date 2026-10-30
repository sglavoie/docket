package app_test

import (
	"errors"
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/models"
)

func locatorsFor(projectID string, taskIDs ...string) []app.TaskLocator {
	locators := make([]app.TaskLocator, 0, len(taskIDs))
	for _, id := range taskIDs {
		locators = append(locators, app.TaskLocator{ProjectID: projectID, TaskID: id})
	}
	return locators
}

// TestReorderSiblingOrderLeavesUnnamedSiblingInPlace is the invariant the edge
// move rests on: a presentation layer names only the peers it can see, and a
// sibling it left out — hidden by a filter or a collapsed parent — must not
// drift.
func TestReorderSiblingOrderLeavesUnnamedSiblingInPlace(t *testing.T) {
	a := typedTask("a", 1, "Alpha")
	b := typedTask("b", 2, "Bravo")
	c := typedTask("c", 3, "Charlie")
	d := typedTask("d", 4, "Delta")
	project := newProject("proj-a", "Alpha", a, b, c, d)
	svc := newTestService(newFakeRepo(project))

	moved := app.TaskLocator{ProjectID: project.ID, TaskID: "b"}
	if _, err := svc.ReorderSiblingOrder(moved, locatorsFor(project.ID, "d", "b")); err != nil {
		t.Fatalf("ReorderSiblingOrder: %v", err)
	}
	assertTaskIDs(t, project.Tasks, "a", "d", "c", "b")
}

func TestReorderSiblingOrderRejectsNonSiblings(t *testing.T) {
	parent := typedTask("parent", 1, "Parent")
	child := typedTask("child", 2, "Child")
	parentID := parent.ID
	child.ParentID = &parentID
	root := typedTask("root", 3, "Root")
	project := newProject("proj-a", "Alpha", parent, child, root)
	svc := newTestService(newFakeRepo(project))

	moved := app.TaskLocator{ProjectID: project.ID, TaskID: "child"}
	_, err := svc.ReorderSiblingOrder(moved, locatorsFor(project.ID, "child", "root"))
	if !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
	assertTaskIDs(t, project.Tasks, "parent", "child", "root")
}

func TestReorderSiblingOrderRejectsRepeatedTask(t *testing.T) {
	a := typedTask("a", 1, "Alpha")
	b := typedTask("b", 2, "Bravo")
	project := newProject("proj-a", "Alpha", a, b)
	svc := newTestService(newFakeRepo(project))

	moved := app.TaskLocator{ProjectID: project.ID, TaskID: "a"}
	_, err := svc.ReorderSiblingOrder(moved, locatorsFor(project.ID, "a", "a"))
	if !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
}

// The moved task is what the transaction is recorded against, so an order that
// omits it describes a different request than the caller made.
func TestReorderSiblingOrderRejectsAnOrderWithoutTheMovedTask(t *testing.T) {
	a := typedTask("a", 1, "Alpha")
	b := typedTask("b", 2, "Bravo")
	c := typedTask("c", 3, "Charlie")
	project := newProject("proj-a", "Alpha", a, b, c)
	svc := newTestService(newFakeRepo(project))

	moved := app.TaskLocator{ProjectID: project.ID, TaskID: "a"}
	_, err := svc.ReorderSiblingOrder(moved, locatorsFor(project.ID, "c", "b"))
	if !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
	assertTaskIDs(t, project.Tasks, "a", "b", "c")
}

func pinnedTask(id string, shortID int, title string, at time.Time) *models.Task {
	task := typedTask(id, shortID, title)
	pinnedAt := at
	task.Pinned, task.PinnedAt = true, &pinnedAt
	return task
}

// pinOrder returns the task IDs in PinnedAt-descending order, which is the order
// the Pinned section renders.
func pinOrder(tasks ...*models.Task) []string {
	sorted := append([]*models.Task(nil), tasks...)
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0 && sorted[j].PinnedAt.After(*sorted[j-1].PinnedAt); j-- {
			sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
		}
	}
	ids := make([]string, 0, len(sorted))
	for _, task := range sorted {
		ids = append(ids, task.ID)
	}
	return ids
}

func TestReorderPinnedOrderRedistributesTheChainsOwnTimestamps(t *testing.T) {
	base := time.Date(2026, 8, 15, 9, 0, 0, 0, time.UTC)
	a := pinnedTask("a", 1, "Alpha", base)
	b := pinnedTask("b", 2, "Bravo", base.Add(-time.Hour))
	c := pinnedTask("c", 3, "Charlie", base.Add(-2*time.Hour))
	project := newProject("proj-a", "Alpha", a, b, c)
	svc := newTestService(newFakeRepo(project))

	moved := app.TaskLocator{ProjectID: project.ID, TaskID: "a"}
	if _, err := svc.ReorderPinnedTasksOrder(moved, locatorsFor(project.ID, "b", "c", "a")); err != nil {
		t.Fatalf("ReorderPinnedTasksOrder: %v", err)
	}
	if got, want := pinOrder(a, b, c), []string{"b", "c", "a"}; !equalStrings(got, want) {
		t.Fatalf("pin order = %v, want %v", got, want)
	}
	if !a.PinnedAt.Equal(c.PinnedAt.Add(-time.Hour)) {
		t.Fatalf("chain reused a timestamp outside its own set: %v", a.PinnedAt)
	}
}

func TestReorderPinnedOrderBreaksTiesStrictly(t *testing.T) {
	base := time.Date(2026, 8, 15, 9, 0, 0, 0, time.UTC)
	a := pinnedTask("a", 1, "Alpha", base)
	b := pinnedTask("b", 2, "Bravo", base)
	c := pinnedTask("c", 3, "Charlie", base)
	project := newProject("proj-a", "Alpha", a, b, c)
	svc := newTestService(newFakeRepo(project))

	moved := app.TaskLocator{ProjectID: project.ID, TaskID: "c"}
	if _, err := svc.ReorderPinnedTasksOrder(moved, locatorsFor(project.ID, "c", "b", "a")); err != nil {
		t.Fatalf("ReorderPinnedTasksOrder: %v", err)
	}
	if !c.PinnedAt.After(*b.PinnedAt) || !b.PinnedAt.After(*a.PinnedAt) {
		t.Fatalf("tied pins did not become strictly decreasing: %v %v %v", c.PinnedAt, b.PinnedAt, a.PinnedAt)
	}
}

func TestReorderPinnedOrderRejectsUnpinnedTask(t *testing.T) {
	base := time.Date(2026, 8, 15, 9, 0, 0, 0, time.UTC)
	a := pinnedTask("a", 1, "Alpha", base)
	b := typedTask("b", 2, "Bravo")
	project := newProject("proj-a", "Alpha", a, b)
	svc := newTestService(newFakeRepo(project))

	moved := app.TaskLocator{ProjectID: project.ID, TaskID: "b"}
	_, err := svc.ReorderPinnedTasksOrder(moved, locatorsFor(project.ID, "b", "a"))
	if !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
}

// TestMoveChecklistItemSplicesRatherThanSwaps separates the new seam from
// ReorderChecklistItem: a swap over a multi-position move would displace the
// destination item instead of shifting the whole run.
func TestMoveChecklistItemSplicesRatherThanSwaps(t *testing.T) {
	task := typedTask("t1", 1, "Task")
	task.Checklist = []models.ChecklistItem{
		{Text: "one"}, {Text: "two"}, {Text: "three"}, {Text: "four"},
	}
	project := newProject("proj-a", "Alpha", task)
	svc := newTestService(newFakeRepo(project))
	locator := app.TaskLocator{ProjectID: project.ID, TaskID: task.ID}

	if _, err := svc.MoveChecklistItem(locator, 0, 3, task.Checklist); err != nil {
		t.Fatalf("MoveChecklistItem: %v", err)
	}
	if got, want := checklistTexts(task), []string{"two", "three", "four", "one"}; !equalStrings(got, want) {
		t.Fatalf("checklist = %v, want %v", got, want)
	}

	if _, err := svc.MoveChecklistItem(locator, 3, 0, task.Checklist); err != nil {
		t.Fatalf("MoveChecklistItem back to top: %v", err)
	}
	if got, want := checklistTexts(task), []string{"one", "two", "three", "four"}; !equalStrings(got, want) {
		t.Fatalf("checklist = %v, want %v", got, want)
	}
}

func TestMoveChecklistItemRejectsOutOfRangeIndex(t *testing.T) {
	task := typedTask("t1", 1, "Task")
	task.Checklist = []models.ChecklistItem{{Text: "one"}, {Text: "two"}}
	project := newProject("proj-a", "Alpha", task)
	svc := newTestService(newFakeRepo(project))

	_, err := svc.MoveChecklistItem(app.TaskLocator{ProjectID: project.ID, TaskID: task.ID}, 0, 5, task.Checklist)
	if !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
}

func checklistTexts(task *models.Task) []string {
	texts := make([]string, 0, len(task.Checklist))
	for _, item := range task.Checklist {
		texts = append(texts, item.Text)
	}
	return texts
}

func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range want {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
