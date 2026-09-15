package tui

import (
	"maps"
	"slices"
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

// foldAllProject seeds one three-generation tree beside a childless sibling, so
// a bulk fold has both something to collapse and something it must leave alone.
func foldAllProject(t *testing.T) (*outliner.OutlinerModel, *models.Project, map[string]*models.Task) {
	t.Helper()
	parent := primaryRowsTask("parent", models.StateBacklog)
	child := primaryRowsTask("child", models.StateBacklog)
	child.ParentID = &parent.ID
	grandchild := primaryRowsTask("grandchild", models.StateBacklog)
	grandchild.ParentID = &child.ID
	loner := primaryRowsTask("loner", models.StateBacklog)

	project := primaryRowsProject("alpha", parent, child, grandchild, loner)
	outlinerView, _ := primaryRowsOutliner(project)
	outlinerView.SelectOutlinerMode(chrome.ModeAll)

	return outlinerView, project, map[string]*models.Task{
		"parent": parent, "child": child, "grandchild": grandchild, "loner": loner,
	}
}

// One press folds the whole tree and the next press puts it back: t is a toggle,
// so the round trip is two presses of one key rather than two different keys.
func TestToggleAllSubtreesFoldsEveryDescendantAndRestoresThem(t *testing.T) {
	outlinerView, project, tasks := foldAllProject(t)

	if got := visibleTaskIDs(outlinerView); !slices.Contains(got, "grandchild") {
		t.Fatalf("seeded rows = %v, want the full tree expanded by default", got)
	}

	if !outlinerView.ToggleAllSubtrees() {
		t.Fatal("toggleAllSubtrees reported no change on an all-expanded tree")
	}
	if got := visibleTaskIDs(outlinerView); !slices.Equal(got, []string{"parent", "loner"}) {
		t.Fatalf("folded rows = %v, want roots only", got)
	}

	// A childless task has nothing to fold, so it never earns an entry.
	if _, exists := outlinerView.Folds.ExpandedNodes[taskref.TaskIdentity{ProjectID: project.ID, TaskID: tasks["loner"].ID}]; exists {
		t.Error("a childless task gained a fold entry")
	}

	if !outlinerView.ToggleAllSubtrees() {
		t.Fatal("the second press reported no change on a folded tree")
	}
	if got := visibleTaskIDs(outlinerView); !slices.Equal(got, []string{"parent", "child", "grandchild", "loner"}) {
		t.Fatalf("restored rows = %v, want the whole tree back", got)
	}
	if len(outlinerView.Folds.ExpandedNodes) != 0 {
		t.Errorf("the restoring press left %d fold entries, want the canonical empty map", len(outlinerView.Folds.ExpandedNodes))
	}
}

// The direction rule is any, not all: one hand-folded node is enough to make the
// next press unfold everything rather than fold the rest to match it.
func TestToggleAllSubtreesUnfoldsWhenAnySingleSubtreeIsFolded(t *testing.T) {
	outlinerView, project, tasks := foldAllProject(t)

	// Fold the inner node only, leaving its parent expanded — the mixed state a
	// direction rule keyed on "all folded" would read the wrong way.
	outlinerView.Folds.ExpandedNodes[taskref.TaskIdentity{ProjectID: project.ID, TaskID: tasks["child"].ID}] = false
	outlinerView.RebuildRows()
	if got := visibleTaskIDs(outlinerView); !slices.Equal(got, []string{"parent", "child", "loner"}) {
		t.Fatalf("hand-folded rows = %v, want the grandchild hidden and the rest visible", got)
	}

	if !outlinerView.ToggleAllSubtrees() {
		t.Fatal("a mixed tree reported no change")
	}
	if got := visibleTaskIDs(outlinerView); !slices.Equal(got, []string{"parent", "child", "grandchild", "loner"}) {
		t.Fatalf("rows after the press = %v, want everything unfolded", got)
	}
	if len(outlinerView.Folds.ExpandedNodes) != 0 {
		t.Errorf("unfolding left %d fold entries, want the canonical empty map", len(outlinerView.Folds.ExpandedNodes))
	}
}

// The cursor cannot stay on a row a fold just hid, so it rides up to the
// nearest ancestor that still renders.
func TestToggleAllSubtreesRidesTheCursorToAVisibleAncestor(t *testing.T) {
	outlinerView, project, tasks := foldAllProject(t)

	deepest := taskref.TaskIdentity{ProjectID: project.ID, TaskID: tasks["grandchild"].ID}
	if !outlinerView.MoveCursorToIdentity(deepest) {
		t.Fatal("grandchild row is missing before the fold")
	}

	if !outlinerView.ToggleAllSubtrees() {
		t.Fatal("toggleAllSubtrees reported no change")
	}
	row := outlinerView.Rows[outlinerView.Cursor]
	if !row.IsTaskRow() || row.Task == nil || row.Task.ID != tasks["parent"].ID {
		t.Fatalf("cursor after fold = %+v, want the root ancestor", row)
	}

	// Unfolding puts it back on the root it rode to, not the row it left.
	if !outlinerView.ToggleAllSubtrees() {
		t.Fatal("the restoring press reported no change")
	}
	if row := outlinerView.Rows[outlinerView.Cursor]; row.Task == nil || row.Task.ID != tasks["parent"].ID {
		t.Fatalf("cursor after unfold = %+v, want the ancestor it rode to", row)
	}
}

// t owns expandedNodes alone: s drives the section headers inside
// collapsedSections, p drives the project headers beside them, and a checklist
// keeps its own default-collapsed map.
func TestToggleAllSubtreesLeavesTheOtherFoldAxesAlone(t *testing.T) {
	outlinerView, project, tasks := foldAllProject(t)

	checklistOwner := taskref.TaskIdentity{ProjectID: project.ID, TaskID: tasks["parent"].ID}
	outlinerView.Folds.SetChecklist(checklistOwner, true)
	headers := visibleSectionHeaderRows(outlinerView)
	if len(headers) == 0 {
		t.Fatal("All mode emitted no section headers")
	}
	outlinerView.SetSectionRowCollapsed(headers[0], true)
	outlinerView.RebuildRows()

	sectionsBefore := maps.Clone(outlinerView.Folds.CollapsedSections)
	checklistsBefore := maps.Clone(outlinerView.Folds.ExpandedChecklists)

	outlinerView.ToggleAllSubtrees()
	outlinerView.ToggleAllSubtrees()

	if !maps.Equal(outlinerView.Folds.CollapsedSections, sectionsBefore) {
		t.Errorf("the subtree toggle rewrote sections: got %v, want %v", outlinerView.Folds.CollapsedSections, sectionsBefore)
	}
	if !maps.Equal(outlinerView.Folds.ExpandedChecklists, checklistsBefore) {
		t.Errorf("the subtree toggle rewrote checklists: got %v, want %v", outlinerView.Folds.ExpandedChecklists, checklistsBefore)
	}
}

// A view with no parent anywhere has no fold to write in either direction, so
// the toggle reports the refusal the handler turns into a status.
func TestToggleAllSubtreesReportsWithoutAFoldableSubtree(t *testing.T) {
	loner := primaryRowsTask("loner", models.StateBacklog)
	outlinerView, _ := primaryRowsOutliner(primaryRowsProject("alpha", loner))
	outlinerView.SelectOutlinerMode(chrome.ModeAll)

	if outlinerView.ToggleAllSubtrees() {
		t.Fatal("a flat view reported a subtree fold")
	}
	if len(outlinerView.Folds.ExpandedNodes) != 0 {
		t.Errorf("a refused press wrote %d fold entries, want none", len(outlinerView.Folds.ExpandedNodes))
	}
}
