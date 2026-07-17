package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestProjectViewToggle_DirectKeysKeepProject(t *testing.T) {
	app, project := newTestAppModelWithProject()

	t.Run("project tree opens outliner", func(t *testing.T) {
		tree := NewProjectTreeModel(project, app.configManager, app.runtime)
		action := findTopLevelBinding(t, newProjectTreeKeymapBuilder(tree).primaryViewEntries(), "ctrl+p")
		msg := runEntryAction(t, action)
		show, ok := msg.(ShowOutlinerMsg)
		if !ok {
			t.Fatalf("expected ShowOutlinerMsg, got %T", msg)
		}
		if show.Project != project {
			t.Fatalf("Project = %v, want %v", show.Project, project)
		}
	})

	t.Run("single-project outliner opens tree", func(t *testing.T) {
		outliner := NewOutlinerModel(project, app.configManager, app.runtime)
		action := findTopLevelBinding(t, newOutlinerKeymapBuilder(outliner).primaryViewEntries(), "ctrl+p")
		msg := runEntryAction(t, action)
		show, ok := msg.(ShowProjectTreeMsg)
		if !ok {
			t.Fatalf("expected ShowProjectTreeMsg, got %T", msg)
		}
		if show.Project != project {
			t.Fatalf("Project = %v, want %v", show.Project, project)
		}
	})
}

func TestProjectViewToggle_ReplacesInPlaceAndPreservesState(t *testing.T) {
	app, project := newTestAppModelWithProject()
	tree := NewProjectTreeModel(project, app.configManager, app.runtime)
	tree.SetSize(app.width, app.height-2)
	app.projectTree = tree
	app.views.Register(ViewProjectTree, tree)
	app.pushView(ViewProjectTree)
	initialDepth := len(app.viewStack)

	tree.filter.HideDimmed = true
	tree.sort.Field = SortByPriority
	tree.sidebar.SetEnabled(true)

	var treeFoldTaskID string
	for _, node := range tree.nodes {
		if len(node.Task.Children) > 0 {
			treeFoldTaskID = node.Task.ID
			node.Expanded = false
			break
		}
	}
	if treeFoldTaskID == "" {
		t.Fatal("expected a foldable project-tree node")
	}
	tree.updateVisibility()
	visible := tree.visibleNodes()
	if len(visible) == 0 {
		t.Fatal("expected visible project-tree nodes")
	}
	tree.cursor = len(visible) - 1
	treeCursorTaskID := tree.getCursorTask().ID

	assertProjectViewTransition(t, app, ShowOutlinerMsg{Project: project})
	if app.currentProject != project {
		t.Fatalf("current project changed to %v, want %v", app.currentProject, project)
	}
	if app.currentView() != ViewOutliner {
		t.Fatalf("current view = %v, want %v", app.currentView(), ViewOutliner)
	}
	if len(app.viewStack) != initialDepth {
		t.Fatalf("stack depth = %d, want %d", len(app.viewStack), initialDepth)
	}
	if app.views.Get(ViewOutliner) != app.outliner {
		t.Fatal("top outliner was not registered as the replacement view")
	}
	if !app.outliner.filter.HideDimmed || app.outliner.sort.Field != SortByPriority || !app.outliner.sidebar.IsEnabled() {
		t.Fatalf("shared tree state did not transfer to outliner: filter=%+v sort=%+v sidebar=%v",
			app.outliner.filter, app.outliner.sort, app.outliner.sidebar.IsEnabled())
	}

	var outlinerCursorTaskID string
	for i := len(app.outliner.rows) - 1; i >= 0; i-- {
		row := app.outliner.rows[i]
		if row.isTaskRow() && row.task != nil && row.task.ID != treeFoldTaskID {
			app.outliner.cursor = i
			outlinerCursorTaskID = row.task.ID
			break
		}
	}
	if outlinerCursorTaskID == "" {
		t.Fatal("expected a non-folded outliner cursor task")
	}
	app.outliner.expandedNodes[treeFoldTaskID] = false
	app.outliner.collapsedSections[models.SectionInbox] = true
	app.outliner.sort.Field = SortByCreatedAt

	assertProjectViewTransition(t, app, ShowProjectTreeMsg{Project: project})
	if app.currentProject != project {
		t.Fatalf("current project changed to %v, want %v", app.currentProject, project)
	}
	if app.currentView() != ViewProjectTree {
		t.Fatalf("current view = %v, want %v", app.currentView(), ViewProjectTree)
	}
	if len(app.viewStack) != initialDepth {
		t.Fatalf("stack depth = %d, want %d", len(app.viewStack), initialDepth)
	}
	if app.views.Get(ViewProjectTree) != app.projectTree {
		t.Fatal("top project tree was not registered as the replacement view")
	}
	if app.currentProject != project || !app.projectTree.filter.HideDimmed || app.projectTree.sort.Field != SortByCreatedAt || !app.projectTree.sidebar.IsEnabled() {
		t.Fatalf("shared outliner state did not transfer to tree: filter=%+v sort=%+v sidebar=%v",
			app.projectTree.filter, app.projectTree.sort, app.projectTree.sidebar.IsEnabled())
	}
	if node := app.projectTree.findNodeByTaskID(treeFoldTaskID); node == nil || node.Expanded {
		t.Fatalf("project-tree fold state was not restored for %q", treeFoldTaskID)
	}
	if task := app.projectTree.getCursorTask(); task == nil || task.ID != treeCursorTaskID {
		t.Fatalf("project-tree cursor task = %v, want %q", task, treeCursorTaskID)
	}

	assertProjectViewTransition(t, app, ShowOutlinerMsg{Project: project})
	if app.currentView() != ViewOutliner || len(app.viewStack) != initialDepth {
		t.Fatalf("second outliner toggle left view=%v depth=%d, want view=%v depth=%d",
			app.currentView(), len(app.viewStack), ViewOutliner, initialDepth)
	}
	if expanded, ok := app.outliner.expandedNodes[treeFoldTaskID]; !ok || expanded {
		t.Fatalf("outliner fold state for %q = (%v, present=%v), want collapsed", treeFoldTaskID, expanded, ok)
	}
	if !app.outliner.collapsedSections[models.SectionInbox] {
		t.Fatal("outliner collapsed-section state was not restored")
	}
	if task := app.outliner.getCursorTask(); task == nil || task.ID != outlinerCursorTaskID {
		t.Fatalf("outliner cursor task = %v, want %q", task, outlinerCursorTaskID)
	}
}

func assertProjectViewTransition(t *testing.T, app *AppModel, msg any) {
	t.Helper()
	handled, cmd := app.handleViewMessages(msg)
	if !handled {
		t.Fatalf("project-view transition did not handle %T", msg)
	}
	if cmd != nil {
		t.Fatalf("project-view transition returned unexpected command for %T", msg)
	}
}
