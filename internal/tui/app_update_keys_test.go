package tui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/external"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

type confirmQueueToggleView struct {
	BaseView
	task          *models.Task
	project       *models.Project
	confirmActive bool
}

func (v *confirmQueueToggleView) Init() tea.Cmd { return nil }

func (v *confirmQueueToggleView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return v, nil
}

func (v *confirmQueueToggleView) View() tea.View { return tea.NewView("") }

func (v *confirmQueueToggleView) getTaskRef() (*models.Task, *models.Project) {
	return v.task, v.project
}

func (v *confirmQueueToggleView) IsConfirmActive() bool { return v.confirmActive }

type undoRedoRecorderTaskRefView struct {
	BaseView
	task         *models.Task
	project      *models.Project
	receivedKeys []string
}

func (v *undoRedoRecorderTaskRefView) Init() tea.Cmd { return nil }

func (v *undoRedoRecorderTaskRefView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyPressMsg); ok {
		switch key.String() {
		case "u", "ctrl+r":
			v.receivedKeys = append(v.receivedKeys, key.String())
		}
	}
	return v, nil
}

func (v *undoRedoRecorderTaskRefView) View() tea.View { return tea.NewView("") }

func (v *undoRedoRecorderTaskRefView) getTaskRef() (*models.Task, *models.Project) {
	return v.task, v.project
}

type ctrlORecorderView struct {
	BaseView
	received bool
}

func (v *ctrlORecorderView) Init() tea.Cmd { return nil }

func (v *ctrlORecorderView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyPressMsg); ok && key.String() == "ctrl+o" {
		v.received = true
	}
	return v, nil
}

func (v *ctrlORecorderView) View() tea.View { return tea.NewView("") }

type ctrlORecorderTaskRefView struct {
	ctrlORecorderView
	task         *models.Task
	project      *models.Project
	hasTextInput bool
}

func (v *ctrlORecorderTaskRefView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyPressMsg); ok && key.String() == "ctrl+o" {
		v.received = true
	}
	return v, nil
}

func (v *ctrlORecorderTaskRefView) getTaskRef() (*models.Task, *models.Project) {
	return v.task, v.project
}

func (v *ctrlORecorderTaskRefView) HasTextInput() bool {
	return v.hasTextInput
}

func ctrlOKeyPress() tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: 'o', Mod: tea.ModCtrl}
}

type ctrlNRecorderView struct {
	BaseView
	received bool
}

func (v *ctrlNRecorderView) Init() tea.Cmd { return nil }

func (v *ctrlNRecorderView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyPressMsg); ok && key.String() == "ctrl+n" {
		v.received = true
	}
	return v, nil
}

func (v *ctrlNRecorderView) View() tea.View { return tea.NewView("") }

type ctrlNRecorderTaskRefView struct {
	ctrlNRecorderView
	task         *models.Task
	project      *models.Project
	hasTextInput bool
}

func (v *ctrlNRecorderTaskRefView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyPressMsg); ok && key.String() == "ctrl+n" {
		v.received = true
	}
	return v, nil
}

func (v *ctrlNRecorderTaskRefView) getTaskRef() (*models.Task, *models.Project) {
	return v.task, v.project
}

func (v *ctrlNRecorderTaskRefView) HasTextInput() bool {
	return v.hasTextInput
}

func ctrlNKeyPress() tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: 'n', Mod: tea.ModCtrl}
}

func ctrlEscKeyPress() tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: tea.KeyEscape, Mod: tea.ModCtrl}
}

func TestHandleGlobalKeyPress_CtrlEscJumpsToDashboard(t *testing.T) {
	app := newTestAppModel()
	app.viewStack = []ViewType{ViewDashboard, ViewProjectTree, ViewTaskForm}
	app.pendingSemicolonPrefix = true
	app.pendingSandboxReset = true
	app.showQuickSwitch = true
	app.peekActive = true

	handled, cmd := app.handleGlobalKeyPress(ctrlEscKeyPress())
	if !handled {
		t.Fatal("expected ctrl+esc to be handled")
	}
	if cmd != nil {
		t.Fatal("dashboard jump should not need a command")
	}
	if app.currentView() != ViewDashboard {
		t.Fatalf("currentView = %v, want Dashboard", app.currentView())
	}
	if len(app.viewStack) != 1 {
		t.Fatalf("stack len = %d, want 1", len(app.viewStack))
	}
	if app.pendingSemicolonPrefix || app.pendingSandboxReset || app.showQuickSwitch || app.peekActive {
		t.Fatal("ctrl+esc should clear transient navigation state")
	}
}

func TestAppUpdate_CtrlEscBypassesTextInputAndLocalView(t *testing.T) {
	app, project := newTestAppModelWithProject()
	view := &ctrlNRecorderTaskRefView{
		task:         project.Tasks[0],
		project:      project,
		hasTextInput: true,
	}
	app.views.Register(ViewTaskForm, view)
	app.viewStack = []ViewType{ViewDashboard, ViewTaskForm}

	app.Update(ctrlEscKeyPress())
	if app.currentView() != ViewDashboard {
		t.Fatalf("currentView = %v, want Dashboard", app.currentView())
	}
	if view.received {
		t.Fatal("ctrl+esc should not fall through to the local text-input view")
	}
}

func TestHandleGlobalKeyPress_QIgnoredDuringConfirmDialog(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.focusQueueIDs = map[string]bool{}
	app.dailyPlanIDs = map[string]bool{}

	view := &confirmQueueToggleView{
		task:          project.Tasks[0],
		project:       project,
		confirmActive: true,
	}
	app.views.Register(ViewDailyReview, view)
	app.viewStack = []ViewType{ViewDashboard, ViewDailyReview}

	handled, _ := app.handleGlobalKeyPress(tea.KeyPressMsg{Code: 'q', Text: "q"})
	if handled {
		t.Fatal("expected q to fall through while confirm dialog is active")
	}
	if len(app.focusQueue) != 0 {
		t.Fatalf("expected queue unchanged, got %d items", len(app.focusQueue))
	}
	if app.focusQueueIDs[project.Tasks[0].ID] {
		t.Fatal("expected task to remain out of queue while confirm dialog is active")
	}
}

func TestHandleGlobalKeyPress_ProjectArchiveConfirmHandlesYesAtAppLevel(t *testing.T) {
	app, project := newTestAppModelWithProject()

	handled, cmd := app.handleProjectMessages(ProjectArchiveRequestMsg{Project: project, Archive: true})
	if !handled {
		t.Fatal("ProjectArchiveRequestMsg should be handled")
	}
	if cmd != nil {
		t.Fatalf("ProjectArchiveRequestMsg should only open confirmation, got cmd %T", cmd)
	}

	handled, cmd = app.handleGlobalKeyPress(syntheticKey('y'))
	if !handled {
		t.Fatal("expected y to be handled by app-owned project archive confirmation")
	}
	if cmd == nil {
		t.Fatal("expected y to return project archive confirmation command")
	}
	if project.Archived {
		t.Fatal("project should not archive until projectArchiveConfirmedMsg is handled")
	}
	if app.projectArchiveConfirm.IsActive() {
		t.Fatal("confirm dialog should close after y")
	}
	if app.pendingProjectArchive != project {
		t.Fatal("pending project should remain until confirmProjectArchive consumes it")
	}

	result := cmd()
	if _, ok := result.(projectArchiveConfirmedMsg); !ok {
		t.Fatalf("confirmation command returned %T, want projectArchiveConfirmedMsg", result)
	}
}

func TestHandleGlobalKeyPress_ProjectArchiveConfirmCancelClearsPendingState(t *testing.T) {
	tests := []struct {
		name string
		key  tea.KeyPressMsg
	}{
		{name: "n", key: syntheticKey('n')},
		{name: "N", key: syntheticKey('N')},
		{name: "esc", key: syntheticSpecialKey(tea.KeyEsc)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app, project := newTestAppModelWithProject()

			app.handleProjectMessages(ProjectArchiveRequestMsg{Project: project, Archive: true})
			handled, cmd := app.handleGlobalKeyPress(tt.key)

			if !handled {
				t.Fatal("expected cancel key to be handled by app-owned project archive confirmation")
			}
			if cmd != nil {
				t.Fatalf("cancel key should not return command, got %T", cmd)
			}
			if project.Archived {
				t.Fatal("project should not archive on cancel")
			}
			if app.pendingProjectArchive != nil {
				t.Fatal("pendingProjectArchive should be cleared on cancel")
			}
			if app.pendingProjectArchiveArchive {
				t.Fatal("pendingProjectArchiveArchive should be cleared on cancel")
			}
			if app.projectArchiveConfirm.IsActive() {
				t.Fatal("confirm dialog should close on cancel")
			}
		})
	}
}

func TestHandleGlobalKeyPress_SpaceFallsThroughWithoutCursorTask(t *testing.T) {
	app := newTestAppModel()

	handled, _ := app.handleGlobalKeyPress(syntheticSpecialKey(tea.KeySpace))
	if handled {
		t.Fatal("expected space to fall through when there is no cursor task")
	}
}

func TestCustomContextEditorSpaceTogglesProjectThroughApp(t *testing.T) {
	app, project := newTestAppModelWithProject()
	// Custom contexts are machine-local state; the editor reads them from the
	// runtime snapshot (which diverges from app.config in the in-memory harness).
	app.runtime.Machine().SetCustomContext("Work", []string{project.ID})
	editor := NewCustomContextEditorModel(app.runtime, "Work")
	app.showView(ViewCustomContextEditor, editor)

	if !editor.selectedIDs[project.ID] {
		t.Fatal("expected project to start selected in existing custom context")
	}

	app.Update(syntheticSpecialKey(tea.KeyTab))
	if !editor.projectListMode {
		t.Fatal("expected tab to move focus to project list")
	}

	app.Update(syntheticSpecialKey(tea.KeySpace))
	if editor.selectedIDs[project.ID] {
		t.Fatal("expected space to toggle selected project off through app dispatch")
	}
	if app.peekActive {
		t.Fatal("space should not activate peek in custom context editor")
	}
}

func TestHandleGlobalKeyPress_CtrlOOpensTaskReference(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.runtime.Machine().Editor = "vim"
	app.runtime.Machine().ExternalEditor = "code"
	task := project.Tasks[0]
	taskTitle := task.Title

	view := &confirmQueueToggleView{
		task:    task,
		project: project,
	}
	app.views.Register(ViewDailyReview, view)
	app.viewStack = []ViewType{ViewDashboard, ViewDailyReview}

	oldCreate := createReferenceSnapshotFile
	oldOpen := openReferenceFile
	t.Cleanup(func() {
		createReferenceSnapshotFile = oldCreate
		openReferenceFile = oldOpen
	})

	var snapshotContent string
	var snapshotPrefix string
	var snapshotSuffix string
	createReferenceSnapshotFile = func(content string, prefix string, suffix string) (string, error) {
		snapshotContent = content
		snapshotPrefix = prefix
		snapshotSuffix = suffix
		return "/tmp/tt-task-reference.md", nil
	}

	var openedEditor string
	var openedPath string
	openReferenceFile = func(editor string, path string) (external.ReferenceOpenResult, error) {
		openedEditor = editor
		openedPath = path
		return external.ReferenceOpenResult{
			Status: external.ReferenceOpenStatusOpened,
			Editor: editor,
			Path:   path,
		}, nil
	}

	handled, cmd := app.handleGlobalKeyPress(ctrlOKeyPress())
	if !handled {
		t.Fatal("expected ctrl+o to be handled for a task-focused view")
	}
	if cmd == nil {
		t.Fatal("expected status timeout command")
	}
	if openedEditor != "code" {
		t.Fatalf("opened editor = %q, want code", openedEditor)
	}
	if openedPath != "/tmp/tt-task-reference.md" {
		t.Fatalf("opened path = %q", openedPath)
	}
	if snapshotPrefix != "tt-task-reference-" || snapshotSuffix != ".md" {
		t.Fatalf("snapshot naming = %q %q", snapshotPrefix, snapshotSuffix)
	}
	if !strings.Contains(snapshotContent, taskTitle) {
		t.Fatalf("snapshot content should include task title, got %q", snapshotContent)
	}
	if !strings.Contains(snapshotContent, external.ReferenceSnapshotWarning) {
		t.Fatalf("snapshot content should include reference warning, got %q", snapshotContent)
	}
	if app.statusMessage != "Opened task reference copy" {
		t.Fatalf("status = %q, want opened reference status", app.statusMessage)
	}
}

func TestHandleGlobalKeyPress_CtrlNOpensTaskProjectNotes(t *testing.T) {
	app, project := newTestAppModelWithProject()
	view := &confirmQueueToggleView{
		task:    project.Tasks[0],
		project: project,
	}
	app.views.Register(ViewDailyReview, view)
	app.viewStack = []ViewType{ViewDashboard, ViewDailyReview}

	handled, cmd := app.handleGlobalKeyPress(ctrlNKeyPress())
	if !handled {
		t.Fatal("expected ctrl+n to be handled for a task-focused view")
	}
	if cmd == nil {
		t.Fatal("expected ShowNotesMsg command")
	}

	result := cmd()
	showNotes, ok := result.(ShowNotesMsg)
	if !ok {
		t.Fatalf("expected ShowNotesMsg, got %T", result)
	}
	if showNotes.Project != project {
		t.Fatal("ShowNotesMsg should target the selected task's project")
	}
}

func TestHandleGlobalKeyPress_CtrlOOpensTaskReferenceFromPrimaryViews(t *testing.T) {
	tests := []struct {
		name      string
		viewType  ViewType
		setupView func(*AppModel, *models.Project) View
	}{
		{
			name:     "project tree",
			viewType: ViewProjectTree,
			setupView: func(app *AppModel, project *models.Project) View {
				tree := NewProjectTreeModel(project, app.configManager, app.runtime)
				app.projectTree = tree
				return tree
			},
		},
		{
			name:     "outliner",
			viewType: ViewOutliner,
			setupView: func(app *AppModel, project *models.Project) View {
				outliner := NewOutlinerModel(project, app.configManager, app.runtime)
				outliner.rebuildRows()
				for i, row := range outliner.rows {
					if row.isTaskRow() && row.task != nil {
						outliner.cursor = i
						break
					}
				}
				app.outliner = outliner
				return outliner
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			app, project := newTestAppModelWithProject()
			app.runtime.Machine().Editor = "vim"
			app.runtime.Machine().ExternalEditor = "code"

			view := tc.setupView(app, project)
			refProvider, ok := view.(taskReferenceProvider)
			if !ok {
				t.Fatalf("%s does not provide task references", tc.viewType)
			}
			task, _ := refProvider.getTaskRef()
			if task == nil {
				t.Fatalf("%s did not resolve a cursor task", tc.viewType)
			}

			app.views.Register(tc.viewType, view)
			app.viewStack = []ViewType{ViewDashboard, tc.viewType}

			oldCreate := createReferenceSnapshotFile
			oldOpen := openReferenceFile
			t.Cleanup(func() {
				createReferenceSnapshotFile = oldCreate
				openReferenceFile = oldOpen
			})

			var snapshotContent string
			createReferenceSnapshotFile = func(content string, prefix string, suffix string) (string, error) {
				snapshotContent = content
				return "/tmp/tt-task-reference.md", nil
			}
			var openedEditor string
			openReferenceFile = func(editor string, path string) (external.ReferenceOpenResult, error) {
				openedEditor = editor
				return external.ReferenceOpenResult{
					Status: external.ReferenceOpenStatusOpened,
					Editor: editor,
					Path:   path,
				}, nil
			}

			handled, cmd := app.handleGlobalKeyPress(ctrlOKeyPress())
			if !handled {
				t.Fatal("expected ctrl+o to be handled by global reference open")
			}
			if cmd == nil {
				t.Fatal("expected status timeout command")
			}
			if !strings.Contains(snapshotContent, task.Title) {
				t.Fatalf("snapshot content should include selected task title %q, got:\n%s", task.Title, snapshotContent)
			}
			if openedEditor != "code" {
				t.Fatalf("opened editor = %q, want code", openedEditor)
			}
			if app.statusMessage != "Opened task reference copy" {
				t.Fatalf("status = %q, want opened reference status", app.statusMessage)
			}
		})
	}
}

func TestHandleGlobalKeyPress_CtrlOShowsStatusWithoutCursorTask(t *testing.T) {
	app := newTestAppModel()
	view := &confirmQueueToggleView{}
	app.views.Register(ViewDailyReview, view)
	app.viewStack = []ViewType{ViewDashboard, ViewDailyReview}

	handled, cmd := app.handleGlobalKeyPress(ctrlOKeyPress())
	if !handled {
		t.Fatal("expected ctrl+o to be handled for task-capable view without cursor task")
	}
	if cmd == nil {
		t.Fatal("expected status timeout command")
	}
	if !strings.Contains(app.statusMessage, "No task selected") {
		t.Fatalf("status = %q, want no task selected", app.statusMessage)
	}
}

func TestHandleGlobalKeyPress_CtrlNShowsStatusWithoutCursorTask(t *testing.T) {
	app := newTestAppModel()
	view := &confirmQueueToggleView{}
	app.views.Register(ViewDailyReview, view)
	app.viewStack = []ViewType{ViewDashboard, ViewDailyReview}

	handled, cmd := app.handleGlobalKeyPress(ctrlNKeyPress())
	if !handled {
		t.Fatal("expected ctrl+n to be handled for task-capable view without cursor task")
	}
	if cmd == nil {
		t.Fatal("expected status timeout command")
	}
	if app.statusMessage != "No task selected for Notes" {
		t.Fatalf("status = %q, want no task selected for Notes", app.statusMessage)
	}
}

func TestAppUpdate_CtrlOFallsThroughToLocalViewWithoutTaskReference(t *testing.T) {
	app := newTestAppModel()
	view := &ctrlORecorderView{}
	app.views.Register(ViewTaskAttachments, view)
	app.viewStack = []ViewType{ViewDashboard, ViewTaskAttachments}

	app.Update(ctrlOKeyPress())
	if !view.received {
		t.Fatal("expected local view to receive ctrl+o")
	}
	if app.statusMessage != "" {
		t.Fatalf("status = %q, want no global status", app.statusMessage)
	}
}

func TestAppUpdate_CtrlNFallsThroughToLocalViewWithoutTaskReference(t *testing.T) {
	app := newTestAppModel()
	view := &ctrlNRecorderView{}
	app.views.Register(ViewTaskAttachments, view)
	app.viewStack = []ViewType{ViewDashboard, ViewTaskAttachments}

	app.Update(ctrlNKeyPress())
	if !view.received {
		t.Fatal("expected local view to receive ctrl+n")
	}
	if app.statusMessage != "" {
		t.Fatalf("status = %q, want no global status", app.statusMessage)
	}
}

func TestAppUpdate_CtrlOFallsThroughWhenTaskViewHasTextInput(t *testing.T) {
	app, project := newTestAppModelWithProject()
	view := &ctrlORecorderTaskRefView{
		task:         project.Tasks[0],
		project:      project,
		hasTextInput: true,
	}
	app.views.Register(ViewTaskForm, view)
	app.viewStack = []ViewType{ViewDashboard, ViewTaskForm}

	oldOpen := openReferenceFile
	t.Cleanup(func() {
		openReferenceFile = oldOpen
	})
	openReferenceFile = func(string, string) (external.ReferenceOpenResult, error) {
		t.Fatal("reference opener should not run while a text input is active")
		return external.ReferenceOpenResult{}, nil
	}

	app.Update(ctrlOKeyPress())
	if !view.received {
		t.Fatal("expected text-input view to receive ctrl+o")
	}
	if app.statusMessage != "" {
		t.Fatalf("status = %q, want no global status", app.statusMessage)
	}
}

func TestAppUpdate_CtrlNFallsThroughWhenTaskViewHasTextInput(t *testing.T) {
	app, project := newTestAppModelWithProject()
	view := &ctrlNRecorderTaskRefView{
		task:         project.Tasks[0],
		project:      project,
		hasTextInput: true,
	}
	app.views.Register(ViewTaskForm, view)
	app.viewStack = []ViewType{ViewDashboard, ViewTaskForm}

	app.Update(ctrlNKeyPress())
	if !view.received {
		t.Fatal("expected text-input view to receive ctrl+n")
	}
	if app.statusMessage != "" {
		t.Fatalf("status = %q, want no global status", app.statusMessage)
	}
}

func TestHandleGlobalKeyPress_UndoRedoRoutesForTaskSecondaryView(t *testing.T) {
	tests := []struct {
		name string
		key  tea.KeyPressMsg
	}{
		{name: "undo", key: syntheticKey('u')},
		{name: "redo", key: tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app, project := newTestAppModelWithProject()
			project.Path = t.TempDir()
			view := &undoRedoRecorderTaskRefView{
				task:    project.Tasks[0],
				project: project,
			}
			app.views.Register(ViewPriorityView, view)
			app.viewStack = []ViewType{ViewDashboard, ViewPriorityView}

			handled, cmd := app.handleGlobalKeyPress(tt.key)
			if !handled {
				t.Fatalf("%s should be handled by app-level persistent routing", tt.name)
			}
			if cmd == nil {
				t.Fatalf("%s should return a persistent undo/redo command", tt.name)
			}
			if got := cmd(); got == nil {
				t.Fatalf("%s command returned nil, want no-history ErrorMsg", tt.name)
			} else if _, ok := got.(ErrorMsg); !ok {
				t.Fatalf("%s command returned %T, want ErrorMsg", tt.name, got)
			}
			if len(view.receivedKeys) != 0 {
				t.Fatalf("global %s should not fall through to local view, got %v", tt.name, view.receivedKeys)
			}
		})
	}
}

func TestHandleGlobalKeyPress_UndoRedoPreservesProtectedLocalViews(t *testing.T) {
	tests := []struct {
		name     string
		viewType ViewType
		key      tea.KeyPressMsg
	}{
		{name: "inbox undo", viewType: ViewInboxProcessor, key: syntheticKey('u')},
		{name: "attachments redo", viewType: ViewTaskAttachments, key: tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl}},
		{name: "primary undo", viewType: ViewProjectTree, key: syntheticKey('u')},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app, project := newTestAppModelWithProject()
			view := &undoRedoRecorderTaskRefView{
				task:    project.Tasks[0],
				project: project,
			}
			app.views.Register(tt.viewType, view)
			app.viewStack = []ViewType{ViewDashboard, tt.viewType}

			handled, cmd := app.handleGlobalKeyPress(tt.key)
			if handled || cmd != nil {
				t.Fatalf("global handler should not intercept %s in %s", tt.key.String(), tt.viewType)
			}
		})
	}
}

func TestHandleGlobalKeyPress_PersistentUndoWinsOverOlderFocusToggle(t *testing.T) {
	app, project := newTestAppModelWithProject()
	project.Path = t.TempDir()
	if err := storage.SaveProjectLocked(project); err != nil {
		t.Fatalf("SaveProjectLocked: %v", err)
	}

	task := project.Tasks[0]
	app.dailyPlan = []FocusTask{{Task: task, Project: project}}
	app.dailyPlanIDs = map[string]bool{task.ID: true}
	app.lastDailyPlanToggle = &dailyPlanToggleAction{Task: task, Project: project, WasAdd: true}
	app.lastDailyPlanToggleAt = time.Now().Add(-time.Hour)
	app.lastToggleUndoKind = toggleUndoDailyPlan

	if err := storage.RecordUndoSnapshot(project, "Persistent Edit", task.Title, 10, []string{task.ID}); err != nil {
		t.Fatalf("RecordUndoSnapshot: %v", err)
	}

	view := &undoRedoRecorderTaskRefView{task: task, project: project}
	app.views.Register(ViewFocusView, view)
	app.viewStack = []ViewType{ViewDashboard, ViewFocusView}

	handled, cmd := app.handleGlobalKeyPress(syntheticKey('u'))
	if !handled {
		t.Fatal("expected older local toggle to yield to persistent undo")
	}
	if cmd == nil {
		t.Fatal("expected persistent undo command")
	}
	if app.lastDailyPlanToggle == nil {
		t.Fatal("older local daily-plan toggle should not be consumed")
	}
	if got := cmd(); got == nil {
		t.Fatal("persistent undo command returned nil")
	} else if _, ok := got.(UndoPerformedMsg); !ok {
		t.Fatalf("persistent undo command returned %T, want UndoPerformedMsg", got)
	}
}
