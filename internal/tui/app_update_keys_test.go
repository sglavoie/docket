package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/external"
	"github.com/sglavoie/tt/internal/models"
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

func TestHandleGlobalKeyPress_SpaceFallsThroughWithoutCursorTask(t *testing.T) {
	app := newTestAppModel()

	handled, _ := app.handleGlobalKeyPress(syntheticSpecialKey(tea.KeySpace))
	if handled {
		t.Fatal("expected space to fall through when there is no cursor task")
	}
}

func TestCustomContextEditorSpaceTogglesProjectThroughApp(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.config.SetCustomContext("Work", []string{project.ID})
	editor := NewCustomContextEditorModel(app.config, "Work")
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
	app.config.Editor = "vim"
	app.config.ExternalEditor = "code"
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
				tree := NewProjectTreeModel(project, app.config, app.configManager)
				app.projectTree = tree
				return tree
			},
		},
		{
			name:     "outliner",
			viewType: ViewOutliner,
			setupView: func(app *AppModel, project *models.Project) View {
				outliner := NewOutlinerModel(project, app.config, app.configManager)
				outliner.rebuildRows()
				for i, row := range outliner.rows {
					if !row.IsSectionHeader && !row.IsSeparator && row.task != nil {
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
			app.config.Editor = "vim"
			app.config.ExternalEditor = "code"

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
