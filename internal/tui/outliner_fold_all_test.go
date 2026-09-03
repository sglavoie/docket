package tui

import (
	"maps"
	"slices"
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

// foldAllProject seeds one three-generation tree beside a childless sibling, so
// a bulk fold has both something to collapse and something it must leave alone.
func foldAllProject(t *testing.T) (*OutlinerModel, *models.Project, map[string]*models.Task) {
	t.Helper()
	parent := primaryRowsTask("parent", models.StateBacklog)
	child := primaryRowsTask("child", models.StateBacklog)
	child.ParentID = &parent.ID
	grandchild := primaryRowsTask("grandchild", models.StateBacklog)
	grandchild.ParentID = &child.ID
	loner := primaryRowsTask("loner", models.StateBacklog)

	project := primaryRowsProject("alpha", parent, child, grandchild, loner)
	outliner, _ := primaryRowsOutliner(project)
	outliner.selectOutlinerMode(chrome.ModeAll)

	return outliner, project, map[string]*models.Task{
		"parent": parent, "child": child, "grandchild": grandchild, "loner": loner,
	}
}

// One press folds the whole tree and the next press puts it back: t is a toggle,
// so the round trip is two presses of one key rather than two different keys.
func TestToggleAllSubtreesFoldsEveryDescendantAndRestoresThem(t *testing.T) {
	outliner, project, tasks := foldAllProject(t)

	if got := visibleTaskIDs(outliner); !slices.Contains(got, "grandchild") {
		t.Fatalf("seeded rows = %v, want the full tree expanded by default", got)
	}

	if !outliner.toggleAllSubtrees() {
		t.Fatal("toggleAllSubtrees reported no change on an all-expanded tree")
	}
	if got := visibleTaskIDs(outliner); !slices.Equal(got, []string{"parent", "loner"}) {
		t.Fatalf("folded rows = %v, want roots only", got)
	}

	// A childless task has nothing to fold, so it never earns an entry.
	if _, exists := outliner.expandedNodes[taskref.TaskIdentity{ProjectID: project.ID, TaskID: tasks["loner"].ID}]; exists {
		t.Error("a childless task gained a fold entry")
	}

	if !outliner.toggleAllSubtrees() {
		t.Fatal("the second press reported no change on a folded tree")
	}
	if got := visibleTaskIDs(outliner); !slices.Equal(got, []string{"parent", "child", "grandchild", "loner"}) {
		t.Fatalf("restored rows = %v, want the whole tree back", got)
	}
	if len(outliner.expandedNodes) != 0 {
		t.Errorf("the restoring press left %d fold entries, want the canonical empty map", len(outliner.expandedNodes))
	}
}

// The direction rule is any, not all: one hand-folded node is enough to make the
// next press unfold everything rather than fold the rest to match it.
func TestToggleAllSubtreesUnfoldsWhenAnySingleSubtreeIsFolded(t *testing.T) {
	outliner, project, tasks := foldAllProject(t)

	// Fold the inner node only, leaving its parent expanded — the mixed state a
	// direction rule keyed on "all folded" would read the wrong way.
	outliner.expandedNodes[taskref.TaskIdentity{ProjectID: project.ID, TaskID: tasks["child"].ID}] = false
	outliner.rebuildRows()
	if got := visibleTaskIDs(outliner); !slices.Equal(got, []string{"parent", "child", "loner"}) {
		t.Fatalf("hand-folded rows = %v, want the grandchild hidden and the rest visible", got)
	}

	if !outliner.toggleAllSubtrees() {
		t.Fatal("a mixed tree reported no change")
	}
	if got := visibleTaskIDs(outliner); !slices.Equal(got, []string{"parent", "child", "grandchild", "loner"}) {
		t.Fatalf("rows after the press = %v, want everything unfolded", got)
	}
	if len(outliner.expandedNodes) != 0 {
		t.Errorf("unfolding left %d fold entries, want the canonical empty map", len(outliner.expandedNodes))
	}
}

// The cursor cannot stay on a row a fold just hid, so it rides up to the
// nearest ancestor that still renders.
func TestToggleAllSubtreesRidesTheCursorToAVisibleAncestor(t *testing.T) {
	outliner, project, tasks := foldAllProject(t)

	deepest := taskref.TaskIdentity{ProjectID: project.ID, TaskID: tasks["grandchild"].ID}
	if !outliner.moveCursorToIdentity(deepest) {
		t.Fatal("grandchild row is missing before the fold")
	}

	if !outliner.toggleAllSubtrees() {
		t.Fatal("toggleAllSubtrees reported no change")
	}
	row := outliner.rows[outliner.cursor]
	if !row.isTaskRow() || row.task == nil || row.task.ID != tasks["parent"].ID {
		t.Fatalf("cursor after fold = %+v, want the root ancestor", row)
	}

	// Unfolding puts it back on the root it rode to, not the row it left.
	if !outliner.toggleAllSubtrees() {
		t.Fatal("the restoring press reported no change")
	}
	if row := outliner.rows[outliner.cursor]; row.task == nil || row.task.ID != tasks["parent"].ID {
		t.Fatalf("cursor after unfold = %+v, want the ancestor it rode to", row)
	}
}

// t owns expandedNodes alone: s drives the section headers inside
// collapsedSections, p drives the project headers beside them, and a checklist
// keeps its own default-collapsed map.
func TestToggleAllSubtreesLeavesTheOtherFoldAxesAlone(t *testing.T) {
	outliner, project, tasks := foldAllProject(t)

	checklistOwner := taskref.TaskIdentity{ProjectID: project.ID, TaskID: tasks["parent"].ID}
	outliner.setChecklistExpanded(checklistOwner, true)
	headers := outliner.visibleSectionHeaders()
	if len(headers) == 0 {
		t.Fatal("All mode emitted no section headers")
	}
	outliner.setSectionRowCollapsed(headers[0], true)
	outliner.rebuildRows()

	sectionsBefore := maps.Clone(outliner.collapsedSections)
	checklistsBefore := maps.Clone(outliner.expandedChecklists)

	outliner.toggleAllSubtrees()
	outliner.toggleAllSubtrees()

	if !maps.Equal(outliner.collapsedSections, sectionsBefore) {
		t.Errorf("the subtree toggle rewrote sections: got %v, want %v", outliner.collapsedSections, sectionsBefore)
	}
	if !maps.Equal(outliner.expandedChecklists, checklistsBefore) {
		t.Errorf("the subtree toggle rewrote checklists: got %v, want %v", outliner.expandedChecklists, checklistsBefore)
	}
}

// A view with no parent anywhere has no fold to write in either direction, so
// the toggle reports the refusal the handler turns into a status.
func TestToggleAllSubtreesReportsWithoutAFoldableSubtree(t *testing.T) {
	loner := primaryRowsTask("loner", models.StateBacklog)
	outliner, _ := primaryRowsOutliner(primaryRowsProject("alpha", loner))
	outliner.selectOutlinerMode(chrome.ModeAll)

	if outliner.toggleAllSubtrees() {
		t.Fatal("a flat view reported a subtree fold")
	}
	if len(outliner.expandedNodes) != 0 {
		t.Errorf("a refused press wrote %d fold entries, want none", len(outliner.expandedNodes))
	}
}
