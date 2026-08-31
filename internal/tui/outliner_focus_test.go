package tui

import (
	"slices"
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

func TestFocusKeyGrammarDispatchHelpAndGuidanceAgree(t *testing.T) {
	registry := registeredSimplifiedOperationRegistry(t)
	ctx := simplifiedOperationContext()
	ctx.Entity = EntityContext{ProjectID: "project", TaskID: "parent"}
	ctx.CurrentEntity = TaskEntity("project", "parent")
	ctx.ScopeProjectIDs = []string{"project"}

	for _, test := range []struct {
		key  string
		want OperationID
	}{
		{key: "enter", want: operationCoreFocusSubtree},
		{key: "shift+enter", want: operationCoreFocusParent},
		{key: "ctrl+enter", want: operationCorePeek},
	} {
		binding, _, ok := registry.InputBindingForKey(InputStrategySimplified, test.key, ctx)
		if !ok || binding.OperationID != test.want {
			t.Fatalf("%s binding = %s available=%t, want %s", test.key, binding.OperationID, ok, test.want)
		}
	}

	help := registry.HelpItemsForInputStrategy(InputStrategySimplified, ctx)
	footer := guidanceHintsText(registry.GuidanceHintsForInputStrategy(InputStrategySimplified, ctx))
	for key, description := range map[string]string{
		"Enter":       "Focus Subtree",
		"Shift+Enter": "Focus Parent",
		"Ctrl+Enter":  "Toggle Task Peek",
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
	for _, want := range []string{"Enter focus", "Ctrl+Enter peek"} {
		if !strings.Contains(footer, want) {
			t.Errorf("generated footer %q omits %q", footer, want)
		}
	}

	section := simplifiedOperationContext()
	section.Entity = EntityContext{HasSectionHeaderCursor: true}
	binding, _, ok := registry.InputBindingForKey(InputStrategySimplified, "enter", section)
	if !ok || binding.OperationID != operationCoreZoomSection {
		t.Fatalf("section header Enter = %s available=%t, want %s", binding.OperationID, ok, operationCoreZoomSection)
	}
	sectionHelp := registry.HelpItemsForInputStrategy(InputStrategySimplified, section)
	if !slices.ContainsFunc(sectionHelp, func(item HelpItem) bool {
		return item.Key == "Enter" && item.Description == "Zoom to Section"
	}) {
		t.Error("generated help omits Enter Zoom to Section")
	}

	header := simplifiedOperationContext()
	header.Entity = EntityContext{ProjectID: "project"}
	binding, _, ok = registry.InputBindingForKey(InputStrategySimplified, "enter", header)
	if !ok || binding.OperationID != operationCoreActivateGroup {
		t.Fatalf("project header Enter = %s available=%t, want %s", binding.OperationID, ok, operationCoreActivateGroup)
	}
}

func TestCrossProjectFocusShowsDescendantsAndRestoresNestedViewport(t *testing.T) {
	root := &models.Task{ID: "root", Title: "Root", WorkflowState: models.StateBacklog}
	child := &models.Task{ID: "child", Title: "Child", WorkflowState: models.StateBacklog, ParentID: &root.ID}
	grandchild := &models.Task{ID: "grandchild", Title: "Grandchild", WorkflowState: models.StateBacklog, ParentID: &child.ID}
	sibling := &models.Task{ID: "sibling", Title: "Sibling", WorkflowState: models.StateBacklog}
	project := renderFixtureProject(t, "project", "project", "Project", []*models.Task{root, child, grandchild, sibling})
	out := scopeTestOutliner(project)

	if !out.moveCursorToIdentity(TaskIdentity{ProjectID: project.ID, TaskID: root.ID}) {
		t.Fatal("root row is unavailable")
	}
	rootCursor := out.cursor
	out.viewScrollOffset = 4
	if !out.focusSubtreeAtCursor() {
		t.Fatal("root did not enter Focus")
	}
	visible := visibleOutlinerTaskIDs(out)
	if visible[root.ID] || visible[sibling.ID] || !visible[child.ID] || !visible[grandchild.ID] {
		t.Fatalf("root Focus rows = %v, want descendants only", visible)
	}
	if len(out.crossFocusStack) != 1 || out.crossFocusStack[0].ProjectID != project.ID ||
		out.crossFocusStack[0].TaskID != root.ID || out.crossFocusStack[0].CursorIndex != rootCursor ||
		out.crossFocusStack[0].ScrollOffset != 4 {
		t.Fatalf("root Focus frame = %+v", out.crossFocusStack)
	}
	if got := out.crossProjectFocusContext(); got != "Focus: [Project] › Root" {
		t.Fatalf("root Focus context = %q", got)
	}

	if !out.moveCursorToIdentity(TaskIdentity{ProjectID: project.ID, TaskID: child.ID}) {
		t.Fatal("child row is unavailable inside root Focus")
	}
	childCursor := out.cursor
	out.viewScrollOffset = 2
	if !out.focusSubtreeAtCursor() {
		t.Fatal("child did not enter nested Focus")
	}
	visible = visibleOutlinerTaskIDs(out)
	if len(visible) != 1 || !visible[grandchild.ID] {
		t.Fatalf("nested Focus rows = %v, want grandchild only", visible)
	}
	if got := out.crossProjectFocusContext(); got != "Focus: [Project] › Root › Child" {
		t.Fatalf("nested Focus context = %q", got)
	}

	if !out.popFocusLevel() || out.cursor != childCursor || out.viewScrollOffset != 2 ||
		out.currentTaskIdentity() != (TaskIdentity{ProjectID: project.ID, TaskID: child.ID}) {
		t.Fatalf("first restore = cursor %d scroll %d task %+v", out.cursor, out.viewScrollOffset, out.currentTaskIdentity())
	}
	if !out.popFocusLevel() || out.cursor != rootCursor || out.viewScrollOffset != 4 ||
		out.currentTaskIdentity() != (TaskIdentity{ProjectID: project.ID, TaskID: root.ID}) {
		t.Fatalf("root restore = cursor %d scroll %d task %+v", out.cursor, out.viewScrollOffset, out.currentTaskIdentity())
	}
}

func TestFocusParentWidensOneLevelAndClimbsOutOfFocus(t *testing.T) {
	newFixture := func() (*OutlinerModel, *models.Project) {
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
			identity := func(taskID string) TaskIdentity {
				return TaskIdentity{ProjectID: project.ID, TaskID: taskID}
			}
			for _, taskID := range test.seedFocus {
				if !out.moveCursorToIdentity(identity(taskID)) || !out.focusSubtreeAtCursor() {
					t.Fatalf("could not seed Focus on %q", taskID)
				}
			}
			if !out.moveCursorToIdentity(identity(test.cursor)) {
				t.Fatalf("cursor task %q is not listed", test.cursor)
			}
			if !out.focusParentAtCursor() {
				t.Fatal("Focus Parent refused a task that has a parent")
			}

			assertStack := func(step string) {
				t.Helper()
				got := make([]string, 0, len(out.crossFocusStack))
				for _, frame := range out.crossFocusStack {
					got = append(got, frame.TaskID)
				}
				if !slices.Equal(got, test.wantStack) {
					t.Fatalf("%s focus stack = %v, want %v", step, got, test.wantStack)
				}
			}
			assertStack("after widening")
			if got := out.currentTaskIdentity(); got != identity(test.cursor) {
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
			out.crossForceRebuild = true
			out.crossProjectRebuildRows()
			assertStack("after rebuild")
			if out.focusNotice != "" {
				t.Fatalf("rebuild rejected the widened stack: %q", out.focusNotice)
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

	if !out.moveCursorToIdentity(TaskIdentity{ProjectID: project.ID, TaskID: grandchild.ID}) {
		t.Fatal("grandchild row is unavailable")
	}
	for _, want := range [][]string{{child.ID}, {root.ID}, {}} {
		if !out.focusParentAtCursor() {
			t.Fatalf("Focus Parent refused while climbing toward %v", want)
		}
		got := make([]string, 0, len(out.crossFocusStack))
		for _, frame := range out.crossFocusStack {
			got = append(got, frame.TaskID)
		}
		if !slices.Equal(got, want) {
			t.Fatalf("climb step = %v, want %v", got, want)
		}
	}
	// Leaving Focus restores the base projection, where the next press is an
	// ordinary entry on the cursor's parent rather than a further climb.
	if !out.focusParentAtCursor() || len(out.crossFocusStack) != 1 ||
		out.crossFocusStack[0].TaskID != child.ID {
		t.Fatalf("press after leaving Focus = %+v, want a fresh parent Focus", out.crossFocusStack)
	}
}

func TestCollapseOrMoveToParentReachesTheRowlessFocusRoot(t *testing.T) {
	newFixture := func() (*OutlinerModel, *models.Project, *models.Task) {
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
			identity := func(taskID string) TaskIdentity {
				return TaskIdentity{ProjectID: project.ID, TaskID: taskID}
			}
			for _, taskID := range test.seedFocus {
				if !out.moveCursorToIdentity(identity(taskID)) || !out.focusSubtreeAtCursor() {
					t.Fatalf("could not seed Focus on %q", taskID)
				}
			}
			if test.promoteTag {
				grandchild.Tags = []string{"promote"}
				out.filter.Tags = map[string]bool{"promote": true}
				out.adoptFilterCarriers()
				out.rebuildRows()
			}
			if !out.moveCursorToIdentity(identity("grandchild")) {
				t.Fatal("grandchild row is not listed")
			}

			if got := out.collapseOrMoveToParent(); got != test.wantMoved {
				t.Fatalf("collapseOrMoveToParent() = %t, want %t", got, test.wantMoved)
			}
			stack := make([]string, 0, len(out.crossFocusStack))
			for _, frame := range out.crossFocusStack {
				stack = append(stack, frame.TaskID)
			}
			if !slices.Equal(stack, test.wantStack) {
				t.Fatalf("focus stack = %v, want %v", stack, test.wantStack)
			}
			if got := out.currentTaskIdentity(); got != identity(test.wantCursor) {
				t.Fatalf("cursor = %+v, want %s", got, test.wantCursor)
			}
		})
	}
}

func TestFocusParentRefusesATaskWithoutAParent(t *testing.T) {
	model := typedTestApp(t)
	setTestPresentationPath(model, ViewCrossProjectOutliner)
	out := model.crossOutliner
	out.crossForceRebuild = true
	out.crossProjectRebuildRows()
	rooted := false
	for index, row := range out.rows {
		if row.isTaskRow() && row.task != nil && row.task.ParentID == nil {
			out.cursor = index
			rooted = true
			break
		}
	}
	if !rooted {
		t.Fatal("fixture exposed no root task row")
	}

	result := model.operationExecutor.Execute(OperationExecutionRequest{
		ID:      operationCoreFocusParent,
		Context: model.operationCommandContext(),
	})
	if result.Err != nil || result.Status != "No parent to focus" {
		t.Fatalf("root task Focus Parent = status %q err %v", result.Status, result.Err)
	}
	if len(out.crossFocusStack) != 0 {
		t.Fatalf("refused Focus Parent pushed frames: %+v", out.crossFocusStack)
	}
}

func TestFocusLeafFeedbackAndSearchResultJump(t *testing.T) {
	model := typedTestApp(t)
	setTestPresentationPath(model, ViewCrossProjectOutliner)
	out := model.crossOutliner
	out.crossForceRebuild = true
	out.crossProjectRebuildRows()
	for index, row := range out.rows {
		if row.isTaskRow() && row.task != nil && len(row.task.Children) == 0 {
			out.cursor = index
			break
		}
	}
	leafTitle := out.rows[out.cursor].task.Title

	result := model.operationExecutor.Execute(OperationExecutionRequest{
		ID:      operationCoreFocusSubtree,
		Context: model.operationCommandContext(),
	})
	if result.Err != nil || result.Status != "No children to focus" {
		t.Fatalf("leaf Focus result = status %q err %v", result.Status, result.Err)
	}
	if len(out.crossFocusStack) != 0 {
		t.Fatalf("leaf Focus pushed frames: %+v", out.crossFocusStack)
	}

	t.Run("a leaf result jumps without focusing", func(t *testing.T) {
		identity := searchProjectionCursorOnTitle(t, out, leafTitle)
		result := model.operationExecutor.Execute(OperationExecutionRequest{
			ID:      operationCoreFocusSubtree,
			Context: model.operationCommandContext(),
		})
		if result.Err != nil || result.Status != "No children to focus" {
			t.Fatalf("search-projected leaf Enter = status %q err %v", result.Status, result.Err)
		}
		if out.hasSearchProjection() {
			t.Fatal("Enter kept the temporary search projection")
		}
		if got := out.currentTaskIdentity(); got != identity {
			t.Fatalf("cursor = %+v, want the result %+v", got, identity)
		}
		if len(out.crossFocusStack) != 0 {
			t.Fatalf("a leaf result pushed frames: %+v", out.crossFocusStack)
		}
	})

	t.Run("a parent result jumps and focuses", func(t *testing.T) {
		identity := searchProjectionCursorOnTitle(t, out, "User Authentication")
		result := model.operationExecutor.Execute(OperationExecutionRequest{
			ID:      operationCoreFocusSubtree,
			Context: model.operationCommandContext(),
		})
		if result.Err != nil || result.Status != "" {
			t.Fatalf("search-projected Enter = status %q err %v", result.Status, result.Err)
		}
		if out.hasSearchProjection() {
			t.Fatal("Enter kept the temporary search projection")
		}
		if len(out.crossFocusStack) != 1 || out.crossFocusStack[0].TaskID != identity.TaskID {
			t.Fatalf("focus stack = %+v, want one frame rooted at %+v", out.crossFocusStack, identity)
		}
	})
}

func TestFocusParentFromSearchResultJumpsToTheParentFrame(t *testing.T) {
	model := typedTestApp(t)
	setTestPresentationPath(model, ViewCrossProjectOutliner)
	out := model.crossOutliner
	out.crossForceRebuild = true
	out.crossProjectRebuildRows()

	identity := searchProjectionCursorOnTitle(t, out, "Create login form")
	task, _ := out.taskAndProjectForIdentity(identity)
	if task == nil || task.ParentID == nil {
		t.Fatalf("fixture task %+v has no parent", identity)
	}
	parentID := *task.ParentID

	result := model.operationExecutor.Execute(OperationExecutionRequest{
		ID:      operationCoreFocusParent,
		Context: model.operationCommandContext(),
	})
	if result.Err != nil || result.Status != "" {
		t.Fatalf("search-projected Shift+Enter = status %q err %v", result.Status, result.Err)
	}
	if out.hasSearchProjection() {
		t.Fatal("Shift+Enter kept the temporary search projection")
	}
	if len(out.crossFocusStack) != 1 || out.crossFocusStack[0].TaskID != parentID {
		t.Fatalf("focus stack = %+v, want one frame rooted at %q", out.crossFocusStack, parentID)
	}
	if got := out.currentTaskIdentity(); got != identity {
		t.Fatalf("cursor = %+v, want the result %+v", got, identity)
	}
}

func TestFocusFromSearchForcesAllWhenTheRestoredModeHidesTheResult(t *testing.T) {
	model := typedTestApp(t)
	setTestPresentationPath(model, ViewCrossProjectOutliner)
	out := model.crossOutliner
	out.crossForceRebuild = true
	out.crossProjectRebuildRows()
	out.selectOutlinerMode(ModeToday)

	identity := searchProjectionCursorOnTitle(t, out, "User Authentication")
	result := model.operationExecutor.Execute(OperationExecutionRequest{
		ID:      operationCoreFocusSubtree,
		Context: model.operationCommandContext(),
	})
	if result.Err != nil || result.Status != "" {
		t.Fatalf("search-projected Enter = status %q err %v", result.Status, result.Err)
	}
	if out.primaryMode != ModeAll {
		t.Fatalf("mode = %s, want All forced by a result Today cannot list", out.primaryMode)
	}
	if len(out.crossFocusStack) != 1 || out.crossFocusStack[0].TaskID != identity.TaskID {
		t.Fatalf("focus stack = %+v, want one frame rooted at %+v", out.crossFocusStack, identity)
	}
}

// searchProjectionCursorOnTitle opens the flat ranked list the jump has to leave
// and lands its cursor on the named result. The ranked list is the presentation
// the jump exits; a query that only narrows the mode's own lanes leaves the
// cursor where it already is, which is why the toggle is part of the setup.
func searchProjectionCursorOnTitle(t *testing.T, out *OutlinerModel, title string) TaskIdentity {
	t.Helper()
	searchRankedList(out, title)
	if !out.showsRankedList() {
		t.Fatalf("search %q opened no ranked list", title)
	}
	for index, row := range out.rows {
		if row.isTaskRow() && row.task != nil && row.task.Title == title {
			out.cursor = index
			return out.currentTaskIdentity()
		}
	}
	t.Fatalf("search %q listed no row for it", title)
	return TaskIdentity{}
}

func TestEnterZoomsSectionHeadersAndScopesProjectHeaders(t *testing.T) {
	project := scopeTestProject("project", "Project", "task")
	other := scopeTestProject("other", "Other", "other-task")
	out := scopeTestOutliner(project, other)
	sectionIndex := -1
	for index, row := range out.rows {
		if row.IsSectionHeader {
			sectionIndex = index
			break
		}
	}
	if sectionIndex < 0 {
		t.Fatal("the fixture emits no section header")
	}
	fullRows := len(out.rows)

	out.cursor = sectionIndex
	sectionName := out.rows[sectionIndex].SectionName
	if !out.zoomSectionAtCursor() {
		t.Fatal("Enter did not zoom into the selected section header")
	}
	if names := sectionHeaderNames(out); len(names) != 1 || names[0] != sectionName {
		t.Fatalf("zoomed sections = %v, want %q alone", names, sectionName)
	}

	if !RunEscCascade(out.escSteps()) {
		t.Fatal("Esc did not leave the zoom")
	}
	if out.zoom.active() || len(out.rows) != fullRows {
		t.Fatalf("unzoomed state = zoom %+v rows %d, want no zoom and the pre-zoom %d", out.zoom, len(out.rows), fullRows)
	}

	out.cursor = sectionHeaderIndex(t, out, sectionName)
	if out.activateGroupAtCursor() {
		t.Fatal("a section header still reached the project scope act")
	}

	out.cursor = projectHeaderIndex(t, out, project.ID)
	if !out.activateGroupAtCursor() || out.scopedProjectID != project.ID {
		t.Fatalf("Enter on the project header scoped to %q, want %q", out.scopedProjectID, project.ID)
	}
	if out.zoom.active() {
		t.Fatalf("the project header took a zoom %+v", out.zoom)
	}
	if ids := taskRowProjectIDs(out); len(ids) != 1 || ids[0] != project.ID {
		t.Fatalf("scoped task rows = %v, want %q alone", ids, project.ID)
	}
}

func TestFocusConstrainsModesFiltersSortFoldsAndSearch(t *testing.T) {
	now := time.Now()
	tomorrow := now.Add(24 * time.Hour)
	root := primaryRowsTask("root", models.StateBacklog)
	root.Title = "Focus root"

	insideInbox := primaryRowsTask("inside-inbox", models.StateInbox)
	insideToday := primaryRowsTask("inside-today", models.StateBacklog)
	insideNext := primaryRowsTask("inside-next", models.StateBacklog)
	insideBacklog := primaryRowsTask("inside-backlog", models.StateBacklog)
	insideBacklog.Title = "Zulu inside"
	insideDue := primaryRowsTask("inside-due", models.StateWaiting)
	insideDue.DueDate = &tomorrow
	insideRecent := primaryRowsTask("inside-recent", models.StateSomeday)
	insideRecent.UpdatedAt = now
	insideNested := primaryRowsTask("inside-nested", models.StateBacklog)
	insideNested.Title = "Alpha inside"

	for _, task := range []*models.Task{insideInbox, insideToday, insideNext, insideBacklog, insideDue, insideRecent} {
		task.ParentID = &root.ID
	}
	insideNested.ParentID = &insideBacklog.ID

	outsideInbox := primaryRowsTask("outside-inbox", models.StateInbox)
	outsideToday := primaryRowsTask("outside-today", models.StateBacklog)
	outsideNext := primaryRowsTask("outside-next", models.StateBacklog)
	outsideBacklog := primaryRowsTask("outside-backlog", models.StateBacklog)
	outsideBacklog.Title = "Aardvark outside"
	outsideDue := primaryRowsTask("outside-due", models.StateWaiting)
	outsideDue.DueDate = &tomorrow
	outsideRecent := primaryRowsTask("outside-recent", models.StateSomeday)
	outsideRecent.UpdatedAt = now

	project := primaryRowsProject("project", root,
		insideInbox, insideToday, insideNext, insideBacklog, insideDue, insideRecent, insideNested,
		outsideInbox, outsideToday, outsideNext, outsideBacklog, outsideDue, outsideRecent)
	out := scopeTestOutliner(project)
	out.runtime.Planning().SetToday([]config.ProjectTaskLocator{
		{ProjectID: project.ID, TaskID: insideToday.ID},
		{ProjectID: project.ID, TaskID: outsideToday.ID},
	})
	out.runtime.Planning().SetNext([]config.ProjectTaskLocator{
		{ProjectID: project.ID, TaskID: insideNext.ID},
		{ProjectID: project.ID, TaskID: outsideNext.ID},
	})
	out.crossForceRebuild = true
	out.crossProjectRebuildRows()
	if !out.moveCursorToIdentity(TaskIdentity{ProjectID: project.ID, TaskID: root.ID}) || !out.focusSubtreeAtCursor() {
		t.Fatal("could not enter fixture Focus")
	}

	inside := map[string]bool{
		insideInbox.ID: true, insideToday.ID: true, insideNext.ID: true,
		insideBacklog.ID: true, insideDue.ID: true, insideRecent.ID: true, insideNested.ID: true,
	}
	for _, mode := range []OutlinerModeID{ModeAll, ModeToday, ModeInbox, ModeNext, ModeBacklog, ModeDue, ModeRecent} {
		out.selectOutlinerMode(mode)
		visible := visibleOutlinerTaskIDs(out)
		if len(visible) == 0 {
			t.Fatalf("%s Focus projection is unexpectedly empty", mode)
		}
		for taskID := range visible {
			if !inside[taskID] {
				t.Fatalf("%s Focus escaped to task %q; visible=%v", mode, taskID, visible)
			}
		}
	}

	out.selectOutlinerMode(ModeAll)
	insideNested.Tags = []string{"focus-match"}
	outsideBacklog.Tags = []string{"focus-match"}
	out.filter.Tags = map[string]bool{"focus-match": true}
	out.adoptFilterCarriers()
	out.rebuildRows()
	if got := visibleOutlinerTaskIDs(out); len(got) != 1 || !got[insideNested.ID] {
		t.Fatalf("focused tag facet rows = %v, want only %s", got, insideNested.ID)
	}

	out.filter = NewFilterState()
	out.adoptFilterCarriers()
	out.sort = SortState{Field: SortByTitle, Ascending: true}
	out.treeSort = out.sort
	out.rebuildRows()
	ordered := searchVisibleTaskIDs(out)
	if len(ordered) == 0 || ordered[0] == outsideBacklog.ID || !inside[ordered[0]] {
		t.Fatalf("focused title sort escaped branch: %v", ordered)
	}

	out.expandedNodes[TaskIdentity{ProjectID: project.ID, TaskID: insideBacklog.ID}] = false
	out.rebuildRows()
	if visible := visibleOutlinerTaskIDs(out); visible[insideNested.ID] || visible[outsideBacklog.ID] {
		t.Fatalf("focused fold rows = %v, want nested child hidden and outside task absent", visible)
	}

	insideDue.Title = "Needle inside"
	insideDue.ShortID = 41
	outsideDue.Title = "Needle outside"
	outsideDue.ShortID = 42
	out.SetSearchQuery("needle")
	if got := searchVisibleTaskIDs(out); !slices.Equal(got, []string{insideDue.ID}) {
		t.Fatalf("search within Focus = %v, want only %s", got, insideDue.ID)
	}
	out.SetSearchQuery(project.Slug + "#42")
	if got := searchVisibleTaskIDs(out); len(got) != 0 {
		t.Fatalf("exact search escaped Focus to %v", got)
	}
	out.ClearSearch()
	if len(out.crossFocusStack) != 1 || out.crossFocusStack[0].TaskID != root.ID {
		t.Fatalf("search restoration lost Focus: %+v", out.crossFocusStack)
	}

	insideInbox.WorkflowState = models.StateBacklog
	out.selectOutlinerMode(ModeInbox)
	if len(out.rows) != 0 || out.outlinerEmptyMessage() != "No tasks are visible in this Focus. Press Esc to return." {
		t.Fatalf("empty Focus = rows %d message %q", len(out.rows), out.outlinerEmptyMessage())
	}
}

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

	if !out.moveCursorToIdentity(TaskIdentity{ProjectID: project.ID, TaskID: root.ID}) {
		t.Fatal("root row unavailable")
	}
	out.viewScrollOffset = 6
	if !out.focusSubtreeAtCursor() || !out.moveCursorToIdentity(TaskIdentity{ProjectID: project.ID, TaskID: child.ID}) {
		t.Fatal("could not enter root Focus and select child")
	}
	childFrameCursor := out.cursor
	out.viewScrollOffset = 3
	if !out.focusSubtreeAtCursor() {
		t.Fatal("could not enter nested child Focus")
	}

	child.ParentID = nil
	project.BuildTaskTree()
	out.crossForceRebuild = true
	out.crossProjectRebuildRows()
	if len(out.crossFocusStack) != 1 || out.crossFocusStack[0].TaskID != root.ID {
		t.Fatalf("reparent recovery stack = %+v, want root frame", out.crossFocusStack)
	}
	if out.cursor != min(childFrameCursor, len(out.rows)-1) || out.viewScrollOffset != 3 {
		t.Fatalf("reparent recovery viewport = cursor %d scroll %d, want cursor near %d scroll 3",
			out.cursor, out.viewScrollOffset, childFrameCursor)
	}
	if out.focusNotice != "Focus target left this branch — returned to parent" {
		t.Fatalf("reparent recovery notice = %q", out.focusNotice)
	}
	if visible := visibleOutlinerTaskIDs(out); !visible[remaining.ID] || visible[child.ID] || visible[duplicateChild.ID] {
		t.Fatalf("reparent recovery rows = %v, want only surviving alpha branch descendants", visible)
	}

	project.Tasks = []*models.Task{child, grandchild, remaining, outside}
	project.BuildTaskTree()
	out.crossForceRebuild = true
	out.crossProjectRebuildRows()
	if len(out.crossFocusStack) != 0 {
		t.Fatalf("removed root retained Focus frames: %+v", out.crossFocusStack)
	}
	if out.viewScrollOffset != 6 || out.focusNotice != "Focus target left this branch — returned to root" {
		t.Fatalf("root recovery = scroll %d notice %q", out.viewScrollOffset, out.focusNotice)
	}
	if !visibleOutlinerTaskIDs(out)[duplicateRoot.ID] {
		t.Fatal("duplicate task ID in beta should remain visible without satisfying alpha Focus ownership")
	}

	project.Tasks = append(project.Tasks, root)
	root.Title = "Restored by undo"
	project.BuildTaskTree()
	out.crossForceRebuild = true
	out.crossProjectRebuildRows()
	if len(out.crossFocusStack) != 0 {
		t.Fatalf("undo-like restore silently re-entered Focus: %+v", out.crossFocusStack)
	}
}

func TestSingleProjectFocusSurvivesReloadedPointersAndRepairsReparenting(t *testing.T) {
	root := primaryRowsTask("root", models.StateBacklog)
	child := primaryRowsTask("child", models.StateBacklog)
	child.ParentID = &root.ID
	grandchild := primaryRowsTask("grandchild", models.StateBacklog)
	grandchild.ParentID = &child.ID
	project := primaryRowsProject("project", root, child, grandchild)
	out := NewOutlinerModel(project, nil, nil)
	out.rebuildRows()

	if !out.moveCursorToIdentity(TaskIdentity{ProjectID: project.ID, TaskID: root.ID}) || !out.focusSubtreeAtCursor() ||
		!out.moveCursorToIdentity(TaskIdentity{ProjectID: project.ID, TaskID: child.ID}) || !out.focusSubtreeAtCursor() {
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
	out.project = reloaded
	out.rebuildRows()
	if len(out.focusStack) != 2 || out.projectFocusContext() != "Focus: [project] › Reloaded root › Reloaded child" {
		t.Fatalf("reload recovery = stack %+v context %q", out.focusStack, out.projectFocusContext())
	}

	reloadedChild.ParentID = nil
	reloaded.BuildTaskTree()
	out.rebuildRows()
	if len(out.focusStack) != 1 || out.focusStack[0].TaskID != reloadedRoot.ID ||
		out.focusNotice != "Focus target left this branch — returned to parent" {
		t.Fatalf("single-project reparent recovery = stack %+v notice %q", out.focusStack, out.focusNotice)
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
	out.pushProjectScope(project.ID)
	if !out.moveCursorToIdentity(TaskIdentity{ProjectID: project.ID, TaskID: root.ID}) || !out.focusSubtreeAtCursor() {
		t.Fatal("could not seed scoped task Focus")
	}

	out.SetSize(42, 20)
	title := out.buildOutlinerTitleLine(42)
	if strings.Contains(title, "\n") || lipgloss.Width(title) > 42 {
		t.Fatalf("narrow Focus title width=%d title=%q", lipgloss.Width(title), ansi.Strip(title))
	}
	if !strings.Contains(ansi.Strip(title), "Focus:") {
		t.Fatalf("narrow title dropped active Focus context: %q", ansi.Strip(title))
	}

	out.SetSearchQuery("child")
	out.StartRangeSelection(TaskIdentity{ProjectID: project.ID, TaskID: child.ID})
	out.peekMode = true

	assertState := func(step string, peek, liveRange, selection, search bool, focusDepth int, scoped bool) {
		t.Helper()
		if out.peekMode != peek || out.IsRangeSelectionActive() != liveRange ||
			out.multiSelectMode != selection || out.hasSearchProjection() != search ||
			len(out.crossFocusStack) != focusDepth || out.isProjectScoped() != scoped {
			t.Fatalf("%s state = peek %t range %t selection %t search %t focus %d scoped %t",
				step, out.peekMode, out.IsRangeSelectionActive(), out.multiSelectMode,
				out.hasSearchProjection(), len(out.crossFocusStack), out.isProjectScoped())
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
		if !RunEscCascade(out.escSteps()) {
			t.Fatalf("Esc step %d was not handled", index+1)
		}
		assertState("after Esc", want.peek, want.liveRange, want.selection, want.search, want.focusDepth, want.scoped)
	}
}
