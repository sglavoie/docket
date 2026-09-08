package tui

import (
	"slices"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

func TestFocusKeyGrammarDispatchHelpAndGuidanceAgree(t *testing.T) {
	registry := registeredSimplifiedOperationRegistry(t)
	ctx := simplifiedOperationContext()
	ctx.Entity = operation.EntityContext{ProjectID: "project", TaskID: "parent"}
	ctx.CurrentEntity = operation.TaskEntity("project", "parent")
	ctx.ScopeProjectIDs = []string{"project"}

	for _, test := range []struct {
		key  string
		want operation.OperationID
	}{
		{key: "enter", want: operation.OperationCoreFocusSubtree},
		{key: "shift+enter", want: operation.OperationCoreFocusParent},
		{key: "b", want: operation.OperationCorePeek},
	} {
		binding, _, ok := registry.InputBindingForKey(operation.InputStrategySimplified, test.key, ctx)
		if !ok || binding.OperationID != test.want {
			t.Fatalf("%s binding = %s available=%t, want %s", test.key, binding.OperationID, ok, test.want)
		}
	}

	help := registry.HelpItemsForInputStrategy(operation.InputStrategySimplified, ctx)
	footer := guidanceHintsText(registry.GuidanceHintsForInputStrategy(operation.InputStrategySimplified, ctx))
	for key, description := range map[string]string{
		"Enter":       "Focus Subtree",
		"Shift+Enter": "Focus Parent",
		"b":           "Toggle Task Peek",
	} {
		found := false
		for _, item := range help {
			if item.Key == key && item.Description == description {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("generated help omits %s %s", key, description)
		}
	}
	for _, want := range []string{"Enter focus", "b peek"} {
		if !strings.Contains(footer, want) {
			t.Errorf("generated footer %q omits %q", footer, want)
		}
	}

	section := simplifiedOperationContext()
	section.Entity = operation.EntityContext{HasSectionHeaderCursor: true}
	binding, _, ok := registry.InputBindingForKey(operation.InputStrategySimplified, "enter", section)
	if !ok || binding.OperationID != operation.OperationCoreZoomSection {
		t.Fatalf("section header Enter = %s available=%t, want %s", binding.OperationID, ok, operation.OperationCoreZoomSection)
	}
	sectionHelp := registry.HelpItemsForInputStrategy(operation.InputStrategySimplified, section)
	if !slices.ContainsFunc(sectionHelp, func(item operation.HelpItem) bool {
		return item.Key == "Enter" && item.Description == "Zoom to Section"
	}) {
		t.Error("generated help omits Enter Zoom to Section")
	}

	header := simplifiedOperationContext()
	header.Entity = operation.EntityContext{ProjectID: "project"}
	binding, _, ok = registry.InputBindingForKey(operation.InputStrategySimplified, "enter", header)
	if !ok || binding.OperationID != operation.OperationCoreActivateGroup {
		t.Fatalf("project header Enter = %s available=%t, want %s", binding.OperationID, ok, operation.OperationCoreActivateGroup)
	}
}

func TestCrossProjectFocusShowsDescendantsAndRestoresNestedViewport(t *testing.T) {
	root := &models.Task{ID: "root", Title: "Root", WorkflowState: models.StateBacklog}
	child := &models.Task{ID: "child", Title: "Child", WorkflowState: models.StateBacklog, ParentID: &root.ID}
	grandchild := &models.Task{ID: "grandchild", Title: "Grandchild", WorkflowState: models.StateBacklog, ParentID: &child.ID}
	sibling := &models.Task{ID: "sibling", Title: "Sibling", WorkflowState: models.StateBacklog}
	project := renderFixtureProject(t, "project", "project", "Project", []*models.Task{root, child, grandchild, sibling})
	out := scopeTestOutliner(project)

	if !out.MoveCursorToIdentity(taskref.TaskIdentity{ProjectID: project.ID, TaskID: root.ID}) {
		t.Fatal("root row is unavailable")
	}
	rootCursor := out.Cursor
	out.ViewScrollOffset = 4
	if !out.FocusSubtreeAtCursor() {
		t.Fatal("root did not enter Focus")
	}
	visible := visibleOutlinerTaskIDs(out)
	if visible[root.ID] || visible[sibling.ID] || !visible[child.ID] || !visible[grandchild.ID] {
		t.Fatalf("root Focus rows = %v, want descendants only", visible)
	}
	if len(out.Focus.CrossStack) != 1 || out.Focus.CrossStack[0].ProjectID != project.ID ||
		out.Focus.CrossStack[0].TaskID != root.ID || out.Focus.CrossStack[0].CursorIndex != rootCursor ||
		out.Focus.CrossStack[0].ScrollOffset != 4 {
		t.Fatalf("root Focus frame = %+v", out.Focus.CrossStack)
	}
	if got := out.CrossProjectFocusContext(); got != "Focus: [Project] › Root" {
		t.Fatalf("root Focus context = %q", got)
	}

	if !out.MoveCursorToIdentity(taskref.TaskIdentity{ProjectID: project.ID, TaskID: child.ID}) {
		t.Fatal("child row is unavailable inside root Focus")
	}
	childCursor := out.Cursor
	out.ViewScrollOffset = 2
	if !out.FocusSubtreeAtCursor() {
		t.Fatal("child did not enter nested Focus")
	}
	visible = visibleOutlinerTaskIDs(out)
	if len(visible) != 1 || !visible[grandchild.ID] {
		t.Fatalf("nested Focus rows = %v, want grandchild only", visible)
	}
	if got := out.CrossProjectFocusContext(); got != "Focus: [Project] › Root › Child" {
		t.Fatalf("nested Focus context = %q", got)
	}

	if !out.PopFocusLevel() || out.Cursor != childCursor || out.ViewScrollOffset != 2 ||
		out.CurrentTaskIdentity() != (taskref.TaskIdentity{ProjectID: project.ID, TaskID: child.ID}) {
		t.Fatalf("first restore = cursor %d scroll %d task %+v", out.Cursor, out.ViewScrollOffset, out.CurrentTaskIdentity())
	}
	if !out.PopFocusLevel() || out.Cursor != rootCursor || out.ViewScrollOffset != 4 ||
		out.CurrentTaskIdentity() != (taskref.TaskIdentity{ProjectID: project.ID, TaskID: root.ID}) {
		t.Fatalf("root restore = cursor %d scroll %d task %+v", out.Cursor, out.ViewScrollOffset, out.CurrentTaskIdentity())
	}
}

func TestFocusParentWidensOneLevelAndClimbsOutOfFocus(t *testing.T) {
	newFixture := func() (*outliner.OutlinerModel, *models.Project) {
		root := primaryRowsTask("root", models.StateBacklog)
		child := primaryRowsTask("child", models.StateBacklog)
		child.ParentID = &root.ID
		grandchild := primaryRowsTask("grandchild", models.StateBacklog)
		grandchild.ParentID = &child.ID
		greatGrandchild := primaryRowsTask("great-grandchild", models.StateBacklog)
		greatGrandchild.ParentID = &grandchild.ID
		sibling := primaryRowsTask("sibling", models.StateBacklog)
		project := primaryRowsProject("project", root, child, grandchild, greatGrandchild, sibling)
		return scopeTestOutliner(project), project
	}

	for _, test := range []struct {
		name       string
		seedFocus  []string
		cursor     string
		wantStack  []string
		wantHidden []string
	}{
		{
			name:       "unfocused press focuses the cursor's parent",
			cursor:     "grandchild",
			wantStack:  []string{"child"},
			wantHidden: []string{"root", "sibling"},
		},
		{
			name:      "climbing out of a root focus leaves Focus",
			seedFocus: []string{"root"},
			cursor:    "grandchild",
			wantStack: nil,
		},
		{
			name:      "climbing replaces the only frame with its parent",
			seedFocus: []string{"child"},
			cursor:    "grandchild",
			wantStack: []string{"root"},
		},
		{
			name:      "climbing pops when the frame beneath is already the parent",
			seedFocus: []string{"root", "child"},
			cursor:    "grandchild",
			wantStack: []string{"root"},
		},
		{
			name:      "climbing re-pushes when the popped frame spanned more than one level",
			seedFocus: []string{"root", "grandchild"},
			cursor:    "great-grandchild",
			wantStack: []string{"root", "child"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			out, project := newFixture()
			identity := func(taskID string) taskref.TaskIdentity {
				return taskref.TaskIdentity{ProjectID: project.ID, TaskID: taskID}
			}
			for _, taskID := range test.seedFocus {
				if !out.MoveCursorToIdentity(identity(taskID)) || !out.FocusSubtreeAtCursor() {
					t.Fatalf("could not seed Focus on %q", taskID)
				}
			}
			if !out.MoveCursorToIdentity(identity(test.cursor)) {
				t.Fatalf("cursor task %q is not listed", test.cursor)
			}
			if !out.FocusParentAtCursor() {
				t.Fatal("Focus Parent refused a task that has a parent")
			}

			assertStack := func(step string) {
				t.Helper()
				got := make([]string, 0, len(out.Focus.CrossStack))
				for _, frame := range out.Focus.CrossStack {
					got = append(got, frame.TaskID)
				}
				if !slices.Equal(got, test.wantStack) {
					t.Fatalf("%s focus stack = %v, want %v", step, got, test.wantStack)
				}
			}
			assertStack("after widening")
			if got := out.CurrentTaskIdentity(); got != identity(test.cursor) {
				t.Fatalf("cursor = %+v, want the task the press came from", got)
			}
			visible := visibleOutlinerTaskIDs(out)
			for _, taskID := range test.wantHidden {
				if visible[taskID] {
					t.Fatalf("widened projection still lists %q: %v", taskID, visible)
				}
			}

			// A frame that did not descend from the one before it would be
			// truncated here with a recovery notice instead of surviving.
			out.Population.Invalidate()
			out.CrossProjectRebuildRows()
			assertStack("after rebuild")
			if out.Focus.Notice != "" {
				t.Fatalf("rebuild rejected the widened stack: %q", out.Focus.Notice)
			}
		})
	}
}

func TestFocusParentClimbsTheAncestorChainAndLeavesFocus(t *testing.T) {
	root := primaryRowsTask("root", models.StateBacklog)
	child := primaryRowsTask("child", models.StateBacklog)
	child.ParentID = &root.ID
	grandchild := primaryRowsTask("grandchild", models.StateBacklog)
	grandchild.ParentID = &child.ID
	project := primaryRowsProject("project", root, child, grandchild)
	out := scopeTestOutliner(project)

	if !out.MoveCursorToIdentity(taskref.TaskIdentity{ProjectID: project.ID, TaskID: grandchild.ID}) {
		t.Fatal("grandchild row is unavailable")
	}
	for _, want := range [][]string{{child.ID}, {root.ID}, {}} {
		if !out.FocusParentAtCursor() {
			t.Fatalf("Focus Parent refused while climbing toward %v", want)
		}
		got := make([]string, 0, len(out.Focus.CrossStack))
		for _, frame := range out.Focus.CrossStack {
			got = append(got, frame.TaskID)
		}
		if !slices.Equal(got, want) {
			t.Fatalf("climb step = %v, want %v", got, want)
		}
	}
	// Leaving Focus restores the base projection, where the next press is an
	// ordinary entry on the cursor's parent rather than a further climb.
	if !out.FocusParentAtCursor() || len(out.Focus.CrossStack) != 1 ||
		out.Focus.CrossStack[0].TaskID != child.ID {
		t.Fatalf("press after leaving Focus = %+v, want a fresh parent Focus", out.Focus.CrossStack)
	}
}
