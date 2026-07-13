package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

// TestVisibleSiblings_SingleProject covers the bounded-window semantics in
// single-project mode. Rows are bucketed by section header; cursor on A
// (Next bucket) must return only [A, C] — B and D (Inbox) are excluded by
// the intervening section-header/separator window boundaries.
func TestVisibleSiblings_SingleProject(t *testing.T) {
	proj := &models.Project{ID: "proj"}

	a := &models.Task{ID: "a", Title: "A", WorkflowState: models.StateNext}
	b := &models.Task{ID: "b", Title: "B", WorkflowState: models.StateInbox}
	c := &models.Task{ID: "c", Title: "C", WorkflowState: models.StateNext}
	d := &models.Task{ID: "d", Title: "D", WorkflowState: models.StateInbox}

	proj.Tasks = []*models.Task{a, b, c, d}

	m := minimalOutliner()
	m.project = proj
	m.rows = []outlinerRow{
		{IsSectionHeader: true, SectionName: models.SectionInbox}, // 0
		{task: b},           // 1
		{task: d},           // 2
		{IsSeparator: true}, // 3
		{IsSectionHeader: true, SectionName: models.SectionNext}, // 4
		{task: a}, // 5
		{task: c}, // 6
	}
	m.cursor = 5 // cursor on A

	got := m.visibleSiblings(m.cursor)
	wantIDs := []string{"a", "c"}
	if len(got) != len(wantIDs) {
		t.Fatalf("len(siblings) = %d, want %d (got %+v)", len(got), len(wantIDs), got)
	}
	for i, id := range wantIDs {
		if got[i].ID != id {
			t.Errorf("siblings[%d].ID = %q, want %q", i, got[i].ID, id)
		}
	}
}

// TestVisibleSiblings_CrossProject covers cross-project mode: the reorder
// window stops at the separator between projects, so cursor on a1 returns
// [a1, a2] and excludes b1 (which lives behind a separator + header).
func TestVisibleSiblings_CrossProject(t *testing.T) {
	projA := &models.Project{ID: "proj-A"}
	projB := &models.Project{ID: "proj-B"}

	a1 := &models.Task{ID: "a1", Title: "A1"}
	a2 := &models.Task{ID: "a2", Title: "A2"}
	b1 := &models.Task{ID: "b1", Title: "B1"}

	projA.Tasks = []*models.Task{a1, a2}
	projB.Tasks = []*models.Task{b1}

	m := minimalOutliner()
	m.loadedProjects = []*models.Project{projA, projB}
	m.rows = []outlinerRow{
		{IsSectionHeader: true, SectionName: models.SectionInbox, ProjectID: projA.ID}, // 0
		{task: a1, ProjectID: projA.ID}, // 1
		{task: a2, ProjectID: projA.ID}, // 2
		{IsSeparator: true},             // 3
		{IsSectionHeader: true, SectionName: models.SectionInbox, ProjectID: projB.ID}, // 4
		{task: b1, ProjectID: projB.ID}, // 5
	}

	m.cursor = 1
	gotA := m.visibleSiblings(m.cursor)
	if len(gotA) != 2 || gotA[0].ID != "a1" || gotA[1].ID != "a2" {
		t.Errorf("projA siblings = %+v, want [a1 a2]", gotA)
	}

	m.cursor = 5
	gotB := m.visibleSiblings(m.cursor)
	if len(gotB) != 1 || gotB[0].ID != "b1" {
		t.Errorf("projB siblings = %+v, want [b1]", gotB)
	}
}

// TestVisibleSiblings_ChildrenInContiguousRun: when child rows form a
// contiguous run in m.rows, the helper returns them in row order. This
// replaces the old "Parent.Children short-circuit" test — children are now
// resolved via the same bounded-window walk as roots, which avoids the
// stale-cloned-parent dereference path.
func TestVisibleSiblings_ChildrenInContiguousRun(t *testing.T) {
	proj := &models.Project{ID: "proj"}
	parent := &models.Task{ID: "p", Title: "P"}
	c1 := &models.Task{ID: "c1", Parent: parent, ParentID: &parent.ID}
	c2 := &models.Task{ID: "c2", Parent: parent, ParentID: &parent.ID}
	c3 := &models.Task{ID: "c3", Parent: parent, ParentID: &parent.ID}
	parent.Children = []*models.Task{c1, c2, c3}
	proj.Tasks = []*models.Task{parent, c1, c2, c3}

	m := minimalOutliner()
	m.project = proj
	m.rows = []outlinerRow{
		{task: parent},
		{task: c1},
		{task: c2},
		{task: c3},
	}
	m.cursor = 2 // cursor on c2

	got := m.visibleSiblings(m.cursor)
	if len(got) != 3 || got[0].ID != "c1" || got[1].ID != "c2" || got[2].ID != "c3" {
		t.Errorf("child siblings = %+v, want [c1 c2 c3]", got)
	}
}

// TestVisibleSiblings_EmptyRows: helper returns nil (no panic) when m.rows
// is empty and cursorIdx is out of range.
func TestVisibleSiblings_EmptyRows(t *testing.T) {
	proj := &models.Project{ID: "proj"}

	m := minimalOutliner()
	m.project = proj
	m.rows = nil
	m.cursor = 0

	got := m.visibleSiblings(m.cursor)
	if len(got) != 0 {
		t.Errorf("empty rows siblings = %+v, want []", got)
	}
}

// TestVisibleSiblings_StopsAtSectionHeader: walking outward stops at a
// section header — a single-task bucket returns just that task.
func TestVisibleSiblings_StopsAtSectionHeader(t *testing.T) {
	a := &models.Task{ID: "a", Title: "A", WorkflowState: models.StateInbox}
	b := &models.Task{ID: "b", Title: "B", WorkflowState: models.StateNext}

	m := minimalOutliner()
	m.project = &models.Project{ID: "proj"}
	m.rows = []outlinerRow{
		{IsSectionHeader: true, SectionName: models.SectionInbox}, // 0
		{task: a}, // 1
		{IsSectionHeader: true, SectionName: models.SectionNext}, // 2
		{task: b}, // 3
	}
	m.cursor = 1

	got := m.visibleSiblings(m.cursor)
	if len(got) != 1 || got[0].ID != "a" {
		t.Errorf("siblings = %+v, want [a]", got)
	}
}

// TestVisibleSiblings_SkipsExpandedDescendants reproduces the `trail` bug:
// in the Next bucket, a root task, an expanded Epic (also a root), the Epic's
// child, and a second root task appear as consecutive rows. Bounded-window
// semantics must skip the interleaved child row so the two roots and the Epic
// remain mutual siblings — contiguous-run semantics would sever them.
func TestVisibleSiblings_SkipsExpandedDescendants(t *testing.T) {
	proj := &models.Project{ID: "proj"}

	root1 := &models.Task{ID: "root1", Title: "Root 1", WorkflowState: models.StateNext}
	epic := &models.Task{ID: "epic", Title: "Epic", WorkflowState: models.StateNext}
	child := &models.Task{ID: "child", Title: "Child", WorkflowState: models.StateNext, Parent: epic, ParentID: &epic.ID}
	epic.Children = []*models.Task{child}
	root2 := &models.Task{ID: "root2", Title: "Root 2", WorkflowState: models.StateNext}

	proj.Tasks = []*models.Task{root1, epic, child, root2}

	m := minimalOutliner()
	m.project = proj
	m.rows = []outlinerRow{
		{IsSectionHeader: true, SectionName: models.SectionNext}, // 0
		{task: root1}, // 1
		{task: epic},  // 2 (expanded)
		{task: child}, // 3 (expanded descendant of epic)
		{task: root2}, // 4
	}

	// Cursor on the lower root sees the expanded Epic as its previous sibling.
	m.cursor = 4
	got := m.visibleSiblings(m.cursor)
	wantIDs := []string{"root1", "epic", "root2"}
	if len(got) != len(wantIDs) {
		t.Fatalf("root2 siblings = %+v, want %v", got, wantIDs)
	}
	for i, id := range wantIDs {
		if got[i].ID != id {
			t.Errorf("root2 siblings[%d].ID = %q, want %q", i, got[i].ID, id)
		}
	}

	// Cursor on the expanded Epic sees the lower root as its next sibling.
	m.cursor = 2
	got = m.visibleSiblings(m.cursor)
	if len(got) != len(wantIDs) {
		t.Fatalf("epic siblings = %+v, want %v", got, wantIDs)
	}
	for i, id := range wantIDs {
		if got[i].ID != id {
			t.Errorf("epic siblings[%d].ID = %q, want %q", i, got[i].ID, id)
		}
	}
}

// TestVisibleSiblings_StopsAtSeparator: walking outward stops at a separator
// row — useful in cross-project mode where separators delimit projects.
func TestVisibleSiblings_StopsAtSeparator(t *testing.T) {
	a := &models.Task{ID: "a", Title: "A"}
	b := &models.Task{ID: "b", Title: "B"}

	m := minimalOutliner()
	m.project = &models.Project{ID: "proj"}
	m.rows = []outlinerRow{
		{task: a},           // 0
		{IsSeparator: true}, // 1
		{task: b},           // 2
	}
	m.cursor = 0

	got := m.visibleSiblings(m.cursor)
	if len(got) != 1 || got[0].ID != "a" {
		t.Errorf("siblings = %+v, want [a]", got)
	}
}
