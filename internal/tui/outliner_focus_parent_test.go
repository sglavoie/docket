package tui

import (
	"slices"
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

func TestCollapseOrMoveToParentReachesTheRowlessFocusRoot(t *testing.T) {
	newFixture := func() (*outliner.OutlinerModel, *models.Project, *models.Task) {
		root := primaryRowsTask("root", models.StateBacklog)
		child := primaryRowsTask("child", models.StateBacklog)
		child.ParentID = &root.ID
		grandchild := primaryRowsTask("grandchild", models.StateBacklog)
		grandchild.ParentID = &child.ID
		project := primaryRowsProject("project", root, child, grandchild)
		return scopeTestOutliner(project), project, grandchild
	}

	for _, test := range []struct {
		name       string
		seedFocus  []string
		promoteTag bool
		wantMoved  bool
		wantStack  []string
		wantCursor string
	}{
		{
			name:       "a direct child of the only focus root pops it and selects it",
			seedFocus:  []string{"child"},
			wantMoved:  true,
			wantStack:  nil,
			wantCursor: "child",
		},
		{
			name:       "a nested stack pops exactly one frame",
			seedFocus:  []string{"root", "child"},
			wantMoved:  true,
			wantStack:  []string{"root"},
			wantCursor: "child",
		},
		{
			name:       "a row promoted past a filtered parent stays silent",
			seedFocus:  []string{"root"},
			promoteTag: true,
			wantMoved:  false,
			wantStack:  []string{"root"},
			wantCursor: "grandchild",
		},
		{
			name:       "an unfocused view still moves to the rendered parent",
			wantMoved:  true,
			wantStack:  nil,
			wantCursor: "child",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			out, project, grandchild := newFixture()
			identity := func(taskID string) taskref.TaskIdentity {
				return taskref.TaskIdentity{ProjectID: project.ID, TaskID: taskID}
			}
			for _, taskID := range test.seedFocus {
				if !out.MoveCursorToIdentity(identity(taskID)) || !out.FocusSubtreeAtCursor() {
					t.Fatalf("could not seed Focus on %q", taskID)
				}
			}
			if test.promoteTag {
				grandchild.Tags = []string{"promote"}
				out.Filter.Tags = map[string]bool{"promote": true}
				out.AdoptFilterCarriers()
				out.RebuildRows()
			}
			if !out.MoveCursorToIdentity(identity("grandchild")) {
				t.Fatal("grandchild row is not listed")
			}

			if got := out.CollapseOrMoveToParent(); got != test.wantMoved {
				t.Fatalf("collapseOrMoveToParent() = %t, want %t", got, test.wantMoved)
			}
			stack := make([]string, 0, len(out.Focus.CrossStack))
			for _, frame := range out.Focus.CrossStack {
				stack = append(stack, frame.TaskID)
			}
			if !slices.Equal(stack, test.wantStack) {
				t.Fatalf("focus stack = %v, want %v", stack, test.wantStack)
			}
			if got := out.CurrentTaskIdentity(); got != identity(test.wantCursor) {
				t.Fatalf("cursor = %+v, want %s", got, test.wantCursor)
			}
		})
	}
}

func TestFocusParentRefusesATaskWithoutAParent(t *testing.T) {
	model := typedTestApp(t)
	setTestPresentationPath(model, presentation.ViewCrossProjectOutliner)
	out := model.crossOutliner
	out.Population.Invalidate()
	out.CrossProjectRebuildRows()
	rooted := false
	for index, row := range out.Rows {
		if row.IsTaskRow() && row.Task != nil && row.Task.ParentID == nil {
			out.Cursor = index
			rooted = true
			break
		}
	}
	if !rooted {
		t.Fatal("fixture exposed no root task row")
	}

	result := model.operationExecutor.Execute(operation.OperationExecutionRequest{
		ID:      operation.OperationCoreFocusParent,
		Context: model.operationCommandContext(),
	})
	if result.Err != nil || result.Status != "No parent to focus" {
		t.Fatalf("root task Focus Parent = status %q err %v", result.Status, result.Err)
	}
	if len(out.Focus.CrossStack) != 0 {
		t.Fatalf("refused Focus Parent pushed frames: %+v", out.Focus.CrossStack)
	}
}

func TestFocusLeafFeedbackAndSearchResultJump(t *testing.T) {
	model := typedTestApp(t)
	setTestPresentationPath(model, presentation.ViewCrossProjectOutliner)
	out := model.crossOutliner
	out.Population.Invalidate()
	out.CrossProjectRebuildRows()
	for index, row := range out.Rows {
		if row.IsTaskRow() && row.Task != nil && len(row.Task.Children) == 0 {
			out.Cursor = index
			break
		}
	}
	leafTitle := out.Rows[out.Cursor].Task.Title

	result := model.operationExecutor.Execute(operation.OperationExecutionRequest{
		ID:      operation.OperationCoreFocusSubtree,
		Context: model.operationCommandContext(),
	})
	if result.Err != nil || result.Status != "No children to focus" {
		t.Fatalf("leaf Focus result = status %q err %v", result.Status, result.Err)
	}
	if len(out.Focus.CrossStack) != 0 {
		t.Fatalf("leaf Focus pushed frames: %+v", out.Focus.CrossStack)
	}

	t.Run("a leaf result jumps without focusing", func(t *testing.T) {
		identity := searchProjectionCursorOnTitle(t, out, leafTitle)
		result := model.operationExecutor.Execute(operation.OperationExecutionRequest{
			ID:      operation.OperationCoreFocusSubtree,
			Context: model.operationCommandContext(),
		})
		if result.Err != nil || result.Status != "No children to focus" {
			t.Fatalf("search-projected leaf Enter = status %q err %v", result.Status, result.Err)
		}
		if out.HasSearchProjection() {
			t.Fatal("Enter kept the temporary search projection")
		}
		if got := out.CurrentTaskIdentity(); got != identity {
			t.Fatalf("cursor = %+v, want the result %+v", got, identity)
		}
		if len(out.Focus.CrossStack) != 0 {
			t.Fatalf("a leaf result pushed frames: %+v", out.Focus.CrossStack)
		}
	})

	t.Run("a parent result jumps and focuses", func(t *testing.T) {
		identity := searchProjectionCursorOnTitle(t, out, "User Authentication")
		result := model.operationExecutor.Execute(operation.OperationExecutionRequest{
			ID:      operation.OperationCoreFocusSubtree,
			Context: model.operationCommandContext(),
		})
		if result.Err != nil || result.Status != "" {
			t.Fatalf("search-projected Enter = status %q err %v", result.Status, result.Err)
		}
		if out.HasSearchProjection() {
			t.Fatal("Enter kept the temporary search projection")
		}
		if len(out.Focus.CrossStack) != 1 || out.Focus.CrossStack[0].TaskID != identity.TaskID {
			t.Fatalf("focus stack = %+v, want one frame rooted at %+v", out.Focus.CrossStack, identity)
		}
	})
}

func TestFocusParentFromSearchResultJumpsToTheParentFrame(t *testing.T) {
	model := typedTestApp(t)
	setTestPresentationPath(model, presentation.ViewCrossProjectOutliner)
	out := model.crossOutliner
	out.Population.Invalidate()
	out.CrossProjectRebuildRows()

	identity := searchProjectionCursorOnTitle(t, out, "Create login form")
	task, _ := out.TaskAndProjectForIdentity(identity)
	if task == nil || task.ParentID == nil {
		t.Fatalf("fixture task %+v has no parent", identity)
	}
	parentID := *task.ParentID

	result := model.operationExecutor.Execute(operation.OperationExecutionRequest{
		ID:      operation.OperationCoreFocusParent,
		Context: model.operationCommandContext(),
	})
	if result.Err != nil || result.Status != "" {
		t.Fatalf("search-projected Shift+Enter = status %q err %v", result.Status, result.Err)
	}
	if out.HasSearchProjection() {
		t.Fatal("Shift+Enter kept the temporary search projection")
	}
	if len(out.Focus.CrossStack) != 1 || out.Focus.CrossStack[0].TaskID != parentID {
		t.Fatalf("focus stack = %+v, want one frame rooted at %q", out.Focus.CrossStack, parentID)
	}
	if got := out.CurrentTaskIdentity(); got != identity {
		t.Fatalf("cursor = %+v, want the result %+v", got, identity)
	}
}

func TestFocusFromSearchForcesAllWhenTheRestoredModeHidesTheResult(t *testing.T) {
	model := typedTestApp(t)
	setTestPresentationPath(model, presentation.ViewCrossProjectOutliner)
	out := model.crossOutliner
	out.Population.Invalidate()
	out.CrossProjectRebuildRows()
	out.SelectOutlinerMode(chrome.ModeToday)

	identity := searchProjectionCursorOnTitle(t, out, "User Authentication")
	result := model.operationExecutor.Execute(operation.OperationExecutionRequest{
		ID:      operation.OperationCoreFocusSubtree,
		Context: model.operationCommandContext(),
	})
	if result.Err != nil || result.Status != "" {
		t.Fatalf("search-projected Enter = status %q err %v", result.Status, result.Err)
	}
	if out.PrimaryMode != chrome.ModeAll {
		t.Fatalf("mode = %s, want All forced by a result Today cannot list", out.PrimaryMode)
	}
	if len(out.Focus.CrossStack) != 1 || out.Focus.CrossStack[0].TaskID != identity.TaskID {
		t.Fatalf("focus stack = %+v, want one frame rooted at %+v", out.Focus.CrossStack, identity)
	}
}
