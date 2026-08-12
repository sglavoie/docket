package tui

import (
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
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
		{key: "shift+enter", want: operationCorePeek},
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
		"Shift+Enter": "Toggle Task Peek",
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
	for _, want := range []string{"Enter focus", "Shift+Enter peek"} {
		if !strings.Contains(footer, want) {
			t.Errorf("generated footer %q omits %q", footer, want)
		}
	}

	header := simplifiedOperationContext()
	binding, _, ok := registry.InputBindingForKey(InputStrategySimplified, "enter", header)
	if !ok || binding.OperationID != operationCoreActivateGroup {
		t.Fatalf("header Enter = %s available=%t, want %s", binding.OperationID, ok, operationCoreActivateGroup)
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

func TestFocusLeafFeedbackAndSearchRefusalGuidance(t *testing.T) {
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

	query := out.rows[out.cursor].task.Title
	out.SetSearchQuery(query)
	if !out.hasSearchProjection() || !out.currentTaskIdentity().Valid() {
		t.Fatalf("search %q did not retain a selected task row", query)
	}
	descriptor, ok := model.operationRegistry.Descriptor(operationCoreFocusSubtree)
	if !ok || descriptor.Applicable(model.operationCommandContext()) {
		t.Fatal("Focus remained applicable during a temporary search projection")
	}
	handled, cmd := model.handleSimplifiedInputKey(syntheticSpecialKey(tea.KeyEnter))
	if !handled || cmd == nil {
		t.Fatalf("search-projected Enter = handled %t cmd %v, want consumed status timeout", handled, cmd)
	}
	const want = "Focus is unavailable during search; press Esc to clear search first"
	if model.statusMessage != want {
		t.Fatalf("search-projected Enter status = %q, want %q", model.statusMessage, want)
	}
}

func TestEnterActivatesSectionAndProjectHeaders(t *testing.T) {
	project := scopeTestProject("project", "Project", "task")
	out := scopeTestOutliner(project)
	sectionIndex := -1
	projectIndex := -1
	for index, row := range out.rows {
		if sectionIndex < 0 && row.IsSectionHeader {
			sectionIndex = index
		}
		if projectIndex < 0 && row.IsProjectHeader && row.ProjectID == project.ID {
			projectIndex = index
		}
	}
	if sectionIndex < 0 || projectIndex < 0 {
		t.Fatalf("header fixture = section %d project %d", sectionIndex, projectIndex)
	}

	out.cursor = sectionIndex
	sectionKey := out.rows[sectionIndex].effectiveCollapseKey()
	if !out.activateGroupAtCursor() || !out.collapsedSections[sectionKey] {
		t.Fatal("Enter did not collapse the selected section header")
	}
	if !out.activateGroupAtCursor() || out.collapsedSections[sectionKey] {
		t.Fatal("Enter did not expand the selected section header")
	}
	out.crossForceRebuild = true
	out.crossProjectRebuildRows()
	for index, row := range out.rows {
		if row.IsProjectHeader && row.ProjectID == project.ID {
			out.cursor = index
			if !out.activateGroupAtCursor() || out.scopedProjectID != project.ID {
				t.Fatal("Enter did not focus the selected project header")
			}
			return
		}
	}
	t.Fatal("project header disappeared after section activation")
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
	out.rebuildRows()
	if got := visibleOutlinerTaskIDs(out); len(got) != 1 || !got[insideNested.ID] {
		t.Fatalf("focused tag facet rows = %v, want only %s", got, insideNested.ID)
	}

	out.filter = NewFilterState()
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
