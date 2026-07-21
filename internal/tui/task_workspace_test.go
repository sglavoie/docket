package tui

import (
	"bytes"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

type staticTaskWorkspaceSource struct {
	data TaskWorkspaceData
}

func (s staticTaskWorkspaceSource) LoadTaskWorkspaceData(scope EntityScope) (TaskWorkspaceData, error) {
	data := s.data
	if scope.Kind == ScopeProject {
		data.Projects = slices.DeleteFunc(slices.Clone(data.Projects), func(project *models.Project) bool {
			return project == nil || project.ID != scope.ProjectID
		})
	}
	return data, nil
}

func workspaceTestTask(id string, state models.WorkflowState) *models.Task {
	return &models.Task{ID: id, Title: id, WorkflowState: state, Priority: 3}
}

func workspaceTestQuery(t *testing.T) (*TaskWorkspaceState, TaskWorkspaceQuery, *models.Project, *models.Project, time.Time) {
	t.Helper()
	now := time.Date(2026, 7, 20, 10, 0, 0, 0, time.Local)
	due := now.AddDate(0, 0, 3)
	snoozed := now.AddDate(0, 0, 2)
	parkedAt := now.Add(-time.Hour)
	completedAt := now.Add(-24 * time.Hour)

	blocker := workspaceTestTask("blocker", models.StateReady)
	blocked := workspaceTestTask("blocked", models.StateReady)
	blocked.DependsOn = []string{blocker.ID}
	project1 := &models.Project{ID: "p1", Name: "One", Tasks: []*models.Task{
		workspaceTestTask("active", models.StateReady),
		workspaceTestTask("working", models.StateInProgress),
		workspaceTestTask("inbox", models.StateInbox),
		workspaceTestTask("next", models.StateNext),
		workspaceTestTask("waiting", models.StateWaiting),
		workspaceTestTask("review", models.StateReview),
		workspaceTestTask("someday", models.StateSomeday),
		{ID: "done", Title: "done", WorkflowState: models.StateDone, Priority: 3, CompletedAt: &completedAt},
		{ID: "due", Title: "due", WorkflowState: models.StateReady, Priority: 3, DueDate: &due},
		{ID: "high", Title: "high", WorkflowState: models.StateReady, Priority: 1},
		blocker,
		blocked,
		{ID: "parked", Title: "parked", WorkflowState: models.StateReady, Priority: 3, ParkedAt: &parkedAt},
		{ID: "snoozed", Title: "snoozed", WorkflowState: models.StateReady, Priority: 3, SnoozedUntil: &snoozed},
		{ID: "pinned", Title: "pinned", WorkflowState: models.StateReady, Priority: 3, Pinned: true},
	}}
	project2 := &models.Project{ID: "p2", Name: "Two", Tasks: []*models.Task{
		workspaceTestTask("other", models.StateReady),
	}}
	state, err := NewTaskWorkspaceState(TaskWorkspaceParams{Scope: AppEntityScope(), Layout: WorkspaceOutline, Focus: WorkspaceActive})
	if err != nil {
		t.Fatal(err)
	}
	query := NewTaskWorkspaceQuery(staticTaskWorkspaceSource{data: TaskWorkspaceData{
		Projects: []*models.Project{project1, project2},
		Recent:   []config.RecentTaskVisit{{ProjectID: "p2", TaskID: "other"}, {ProjectID: "p1", TaskID: "done"}},
	}})
	query.now = func() time.Time { return now }
	return state, query, project1, project2, now
}

func workspaceSnapshotTaskIDs(snapshot TaskWorkspaceSnapshot) []string {
	ids := make([]string, len(snapshot.Items))
	for i, item := range snapshot.Items {
		ids[i] = item.Identity.TaskID
	}
	return ids
}

func assertWorkspaceTaskIDs(t *testing.T, got []string, want ...string) {
	t.Helper()
	sort.Strings(got)
	sort.Strings(want)
	if !slices.Equal(got, want) {
		t.Fatalf("task IDs = %v, want %v", got, want)
	}
}

func TestTaskWorkspaceQueryScopesAndEveryFocus(t *testing.T) {
	state, query, _, _, _ := workspaceTestQuery(t)
	cases := []struct {
		focus TaskWorkspaceFocus
		want  []string
	}{
		{WorkspaceActive, []string{"active", "working", "due", "high", "blocker", "blocked", "parked", "pinned", "other"}},
		{WorkspaceAll, []string{"active", "working", "inbox", "next", "waiting", "review", "someday", "due", "high", "blocker", "blocked", "parked", "pinned", "other"}},
		{WorkspaceInbox, []string{"inbox"}},
		{WorkspaceNext, []string{"next"}},
		{WorkspaceWaiting, []string{"waiting"}},
		{WorkspaceReview, []string{"review"}},
		{WorkspaceSomeday, []string{"someday"}},
		{WorkspaceCompleted, []string{"done"}},
		{WorkspaceDueSoon, []string{"due"}},
		{WorkspaceHighPriority, []string{"high"}},
		{WorkspaceBlocked, []string{"blocked"}},
		{WorkspaceParked, []string{"parked"}},
		{WorkspaceSnoozed, []string{"snoozed"}},
		{WorkspacePinned, []string{"pinned"}},
		{WorkspaceRecent, []string{"other", "done"}},
	}
	for _, tc := range cases {
		t.Run(tc.focus.String(), func(t *testing.T) {
			if err := state.SetFocus(tc.focus, ""); err != nil {
				t.Fatal(err)
			}
			snapshot, err := query.Run(state)
			if err != nil {
				t.Fatal(err)
			}
			assertWorkspaceTaskIDs(t, workspaceSnapshotTaskIDs(snapshot), tc.want...)
		})
	}

	if err := state.SetScope(ProjectEntityScope("p2")); err != nil {
		t.Fatal(err)
	}
	if err := state.SetFocus(WorkspaceAll, ""); err != nil {
		t.Fatal(err)
	}
	snapshot, err := query.Run(state)
	if err != nil {
		t.Fatal(err)
	}
	assertWorkspaceTaskIDs(t, workspaceSnapshotTaskIDs(snapshot), "other")
}

func TestTaskWorkspaceStatePreservesIdentityAndPrunesOnlyHiddenSelection(t *testing.T) {
	state, query, _, _, _ := workspaceTestQuery(t)
	_, err := query.Run(state)
	if err != nil {
		t.Fatal(err)
	}
	p1 := TaskIdentity{ProjectID: "p1", TaskID: "active"}
	p2 := TaskIdentity{ProjectID: "p2", TaskID: "other"}
	if !state.SetCursor(p2) || !state.ToggleSelection(p1) || !state.ToggleSelection(p2) {
		t.Fatal("failed to seed cursor and selections")
	}
	if layoutErr := state.SetLayout(WorkspaceBoard); layoutErr != nil {
		t.Fatal(layoutErr)
	}
	if _, err = query.Run(state); err != nil {
		t.Fatal(err)
	}
	if cursor, _ := state.Cursor(); cursor != p2 {
		t.Fatalf("cursor after layout change = %+v, want %+v", cursor, p2)
	}
	if got := state.Selected(); !slices.Equal(got, []TaskIdentity{p1, p2}) {
		t.Fatalf("selection after layout change = %+v", got)
	}
	if !state.SetZoomRoot(p2) {
		t.Fatal("failed to store zoom root by identity")
	}

	if scopeErr := state.SetScope(ProjectEntityScope("p1")); scopeErr != nil {
		t.Fatal(scopeErr)
	}
	if _, zoomed := state.ZoomRoot(); zoomed {
		t.Fatal("scope change retained an out-of-scope zoom root")
	}
	if _, err = query.Run(state); err != nil {
		t.Fatal(err)
	}
	if cursor, _ := state.Cursor(); cursor != p1 {
		t.Fatalf("cursor after scope change = %+v, want first visible %+v", cursor, p1)
	}
	if got := state.Selected(); !slices.Equal(got, []TaskIdentity{p1}) {
		t.Fatalf("selection after scope change = %+v, want visible selection", got)
	}

	if focusErr := state.SetFocus(WorkspaceInbox, ""); focusErr != nil {
		t.Fatal(focusErr)
	}
	if _, err = query.Run(state); err != nil {
		t.Fatal(err)
	}
	if len(state.Selected()) != 0 {
		t.Fatalf("hidden selection was not pruned: %+v", state.Selected())
	}
	if cursor, _ := state.Cursor(); cursor.TaskID != "inbox" {
		t.Fatalf("cursor did not reconcile to visible identity: %+v", cursor)
	}
}

func TestTaskWorkspaceSavedFiltersAndOrganization(t *testing.T) {
	state, query, project, _, now := workspaceTestQuery(t)
	project.GetTaskByID("active").Tags = []string{"backend"}
	criteria := config.SavedFilter{
		ID: "criteria", Name: "High", Mode: "criteria",
		Criteria: []config.SmartListCriterion{{Field: "priority", Operation: "less_than", Value: "2"}},
	}
	snapshotFilter := config.SavedFilter{
		ID: "snapshot", Name: "Backend", Mode: "snapshot", FilterTags: []string{"backend"}, FilterActionable: true,
	}
	query.source = staticTaskWorkspaceSource{data: TaskWorkspaceData{Projects: []*models.Project{project}, SavedFilters: []config.SavedFilter{criteria, snapshotFilter}}}
	query.now = func() time.Time { return now }

	if err := state.SetFocus(WorkspaceSavedFilter, criteria.ID); err != nil {
		t.Fatal(err)
	}
	snapshot, err := query.Run(state)
	if err != nil {
		t.Fatal(err)
	}
	assertWorkspaceTaskIDs(t, workspaceSnapshotTaskIDs(snapshot), "high")

	if focusErr := state.SetFocus(WorkspaceSavedFilter, snapshotFilter.ID); focusErr != nil {
		t.Fatal(focusErr)
	}
	snapshot, err = query.Run(state)
	if err != nil {
		t.Fatal(err)
	}
	assertWorkspaceTaskIDs(t, workspaceSnapshotTaskIDs(snapshot), "active")

	if err := state.SetGrouping(WorkspaceGroupStatus); err != nil {
		t.Fatal(err)
	}
	if err := state.SetSorting(SortState{Field: SortByPriority, Ascending: true}); err != nil {
		t.Fatal(err)
	}
	if err := state.SetColumns([]TaskWorkspaceColumn{WorkspaceColumnTitle, WorkspaceColumnProject}); err != nil {
		t.Fatal(err)
	}
	if err := state.SetBoardAxis(WorkspaceBoardByPriority); err != nil {
		t.Fatal(err)
	}
	if state.Grouping != WorkspaceGroupStatus || state.Sorting.Field != SortByPriority ||
		!slices.Equal(state.Columns, []TaskWorkspaceColumn{WorkspaceColumnTitle, WorkspaceColumnProject}) ||
		state.BoardAxis != WorkspaceBoardByPriority {
		t.Fatalf("organization state was not centralized: %+v", state)
	}
}

func TestTaskWorkspaceNextUsesProjectFocusQueueOrder(t *testing.T) {
	state, query, project, _, now := workspaceTestQuery(t)
	first := workspaceTestTask("queue-first", models.StateNext)
	second := workspaceTestTask("queue-second", models.StateNext)
	unindexed := workspaceTestTask("queue-unindexed", models.StateNext)
	first.Priority, second.Priority, unindexed.Priority = 5, 1, 2
	project.Tasks = append(project.Tasks, first, second, unindexed)
	query.source = staticTaskWorkspaceSource{data: TaskWorkspaceData{
		Projects: []*models.Project{project},
		FocusQueue: []config.FocusQueueItem{
			{ProjectID: project.ID, TaskID: first.ID},
			{ProjectID: project.ID, TaskID: second.ID},
		},
	}}
	query.now = func() time.Time { return now }
	if err := state.SetScope(ProjectEntityScope(project.ID)); err != nil {
		t.Fatal(err)
	}
	if err := state.SetFocus(WorkspaceNext, ""); err != nil {
		t.Fatal(err)
	}
	snapshot, err := query.Run(state)
	if err != nil {
		t.Fatal(err)
	}
	got := workspaceSnapshotTaskIDs(snapshot)
	if len(got) < 4 || !slices.Equal(got[:2], []string{first.ID, second.ID}) ||
		slices.Index(got, unindexed.ID) > slices.Index(got, "next") {
		t.Fatalf("Next/Project Focus order = %v, want indexed order then unindexed", got)
	}
}

func TestTaskWorkspaceKeyGrammarCapabilitiesAndViewMenu(t *testing.T) {
	state, query, _, _, _ := workspaceTestQuery(t)
	snapshot, err := query.Run(state)
	if err != nil {
		t.Fatal(err)
	}
	expectedKeys := []string{
		"j", "down", "k", "up", "g", "G", "ctrl+u", "ctrl+d", "h", "l", "n", "N", "z", "backspace", "\\",
		"enter", "shift+enter", "space", "/", "v",
		"a", "e", "x", "s", "p", "!", "d", "t", "Z", "m", "J", "K", "P", "y", "u", "ctrl+r",
	}
	bindings := TaskWorkspaceBindings(state.Capabilities(snapshot))
	actualKeys := make([]string, len(bindings))
	for i, binding := range bindings {
		actualKeys[i] = binding.Key
	}
	sort.Strings(actualKeys)
	sort.Strings(expectedKeys)
	if !slices.Equal(actualKeys, expectedKeys) {
		t.Fatalf("workspace keys = %v, want exact grammar %v", actualKeys, expectedKeys)
	}
	if slices.Contains(actualKeys, "tab") {
		t.Fatal("Tab must be unbound in Task Workspace")
	}

	ctx := state.CommandContext(snapshot)
	if ctx.Capabilities.Has(presentationCapability(ViewOutliner)) || !ctx.Capabilities.Has(CapabilityTaskWorkspace) {
		t.Fatalf("workspace command context leaked legacy PresentationID capability: %+v", ctx.Capabilities)
	}
	registry := NewCommandRegistry()
	commands := TaskWorkspaceCommands(nil)
	for _, command := range commands {
		if len(command.Views) != 0 {
			t.Fatalf("workspace command %q depends on legacy views: %v", command.Name, command.Views)
		}
		if err := registry.Register(command); err != nil {
			t.Fatal(err)
		}
	}
	if got := registry.GetForContext(ctx); len(got) != len(commands) {
		t.Fatalf("capability-filtered workspace commands = %d, want %d", len(got), len(commands))
	}

	options := []TaskWorkspaceSavedFilterOption{{ID: "mine", Name: "Mine"}}
	menu := TaskWorkspaceViewMenu(state, snapshot, options)
	if slices.ContainsFunc(menu, func(item TaskWorkspaceMenuItem) bool { return strings.HasPrefix(item.ID, "organize:board-axis:") }) {
		t.Fatal("Board axis should be capability-filtered outside Board")
	}
	if err := state.SetLayout(WorkspaceBoard); err != nil {
		t.Fatal(err)
	}
	menu = TaskWorkspaceViewMenu(state, snapshot, options)
	if !slices.ContainsFunc(menu, func(item TaskWorkspaceMenuItem) bool { return item.ID == "organize:board-axis:status" }) {
		t.Fatal("Board axis missing in Board")
	}
	if err := state.SetGrouping(WorkspaceGroupStatus); err != nil {
		t.Fatal(err)
	}
	if err := state.SetSorting(SortState{Field: SortByPriority, Ascending: true}); err != nil {
		t.Fatal(err)
	}
	if err := state.SetColumns([]TaskWorkspaceColumn{WorkspaceColumnTitle, WorkspaceColumnProject}); err != nil {
		t.Fatal(err)
	}
	if err := state.SetBoardAxis(WorkspaceBoardByPriority); err != nil {
		t.Fatal(err)
	}
	savedIndex := slices.IndexFunc(menu, func(item TaskWorkspaceMenuItem) bool { return item.ID == "smart:saved:mine" })
	if savedIndex < 0 || menu[savedIndex].Apply == nil {
		t.Fatal("saved filter missing from declarative Smart menu")
	}
	if err := menu[savedIndex].Apply(state); err != nil {
		t.Fatal(err)
	}
	if state.Focus != WorkspaceSavedFilter || state.SavedFilterID != "mine" {
		t.Fatalf("saved-filter menu did not update shared state: %+v", state.RouteParams())
	}
	if state.Layout != WorkspaceBoard || state.BoardAxis != WorkspaceBoardByPriority ||
		state.Grouping != WorkspaceGroupStatus || state.Sorting.Field != SortByPriority ||
		!slices.Equal(state.Columns, []TaskWorkspaceColumn{WorkspaceColumnTitle, WorkspaceColumnProject}) {
		t.Fatalf("saved filter replaced layout-owned presentation defaults: %+v", state)
	}
}

func TestTaskWorkspaceBulkMutationUsesOneLedgerUndoRedo(t *testing.T) {
	dataDir := t.TempDir()
	first := seedTask(t, 1, "First workspace target", models.StateReady)
	second := seedTask(t, 2, "Second workspace target", models.StateReady)
	project := newLedgerUndoProject(t, dataDir, "task-workspace", first)
	project.Tasks = append(project.Tasks, second)
	project.NextShortID = 3
	project.BuildTaskTree()

	mutator := newLedgerUndoApp(t, dataDir, project, ViewOutliner, first.ID, true)
	before := readTasksFile(t, project)
	secondary := newSecondaryCoordinators(mutator.appService, mutator.configManager)
	stateParams := TaskWorkspaceParams{Scope: ProjectEntityScope(project.ID), Layout: WorkspaceOutline, Focus: WorkspaceActive}
	controller, err := NewTaskWorkspaceController(
		stateParams,
		NewTaskWorkspaceQuery(staticTaskWorkspaceSource{data: TaskWorkspaceData{Projects: []*models.Project{project}}}),
		NewFeatureTaskWorkspaceMutationAdapter(secondary.TaskItem, secondary.Bulk),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := controller.Refresh(); err != nil {
		t.Fatal(err)
	}
	if !controller.State.ToggleSelection(TaskIdentity{ProjectID: project.ID, TaskID: first.ID}) ||
		!controller.State.ToggleSelection(TaskIdentity{ProjectID: project.ID, TaskID: second.ID}) {
		t.Fatal("failed to select both workspace targets")
	}
	cmd := controller.Transition(models.StateNext)
	if cmd == nil {
		t.Fatal("bulk workspace transition returned nil command")
	}
	result := cmd()
	msg, ok := result.(BatchTransactionResultMsg)
	if !ok {
		t.Fatalf("bulk workspace transition returned %T", result)
	}
	mutator.handleBatchTransactionResult(msg)
	after := readTasksFile(t, project)
	if bytes.Equal(before, after) {
		t.Fatal("workspace bulk transition did not change tasks.md")
	}

	restarted := newLedgerUndoApp(t, dataDir, restartLedgerProject(project), ViewOutliner, first.ID, false)
	restarted = sendAppKey(restarted, syntheticKey('u'))
	if got := readTasksFile(t, project); !bytes.Equal(got, before) {
		t.Fatalf("one workspace undo did not restore the complete bulk ChangeSet\n got: %q\nwant: %q", got, before)
	}
	for _, id := range []string{first.ID, second.ID} {
		if task := restarted.currentProject.GetTaskByID(id); task == nil || task.State() != models.StateReady {
			t.Fatalf("task %s was not restored by workspace undo: %+v", id, task)
		}
	}

	restarted = sendAppKey(restarted, syntheticCtrlKey('r'))
	if got := readTasksFile(t, project); !bytes.Equal(got, after) {
		t.Fatalf("one workspace redo did not replay the complete bulk ChangeSet\n got: %q\nwant: %q", got, after)
	}
	for _, id := range []string{first.ID, second.ID} {
		if task := restarted.currentProject.GetTaskByID(id); task == nil || task.State() != models.StateNext {
			t.Fatalf("task %s was not replayed by workspace redo: %+v", id, task)
		}
	}

	ledger := storage.NewWorkspaceLedger(config.UndoLedgerDir(dataDir), testWorkspaceLedgerDepth, []string{dataDir})
	if _, ok, err := ledger.SelectNewestUndo(storage.GlobalScope()); err != nil || !ok {
		t.Fatalf("workspace ledger missing replayed bulk entry: ok=%v err=%v", ok, err)
	}
}
