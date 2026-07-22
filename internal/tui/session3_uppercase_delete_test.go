package tui

import (
	"slices"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// newWorkspaceDeleteView builds a live project-scoped Task Workspace over an
// in-memory project so the session-3 uppercase-X tests can drive handleAction
// directly and inspect the emitted TaskDeleteRequestedMsg without the full
// transactional stack (session 2 already covers the commit path).
func newWorkspaceDeleteView(t *testing.T) (*TaskWorkspaceView, *models.Project, []TaskIdentity) {
	t.Helper()
	project := &models.Project{ID: "p1", Name: "One", Tasks: []*models.Task{
		{ID: "t1", ShortID: 1, Title: "first", WorkflowState: models.StateReady, Priority: 3},
		{ID: "t2", ShortID: 2, Title: "second", WorkflowState: models.StateReady, Priority: 3},
		{ID: "t3", ShortID: 3, Title: "third", WorkflowState: models.StateReady, Priority: 3},
	}}
	params := TaskWorkspaceParams{Scope: ProjectEntityScope(project.ID), Layout: WorkspaceOutline, Focus: WorkspaceActive}
	controller, err := NewTaskWorkspaceController(
		params,
		NewTaskWorkspaceQuery(staticTaskWorkspaceSource{data: TaskWorkspaceData{Projects: []*models.Project{project}}}),
		NewFeatureTaskWorkspaceMutationAdapter(TaskItemCoordinator{}, BulkCoordinator{}),
	)
	if err != nil {
		t.Fatal(err)
	}
	view, err := NewTaskWorkspaceView(controller)
	if err != nil {
		t.Fatal(err)
	}
	if err := controller.Refresh(); err != nil {
		t.Fatal(err)
	}
	ids := make([]TaskIdentity, 0, len(project.Tasks))
	for _, task := range project.Tasks {
		ids = append(ids, TaskIdentity{ProjectID: project.ID, TaskID: task.ID})
	}
	return view, project, ids
}

func deleteRequestFromCmd(t *testing.T, cmd tea.Cmd) TaskDeleteRequestedMsg {
	t.Helper()
	if cmd == nil {
		t.Fatal("expected a command emitting TaskDeleteRequestedMsg, got nil")
	}
	msg, ok := cmd().(TaskDeleteRequestedMsg)
	if !ok {
		t.Fatalf("command emitted %T, want TaskDeleteRequestedMsg", cmd())
	}
	return msg
}

func requestTaskIDs(msg TaskDeleteRequestedMsg) []string {
	ids := make([]string, 0, len(msg.Refs))
	for _, ref := range msg.Refs {
		ids = append(ids, ref.Task.ID)
	}
	slices.Sort(ids)
	return ids
}

// --- Task Workspace direct key -------------------------------------------------

func TestSession3WorkspaceUppercaseXDeletesCursorTaskWhenNoSelection(t *testing.T) {
	view, project, ids := newWorkspaceDeleteView(t)
	if !view.controller.State.SetCursor(ids[1]) {
		t.Fatal("failed to place cursor on the middle task")
	}

	msg := deleteRequestFromCmd(t, view.handleAction(WorkspaceActionDelete))
	if got := requestTaskIDs(msg); !slices.Equal(got, []string{"t2"}) {
		t.Fatalf("cursor-only delete targets = %v, want [t2]", got)
	}
	if msg.Refs[0].Project.ID != project.ID {
		t.Errorf("ref project = %q, want %q", msg.Refs[0].Project.ID, project.ID)
	}
	if msg.Origin != ViewOutliner {
		t.Errorf("origin = %v, want ViewOutliner (project-scope outline)", msg.Origin)
	}
}

func TestSession3WorkspaceUppercaseXDeletesEntireSelection(t *testing.T) {
	view, _, ids := newWorkspaceDeleteView(t)
	// A nonempty selection wins over the cursor.
	view.controller.State.SetCursor(ids[0])
	if !view.controller.State.ToggleSelection(ids[1]) || !view.controller.State.ToggleSelection(ids[2]) {
		t.Fatal("failed to seed a multi-selection")
	}

	msg := deleteRequestFromCmd(t, view.handleAction(WorkspaceActionDelete))
	if got := requestTaskIDs(msg); !slices.Equal(got, []string{"t2", "t3"}) {
		t.Fatalf("multi-select delete targets = %v, want [t2 t3]", got)
	}
}

// TestSession3WorkspaceLowercaseXStaysCompletion locks in that lowercase x
// keeps its completion semantics while uppercase X is the new delete verb, and
// that both are advertised as distinct task-scoped Mutation bindings.
func TestSession3WorkspaceLowercaseXStaysCompletion(t *testing.T) {
	byKey := map[string]TaskWorkspaceBinding{}
	for _, b := range TaskWorkspaceBindingCatalog() {
		byKey[b.Key] = b
	}
	if byKey["x"].Action != WorkspaceActionComplete {
		t.Errorf("x action = %q, want complete", byKey["x"].Action)
	}
	if byKey["X"].Action != WorkspaceActionDelete {
		t.Errorf("X action = %q, want delete", byKey["X"].Action)
	}
	if byKey["X"].Description != "Delete task" {
		t.Errorf("X description = %q, want %q", byKey["X"].Description, "Delete task")
	}
	// Both are task-scoped Mutation bindings that stay distinct.
	if byKey["x"].Category != "Mutation" || byKey["X"].Category != "Mutation" {
		t.Errorf("x/X categories = %q/%q, want Mutation", byKey["x"].Category, byKey["X"].Category)
	}
}

// --- Planner direct key --------------------------------------------------------

func TestSession3PlannerUppercaseXDeletesTargets(t *testing.T) {
	m, task, project := focusBoardTaskActionFixture(t)

	action := findSharedTaskKeymapAction(t, m.buildKeyMap(), "X")
	_, cmd := action()
	msg := deleteRequestFromCmd(t, cmd)
	if got := requestTaskIDs(msg); !slices.Equal(got, []string{task.ID}) {
		t.Fatalf("planner delete targets = %v, want [%s]", got, task.ID)
	}
	if msg.Refs[0].Project.ID != project.ID {
		t.Errorf("planner ref project = %q, want %q", msg.Refs[0].Project.ID, project.ID)
	}
	if msg.Origin != ViewFocusBoard {
		t.Errorf("planner delete origin = %v, want ViewFocusBoard", msg.Origin)
	}

	// Lowercase x keeps completing on the Planner too.
	if complete := findSharedTaskKeymapAction(t, m.buildKeyMap(), "x"); complete == nil {
		t.Fatal("planner lost lowercase x completion binding")
	}
}

// --- Task Detail shell interception -------------------------------------------

func TestSession3TaskDetailUppercaseXDeletesFromEveryTab(t *testing.T) {
	for _, tab := range taskDetailTabOrder {
		app := typedTestApp(t)
		task := app.currentProject.Tasks[0]
		_ = app.activateTaskDetail(task, app.currentProject, tab)

		handled, cmd := app.handleGlobalKeyPress(syntheticKey('X'))
		if !handled {
			t.Fatalf("tab %s: uppercase X should be intercepted by the shell", tab)
		}
		msg := deleteRequestFromCmd(t, cmd)
		if got := requestTaskIDs(msg); !slices.Equal(got, []string{task.ID}) {
			t.Fatalf("tab %s: delete targets = %v, want [%s]", tab, got, task.ID)
		}
		if msg.Origin != ViewTaskDetail {
			t.Errorf("tab %s: origin = %v, want ViewTaskDetail", tab, msg.Origin)
		}
	}
}

// TestSession3TaskDetailUppercaseXSuppressedWhileConfirmArmed proves an armed
// shell confirmation swallows uppercase X instead of stacking a second delete
// request.
func TestSession3TaskDetailUppercaseXSuppressedWhileConfirmArmed(t *testing.T) {
	app := typedTestApp(t)
	task := app.currentProject.Tasks[0]
	_ = app.activateTaskDetail(task, app.currentProject, TaskOverview)

	app.handleTaskDeleteRequested(TaskDeleteRequestedMsg{
		Refs:   []TaskProjectRef{{Task: task, Project: app.currentProject}},
		Origin: ViewTaskDetail,
	})
	if !app.taskDeleteConfirm.IsActive() {
		t.Fatal("confirm should be armed")
	}
	before := app.pendingTaskDelete

	handled, cmd := app.handleGlobalKeyPress(syntheticKey('X'))
	if !handled {
		t.Fatal("armed confirm must consume uppercase X")
	}
	if cmd != nil {
		if _, ok := cmd().(TaskDeleteRequestedMsg); ok {
			t.Fatal("X while confirm is armed must not queue another delete request")
		}
	}
	if app.pendingTaskDelete.roots[0].Task.ID != before.roots[0].Task.ID {
		t.Fatal("pending deletion changed while a confirm was armed")
	}
}

// --- Palette parity ------------------------------------------------------------

// TestSession3PaletteWorkspaceDeleteMatchesKeyboard confirms the palette
// catalog exposes the qualified, collision-safe Delete command and that its
// handler routes back through the same handleAction dispatcher the keyboard
// uses, so palette and keyboard deletion produce the identical request message.
func TestSession3PaletteWorkspaceDeleteMatchesKeyboard(t *testing.T) {
	view, _, ids := newWorkspaceDeleteView(t)
	view.controller.State.SetCursor(ids[0])
	if !view.controller.State.ToggleSelection(ids[0]) || !view.controller.State.ToggleSelection(ids[2]) {
		t.Fatal("failed to seed selection for parity check")
	}

	commands := TaskWorkspaceCommands(func(action TaskWorkspaceAction) tea.Msg {
		cmd := view.handleAction(action)
		if cmd == nil {
			return nil
		}
		return PaletteCommandResultMsg{Cmd: cmd}
	})
	var deleteCmd *Command
	for i := range commands {
		if commands[i].Name == "Task Workspace Delete task" {
			deleteCmd = &commands[i]
			break
		}
	}
	if deleteCmd == nil {
		t.Fatal("palette catalog missing qualified 'Task Workspace Delete task' command")
	}
	if deleteCmd.Shortcut != "X" {
		t.Errorf("palette delete shortcut = %q, want X", deleteCmd.Shortcut)
	}
	if len(deleteCmd.Views) != 0 {
		t.Errorf("workspace palette command must stay capability-scoped, got Views=%v", deleteCmd.Views)
	}

	result, ok := deleteCmd.Handler().(PaletteCommandResultMsg)
	if !ok {
		t.Fatalf("palette delete handler returned %T, want PaletteCommandResultMsg", deleteCmd.Handler())
	}
	paletteMsg := deleteRequestFromCmd(t, result.Cmd)

	keyboardMsg := deleteRequestFromCmd(t, view.handleAction(WorkspaceActionDelete))
	if !slices.Equal(requestTaskIDs(paletteMsg), requestTaskIDs(keyboardMsg)) {
		t.Fatalf("palette targets %v != keyboard targets %v", requestTaskIDs(paletteMsg), requestTaskIDs(keyboardMsg))
	}
	if !slices.Equal(requestTaskIDs(paletteMsg), []string{"t1", "t3"}) {
		t.Fatalf("delete targets = %v, want selection [t1 t3]", requestTaskIDs(paletteMsg))
	}
}

// --- Registry invariant --------------------------------------------------------

func TestSession3RegistryRejectsDifferentCommandReusingOperationID(t *testing.T) {
	registry := NewCommandRegistry()
	first := Command{OperationID: "tui.command.shared", Name: "Alpha", Handler: func() tea.Msg { return nil }}
	if err := registry.Register(first); err != nil {
		t.Fatalf("first Register: %v", err)
	}
	clash := Command{OperationID: "tui.command.shared", Name: "Beta", Handler: func() tea.Msg { return nil }}
	if err := registry.Register(clash); err == nil {
		t.Fatal("a different command reusing an operation ID must be rejected")
	}

	// The same logical command registered for a second, non-overlapping view
	// keeps its operation ID and is allowed.
	sameA := Command{OperationID: "tui.command.scoped", Name: "Gamma", Views: []PresentationID{ViewOutliner}, Handler: func() tea.Msg { return nil }}
	sameB := Command{OperationID: "tui.command.scoped", Name: "Gamma", Views: []PresentationID{ViewCrossProjectOutliner}, Handler: func() tea.Msg { return nil }}
	if err := registry.Register(sameA); err != nil {
		t.Fatalf("scoped Register A: %v", err)
	}
	if err := registry.Register(sameB); err != nil {
		t.Fatalf("same-command second-view Register must be allowed, got: %v", err)
	}
}

// --- Help / footer advertising -------------------------------------------------

func TestSession3UppercaseXDeleteIsAdvertisedEverywhere(t *testing.T) {
	// Task Workspace help derives from its capability catalog.
	view, _, ids := newWorkspaceDeleteView(t)
	view.controller.State.SetCursor(ids[0])
	if !hasHelpKeyDescription(view.GetHelpItems(), "X", "Delete task") {
		t.Error("Task Workspace help does not advertise X: Delete task")
	}

	// Planner help derives from the Planner keymap.
	planner, _, _ := focusBoardTaskActionFixture(t)
	if !hasHelpKeyDescription(planner.GetHelpItems(), "X", "Delete task") {
		t.Error("Planner help does not advertise X: Delete task")
	}

	// Task Detail advertises the shell-owned app-level binding on every tab.
	app := typedTestApp(t)
	task := app.currentProject.Tasks[0]
	for _, tab := range taskDetailTabOrder {
		_ = app.activateTaskDetail(task, app.currentProject, tab)
		if !hasHelpKeyDescription(app.taskDetail.GetHelpItems(), "X", "Delete task") {
			t.Errorf("Task Detail %s tab help does not advertise X: Delete task", tab)
		}
	}
}

func hasHelpKeyDescription(items []HelpItem, key, description string) bool {
	for _, item := range items {
		if item.Key == key && item.Description == description {
			return true
		}
	}
	return false
}
