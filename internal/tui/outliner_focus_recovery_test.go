package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

func TestFocusRecoveryUnwindsMovedAndRemovedFramesWithOwningViewport(t *testing.T) {
	root := primaryRowsTask("root", models.StateBacklog)
	child := primaryRowsTask("child", models.StateBacklog)
	child.ParentID = &root.ID
	grandchild := primaryRowsTask("grandchild", models.StateBacklog)
	grandchild.ParentID = &child.ID
	remaining := primaryRowsTask("remaining", models.StateBacklog)
	remaining.ParentID = &root.ID
	outside := primaryRowsTask("outside", models.StateBacklog)
	project := primaryRowsProject("alpha", root, child, grandchild, remaining, outside)

	duplicateRoot := primaryRowsTask(root.ID, models.StateBacklog)
	duplicateChild := primaryRowsTask("beta-child", models.StateBacklog)
	duplicateChild.ParentID = &duplicateRoot.ID
	beta := primaryRowsProject("beta", duplicateRoot, duplicateChild)
	out := scopeTestOutliner(project, beta)

	if !out.MoveCursorToIdentity(taskref.TaskIdentity{ProjectID: project.ID, TaskID: root.ID}) {
		t.Fatal("root row unavailable")
	}
	out.ViewScrollOffset = 6
	if !out.FocusSubtreeAtCursor() || !out.MoveCursorToIdentity(taskref.TaskIdentity{ProjectID: project.ID, TaskID: child.ID}) {
		t.Fatal("could not enter root Focus and select child")
	}
	childFrameCursor := out.Cursor
	out.ViewScrollOffset = 3
	if !out.FocusSubtreeAtCursor() {
		t.Fatal("could not enter nested child Focus")
	}

	child.ParentID = nil
	project.BuildTaskTree()
	out.Population.Invalidate()
	out.CrossProjectRebuildRows()
	if len(out.Focus.CrossStack) != 1 || out.Focus.CrossStack[0].TaskID != root.ID {
		t.Fatalf("reparent recovery stack = %+v, want root frame", out.Focus.CrossStack)
	}
	if out.Cursor != min(childFrameCursor, len(out.Rows)-1) || out.ViewScrollOffset != 3 {
		t.Fatalf("reparent recovery viewport = cursor %d scroll %d, want cursor near %d scroll 3",
			out.Cursor, out.ViewScrollOffset, childFrameCursor)
	}
	if out.Focus.Notice != "Focus target left this branch — returned to parent" {
		t.Fatalf("reparent recovery notice = %q", out.Focus.Notice)
	}
	if visible := visibleOutlinerTaskIDs(out); !visible[remaining.ID] || visible[child.ID] || visible[duplicateChild.ID] {
		t.Fatalf("reparent recovery rows = %v, want only surviving alpha branch descendants", visible)
	}

	project.Tasks = []*models.Task{child, grandchild, remaining, outside}
	project.BuildTaskTree()
	out.Population.Invalidate()
	out.CrossProjectRebuildRows()
	if len(out.Focus.CrossStack) != 0 {
		t.Fatalf("removed root retained Focus frames: %+v", out.Focus.CrossStack)
	}
	if out.ViewScrollOffset != 6 || out.Focus.Notice != "Focus target left this branch — returned to root" {
		t.Fatalf("root recovery = scroll %d notice %q", out.ViewScrollOffset, out.Focus.Notice)
	}
	if !visibleOutlinerTaskIDs(out)[duplicateRoot.ID] {
		t.Fatal("duplicate task ID in beta should remain visible without satisfying alpha Focus ownership")
	}

	project.Tasks = append(project.Tasks, root)
	root.Title = "Restored by undo"
	project.BuildTaskTree()
	out.Population.Invalidate()
	out.CrossProjectRebuildRows()
	if len(out.Focus.CrossStack) != 0 {
		t.Fatalf("undo-like restore silently re-entered Focus: %+v", out.Focus.CrossStack)
	}
}

func TestSingleProjectFocusSurvivesReloadedPointersAndRepairsReparenting(t *testing.T) {
	root := primaryRowsTask("root", models.StateBacklog)
	child := primaryRowsTask("child", models.StateBacklog)
	child.ParentID = &root.ID
	grandchild := primaryRowsTask("grandchild", models.StateBacklog)
	grandchild.ParentID = &child.ID
	project := primaryRowsProject("project", root, child, grandchild)
	out := outliner.NewOutlinerModel(project, nil, nil)
	out.RebuildRows()

	if !out.MoveCursorToIdentity(taskref.TaskIdentity{ProjectID: project.ID, TaskID: root.ID}) || !out.FocusSubtreeAtCursor() ||
		!out.MoveCursorToIdentity(taskref.TaskIdentity{ProjectID: project.ID, TaskID: child.ID}) || !out.FocusSubtreeAtCursor() {
		t.Fatal("could not seed nested single-project Focus")
	}

	reloadedRoot := primaryRowsTask(root.ID, models.StateBacklog)
	reloadedRoot.Title = "Reloaded root"
	reloadedChild := primaryRowsTask(child.ID, models.StateBacklog)
	reloadedChild.Title = "Reloaded child"
	reloadedChild.ParentID = &reloadedRoot.ID
	reloadedGrandchild := primaryRowsTask(grandchild.ID, models.StateBacklog)
	reloadedGrandchild.ParentID = &reloadedChild.ID
	reloaded := primaryRowsProject(project.ID, reloadedRoot, reloadedChild, reloadedGrandchild)
	out.Project = reloaded
	out.RebuildRows()
	if len(out.Focus.Stack) != 2 || out.ProjectFocusContext() != "Focus: [project] › Reloaded root › Reloaded child" {
		t.Fatalf("reload recovery = stack %+v context %q", out.Focus.Stack, out.ProjectFocusContext())
	}

	reloadedChild.ParentID = nil
	reloaded.BuildTaskTree()
	out.RebuildRows()
	if len(out.Focus.Stack) != 1 || out.Focus.Stack[0].TaskID != reloadedRoot.ID ||
		out.Focus.Notice != "Focus target left this branch — returned to parent" {
		t.Fatalf("single-project reparent recovery = stack %+v notice %q", out.Focus.Stack, out.Focus.Notice)
	}
}

func TestFocusEscPriorityAndNarrowBreadcrumb(t *testing.T) {
	root := primaryRowsTask("root", models.StateBacklog)
	root.Title = "A very long focus root title that must fit one row"
	child := primaryRowsTask("child", models.StateBacklog)
	child.ParentID = &root.ID
	project := primaryRowsProject("project", root, child)
	project.Name = "A Long Project Name"
	out := scopeTestOutliner(project)
	out.PushProjectScope(project.ID)
	if !out.MoveCursorToIdentity(taskref.TaskIdentity{ProjectID: project.ID, TaskID: root.ID}) || !out.FocusSubtreeAtCursor() {
		t.Fatal("could not seed scoped task Focus")
	}

	out.SetSize(42, 20)
	title := out.BuildOutlinerTitleLine(42)
	if strings.Contains(title, "\n") || lipgloss.Width(title) > 42 {
		t.Fatalf("narrow Focus title width=%d title=%q", lipgloss.Width(title), ansi.Strip(title))
	}
	if !strings.Contains(ansi.Strip(title), "Focus:") {
		t.Fatalf("narrow title dropped active Focus context: %q", ansi.Strip(title))
	}

	out.SetSearchQuery("child")
	out.StartRangeSelection(taskref.TaskIdentity{ProjectID: project.ID, TaskID: child.ID})
	out.PeekMode = true

	assertState := func(step string, peek, liveRange, selection, search bool, focusDepth int, scoped bool) {
		t.Helper()
		if out.PeekMode != peek || out.IsRangeSelectionActive() != liveRange ||
			out.MultiSelectMode != selection || out.HasSearchProjection() != search ||
			len(out.Focus.CrossStack) != focusDepth || out.IsProjectScoped() != scoped {
			t.Fatalf("%s state = peek %t range %t selection %t search %t focus %d scoped %t",
				step, out.PeekMode, out.IsRangeSelectionActive(), out.MultiSelectMode,
				out.HasSearchProjection(), len(out.Focus.CrossStack), out.IsProjectScoped())
		}
	}
	assertState("initial", true, true, true, true, 1, true)

	for index, want := range []struct {
		peek, liveRange, selection, search bool
		focusDepth                         int
		scoped                             bool
	}{
		{false, true, true, true, 1, true},
		{false, false, true, true, 1, true},
		{false, false, false, true, 1, true},
		{false, false, false, false, 1, true},
		{false, false, false, false, 0, true},
		{false, false, false, false, 0, false},
	} {
		if !listsurface.RunEscCascade(out.EscSteps()) {
			t.Fatalf("Esc step %d was not handled", index+1)
		}
		assertState("after Esc", want.peek, want.liveRange, want.selection, want.search, want.focusDepth, want.scoped)
	}
}
